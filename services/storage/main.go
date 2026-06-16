package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var db *pgxpool.Pool

func main() {
	var err error
	db, err = pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}

	// Ensure tables exist
	_, _ = db.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS storage_buckets (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name TEXT NOT NULL UNIQUE,
		public BOOLEAN DEFAULT true,
		created_at TIMESTAMPTZ DEFAULT now()
	)`)
	_, _ = db.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS storage_files (
		bucket TEXT NOT NULL,
		filename TEXT NOT NULL,
		data BYTEA,
		mime_type TEXT,
		size BIGINT,
		PRIMARY KEY (bucket, filename)
	)`)

	r := chi.NewRouter()

	// Bucket management
	r.Get("/buckets", listBucketsHandler)
	r.Post("/buckets", createBucketHandler)
	r.Delete("/buckets/{name}", deleteBucketHandler)

	// File operations
	r.Post("/upload/{bucket}/{filename}", uploadHandler)
	r.Get("/download/{bucket}/{filename}", downloadHandler)
	r.Delete("/delete/{bucket}/{filename}", deleteHandler)

	log.Println("Storage API with buckets on :3004")
	log.Fatal(http.ListenAndServe(":3004", r))
}

func listBucketsHandler(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query(context.Background(), `SELECT id, name, public, created_at FROM storage_buckets ORDER BY created_at DESC`)
	defer rows.Close()
	var buckets []map[string]interface{}
	for rows.Next() {
		var id, name string
		var public bool
		var createdAt time.Time
		rows.Scan(&id, &name, &public, &createdAt)
		buckets = append(buckets, map[string]interface{}{"id": id, "name": name, "public": public, "created_at": createdAt})
	}
	json.NewEncoder(w).Encode(buckets)
}

func createBucketHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name string; Public bool `json:"public"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" {
		http.Error(w, `{"error":"bucket name required"}`, 400)
		return
	}
	_, err := db.Exec(context.Background(), `INSERT INTO storage_buckets (name, public) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING`, req.Name, req.Public)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, 500)
		return
	}
	w.Write([]byte(`{"status":"created"}`))
}

func deleteBucketHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	_, _ = db.Exec(context.Background(), `DELETE FROM storage_files WHERE bucket=$1`, name)
	_, _ = db.Exec(context.Background(), `DELETE FROM storage_buckets WHERE name=$1`, name)
	w.Write([]byte(`{"status":"deleted"}`))
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	filename := chi.URLParam(r, "filename")
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", 400)
		return
	}
	defer file.Close()
	data, _ := io.ReadAll(file)
	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}
	_, err = db.Exec(context.Background(),
		`INSERT INTO storage_files (bucket, filename, data, mime_type, size) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (bucket, filename) DO UPDATE SET data=$3, mime_type=$4, size=$5`,
		bucket, filename, data, mime, len(data))
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	w.Write([]byte("uploaded"))
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	filename := chi.URLParam(r, "filename")
	var data []byte
	var mime string
	err := db.QueryRow(context.Background(), `SELECT data, mime_type FROM storage_files WHERE bucket=$1 AND filename=$2`, bucket, filename).Scan(&data, &mime)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Write(data)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	filename := chi.URLParam(r, "filename")
	_, _ = db.Exec(context.Background(), `DELETE FROM storage_files WHERE bucket=$1 AND filename=$2`, bucket, filename)
	w.Write([]byte("deleted"))
}
