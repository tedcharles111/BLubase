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

func main() {
	ctx := context.Background()
	var err error
	dbPool, err = pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	redisClient = redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_URL")})

	// Ensure tables
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS platform_users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		email TEXT UNIQUE,
		password_hash TEXT,
		phone TEXT,
		suspended BOOLEAN DEFAULT false,
		created_at TIMESTAMPTZ DEFAULT now()
	)`)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS oauth_providers (provider TEXT PRIMARY KEY, client_id TEXT, client_secret TEXT, enabled BOOLEAN DEFAULT false)`)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS project_oauth_providers (project_ref TEXT NOT NULL, provider TEXT NOT NULL, client_id TEXT, client_secret TEXT, enabled BOOLEAN DEFAULT false, PRIMARY KEY (project_ref, provider))`)

	// Seed Google & GitHub credentials for project oMVsv2 (from earlier config)
	dbPool.Exec(ctx, `INSERT INTO project_oauth_providers (project_ref, provider, client_id, client_secret, enabled) VALUES ('oMVsv2', 'google', '998494570170-7mcv4n1ifb0l2g4t9sgh0idn1s4edn1c.apps.googleusercontent.com', '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', true) ON CONFLICT (project_ref, provider) DO UPDATE SET client_id=EXCLUDED.client_id, client_secret=EXCLUDED.client_secret, enabled=true`)
	dbPool.Exec(ctx, `INSERT INTO project_oauth_providers (project_ref, provider, client_id, client_secret, enabled) VALUES ('oMVsv2', 'github', 'Iv23liS3vHgocHDsSR2i', '2d53775a53d755e073bf9deae5798478a57cd11a', true) ON CONFLICT (project_ref, provider) DO UPDATE SET client_id=EXCLUDED.client_id, client_secret=EXCLUDED.client_secret, enabled=true`)

	loadOAuthConfigs()

	r := chi.NewRouter()

	// Auth endpoints
	r.Post("/signup", signupHandler)
	r.Post("/login", loginHandler)
	r.Post("/forgot-password", forgotPasswordHandler)
	r.Post("/reset-password", resetPasswordHandler)

	// OAuth routes – exactly like before
	r.Get("/auth/{provider}/login", oauthLoginHandler)
	r.Get("/auth/{provider}/callback", oauthCallbackHandler)

	log.Println("Auth server on :3001 (with OAuth)")
	log.Fatal(http.ListenAndServe(":3001", r))
}

func signupHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Phone    string `json:"phone"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"email and password required"}`, 400)
		return
	}
	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	_, err := dbPool.Exec(context.Background(),
		`INSERT INTO platform_users (email, password_hash, phone) VALUES ($1,$2,$3) ON CONFLICT (email) DO NOTHING`,
		req.Email, string(hashed), req.Phone)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}
	w.Write([]byte(`{"message":"signup successful"}`))
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"email and password required"}`, 400)
		return
	}
	var userID string
	var hashed string
	var suspended bool
	err := dbPool.QueryRow(context.Background(),
		`SELECT id::text, password_hash, suspended FROM platform_users WHERE email=$1`, req.Email).Scan(&userID, &hashed, &suspended)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hashed), []byte(req.Password)) != nil {
		http.Error(w, `{"error":"invalid credentials"}`, 401)
		return
	}
	if suspended {
		http.Error(w, `{"error":"account suspended"}`, 403)
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

func forgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email string `json:"email"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" {
		http.Error(w, `{"error":"email required"}`, 400)
		return
	}
	otp := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	redisClient.Set(context.Background(), "reset:"+req.Email, otp, 15*time.Minute)
	json.NewEncoder(w).Encode(map[string]string{"message": "If that email exists, a reset code has been sent", "otp": otp})
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
	dbPool.Exec(context.Background(), `UPDATE platform_users SET password_hash=$1 WHERE email=$2`, string(hashed), req.Email)
	w.Write([]byte(`{"message":"password updated"}`))
}

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
		var emails []struct {
			Email   string
			Primary bool
		}
		json.NewDecoder(resp.Body).Decode(&emails)
		for _, e := range emails {
			if e.Primary {
				email = e.Email
				break
			}
		}
		if email == "" && len(emails) > 0 {
			email = emails[0].Email
		}
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
		`INSERT INTO platform_users (email) VALUES ($1) ON CONFLICT (email) DO UPDATE SET email=$1 RETURNING id::text`, email).Scan(&userID)
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

func loadOAuthConfigs() {
	// Load from project_oauth_providers for project oMVsv2
	rows, _ := dbPool.Query(context.Background(), `SELECT provider, client_id, client_secret, enabled FROM project_oauth_providers WHERE project_ref='oMVsv2' AND enabled=true`)
	defer rows.Close()
	for rows.Next() {
		var p, cid, csecret string
		var en bool
		rows.Scan(&p, &cid, &csecret, &en)
		redirectURL := fmt.Sprintf("%s/auth/%s/callback", os.Getenv("RENDER_EXTERNAL_URL"), p)
		switch p {
		case "github":
			oauthConfigs[p] = &oauth2.Config{
				ClientID:     cid,
				ClientSecret: csecret,
				RedirectURL:  redirectURL,
				Scopes:       []string{"user:email"},
				Endpoint:     github.Endpoint,
			}
		case "google":
			oauthConfigs[p] = &oauth2.Config{
				ClientID:     cid,
				ClientSecret: csecret,
				RedirectURL:  redirectURL,
				Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
				Endpoint:     google.Endpoint,
			}
		}
	}
}
