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
	if err != nil { log.Fatal(err) }

	r := chi.NewRouter()
	r.Post("/projects", createProjectHandler)
	r.Get("/projects", listProjectsHandler)
	r.Get("/projects/{ref}/users", listProjectUsersHandler)
	r.Post("/projects/{ref}/users", addProjectUserHandler)
	r.Delete("/projects/{ref}/users/{id}", deleteProjectUserHandler)
	r.Post("/projects/{ref}/auth/signup", projectSignupHandler)
	r.Post("/projects/{ref}/auth/login", projectLoginHandler)
	r.Get("/projects/{ref}/auth-settings", getProjectAuthSettingsHandler)
	r.Put("/projects/{ref}/auth-settings", updateProjectAuthSettingsHandler)

	log.Println("Project manager on :3002")
	log.Fatal(http.ListenAndServe(":3002", r))
}

func extractUserID(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") { return "" }
	tokenStr := strings.TrimPrefix(auth, "Bearer ")
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) { return jwtSignKey, nil })
	if err != nil { return "" }
	sub, _ := claims["sub"].(string)
	return sub
}

func getProjectRef(r *http.Request) string {
	ref := chi.URLParam(r, "ref")
	if ref == "" { ref = r.Header.Get("x-project-ref") }
	return ref
}

func createProjectHandler(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" { http.Error(w, `{"error":"unauthorized"}`, 401); return }
	var req struct{ Name string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" { http.Error(w, `{"error":"name required"}`, 400); return }
	ref := make([]byte, 6)
	rand.Read(ref)
	refStr := base64.URLEncoding.EncodeToString(ref)[:6]
	anonToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"ref": refStr, "role": "anon", "iat": time.Now().Unix()})
	anonKey, _ := anonToken.SignedString(jwtSignKey)

	tableName := fmt.Sprintf("project_%s_users", refStr)
	_, _ = controlDB.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS `+tableName+` (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(), email TEXT UNIQUE, password_hash TEXT, phone TEXT, created_at TIMESTAMPTZ DEFAULT now()
	)`)

	var projectID string
	err := controlDB.QueryRow(context.Background(),
		`INSERT INTO projects (name, ref, owner_id, anon_key) VALUES ($1,$2,$3,$4) RETURNING id`,
		req.Name, refStr, userID, anonKey).Scan(&projectID)
	if err != nil { http.Error(w, `{"error":"database error"}`, 500); return }
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id": projectID, "name": req.Name, "ref": refStr,
		"anon_key": anonKey, "status": "active", "region": "us-east-1",
	})
}

func listProjectsHandler(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" { json.NewEncoder(w).Encode([]interface{}{}); return }
	rows, _ := controlDB.Query(context.Background(),
		`SELECT id, name, ref, anon_key, created_at FROM projects WHERE owner_id=$1`, userID)
	defer rows.Close()
	var projects []map[string]interface{}
	for rows.Next() {
		var id, name, ref, anonKey string
		var createdAt time.Time
		rows.Scan(&id, &name, &ref, &anonKey, &createdAt)
		projects = append(projects, map[string]interface{}{
			"id": id, "name": name, "ref": ref, "anon_key": anonKey,
			"status": "active", "region": "us-east-1", "created_at": createdAt.Format(time.RFC3339),
		})
	}
	json.NewEncoder(w).Encode(projects)
}

func listProjectUsersHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	tableName := fmt.Sprintf("project_%s_users", ref)
	rows, _ := controlDB.Query(context.Background(),
		`SELECT id, email, phone, created_at FROM `+tableName+` ORDER BY created_at DESC`)
	defer rows.Close()
	var users []map[string]interface{}
	for rows.Next() {
		var id, email, phone string
		var createdAt time.Time
		rows.Scan(&id, &email, &phone, &createdAt)
		users = append(users, map[string]interface{}{"id": id, "email": email, "phone": phone, "created_at": createdAt})
	}
	if users == nil { users = []map[string]interface{}{} }
	json.NewEncoder(w).Encode(users)
}

func addProjectUserHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	tableName := fmt.Sprintf("project_%s_users", ref)
	var req struct{ Email, Password, Phone string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" || req.Password == "" { http.Error(w, `{"error":"email and password required"}`, 400); return }
	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	_, err := controlDB.Exec(context.Background(),
		`INSERT INTO `+tableName+` (email, password_hash, phone) VALUES ($1,$2,$3) ON CONFLICT (email) DO NOTHING`,
		req.Email, string(hashed), req.Phone)
	if err != nil { http.Error(w, `{"error":"database error"}`, 500); return }
	w.Write([]byte(`{"status":"created"}`))
}

func deleteProjectUserHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	userID := chi.URLParam(r, "id")
	tableName := fmt.Sprintf("project_%s_users", ref)
	_, err := controlDB.Exec(context.Background(), `DELETE FROM `+tableName+` WHERE id=$1`, userID)
	if err != nil { http.Error(w, `{"error":"database error"}`, 500); return }
	w.Write([]byte(`{"status":"deleted"}`))
}

func projectSignupHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	tableName := fmt.Sprintf("project_%s_users", ref)

	// Check if signups are allowed for this project
	var allowSignups bool
	err := controlDB.QueryRow(context.Background(),
		`SELECT allow_signups FROM project_auth_settings WHERE project_ref=$1`, ref).Scan(&allowSignups)
	if err == nil && !allowSignups {
		http.Error(w, `{"error":"signups are disabled for this project"}`, 403)
		return
	}

	var req struct{ Email, Password, Phone string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" || req.Password == "" { http.Error(w, `{"error":"email and password required"}`, 400); return }
	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	_, err = controlDB.Exec(context.Background(),
		`INSERT INTO `+tableName+` (email, password_hash, phone) VALUES ($1,$2,$3) ON CONFLICT (email) DO NOTHING`,
		req.Email, string(hashed), req.Phone)
	if err != nil { http.Error(w, `{"error":"database error"}`, 500); return }

	var newUserID string
	err = controlDB.QueryRow(context.Background(),
		`SELECT id FROM `+tableName+` WHERE email=$1`, req.Email).Scan(&newUserID)
	if err != nil { http.Error(w, `{"error":"user created but could not fetch ID"}`, 500); return }

	json.NewEncoder(w).Encode(map[string]string{
		"message": "signup successful",
		"userId":  newUserID,
	})
}

func projectLoginHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	tableName := fmt.Sprintf("project_%s_users", ref)
	var req struct{ Email, Password string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" || req.Password == "" { http.Error(w, `{"error":"email and password required"}`, 400); return }
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

func getProjectAuthSettingsHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	var allowSignups, allowManual, allowAnon, confirmEmail bool
	err := controlDB.QueryRow(context.Background(),
		`SELECT allow_signups, allow_manual_linking, allow_anonymous_signins, confirm_email FROM project_auth_settings WHERE project_ref=$1`, ref).
		Scan(&allowSignups, &allowManual, &allowAnon, &confirmEmail)
	if err != nil {
		allowSignups = true
		allowManual = true
		allowAnon = false
		confirmEmail = false
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"allow_signups":           allowSignups,
		"allow_manual_linking":    allowManual,
		"allow_anonymous_signins": allowAnon,
		"confirm_email":           confirmEmail,
	})
}

func updateProjectAuthSettingsHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	var req struct {
		AllowSignups          bool `json:"allow_signups"`
		AllowManualLinking    bool `json:"allow_manual_linking"`
		AllowAnonymousSignins bool `json:"allow_anonymous_signins"`
		ConfirmEmail          bool `json:"confirm_email"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	_, err := controlDB.Exec(context.Background(),
		`INSERT INTO project_auth_settings (project_ref, allow_signups, allow_manual_linking, allow_anonymous_signins, confirm_email)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (project_ref) DO UPDATE SET
		 allow_signups=$2, allow_manual_linking=$3, allow_anonymous_signins=$4, confirm_email=$5`,
		ref, req.AllowSignups, req.AllowManualLinking, req.AllowAnonymousSignins, req.ConfirmEmail)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}
	w.Write([]byte(`{"status":"updated"}`))
}

import (
	"net/url"
	"time"
)

func connectDB(ctx context.Context, rawURL string) (*pgxpool.Pool, error) {
	// Ensure sslmode=require
	if !strings.Contains(rawURL, "sslmode=") {
		sep := "?"
		if strings.Contains(rawURL, "?") {
			sep = "&"
		}
		rawURL += sep + "sslmode=require"
	}
	// Retry up to 10 times
	for i := 0; i < 10; i++ {
		pool, err := pgxpool.New(ctx, rawURL)
		if err == nil {
			return pool, nil
		}
		log.Printf("Database connection attempt %d failed: %v. Retrying in 5s...", i+1, err)
		time.Sleep(5 * time.Second)
	}
	return pgxpool.New(ctx, rawURL)
}
