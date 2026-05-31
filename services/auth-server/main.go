package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

var (
	dbPool      *pgxpool.Pool
	redisClient *redis.Client
	jwtSecret   = []byte(os.Getenv("JWT_SECRET"))
	oauthConfigs = map[string]*oauth2.Config{}
)

func main() {
	ctx := context.Background()
	var err error
	dbPool, err = pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil { log.Fatal(err) }
	redisClient = redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_URL")})

	// Ensure extra tables exist
	_, _ = dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS email_templates (
		name TEXT PRIMARY KEY,
		subject TEXT NOT NULL,
		body TEXT NOT NULL
	)`)
	_, _ = dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS smtp_config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`)
	_, _ = dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS oauth_providers (
		provider TEXT PRIMARY KEY,
		client_id TEXT,
		client_secret TEXT,
		enabled BOOLEAN DEFAULT false
	)`)

	// Load OAuth providers from DB into configs
	loadOAuthConfigs()

	r := chi.NewRouter()
	// No CORS middleware – nginx handles it

	// Auth endpoints
	r.Post("/signup", signupHandler)
	r.Post("/login", loginHandler)

	// Email templates admin
	r.Get("/admin/templates", listTemplatesHandler)
	r.Post("/admin/templates", createTemplateHandler)
	r.Delete("/admin/templates/{name}", deleteTemplateHandler)

	// SMTP config admin
	r.Get("/admin/smtp", getSMTPHandler)
	r.Put("/admin/smtp", updateSMTPHandler)

	// OAuth providers admin
	r.Get("/admin/oauth-providers", listOAuthProvidersHandler)
	r.Post("/admin/oauth-providers", createOAuthProviderHandler)
	r.Put("/admin/oauth-providers/{provider}", updateOAuthProviderHandler)

	// OAuth login / callback
	r.Get("/auth/{provider}/login", oauthLoginHandler)
	r.Get("/auth/{provider}/callback", oauthCallbackHandler)

	log.Println("Auth server on :3001")
	log.Fatal(http.ListenAndServe(":3001", r))
}

// ----------------------------------------------------------------------
//  Auth Handlers
// ----------------------------------------------------------------------
func signupHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"email and password required"}`, 400)
		return
	}
	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	_, err := dbPool.Exec(context.Background(),
		`INSERT INTO users (email, password_hash) VALUES ($1,$2) ON CONFLICT (email) DO NOTHING`,
		req.Email, string(hashed))
	if err != nil {
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}
	w.Write([]byte(`{"message":"signup successful"}`))
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"email and password required"}`, 400)
		return
	}
	var userID, hashed string
	err := dbPool.QueryRow(context.Background(),
		`SELECT id, password_hash FROM users WHERE email=$1`, req.Email).Scan(&userID, &hashed)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hashed), []byte(req.Password)) != nil {
		http.Error(w, `{"error":"invalid credentials"}`, 401)
		return
	}
	claims := jwt.MapClaims{
		"sub": userID, "email": req.Email,
		"iat": time.Now().Unix(), "exp": time.Now().Add(24*time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString(jwtSecret)
	json.NewEncoder(w).Encode(map[string]interface{}{"token": signed, "userId": userID})
}

// ----------------------------------------------------------------------
//  Email Templates
// ----------------------------------------------------------------------
func listTemplatesHandler(w http.ResponseWriter, r *http.Request) {
	rows, _ := dbPool.Query(context.Background(), `SELECT name, subject, body FROM email_templates`)
	defer rows.Close()
	templates := map[string]interface{}{}
	for rows.Next() {
		var name, subject, body string
		rows.Scan(&name, &subject, &body)
		templates[name] = map[string]string{"subject": subject, "body": body}
	}
	json.NewEncoder(w).Encode(templates)
}

func createTemplateHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name, Subject, Body string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" || req.Subject == "" || req.Body == "" {
		http.Error(w, `{"error":"name, subject, body required"}`, 400)
		return
	}
	_, err := dbPool.Exec(context.Background(),
		`INSERT INTO email_templates (name, subject, body) VALUES ($1,$2,$3) ON CONFLICT (name) DO UPDATE SET subject=$2, body=$3`,
		req.Name, req.Subject, req.Body)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}
	w.Write([]byte(`{"status":"created"}`))
}

func deleteTemplateHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	_, _ = dbPool.Exec(context.Background(), `DELETE FROM email_templates WHERE name=$1`, name)
	w.Write([]byte(`{"status":"deleted"}`))
}

// ----------------------------------------------------------------------
//  SMTP Config
// ----------------------------------------------------------------------
func getSMTPHandler(w http.ResponseWriter, r *http.Request) {
	rows, _ := dbPool.Query(context.Background(), `SELECT key, value FROM smtp_config`)
	defer rows.Close()
	cfg := map[string]string{}
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		cfg[k] = v
	}
	json.NewEncoder(w).Encode(cfg)
}

func updateSMTPHandler(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	json.NewDecoder(r.Body).Decode(&req)
	for k, v := range req {
		dbPool.Exec(context.Background(),
			`INSERT INTO smtp_config (key, value) VALUES ($1,$2) ON CONFLICT (key) DO UPDATE SET value=$2`, k, v)
	}
	w.Write([]byte(`{"status":"updated"}`))
}

// ----------------------------------------------------------------------
//  OAuth Providers (admin)
// ----------------------------------------------------------------------
func listOAuthProvidersHandler(w http.ResponseWriter, r *http.Request) {
	rows, _ := dbPool.Query(context.Background(), `SELECT provider, client_id, client_secret, enabled FROM oauth_providers`)
	defer rows.Close()
	providers := []map[string]interface{}{}
	for rows.Next() {
		var p, cid, csecret string
		var en bool
		rows.Scan(&p, &cid, &csecret, &en)
		providers = append(providers, map[string]interface{}{
			"provider": p, "client_id": cid, "client_secret": "***", "enabled": en,
		})
	}
	json.NewEncoder(w).Encode(providers)
}

func createOAuthProviderHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Provider, ClientID, ClientSecret string; Enabled bool }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Provider == "" || req.ClientID == "" || req.ClientSecret == "" {
		http.Error(w, `{"error":"provider, client_id, client_secret required"}`, 400)
		return
	}
	_, err := dbPool.Exec(context.Background(),
		`INSERT INTO oauth_providers (provider, client_id, client_secret, enabled) VALUES ($1,$2,$3,$4) ON CONFLICT (provider) DO UPDATE SET client_id=$2, client_secret=$3, enabled=$4`,
		req.Provider, req.ClientID, req.ClientSecret, req.Enabled)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}
	loadOAuthConfigs() // reload
	w.Write([]byte(`{"status":"created"}`))
}

func updateOAuthProviderHandler(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	var req struct{ ClientID, ClientSecret string; Enabled bool }
	json.NewDecoder(r.Body).Decode(&req)
	if req.ClientID == "" || req.ClientSecret == "" {
		http.Error(w, `{"error":"client_id, client_secret required"}`, 400)
		return
	}
	_, err := dbPool.Exec(context.Background(),
		`UPDATE oauth_providers SET client_id=$1, client_secret=$2, enabled=$3 WHERE provider=$4`,
		req.ClientID, req.ClientSecret, req.Enabled, provider)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}
	loadOAuthConfigs()
	w.Write([]byte(`{"status":"updated"}`))
}

// ----------------------------------------------------------------------
//  OAuth Login Flow
// ----------------------------------------------------------------------
func oauthLoginHandler(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	config, ok := oauthConfigs[provider]
	if !ok {
		http.Error(w, "provider not configured", 404)
		return
	}
	state := make([]byte, 16)
	rand.Read(state)
	stateStr := base64.URLEncoding.EncodeToString(state)
	// store state in Redis (short-lived)
	redisClient.Set(context.Background(), "oauth:"+stateStr, provider, 5*time.Minute)
	url := config.AuthCodeURL(stateStr)
	http.Redirect(w, r, url, http.StatusFound)
}

func oauthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	config, ok := oauthConfigs[provider]
	if !ok {
		http.Error(w, "provider not configured", 404)
		return
	}
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	// validate state
	val, _ := redisClient.Get(context.Background(), "oauth:"+state).Result()
	if val != provider {
		http.Error(w, "invalid state", 400)
		return
	}
	redisClient.Del(context.Background(), "oauth:"+state)

	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "token exchange failed: "+err.Error(), 500)
		return
	}
	// fetch user info (simplified)
	var email string
	switch provider {
	case "github":
		req, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		resp, _ := http.DefaultClient.Do(req)
		defer resp.Body.Close()
		var emails []struct{ Email string; Primary bool }
		json.NewDecoder(resp.Body).Decode(&emails)
		for _, e := range emails {
			if e.Primary { email = e.Email; break }
		}
		if email == "" && len(emails) > 0 { email = emails[0].Email }
	case "google":
		resp, _ := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
		defer resp.Body.Close()
		var guser struct{ Email string }
		json.NewDecoder(resp.Body).Decode(&guser)
		email = guser.Email
	default:
		http.Error(w, "unsupported provider for userinfo", 500)
		return
	}
	if email == "" {
		http.Error(w, "could not fetch email", 500)
		return
	}

	// Upsert user
	var userID string
	err = dbPool.QueryRow(context.Background(),
		`INSERT INTO users (email) VALUES ($1) ON CONFLICT (email) DO UPDATE SET email=$1 RETURNING id`, email).Scan(&userID)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}

	// Generate JWT
	claims := jwt.MapClaims{
		"sub": userID, "email": email,
		"iat": time.Now().Unix(), "exp": time.Now().Add(24*time.Hour).Unix(),
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := jwtToken.SignedString(jwtSecret)
	json.NewEncoder(w).Encode(map[string]interface{}{"token": signed, "userId": userID})
}

// ----------------------------------------------------------------------
//  Load OAuth configs from DB
// ----------------------------------------------------------------------
func loadOAuthConfigs() {
	rows, _ := dbPool.Query(context.Background(),
		`SELECT provider, client_id, client_secret, enabled FROM oauth_providers WHERE enabled=true`)
	defer rows.Close()
	for rows.Next() {
		var p, cid, csecret string
		var en bool
		rows.Scan(&p, &cid, &csecret, &en)
		redirectURL := fmt.Sprintf("%s/auth/%s/callback", os.Getenv("RENDER_EXTERNAL_URL"), p)
		switch p {
		case "github":
			oauthConfigs[p] = &oauth2.Config{
				ClientID: cid, ClientSecret: csecret,
				RedirectURL: redirectURL,
				Scopes: []string{"user:email"},
				Endpoint: github.Endpoint,
			}
		case "google":
			oauthConfigs[p] = &oauth2.Config{
				ClientID: cid, ClientSecret: csecret,
				RedirectURL: redirectURL,
				Scopes: []string{"https://www.googleapis.com/auth/userinfo.email"},
				Endpoint: google.Endpoint,
			}
		// add more providers (facebook, figma, etc.) as needed
		}
	}
}
