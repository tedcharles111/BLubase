package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

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

	_, err = db.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS storage_files (
			bucket TEXT,
			filename TEXT,
			data BYTEA,
			mime_type TEXT,
			size BIGINT,
			PRIMARY KEY (bucket, filename)
		);
	`)
	if err != nil {
		log.Fatal(err)
	}

	r := chi.NewRouter()
	// Use a single route that captures the bucket, then manually extracts the filename
	r.Post("/upload/{bucket}", uploadHandler)
	r.Get("/download/{bucket}", downloadHandler)
	r.Delete("/delete/{bucket}", deleteHandler)
	log.Println("Storage API on :3004 (manual path extraction)")
	log.Fatal(http.ListenAndServe(":3004", r))
}

// extractFilename returns the part of the URL after "/<bucket>/"
func extractFilename(r *http.Request, bucket string) string {
	path := r.URL.Path
	prefix := fmt.Sprintf("/%s/", bucket)
	idx := strings.Index(path, prefix)
	if idx == -1 {
		return ""
	}
	// Return everything after the prefix
	return path[idx+len(prefix):]
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	filename := extractFilename(r, bucket)
	if filename == "" {
		http.Error(w, "filename required", 400)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", 400)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "read error", 500)
		return
	}
	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}
	_, err = db.Exec(context.Background(),
		`INSERT INTO storage_files (bucket, filename, data, mime_type, size)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (bucket, filename) DO UPDATE SET data=$3, mime_type=$4, size=$5`,
		bucket, filename, data, mime, len(data))
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}

	url := fmt.Sprintf("https://blubase.onrender.com/storage/download/%s/%s", bucket, filename)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "uploaded",
		"url":    url,
	})
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	filename := extractFilename(r, bucket)
	if filename == "" {
		http.Error(w, "filename required", 400)
		return
	}
	var data []byte
	var mime string
	err := db.QueryRow(context.Background(),
		`SELECT data, mime_type FROM storage_files WHERE bucket=$1 AND filename=$2`,
		bucket, filename).Scan(&data, &mime)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Write(data)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	filename := extractFilename(r, bucket)
	if filename == "" {
		http.Error(w, "filename required", 400)
		return
	}
	_, err := db.Exec(context.Background(),
		`DELETE FROM storage_files WHERE bucket=$1 AND filename=$2`, bucket, filename)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	w.Write([]byte("deleted"))
}
