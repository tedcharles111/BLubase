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

	// Ensure the platform_users table exists
	_, _ = dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS platform_users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		email TEXT UNIQUE,
		password_hash TEXT,
		phone TEXT,
		username TEXT,
		display_name TEXT,
		photoURL TEXT,
		avatar_url TEXT,
		status TEXT DEFAULT 'active',
		tier TEXT DEFAULT 'free',
		prompts_used_today INT DEFAULT 0,
		last_seen TIMESTAMPTZ DEFAULT now(),
		is_admin BOOLEAN DEFAULT false,
		created_at TIMESTAMPTZ DEFAULT now()
	)`)

	r := chi.NewRouter()
	r.Post("/signup", signupHandler)
	r.Post("/login", loginHandler)
	r.Post("/forgot-password", forgotPasswordHandler)
	r.Post("/reset-password", resetPasswordHandler)
	r.Get("/auth/google/login", googleLoginHandler)
	r.Get("/auth/github/login", githubLoginHandler)

	log.Println("Auth server on :3001")
	log.Fatal(http.ListenAndServe(":3001", r))
}

// ---------- Signup ----------
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

// ---------- Login ----------
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

	// --- Hardcoded dev account (guaranteed to work) ---
	if req.Email == "dev@blubase.dev" && req.Password == "DevPassword123" {
		claims := jwt.MapClaims{
			"sub":   "5af97d13-e81a-40f1-bc8f-75655a065543",
			"email": req.Email,
			"iat":   time.Now().Unix(),
			"exp":   time.Now().Add(24 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, _ := token.SignedString(jwtSecret)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"token":  signed,
			"userId": "5af97d13-e81a-40f1-bc8f-75655a065543",
		})
		return
	}

	// --- Normal users ---
	var userID, hashed string
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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":  signed,
		"userId": userID,
	})
}

// ---------- Forgot Password (returns OTP) ----------
func forgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email string `json:"email"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" {
		http.Error(w, `{"error":"email required"}`, 400)
		return
	}
	otp := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	// In production, send via email. For now, return it in the response.
	log.Printf("OTP for %s: %s", req.Email, otp)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "If that email exists, a reset code has been sent",
		"otp":     otp,
	})
}

// ---------- Reset Password ----------
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
	// In production, verify OTP from Redis/cache. For now, just update the password.
	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	_, err := dbPool.Exec(context.Background(),
		`UPDATE platform_users SET password_hash=$1 WHERE email=$2`,
		string(hashed), req.Email)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}
	w.Write([]byte(`{"message":"password updated"}`))
}

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
