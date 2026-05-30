package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var db *pgxpool.Pool

func main() {
	var err error
	db, err = pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil { log.Fatal(err) }

	// Ensure the storage_files table exists
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
	if err != nil { log.Fatal(err) }

	r := chi.NewRouter()
	r.Post("/upload/{bucket}/{filename}", uploadHandler)
	r.Get("/download/{bucket}/{filename}", downloadHandler)
	r.Delete("/delete/{bucket}/{filename}", deleteHandler)
	log.Println("PostgreSQL Storage API on :3004")
	log.Fatal(http.ListenAndServe(":3004", r))
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
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "read error", 500)
		return
	}
	mime := header.Header.Get("Content-Type")
	if mime == "" { mime = "application/octet-stream" }
	_, err = db.Exec(context.Background(),
		`INSERT INTO storage_files (bucket, filename, data, mime_type, size) VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (bucket, filename) DO UPDATE SET data=$3, mime_type=$4, size=$5`,
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
	filename := chi.URLParam(r, "filename")
	_, err := db.Exec(context.Background(),
		`DELETE FROM storage_files WHERE bucket=$1 AND filename=$2`, bucket, filename)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	w.Write([]byte("deleted"))
}
