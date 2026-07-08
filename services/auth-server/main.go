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
	"golang.org/x/crypto/bcrypt"
)

var (
	dbPool    *pgxpool.Pool
	jwtSecret = []byte(os.Getenv("JWT_SECRET"))
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

	// OAuth callbacks (pure redirects, no database)
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

// ---------- OAuth Handlers (pure redirects, no database) ----------
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

func githubCallbackHandler(w http.ResponseWriter, r *http.Request) {
	redirectURL := "https://themultiverse.build/dashboard?token=github_oauth_demo"
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
