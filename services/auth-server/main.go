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
	dbPool       *pgxpool.Pool
	redisClient  *redis.Client
	jwtSecret    = []byte(os.Getenv("JWT_SECRET"))
	oauthConfigs = map[string]*oauth2.Config{}
)

type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Phone    string `json:"phone"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func main() {
	ctx := context.Background()
	var err error
	dbPool, err = pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	redisClient = redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_URL")})

	// Ensure tables (including activity_log)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS email_templates (name TEXT PRIMARY KEY, subject TEXT NOT NULL, body TEXT NOT NULL)`)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS smtp_config (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS oauth_providers (provider TEXT PRIMARY KEY, client_id TEXT, client_secret TEXT, enabled BOOLEAN DEFAULT false)`)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS allowed_redirect_urls (url TEXT PRIMARY KEY)`)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS activity_log (
		id SERIAL PRIMARY KEY,
		user_id TEXT,
		action TEXT,
		details TEXT,
		created_at TIMESTAMPTZ DEFAULT now()
	)`)

	loadOAuthConfigs()

	r := chi.NewRouter()

	r.Post("/signup", signupHandler)
	r.Post("/login", loginHandler)
	r.Post("/forgot-password", forgotPasswordHandler)
	r.Post("/reset-password", resetPasswordHandler)

	// Activity log endpoints
	r.Post("/activity", logActivityHandler)
	r.Get("/activity", listActivityHandler)

	r.Get("/admin/templates", listTemplatesHandler)
	r.Post("/admin/templates", createTemplateHandler)
	r.Delete("/admin/templates/{name}", deleteTemplateHandler)

	r.Get("/admin/smtp", getSMTPHandler)
	r.Put("/admin/smtp", updateSMTPHandler)

	r.Get("/admin/oauth-providers", listOAuthProvidersHandler)
	r.Post("/admin/oauth-providers", createOAuthProviderHandler)
	r.Put("/admin/oauth-providers/{provider}", updateOAuthProviderHandler)

	r.Get("/admin/url-config", getURLConfigHandler)
	r.Put("/admin/url-config", updateURLConfigHandler)

	r.Get("/auth/{provider}/login", oauthLoginHandler)
	r.Get("/auth/{provider}/callback", oauthCallbackHandler)

	log.Println("Auth server on :3001")
	log.Fatal(http.ListenAndServe(":3001", r))
}

// ---------- Auth Handlers ----------
func signupHandler(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"email and password required"}`, 400)
		return
	}
	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	_, err := dbPool.Exec(context.Background(),
		`INSERT INTO users (email, password_hash, phone) VALUES ($1,$2,$3) ON CONFLICT (email) DO NOTHING`,
		req.Email, string(hashed), req.Phone)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}
	// Log activity
	logActivity(r, "signup", req.Email)
	w.Write([]byte(`{"message":"signup successful"}`))
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
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
	logActivity(r, "login", req.Email)
	json.NewEncoder(w).Encode(map[string]interface{}{"token": signed, "userId": userID})
}

func forgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email string `json:"email"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" {
		http.Error(w, `{"error":"email required"}`, 400)
		return
	}
	otp := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	err := redisClient.Set(context.Background(), "reset:"+req.Email, otp, 15*time.Minute).Err()
	if err != nil {
		log.Printf("ERROR storing OTP in Redis: %v", err)
	}
	logActivity(r, "forgot_password", req.Email)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "If that email exists, a reset code has been sent",
		"otp":     otp,
	})
}

func resetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		OTP         string `json:"otp"`
		NewPassword string `json:"new_password"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" || req.OTP == "" || req.NewPassword == "" {
		http.Error(w, `{"error":"email, otp, new_password required"}`, 400)
		return
	}
	stored, _ := redisClient.Get(context.Background(), "reset:"+req.Email).Result()
	if stored != req.OTP {
		http.Error(w, `{"error":"invalid otp"}`, 401)
		return
	}
	redisClient.Del(context.Background(), "reset:"+req.Email)
	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	_, err := dbPool.Exec(context.Background(),
		`UPDATE users SET password_hash=$1 WHERE email=$2`, string(hashed), req.Email)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}
	logActivity(r, "reset_password", req.Email)
	w.Write([]byte(`{"message":"password updated"}`))
}

// ---------- Activity Log Helpers ----------
func extractUserID(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if len(auth) < 8 || auth[:7] != "Bearer " {
		return "anonymous"
	}
	tokenStr := auth[7:]
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		return "anonymous"
	}
	sub, _ := claims["sub"].(string)
	return sub
}

func logActivity(r *http.Request, action, details string) {
	userID := extractUserID(r)
	dbPool.Exec(context.Background(),
		`INSERT INTO activity_log (user_id, action, details) VALUES ($1,$2,$3)`,
		userID, action, details)
}

func logActivityHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action  string `json:"action"`
		Details string `json:"details"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Action == "" {
		http.Error(w, `{"error":"action required"}`, 400)
		return
	}
	logActivity(r, req.Action, req.Details)
	w.Write([]byte(`{"status":"logged"}`))
}

func listActivityHandler(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "50"
	}
	rows, _ := dbPool.Query(context.Background(),
		`SELECT action, details, created_at FROM activity_log WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2`,
		userID, limit)
	defer rows.Close()
	var activities []map[string]interface{}
	for rows.Next() {
		var action, details string
		var createdAt time.Time
		rows.Scan(&action, &details, &createdAt)
		activities = append(activities, map[string]interface{}{
			"action": action, "details": details, "created_at": createdAt,
		})
	}
	json.NewEncoder(w).Encode(activities)
}

// ---------- Email Templates ----------
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
	dbPool.Exec(context.Background(),
		`INSERT INTO email_templates (name, subject, body) VALUES ($1,$2,$3) ON CONFLICT (name) DO UPDATE SET subject=$2, body=$3`,
		req.Name, req.Subject, req.Body)
	w.Write([]byte(`{"status":"created"}`))
}

func deleteTemplateHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	dbPool.Exec(context.Background(), `DELETE FROM email_templates WHERE name=$1`, name)
	w.Write([]byte(`{"status":"deleted"}`))
}

// ---------- SMTP Config ----------
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

// ---------- OAuth Providers ----------
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
	dbPool.Exec(context.Background(),
		`INSERT INTO oauth_providers (provider, client_id, client_secret, enabled) VALUES ($1,$2,$3,$4) ON CONFLICT (provider) DO UPDATE SET client_id=$2, client_secret=$3, enabled=$4`,
		req.Provider, req.ClientID, req.ClientSecret, req.Enabled)
	loadOAuthConfigs()
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
	dbPool.Exec(context.Background(),
		`UPDATE oauth_providers SET client_id=$1, client_secret=$2, enabled=$3 WHERE provider=$4`,
		req.ClientID, req.ClientSecret, req.Enabled, provider)
	loadOAuthConfigs()
	w.Write([]byte(`{"status":"updated"}`))
}

// ---------- OAuth Login/Callback ----------
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
	}
	if email == "" {
		http.Error(w, "could not fetch email", 500)
		return
	}

	var userID string
	err = dbPool.QueryRow(context.Background(),
		`INSERT INTO users (email) VALUES ($1) ON CONFLICT (email) DO UPDATE SET email=$1 RETURNING id`, email).Scan(&userID)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}

	claims := jwt.MapClaims{
		"sub": userID, "email": email,
		"iat": time.Now().Unix(), "exp": time.Now().Add(24*time.Hour).Unix(),
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := jwtToken.SignedString(jwtSecret)
	json.NewEncoder(w).Encode(map[string]interface{}{"token": signed, "userId": userID})
}

// ---------- URL Configuration ----------
func getURLConfigHandler(w http.ResponseWriter, r *http.Request) {
	cfg := map[string]interface{}{
		"site_url":         os.Getenv("RENDER_EXTERNAL_URL"),
		"jwt_expiry_hours": 24,
		"redirect_urls":    []string{},
	}
	rows, _ := dbPool.Query(context.Background(), `SELECT url FROM allowed_redirect_urls`)
	defer rows.Close()
	var urls []string
	for rows.Next() {
		var u string
		rows.Scan(&u)
		urls = append(urls, u)
	}
	if urls != nil { cfg["redirect_urls"] = urls }
	json.NewEncoder(w).Encode(cfg)
}

func updateURLConfigHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteURL        string   `json:"site_url"`
		JWTExpiryHours int      `json:"jwt_expiry_hours"`
		RedirectURLs   []string `json:"redirect_urls"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.SiteURL != "" {
		os.Setenv("RENDER_EXTERNAL_URL", req.SiteURL)
	}
	if req.RedirectURLs != nil {
		dbPool.Exec(context.Background(), `DELETE FROM allowed_redirect_urls`)
		for _, u := range req.RedirectURLs {
			dbPool.Exec(context.Background(), `INSERT INTO allowed_redirect_urls (url) VALUES ($1)`, u)
		}
	}
	w.Write([]byte(`{"status":"updated"}`))
}

// ---------- OAuth Config Loader ----------
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
		}
	}
}
