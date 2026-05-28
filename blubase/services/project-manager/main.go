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
	if err != nil {
		log.Fatal(err)
	}
	minioClient, err = minio.New(os.Getenv("MINIO_ENDPOINT"), &minio.Options{
		Creds: credentials.NewStaticV4(os.Getenv("MINIO_ACCESS_KEY"), os.Getenv("MINIO_SECRET_KEY"), ""),
	})
	if err != nil {
		log.Fatal(err)
	}

	r := chi.NewRouter()
	r.Post("/projects", createProjectHandler)
	r.Get("/projects", listProjectsHandler)
	log.Println("Project manager on :3002")
	log.Fatal(http.ListenAndServe(":3002", r))
}

func createProjectHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name string }
	json.NewDecoder(r.Body).Decode(&req)
	ref := make([]byte, 6)
	rand.Read(ref)
	refStr := base64.URLEncoding.EncodeToString(ref)[:6]

	anonToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"ref":  refStr,
		"role": "anon",
		"iat":  time.Now().Unix(),
	})
	anonKey, _ := anonToken.SignedString(jwtSignKey)

	bucketName := fmt.Sprintf("project-%s", refStr)
	minioClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})

	_, err := controlDB.Exec(context.Background(),
		`INSERT INTO projects (name, ref, owner_id, anon_key, bucket_name) VALUES ($1,$2,$3,$4,$5)`,
		req.Name, refStr, "00000000-0000-0000-0000-000000000000", anonKey, bucketName)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{
		"ref":      refStr,
		"url":      fmt.Sprintf("https://%s.blubase.local", refStr),
		"anon_key": anonKey,
	})
}

func listProjectsHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("[]"))
}
