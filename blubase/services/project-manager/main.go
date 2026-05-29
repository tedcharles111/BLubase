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
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	controlDB   *pgxpool.Pool
	minioClient *minio.Client
	jwtSignKey  = []byte(os.Getenv("JWT_SECRET"))
)

func main() {
	var err error
	controlDB, err = pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil { log.Fatal(err) }
	minioClient, err = minio.New(os.Getenv("MINIO_ENDPOINT"), &minio.Options{
		Creds: credentials.NewStaticV4(os.Getenv("MINIO_ACCESS_KEY"), os.Getenv("MINIO_SECRET_KEY"), ""),
	})
	if err != nil { log.Fatal(err) }

	r := chi.NewRouter()
	r.Post("/projects", createProjectHandler)
	r.Get("/projects", listProjectsHandler)
	log.Println("Project manager running on :3002")
	log.Fatal(http.ListenAndServe(":3002", r))
}

func createProjectHandler(w http.ResponseWriter, r *http.Request) {
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

	bucketName := fmt.Sprintf("project-%s", refStr)
	err := minioClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})
	if err != nil {
		log.Println("MinIO bucket creation error:", err)
	}

	var projectID string
	err = controlDB.QueryRow(context.Background(),
		`INSERT INTO projects (name, ref, anon_key, bucket_name)
		 VALUES ($1,$2,$3,$4) RETURNING id`,
		req.Name, refStr, anonKey, bucketName).Scan(&projectID)
	if err != nil {
		log.Println("Insert error:", err)    // <-- this prints the real reason
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":       projectID,
		"name":     req.Name,
		"ref":      refStr,
		"anon_key": anonKey,
		"status":   "active",
		"region":   "us-east-1",
	})
}

func listProjectsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := controlDB.Query(context.Background(),
		`SELECT id, name, ref, anon_key, bucket_name, created_at FROM projects`)
	if err != nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	defer rows.Close()

	var projects []map[string]interface{}
	for rows.Next() {
		var id, name, ref, anonKey, bucketName string
		var createdAt time.Time
		rows.Scan(&id, &name, &ref, &anonKey, &bucketName, &createdAt)
		projects = append(projects, map[string]interface{}{
			"id": id, "name": name, "ref": ref,
			"anon_key": anonKey, "status": "active", "region": "us-east-1",
			"created_at": createdAt.Format(time.RFC3339),
		})
	}
	json.NewEncoder(w).Encode(projects)
}
