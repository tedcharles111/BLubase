package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
	r.Get("/schema", schemaVisualizerHandler)  // new
	log.Println("Project manager on :3002")
	log.Fatal(http.ListenAndServe(":3002", r))
}

func extractUserID(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") { return "" }
	tokenStr := strings.TrimPrefix(auth, "Bearer ")
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return jwtSignKey, nil
	})
	if err != nil { return "" }
	sub, _ := claims["sub"].(string)
	return sub
}

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
	projects := []map[string]interface{}{}
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

func schemaVisualizerHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := controlDB.Query(context.Background(),
		`SELECT table_name, column_name, data_type FROM information_schema.columns WHERE table_schema='public' ORDER BY table_name, ordinal_position`)
	if err != nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	defer rows.Close()
	type Col struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	schema := map[string][]Col{}
	for rows.Next() {
		var t, c, d string
		rows.Scan(&t, &c, &d)
		schema[t] = append(schema[t], Col{Name: c, Type: d})
	}
	json.NewEncoder(w).Encode(schema)
}
