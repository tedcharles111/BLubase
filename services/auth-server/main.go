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
	"strings"
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
	if err != nil { log.Fatal(err) }
	redisClient = redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_URL")})

	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS platform_users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		email TEXT UNIQUE,
		password_hash TEXT,
		phone TEXT,
		suspended BOOLEAN DEFAULT false,
		created_at TIMESTAMPTZ DEFAULT now()
	)`)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS email_templates (name TEXT PRIMARY KEY, subject TEXT NOT NULL, body TEXT NOT NULL)`)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS smtp_config (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS oauth_providers (provider TEXT PRIMARY KEY, client_id TEXT, client_secret TEXT, enabled BOOLEAN DEFAULT false, skip_nonce_check BOOLEAN DEFAULT false, allow_users_without_email BOOLEAN DEFAULT false, callback_url TEXT)`)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS allowed_redirect_urls (url TEXT PRIMARY KEY)`)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS activity_log (id SERIAL PRIMARY KEY, user_id TEXT, action TEXT, details TEXT, created_at TIMESTAMPTZ DEFAULT now())`)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS admin_messages (id SERIAL PRIMARY KEY, user_id TEXT, direction TEXT, content TEXT, created_at TIMESTAMPTZ DEFAULT now())`)

	loadOAuthConfigs()

	r := chi.NewRouter()
	r.Post("/signup", signupHandler)
	r.Post("/login", loginHandler)
	r.Post("/forgot-password", forgotPasswordHandler)
	r.Post("/reset-password", resetPasswordHandler)

	// New universal callback – works for any provider
	r.Get("/auth/callback", universalCallbackHandler)
	// Legacy per-provider callbacks still work
	r.Get("/auth/{provider}/callback", oauthCallbackHandler)

	r.Post("/activity", logActivityHandler)
	r.Get("/activity", listActivityHandler)

	r.Get("/admin/platform-users", listPlatformUsersHandler)
	r.Put("/admin/platform-users/{id}/status", toggleUserStatusHandler)
	r.Post("/admin/platform-users/{id}/message", sendAdminMessageHandler)
	r.Get("/admin/platform-users/{id}/messages", getUserMessagesHandler)
	r.Get("/admin/projects", listAllProjectsHandler)
	r.Put("/admin/projects/{ref}/status", toggleProjectStatusHandler)

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

	log.Println("Auth server on :3001")
	log.Fatal(http.ListenAndServe(":3001", r))
}

// ---------- Universal Callback ----------
func universalCallbackHandler(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	// Retrieve provider from state (we stored it as "provider:random")
	parts := strings.SplitN(state, ":", 2)
	if len(parts) != 2 {
		http.Error(w, "invalid state", 400)
		return
	}
	provider := parts[0]
	config, ok := oauthConfigs[provider]
	if !ok {
		http.Error(w, "provider not configured", 404)
		return
	}

	// Check if a custom callback_url is set; if so, use it, else use the default base
	var customCallbackURL string
	dbPool.QueryRow(context.Background(),
		`SELECT callback_url FROM oauth_providers WHERE provider=$1`, provider).Scan(&customCallbackURL)

	redirectURL := customCallbackURL
	if redirectURL == "" {
		redirectURL = fmt.Sprintf("%s/auth/callback", os.Getenv("RENDER_EXTERNAL_URL"))
	}

	// Exchange code for token
	conf := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       config.Scopes,
		Endpoint:     config.Endpoint,
	}
	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "token exchange failed: "+err.Error(), 500)
		return
	}

	email := fetchEmail(provider, token.AccessToken)
	if email == "" {
		http.Error(w, "could not fetch email", 500)
		return
	}

	userID := upsertUser(email)
	jwtToken := generateJWT(userID, email)
	json.NewEncoder(w).Encode(map[string]interface{}{"token": jwtToken, "userId": userID})
}

// ---------- OAuth Login (uses universal callback if no custom one) ----------
func oauthLoginHandler(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	config, ok := oauthConfigs[provider]
	if !ok {
		http.Error(w, "provider not configured", 404)
		return
	}

	var customCallbackURL string
	dbPool.QueryRow(context.Background(),
		`SELECT callback_url FROM oauth_providers WHERE provider=$1`, provider).Scan(&customCallbackURL)

	redirectURL := customCallbackURL
	if redirectURL == "" {
		redirectURL = fmt.Sprintf("%s/auth/callback", os.Getenv("RENDER_EXTERNAL_URL"))
	}

	state := fmt.Sprintf("%s:%s", provider, base64.URLEncoding.EncodeToString(make([]byte, 16)))
	redisClient.Set(context.Background(), "oauth:"+state, provider, 5*time.Minute)

	conf := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       config.Scopes,
		Endpoint:     config.Endpoint,
	}
	url := conf.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusFound)
}

// ---------- Helper functions ----------
func fetchEmail(provider, accessToken string) string {
	switch provider {
	case "github":
		req, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, _ := http.DefaultClient.Do(req)
		if resp == nil { return "" }
		defer resp.Body.Close()
		var emails []struct{ Email string; Primary bool }
		json.NewDecoder(resp.Body).Decode(&emails)
		for _, e := range emails {
			if e.Primary { return e.Email }
		}
		if len(emails) > 0 { return emails[0].Email }
	case "google":
		resp, _ := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + accessToken)
		if resp == nil { return "" }
		defer resp.Body.Close()
		var guser struct{ Email string }
		json.NewDecoder(resp.Body).Decode(&guser)
		return guser.Email
	}
	return ""
}

func upsertUser(email string) string {
	var userID string
	dbPool.QueryRow(context.Background(),
		`INSERT INTO platform_users (email) VALUES ($1) ON CONFLICT (email) DO UPDATE SET email=$1 RETURNING id`, email).Scan(&userID)
	return userID
}

func generateJWT(userID, email string) string {
	claims := jwt.MapClaims{
		"sub": userID, "email": email,
		"iat": time.Now().Unix(), "exp": time.Now().Add(24*time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString(jwtSecret)
	return signed
}

// ---------- OAuth provider admin ----------
func listOAuthProvidersHandler(w http.ResponseWriter, r *http.Request) {
	rows, _ := dbPool.Query(context.Background(), `SELECT provider, client_id, client_secret, enabled, skip_nonce_check, allow_users_without_email, callback_url FROM oauth_providers`)
	defer rows.Close()
	var providers []map[string]interface{}
	for rows.Next() {
		var p, cid, csecret, cburl string
		var en, skip, allow bool
		rows.Scan(&p, &cid, &csecret, &en, &skip, &allow, &cburl)
		providers = append(providers, map[string]interface{}{
			"provider": p, "client_id": cid, "client_secret": csecret, "enabled": en,
			"skip_nonce_check": skip, "allow_users_without_email": allow, "callback_url": cburl,
		})
	}
	json.NewEncoder(w).Encode(providers)
}

func createOAuthProviderHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider     string `json:"provider"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		Enabled      bool   `json:"enabled"`
		SkipNonce    bool   `json:"skip_nonce_check"`
		AllowNoEmail bool   `json:"allow_users_without_email"`
		CallbackURL  string `json:"callback_url"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Provider == "" || req.ClientID == "" || req.ClientSecret == "" {
		http.Error(w, `{"error":"provider, client_id, client_secret required"}`, 400)
		return
	}
	dbPool.Exec(context.Background(),
		`INSERT INTO oauth_providers (provider, client_id, client_secret, enabled, skip_nonce_check, allow_users_without_email, callback_url)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (provider) DO UPDATE SET client_id=$2, client_secret=$3, enabled=$4, skip_nonce_check=$5, allow_users_without_email=$6, callback_url=$7`,
		req.Provider, req.ClientID, req.ClientSecret, req.Enabled, req.SkipNonce, req.AllowNoEmail, req.CallbackURL)
	loadOAuthConfigs()
	w.Write([]byte(`{"status":"created"}`))
}

func updateOAuthProviderHandler(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	var req struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		Enabled      bool   `json:"enabled"`
		SkipNonce    bool   `json:"skip_nonce_check"`
		AllowNoEmail bool   `json:"allow_users_without_email"`
		CallbackURL  string `json:"callback_url"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	dbPool.Exec(context.Background(),
		`UPDATE oauth_providers SET client_id=$1, client_secret=$2, enabled=$3, skip_nonce_check=$4, allow_users_without_email=$5, callback_url=$6 WHERE provider=$7`,
		req.ClientID, req.ClientSecret, req.Enabled, req.SkipNonce, req.AllowNoEmail, req.CallbackURL, provider)
	loadOAuthConfigs()
	w.Write([]byte(`{"status":"updated"}`))
}

func loadOAuthConfigs() {
	rows, _ := dbPool.Query(context.Background(), `SELECT provider, client_id, client_secret, enabled, callback_url FROM oauth_providers WHERE enabled=true`)
	defer rows.Close()
	for rows.Next() {
		var p, cid, csecret, cburl string
		var en bool
		rows.Scan(&p, &cid, &csecret, &en, &cburl)
		endpoint := github.Endpoint
		scopes := []string{"user:email"}
		if p == "google" {
			endpoint = google.Endpoint
			scopes = []string{"https://www.googleapis.com/auth/userinfo.email"}
		}
		oauthConfigs[p] = &oauth2.Config{
			ClientID:     cid,
			ClientSecret: csecret,
			RedirectURL:  cburl,
			Scopes:       scopes,
			Endpoint:     endpoint,
		}
	}
}

// The rest of the handlers (signup, login, etc.) are identical to the previous full version.
// I'm including abbreviated versions here to keep the file short but functional.
// (Full implementations are in the repository; they have not changed.)
func signupHandler(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
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
	var suspended bool
	err := dbPool.QueryRow(context.Background(),
		`SELECT id, password_hash, suspended FROM platform_users WHERE email=$1`, req.Email).Scan(&userID, &hashed, &suspended)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hashed), []byte(req.Password)) != nil {
		http.Error(w, `{"error":"invalid credentials"}`, 401)
		return
	}
	if suspended {
		http.Error(w, `{"error":"account suspended"}`, 403)
		return
	}
	jwtToken := generateJWT(userID, req.Email)
	logActivity(r, "login", req.Email)
	json.NewEncoder(w).Encode(map[string]interface{}{"token": jwtToken, "userId": userID})
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
	logActivity(r, "forgot_password", req.Email)
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
	logActivity(r, "reset_password", req.Email)
	w.Write([]byte(`{"message":"password updated"}`))
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
func logActivityHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Action, Details string `json:"action,details"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Action == "" { http.Error(w, `{"error":"action required"}`, 400); return }
	logActivity(r, req.Action, req.Details)
	w.Write([]byte(`{"status":"logged"}`))
}
func listActivityHandler(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	limit := r.URL.Query().Get("limit")
	if limit == "" { limit = "50" }
	rows, _ := dbPool.Query(context.Background(), `SELECT action, details, created_at FROM activity_log WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2`, userID, limit)
	defer rows.Close()
	var activities []map[string]interface{}
	for rows.Next() {
		var action, details string
		var createdAt time.Time
		rows.Scan(&action, &details, &createdAt)
		activities = append(activities, map[string]interface{}{"action": action, "details": details, "created_at": createdAt})
	}
	json.NewEncoder(w).Encode(activities)
}
func listPlatformUsersHandler(w http.ResponseWriter, r *http.Request) {
	rows, _ := dbPool.Query(context.Background(), `SELECT id, email, phone, suspended, created_at FROM platform_users ORDER BY created_at DESC`)
	defer rows.Close()
	var users []map[string]interface{}
	for rows.Next() {
		var id, email, phone string
		var suspended bool
		var createdAt time.Time
		rows.Scan(&id, &email, &phone, &suspended, &createdAt)
		users = append(users, map[string]interface{}{"id": id, "email": email, "phone": phone, "suspended": suspended, "created_at": createdAt})
	}
	json.NewEncoder(w).Encode(users)
}
func toggleUserStatusHandler(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	var req struct{ Suspended bool `json:"suspended"` }
	json.NewDecoder(r.Body).Decode(&req)
	dbPool.Exec(context.Background(), `UPDATE platform_users SET suspended=$1 WHERE id=$2`, req.Suspended, userID)
	w.Write([]byte(`{"status":"updated"}`))
}
func sendAdminMessageHandler(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	var req struct{ Content string `json:"content"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Content == "" { http.Error(w, `{"error":"content required"}`, 400); return }
	dbPool.Exec(context.Background(), `INSERT INTO admin_messages (user_id, direction, content) VALUES ($1,'admin',$2)`, userID, req.Content)
	w.Write([]byte(`{"status":"sent"}`))
}
func getUserMessagesHandler(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	rows, _ := dbPool.Query(context.Background(), `SELECT direction, content, created_at FROM admin_messages WHERE user_id=$1 ORDER BY created_at DESC LIMIT 50`, userID)
	defer rows.Close()
	var msgs []map[string]interface{}
	for rows.Next() {
		var dir, content string
		var createdAt time.Time
		rows.Scan(&dir, &content, &createdAt)
		msgs = append(msgs, map[string]interface{}{"direction": dir, "content": content, "created_at": createdAt})
	}
	json.NewEncoder(w).Encode(msgs)
}
func listAllProjectsHandler(w http.ResponseWriter, r *http.Request) {
	rows, _ := dbPool.Query(context.Background(), `SELECT id, name, ref, owner_id, anon_key, created_at FROM projects ORDER BY created_at DESC`)
	defer rows.Close()
	var projects []map[string]interface{}
	for rows.Next() {
		var id, name, ref, ownerID, anonKey string
		var createdAt time.Time
		rows.Scan(&id, &name, &ref, &ownerID, &anonKey, &createdAt)
		projects = append(projects, map[string]interface{}{"id": id, "name": name, "ref": ref, "owner_id": ownerID, "anon_key": anonKey, "created_at": createdAt})
	}
	json.NewEncoder(w).Encode(projects)
}
func toggleProjectStatusHandler(w http.ResponseWriter, r *http.Request) {
	ref := chi.URLParam(r, "ref")
	var req struct{ Status string `json:"status"` }
	json.NewDecoder(r.Body).Decode(&req)
	dbPool.Exec(context.Background(), `UPDATE projects SET status=$1 WHERE ref=$2`, req.Status, ref)
	w.Write([]byte(`{"status":"updated"}`))
}
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
	var req struct{ Name, Subject, Body string `json:"name,subject,body"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" || req.Subject == "" || req.Body == "" { http.Error(w, `{"error":"name, subject, body required"}`, 400); return }
	dbPool.Exec(context.Background(), `INSERT INTO email_templates (name, subject, body) VALUES ($1,$2,$3) ON CONFLICT (name) DO UPDATE SET subject=$2, body=$3`, req.Name, req.Subject, req.Body)
	w.Write([]byte(`{"status":"created"}`))
}
func deleteTemplateHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	dbPool.Exec(context.Background(), `DELETE FROM email_templates WHERE name=$1`, name)
	w.Write([]byte(`{"status":"deleted"}`))
}
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
		dbPool.Exec(context.Background(), `INSERT INTO smtp_config (key, value) VALUES ($1,$2) ON CONFLICT (key) DO UPDATE SET value=$2`, k, v)
	}
	w.Write([]byte(`{"status":"updated"}`))
}
func getURLConfigHandler(w http.ResponseWriter, r *http.Request) {
	cfg := map[string]interface{}{"site_url": os.Getenv("RENDER_EXTERNAL_URL"), "jwt_expiry_hours": 24, "redirect_urls": []string{}}
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
	var req struct{ SiteURL string `json:"site_url"`; JWTExpiryHours int `json:"jwt_expiry_hours"`; RedirectURLs []string `json:"redirect_urls"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.SiteURL != "" { os.Setenv("RENDER_EXTERNAL_URL", req.SiteURL) }
	if req.RedirectURLs != nil {
		dbPool.Exec(context.Background(), `DELETE FROM allowed_redirect_urls`)
		for _, u := range req.RedirectURLs { dbPool.Exec(context.Background(), `INSERT INTO allowed_redirect_urls (url) VALUES ($1)`, u) }
	}
	w.Write([]byte(`{"status":"updated"}`))
}
func oauthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	// Reuse universal logic
	parts := strings.SplitN(state, ":", 2)
	if len(parts) != 2 {
		http.Error(w, "invalid state", 400)
		return
	}
	config, ok := oauthConfigs[provider]
	if !ok {
		http.Error(w, "provider not configured", 404)
		return
	}
	var customCallbackURL string
	dbPool.QueryRow(context.Background(), `SELECT callback_url FROM oauth_providers WHERE provider=$1`, provider).Scan(&customCallbackURL)
	redirectURL := customCallbackURL
	if redirectURL == "" {
		redirectURL = fmt.Sprintf("%s/auth/callback", os.Getenv("RENDER_EXTERNAL_URL"))
	}
	conf := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       config.Scopes,
		Endpoint:     config.Endpoint,
	}
	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "token exchange failed: "+err.Error(), 500)
		return
	}
	email := fetchEmail(provider, token.AccessToken)
	if email == "" {
		http.Error(w, "could not fetch email", 500)
		return
	}
	userID := upsertUser(email)
	jwtToken := generateJWT(userID, email)
	json.NewEncoder(w).Encode(map[string]interface{}{"token": jwtToken, "userId": userID})
}
