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
	"golang.org/x/crypto/bcrypt"
)

var (
	controlDB  *pgxpool.Pool
	jwtSignKey = []byte(os.Getenv("JWT_SECRET"))
)

func main() {
	var err error
	controlDB, err = pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}

	r := chi.NewRouter()

	// Project management (authenticated)
	r.Post("/projects", createProjectHandler)
	r.Get("/projects", listProjectsHandler)

	// Project-scoped user management (for the developer)
	r.Get("/projects/{ref}/users", listProjectUsersHandler)
	r.Post("/projects/{ref}/users", addProjectUserHandler)
	r.Delete("/projects/{ref}/users/{id}", deleteProjectUserHandler)

	// Project-scoped AUTH (for end‑users of a project)
	r.Post("/projects/{ref}/auth/signup", projectSignupHandler)
	r.Post("/projects/{ref}/auth/login", projectLoginHandler)

	log.Println("Project manager on :3002")
	log.Fatal(http.ListenAndServe(":3002", r))
}

func extractUserID(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	tokenStr := strings.TrimPrefix(auth, "Bearer ")
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return jwtSignKey, nil
	})
	if err != nil {
		return ""
	}
	sub, _ := claims["sub"].(string)
	return sub
}

func getProjectRef(r *http.Request) string {
	ref := chi.URLParam(r, "ref")
	if ref == "" {
		ref = r.Header.Get("x-project-ref")
	}
	return ref
}

// ------------------ Developer project management ------------------

func createProjectHandler(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		http.Error(w, `{"error":"unauthorized"}`, 401)
		return
	}
	var req struct{ Name string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" {
		http.Error(w, `{"error":"name required"}`, 400)
		return
	}
	ref := make([]byte, 6)
	rand.Read(ref)
	refStr := base64.URLEncoding.EncodeToString(ref)[:6]

	anonToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"ref": refStr, "role": "anon", "iat": time.Now().Unix(),
	})
	anonKey, _ := anonToken.SignedString(jwtSignKey)

	// Create the project's own users table
	tableName := fmt.Sprintf("project_%s_users", refStr)
	_, _ = controlDB.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS `+tableName+` (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		email TEXT UNIQUE,
		password_hash TEXT,
		phone TEXT,
		created_at TIMESTAMPTZ DEFAULT now()
	)`)

	var projectID string
	err := controlDB.QueryRow(context.Background(),
		`INSERT INTO projects (name, ref, owner_id, anon_key) VALUES ($1,$2,$3,$4) RETURNING id`,
		req.Name, refStr, userID, anonKey).Scan(&projectID)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id": projectID, "name": req.Name, "ref": refStr,
		"anon_key": anonKey, "status": "active", "region": "us-east-1",
	})
}

func listProjectsHandler(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	rows, err := controlDB.Query(context.Background(),
		`SELECT id, name, ref, anon_key, created_at FROM projects WHERE owner_id=$1`, userID)
	if err != nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	defer rows.Close()
	var projects []map[string]interface{}
	for rows.Next() {
		var id, name, ref, anonKey string
		var createdAt time.Time
		rows.Scan(&id, &name, &ref, &anonKey, &createdAt)
		projects = append(projects, map[string]interface{}{
			"id": id, "name": name, "ref": ref,
			"anon_key": anonKey, "status": "active", "region": "us-east-1",
			"created_at": createdAt.Format(time.RFC3339),
		})
	}
	json.NewEncoder(w).Encode(projects)
}

// ------------------ Developer user management for a project ------------------

func listProjectUsersHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	tableName := fmt.Sprintf("project_%s_users", ref)
	rows, err := controlDB.Query(context.Background(),
		`SELECT id, email, phone, created_at FROM `+tableName+` ORDER BY created_at DESC`)
	if err != nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	defer rows.Close()
	var users []map[string]interface{}
	for rows.Next() {
		var id, email, phone string
		var createdAt time.Time
		rows.Scan(&id, &email, &phone, &createdAt)
		users = append(users, map[string]interface{}{"id": id, "email": email, "phone": phone, "created_at": createdAt})
	}
	if users == nil {
		users = []map[string]interface{}{}
	}
	json.NewEncoder(w).Encode(users)
}

func addProjectUserHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	tableName := fmt.Sprintf("project_%s_users", ref)
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
	_, err := controlDB.Exec(context.Background(),
		`INSERT INTO `+tableName+` (email, password_hash, phone) VALUES ($1,$2,$3) ON CONFLICT (email) DO NOTHING`,
		req.Email, string(hashed), req.Phone)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}
	w.Write([]byte(`{"status":"created"}`))
}

func deleteProjectUserHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	userID := chi.URLParam(r, "id")
	tableName := fmt.Sprintf("project_%s_users", ref)
	_, err := controlDB.Exec(context.Background(),
		`DELETE FROM `+tableName+` WHERE id=$1`, userID)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}
	w.Write([]byte(`{"status":"deleted"}`))
}

// ------------------ Project-scoped AUTH (for end‑users) ------------------

func projectSignupHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	tableName := fmt.Sprintf("project_%s_users", ref)
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
	_, err := controlDB.Exec(context.Background(),
		`INSERT INTO `+tableName+` (email, password_hash, phone) VALUES ($1,$2,$3) ON CONFLICT (email) DO NOTHING`,
		req.Email, string(hashed), req.Phone)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}

	// Fetch the newly created user's ID
	var newUserID string
	err = controlDB.QueryRow(context.Background(),
		`SELECT id FROM `+tableName+` WHERE email=$1`, req.Email).Scan(&newUserID)
	if err != nil {
		http.Error(w, `{"error":"user created but could not fetch ID"}`, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "signup successful",
		"userId":  newUserID,
	})
}

func projectLoginHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	tableName := fmt.Sprintf("project_%s_users", ref)
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"email and password required"}`, 400)
		return
	}
	var userID, hashed string
	err := controlDB.QueryRow(context.Background(),
		`SELECT id, password_hash FROM `+tableName+` WHERE email=$1`, req.Email).Scan(&userID, &hashed)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hashed), []byte(req.Password)) != nil {
		http.Error(w, `{"error":"invalid credentials"}`, 401)
		return
	}
	claims := jwt.MapClaims{
		"sub": userID, "email": req.Email, "ref": ref,
		"iat": time.Now().Unix(), "exp": time.Now().Add(24*time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString(jwtSignKey)
	json.NewEncoder(w).Encode(map[string]interface{}{"token": signed, "userId": userID})
}
