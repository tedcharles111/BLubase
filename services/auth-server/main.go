package main

import (
	"context"
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
	if err != nil { log.Fatal(err) }
	redisClient = redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_URL")})

	loadOAuthConfigs()

	r := chi.NewRouter()
	r.Post("/signup", signupHandler)
	r.Post("/login", loginHandler)
	r.Post("/forgot-password", forgotPasswordHandler)
	r.Post("/reset-password", resetPasswordHandler)
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

	// OAuth routes
	r.Get("/auth/google/login", googleLoginHandler)
	r.Get("/auth/google/callback", googleCallbackHandler)
	r.Get("/auth/github/login", githubLoginHandler)
	r.Get("/auth/github/callback", githubCallbackHandler)

	log.Println("Auth server on :3001")
	log.Fatal(http.ListenAndServe(":3001", r))
}

// ---------- Auth Handlers ----------
func signupHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password, Phone string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"email and password required"}`, 400)
		return
	}
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
	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"email and password required"}`, 400)
		return
	}
	var userID string
	var hashed string
	err := dbPool.QueryRow(context.Background(),
		`SELECT id::text, password_hash FROM platform_users WHERE email=$1`, req.Email).Scan(&userID, &hashed)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hashed), []byte(req.Password)) != nil {
		http.Error(w, `{"error":"invalid credentials"}`, 401)
		return
	}
	claims := jwt.MapClaims{"sub": userID, "email": req.Email, "iat": time.Now().Unix(), "exp": time.Now().Add(24*time.Hour).Unix()}
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
	redisClient.Set(context.Background(), "reset:"+req.Email, otp, 15*time.Minute)
	logActivity(r, "forgot_password", req.Email)
	json.NewEncoder(w).Encode(map[string]string{"message": "If that email exists, a reset code has been sent", "otp": otp})
}

func resetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, OTP, NewPassword string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" || req.OTP == "" || req.NewPassword == "" {
		http.Error(w, `{"error":"email, otp, new_password required"}`, 400); return
	}
	stored, _ := redisClient.Get(context.Background(), "reset:"+req.Email).Result()
	if stored != req.OTP { http.Error(w, `{"error":"invalid otp"}`, 401); return }
	redisClient.Del(context.Background(), "reset:"+req.Email)
	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	dbPool.Exec(context.Background(), `UPDATE platform_users SET password_hash=$1 WHERE email=$2`, string(hashed), req.Email)
	logActivity(r, "reset_password", req.Email)
	w.Write([]byte(`{"message":"password updated"}`))
}

func extractUserID(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if len(auth) < 8 || auth[:7] != "Bearer " { return "anonymous" }
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(auth[7:], claims, func(t *jwt.Token) (interface{}, error) { return jwtSecret, nil })
	if err != nil { return "anonymous" }
	sub, _ := claims["sub"].(string)
	return sub
}

func logActivity(r *http.Request, action, details string) {
	userID := extractUserID(r)
	dbPool.Exec(context.Background(), `INSERT INTO activity_log (user_id, action, details) VALUES ($1,$2,$3)`, userID, action, details)
}

func logActivityHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Action, Details string }
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

// ---------- Admin Handlers ----------
func listPlatformUsersHandler(w http.ResponseWriter, r *http.Request) {
	rows, _ := dbPool.Query(context.Background(), `SELECT id::text, email, phone, suspended, created_at FROM platform_users ORDER BY created_at DESC`)
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
	var req struct{ Suspended bool }
	json.NewDecoder(r.Body).Decode(&req)
	dbPool.Exec(context.Background(), `UPDATE platform_users SET suspended=$1 WHERE id=$2`, req.Suspended, userID)
	w.Write([]byte(`{"status":"updated"}`))
}

func sendAdminMessageHandler(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	var req struct{ Content string }
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
	rows, _ := dbPool.Query(context.Background(), `SELECT id::text, name, ref, owner_id::text, anon_key, created_at FROM projects ORDER BY created_at DESC`)
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
	var req struct{ Status string }
	json.NewDecoder(r.Body).Decode(&req)
	dbPool.Exec(context.Background(), `UPDATE projects SET status=$1 WHERE ref=$2`, req.Status, ref)
	w.Write([]byte(`{"status":"updated"}`))
}

// ---------- Templates, SMTP, OAuth, URL Config (stubs) ----------
func listTemplatesHandler(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(map[string]interface{}{}) }
func createTemplateHandler(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"status":"created"}`)) }
func deleteTemplateHandler(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"status":"deleted"}`)) }
func getSMTPHandler(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(map[string]string{}) }
func updateSMTPHandler(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"status":"updated"}`)) }
func listOAuthProvidersHandler(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode([]map[string]interface{}{}) }
func createOAuthProviderHandler(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"status":"created"}`)) }
func updateOAuthProviderHandler(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"status":"updated"}`)) }
func getURLConfigHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{"site_url": os.Getenv("RENDER_EXTERNAL_URL"), "jwt_expiry_hours": 24, "redirect_urls": []string{}})
}
func updateURLConfigHandler(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"status":"updated"}`)) }

// ---------- OAuth Handlers ----------
func googleLoginHandler(w http.ResponseWriter, r *http.Request) {
	redirectURL := fmt.Sprintf("https://accounts.google.com/o/oauth2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=email",
		"998494570170-7mcv4n1ifb0l2g4t9sgh0idn1s4edn1c.apps.googleusercontent.com",
		"https://blubase.onrender.com/auth/google/callback")
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func githubLoginHandler(w http.ResponseWriter, r *http.Request) {
	redirectURL := fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=user:email",
		"Iv23liS3vHgocHDsSR2i",
		"https://blubase.onrender.com/auth/github/callback")
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func googleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	redirectURL := "https://themultiverse.build/dashboard?token=google_oauth_demo"
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
	code := r.URL.Query().Get("code")
	if code == "" { http.Error(w, `{"error":"missing code"}`, 400); return }
	email := "google_user@example.com" // Replace with real user info fetch
	var userID string
	err := dbPool.QueryRow(context.Background(),
		`INSERT INTO platform_users (email) VALUES ($1) ON CONFLICT (email) DO UPDATE SET email=$1 RETURNING id::text`, email).Scan(&userID)
	if err != nil { http.Error(w, `{"error":"database error"}`, 500); return }
	claims := jwt.MapClaims{"sub": userID, "email": email, "iat": time.Now().Unix(), "exp": time.Now().Add(24*time.Hour).Unix()}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString(jwtSecret)
	http.Redirect(w, r, "https://themultiverse.build/dashboard?token="+signed, http.StatusFound)
}

func githubCallbackHandler(w http.ResponseWriter, r *http.Request) {
	redirectURL := "https://themultiverse.build/dashboard?token=github_oauth_demo"
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
	code := r.URL.Query().Get("code")
	if code == "" { http.Error(w, `{"error":"missing code"}`, 400); return }
	email := "github_user@example.com" // Replace with real user info fetch
	var userID string
	err := dbPool.QueryRow(context.Background(),
		`INSERT INTO platform_users (email) VALUES ($1) ON CONFLICT (email) DO UPDATE SET email=$1 RETURNING id::text`, email).Scan(&userID)
	if err != nil { http.Error(w, `{"error":"database error"}`, 500); return }
	claims := jwt.MapClaims{"sub": userID, "email": email, "iat": time.Now().Unix(), "exp": time.Now().Add(24*time.Hour).Unix()}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString(jwtSecret)
	http.Redirect(w, r, "https://themultiverse.build/dashboard?token="+signed, http.StatusFound)
}

func loadOAuthConfigs() {
	oauthConfigs["google"] = &oauth2.Config{
		ClientID:     "998494570170-7mcv4n1ifb0l2g4t9sgh0idn1s4edn1c.apps.googleusercontent.com",
		ClientSecret: "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",
		RedirectURL:  "https://blubase.onrender.com/auth/google/callback",
		Scopes:       []string{"email"},
		Endpoint:     google.Endpoint,
	}
	oauthConfigs["github"] = &oauth2.Config{
		ClientID:     "Iv23liS3vHgocHDsSR2i",
		ClientSecret: "2d53775a53d755e073bf9deae5798478a57cd11a",
		RedirectURL:  "https://blubase.onrender.com/auth/github/callback",
		Scopes:       []string{"user:email"},
		Endpoint:     github.Endpoint,
	}
}
