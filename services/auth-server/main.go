package main

import (
	"context"
	"encoding/json"
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
	oauthConfigs = map[string]*oauth2.Config{
		"google": {
			ClientID:     "998494570170-7mcv4n1ifb0l2g4t9sgh0idn1s4edn1c.apps.googleusercontent.com",
			ClientSecret: "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",
			RedirectURL:  "https://blubase.onrender.com/auth/google/callback",
			Scopes:       []string{"email"},
			Endpoint:     google.Endpoint,
		},
		"github": {
			ClientID:     "Iv23liS3vHgocHDsSR2i",
			ClientSecret: "2d53775a53d755e073bf9deae5798478a57cd11a",
			RedirectURL:  "https://blubase.onrender.com/auth/github/callback",
			Scopes:       []string{"user:email"},
			Endpoint:     github.Endpoint,
		},
	}
)

func main() {
	ctx := context.Background()
	var err error
	dbPool, err = pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}

	r := chi.NewRouter()
	r.Post("/signup", signupHandler)
	r.Post("/login", loginHandler)

	// OAuth login redirects
	r.Get("/auth/google/login", googleLoginHandler)
	r.Get("/auth/github/login", githubLoginHandler)

	// OAuth callbacks (real)
	r.Get("/auth/google/callback", googleCallbackHandler)
	r.Get("/auth/github/callback", githubCallbackHandler)

	log.Println("Auth server on :3001")
	log.Fatal(http.ListenAndServe(":3001", r))
}

// ---------- Signup / Login (unchanged) ----------
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
	err := dbPool.QueryRow(context.Background(),
		`SELECT id::text, password_hash FROM platform_users WHERE email=$1`, req.Email).Scan(&userID, &hashed)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hashed), []byte(req.Password)) != nil {
		http.Error(w, `{"error":"invalid credentials"}`, 401)
		return
	}
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": req.Email,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString(jwtSecret)
	json.NewEncoder(w).Encode(map[string]interface{}{"token": signed, "userId": userID})
}

// ---------- OAuth Handlers ----------
func googleLoginHandler(w http.ResponseWriter, r *http.Request) {
	url := oauthConfigs["google"].AuthCodeURL("state")
	http.Redirect(w, r, url, http.StatusFound)
}

func githubLoginHandler(w http.ResponseWriter, r *http.Request) {
	url := oauthConfigs["github"].AuthCodeURL("state")
	http.Redirect(w, r, url, http.StatusFound)
}

func googleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, `{"error":"missing code"}`, 400)
		return
	}

	token, err := oauthConfigs["google"].Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, `{"error":"token exchange failed"}`, 500)
		return
	}

	// Fetch user email
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch user info"}`, 500)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var gUser struct{ Email string }
	json.Unmarshal(body, &gUser)
	if gUser.Email == "" {
		http.Error(w, `{"error":"could not fetch email"}`, 500)
		return
	}

	userID := upsertUser(gUser.Email)
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": gUser.Email,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := jwtToken.SignedString(jwtSecret)
	http.Redirect(w, r, "https://themultiverse.build/dashboard?token="+signed, http.StatusFound)
}

func githubCallbackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, `{"error":"missing code"}`, 400)
		return
	}

	token, err := oauthConfigs["github"].Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, `{"error":"token exchange failed"}`, 500)
		return
	}

	// Fetch user emails
	req, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch github emails"}`, 500)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var emails []struct{ Email string; Primary bool }
	json.Unmarshal(body, &emails)
	var primaryEmail string
	for _, e := range emails {
		if e.Primary {
			primaryEmail = e.Email
			break
		}
	}
	if primaryEmail == "" && len(emails) > 0 {
		primaryEmail = emails[0].Email
	}
	if primaryEmail == "" {
		http.Error(w, `{"error":"could not fetch email"}`, 500)
		return
	}

	userID := upsertUser(primaryEmail)
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": primaryEmail,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := jwtToken.SignedString(jwtSecret)
	http.Redirect(w, r, "https://themultiverse.build/dashboard?token="+signed, http.StatusFound)
}

func upsertUser(email string) string {
	var userID string
	err := dbPool.QueryRow(context.Background(),
		`INSERT INTO platform_users (email) VALUES ($1) ON CONFLICT (email) DO UPDATE SET email=$1 RETURNING id::text`, email).Scan(&userID)
	if err != nil {
		log.Println("upsertUser error:", err)
		return ""
	}
	return userID
}
