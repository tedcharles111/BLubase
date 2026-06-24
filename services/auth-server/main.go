package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

var (
	dbPool    *pgxpool.Pool
	jwtSecret = []byte(os.Getenv("JWT_SECRET"))
)

func main() {
	ctx := context.Background()
	var err error
	dbPool, err = pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil { log.Fatal(err) }

	// Ensure tables
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS platform_users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		email TEXT UNIQUE, password_hash TEXT, phone TEXT,
		otp_code TEXT, otp_expiry TIMESTAMPTZ,
		suspended BOOLEAN DEFAULT false,
		created_at TIMESTAMPTZ DEFAULT now()
	)`)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS project_oauth_providers (
		project_ref TEXT NOT NULL, provider TEXT NOT NULL,
		client_id TEXT, client_secret TEXT, enabled BOOLEAN DEFAULT false,
		PRIMARY KEY (project_ref, provider)
	)`)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS oauth_states (
		state TEXT PRIMARY KEY, provider TEXT, project_ref TEXT,
		created_at TIMESTAMPTZ DEFAULT now()
	)`)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS email_templates (name TEXT PRIMARY KEY, subject TEXT NOT NULL, body TEXT NOT NULL)`)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS smtp_config (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS activity_log (id SERIAL PRIMARY KEY, user_id TEXT, action TEXT, details TEXT, created_at TIMESTAMPTZ DEFAULT now())`)

	r := chi.NewRouter()

	// Platform auth
	r.Post("/signup", signupHandler)
	r.Post("/login", loginHandler)
	r.Post("/forgot-password", forgotPasswordHandler)
	r.Post("/reset-password", resetPasswordHandler)

	// Project‑scoped OAuth login (use query param ?project=)
	r.Get("/google/login", googleLoginHandler)
	r.Get("/google/callback", googleCallbackHandler)
	r.Get("/github/login", githubLoginHandler)
	r.Get("/github/callback", githubCallbackHandler)

	// Project‑scoped OAuth admin CRUD (dashboard calls these)
	r.Get("/admin/oauth-providers", listProjectOAuthProvidersHandler)
	r.Post("/admin/oauth-providers", createProjectOAuthProviderHandler)
	r.Put("/admin/oauth-providers/{provider}", updateProjectOAuthProviderHandler)
	r.Delete("/admin/oauth-providers/{provider}", deleteProjectOAuthProviderHandler)

	log.Println("Auth server on :3001")
	log.Fatal(http.ListenAndServe(":3001", r))
}

// ---------- Helpers ----------
func getProjectRef(r *http.Request) string {
	ref := r.Header.Get("x-project-ref")
	if ref == "" { ref = r.URL.Query().Get("project") }
	return ref
}

func extractUserID(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if len(auth) < 8 || auth[:7] != "Bearer " { return "anonymous" }
	tokenStr := auth[7:]
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) { return jwtSecret, nil })
	if err != nil { return "anonymous" }
	sub, _ := claims["sub"].(string)
	return sub
}

func logActivity(r *http.Request, action, details string) {
	userID := extractUserID(r)
	dbPool.Exec(context.Background(), `INSERT INTO activity_log (user_id, action, details) VALUES ($1,$2,$3)`, userID, action, details)
}

// ---------- Auth Handlers ----------
func signupHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password, Phone string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" || req.Password == "" { http.Error(w, `{"error":"email and password required"}`, 400); return }
	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	_, err := dbPool.Exec(context.Background(),
		`INSERT INTO platform_users (email, password_hash, phone) VALUES ($1,$2,$3) ON CONFLICT (email) DO NOTHING`,
		req.Email, string(hashed), req.Phone)
	if err != nil { http.Error(w, `{"error":"database error"}`, 500); return }
	logActivity(r, "signup", req.Email)
	w.Write([]byte(`{"message":"signup successful"}`))
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" || req.Password == "" { http.Error(w, `{"error":"email and password required"}`, 400); return }
	var userID, hashed string
	var suspended bool
	err := dbPool.QueryRow(context.Background(),
		`SELECT id, password_hash, suspended FROM platform_users WHERE email=$1`, req.Email).Scan(&userID, &hashed, &suspended)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hashed), []byte(req.Password)) != nil {
		http.Error(w, `{"error":"invalid credentials"}`, 401); return
	}
	if suspended { http.Error(w, `{"error":"account suspended"}`, 403); return }
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
	var req struct{ Email string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" { http.Error(w, `{"error":"email required"}`, 400); return }
	otp := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	expiry := time.Now().Add(15 * time.Minute)
	_, err := dbPool.Exec(context.Background(),
		`UPDATE platform_users SET otp_code=$1, otp_expiry=$2 WHERE email=$3`, otp, expiry, req.Email)
	if err != nil { log.Printf("ERROR storing OTP: %v", err); http.Error(w, `{"error":"database error"}`, 500); return }
	go sendResendOTP(req.Email, otp)
	logActivity(r, "forgot_password", req.Email)
	json.NewEncoder(w).Encode(map[string]string{"message": "If that email exists, a reset code has been sent"})
}

func resetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, OTP, NewPassword string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" || req.OTP == "" || req.NewPassword == "" { http.Error(w, `{"error":"email, otp, new_password required"}`, 400); return }
	var storedOTP string
	var expiry time.Time
	err := dbPool.QueryRow(context.Background(),
		`SELECT otp_code, otp_expiry FROM platform_users WHERE email=$1`, req.Email).Scan(&storedOTP, &expiry)
	if err != nil || storedOTP != req.OTP || time.Now().After(expiry) { http.Error(w, `{"error":"invalid otp"}`, 401); return }
	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	dbPool.Exec(context.Background(),
		`UPDATE platform_users SET password_hash=$1, otp_code=NULL, otp_expiry=NULL WHERE email=$2`,
		string(hashed), req.Email)
	logActivity(r, "reset_password", req.Email)
	w.Write([]byte(`{"message":"password updated"}`))
}

// ---------- Resend Email Helper ----------
func sendResendOTP(email, otp string) {
	var apiKey string
	_ = dbPool.QueryRow(context.Background(), `SELECT value FROM smtp_config WHERE key='resend_api_key'`).Scan(&apiKey)
	if apiKey == "" { return }
	var fromName, fromEmail string
	_ = dbPool.QueryRow(context.Background(), `SELECT value FROM smtp_config WHERE key='from_name'`).Scan(&fromName)
	_ = dbPool.QueryRow(context.Background(), `SELECT value FROM smtp_config WHERE key='from_email'`).Scan(&fromEmail)
	if fromName == "" { fromName = "Blubase" }
	if fromEmail == "" { fromEmail = "noreply@blubase.dev" }
	payload := fmt.Sprintf(`{"from": "%s <%s>", "to": ["%s"], "subject": "Your password reset code", "html": "<p>Your reset code is: <strong>%s</strong></p>"}`, fromName, fromEmail, email, otp)
	req, _ := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewBuffer([]byte(payload)))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil { log.Printf("Resend error: %v", err); return }
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Resend error %d: %s", resp.StatusCode, string(body))
	}
}

// ---------- OAuth Login Handlers (project‑scoped) ----------
func googleLoginHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	if ref == "" { http.Error(w, "missing project ref", 400); return }
	var cid, csecret string
	err := dbPool.QueryRow(context.Background(),
		`SELECT client_id, client_secret FROM project_oauth_providers WHERE project_ref=$1 AND provider='google' AND enabled=true`, ref).Scan(&cid, &csecret)
	if err != nil { http.Error(w, "provider not configured", 404); return }
	cfg := &oauth2.Config{
		ClientID: cid, ClientSecret: csecret,
		RedirectURL: fmt.Sprintf("%s/auth/google/callback", os.Getenv("RENDER_EXTERNAL_URL")),
		Scopes: []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint: google.Endpoint,
	}
	rawState := fmt.Sprintf("%s:%s", ref, base64.URLEncoding.EncodeToString(make([]byte, 16)))
	stateEnc := base64.URLEncoding.EncodeToString([]byte(rawState))
	dbPool.Exec(context.Background(), `INSERT INTO oauth_states (state, provider, project_ref) VALUES ($1,'google',$2)`, stateEnc, ref)
	http.Redirect(w, r, cfg.AuthCodeURL(stateEnc), http.StatusFound)
}

func googleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	var prov, pref string
	_ = dbPool.QueryRow(context.Background(), `SELECT provider, project_ref FROM oauth_states WHERE state=$1`, state).Scan(&prov, &pref)
	if prov != "google" { http.Error(w, "invalid state", 400); return }
	dbPool.Exec(context.Background(), `DELETE FROM oauth_states WHERE state=$1`, state)
	ref := pref
	var cid, csecret string
	_ = dbPool.QueryRow(context.Background(),
		`SELECT client_id, client_secret FROM project_oauth_providers WHERE project_ref=$1 AND provider='google'`, ref).Scan(&cid, &csecret)
	cfg := &oauth2.Config{ClientID: cid, ClientSecret: csecret, Endpoint: google.Endpoint}
	token, err := cfg.Exchange(context.Background(), code)
	if err != nil { http.Error(w, "token exchange failed", 500); return }
	resp, _ := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	defer resp.Body.Close()
	var guser struct{ Email string }
	json.NewDecoder(resp.Body).Decode(&guser)
	if guser.Email == "" { http.Error(w, "could not fetch email", 500); return }
	var userID string
	_ = dbPool.QueryRow(context.Background(),
		`INSERT INTO platform_users (email) VALUES ($1) ON CONFLICT (email) DO UPDATE SET email=$1 RETURNING id`, guser.Email).Scan(&userID)
	claims := jwt.MapClaims{"sub": userID, "email": guser.Email, "iat": time.Now().Unix(), "exp": time.Now().Add(24*time.Hour).Unix()}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := jwtToken.SignedString(jwtSecret)
	json.NewEncoder(w).Encode(map[string]interface{}{"token": signed, "userId": userID})
}

func githubLoginHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	if ref == "" { http.Error(w, "missing project ref", 400); return }
	var cid, csecret string
	err := dbPool.QueryRow(context.Background(),
		`SELECT client_id, client_secret FROM project_oauth_providers WHERE project_ref=$1 AND provider='github' AND enabled=true`, ref).Scan(&cid, &csecret)
	if err != nil { http.Error(w, "provider not configured", 404); return }
	cfg := &oauth2.Config{
		ClientID: cid, ClientSecret: csecret,
		RedirectURL: fmt.Sprintf("%s/auth/github/callback", os.Getenv("RENDER_EXTERNAL_URL")),
		Scopes: []string{"user:email"},
		Endpoint: github.Endpoint,
	}
	rawState := fmt.Sprintf("%s:%s", ref, base64.URLEncoding.EncodeToString(make([]byte, 16)))
	stateEnc := base64.URLEncoding.EncodeToString([]byte(rawState))
	dbPool.Exec(context.Background(), `INSERT INTO oauth_states (state, provider, project_ref) VALUES ($1,'github',$2)`, stateEnc, ref)
	http.Redirect(w, r, cfg.AuthCodeURL(stateEnc), http.StatusFound)
}

func githubCallbackHandler(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	var prov, pref string
	_ = dbPool.QueryRow(context.Background(), `SELECT provider, project_ref FROM oauth_states WHERE state=$1`, state).Scan(&prov, &pref)
	if prov != "github" { http.Error(w, "invalid state", 400); return }
	dbPool.Exec(context.Background(), `DELETE FROM oauth_states WHERE state=$1`, state)
	ref := pref
	var cid, csecret string
	_ = dbPool.QueryRow(context.Background(),
		`SELECT client_id, client_secret FROM project_oauth_providers WHERE project_ref=$1 AND provider='github'`, ref).Scan(&cid, &csecret)
	cfg := &oauth2.Config{ClientID: cid, ClientSecret: csecret, Endpoint: github.Endpoint}
	token, err := cfg.Exchange(context.Background(), code)
	if err != nil { http.Error(w, "token exchange failed", 500); return }
	req, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	var emails []struct{ Email string; Primary bool }
	json.NewDecoder(resp.Body).Decode(&emails)
	var email string
	for _, e := range emails { if e.Primary { email = e.Email; break } }
	if email == "" && len(emails) > 0 { email = emails[0].Email }
	if email == "" { http.Error(w, "could not fetch email", 500); return }
	var userID string
	_ = dbPool.QueryRow(context.Background(),
		`INSERT INTO platform_users (email) VALUES ($1) ON CONFLICT (email) DO UPDATE SET email=$1 RETURNING id`, email).Scan(&userID)
	claims := jwt.MapClaims{"sub": userID, "email": email, "iat": time.Now().Unix(), "exp": time.Now().Add(24*time.Hour).Unix()}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := jwtToken.SignedString(jwtSecret)
	json.NewEncoder(w).Encode(map[string]interface{}{"token": signed, "userId": userID})
}

// ---------- OAuth Admin CRUD (project‑scoped) ----------
func listProjectOAuthProvidersHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	if ref == "" { http.Error(w, "missing project ref", 400); return }
	rows, _ := dbPool.Query(context.Background(),
		`SELECT provider, client_id, client_secret, enabled FROM project_oauth_providers WHERE project_ref=$1`, ref)
	defer rows.Close()
	var providers []map[string]interface{}
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

func createProjectOAuthProviderHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	if ref == "" { http.Error(w, "missing project ref", 400); return }
	var req struct{ Provider, ClientID, ClientSecret string; Enabled bool }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Provider == "" || req.ClientID == "" || req.ClientSecret == "" {
		http.Error(w, `{"error":"provider, clientId, clientSecret required"}`, 400); return
	}
	dbPool.Exec(context.Background(),
		`INSERT INTO project_oauth_providers (project_ref, provider, client_id, client_secret, enabled)
		 VALUES ($1,$2,$3,$4,$5) ON CONFLICT (project_ref, provider) DO UPDATE SET client_id=$3, client_secret=$4, enabled=$5`,
		ref, req.Provider, req.ClientID, req.ClientSecret, req.Enabled)
	w.Write([]byte(`{"status":"created"}`))
}

func updateProjectOAuthProviderHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	provider := chi.URLParam(r, "provider")
	var req struct{ ClientID, ClientSecret string; Enabled *bool }
	json.NewDecoder(r.Body).Decode(&req)
	if req.ClientID == "" || req.ClientSecret == "" {
		http.Error(w, `{"error":"clientId and clientSecret required"}`, 400); return
	}
	enabled := true
	if req.Enabled != nil { enabled = *req.Enabled }
	dbPool.Exec(context.Background(),
		`UPDATE project_oauth_providers SET client_id=$1, client_secret=$2, enabled=$3 WHERE project_ref=$4 AND provider=$5`,
		req.ClientID, req.ClientSecret, enabled, ref, provider)
	w.Write([]byte(`{"status":"updated"}`))
}

func deleteProjectOAuthProviderHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	provider := chi.URLParam(r, "provider")
	dbPool.Exec(context.Background(),
		`DELETE FROM project_oauth_providers WHERE project_ref=$1 AND provider=$2`, ref, provider)
	w.Write([]byte(`{"status":"deleted"}`))
}
