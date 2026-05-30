package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var minioClient *minio.Client

func main() {
	var err error
	minioClient, err = minio.New(os.Getenv("MINIO_ENDPOINT"), &minio.Options{
		Creds: credentials.NewStaticV4(os.Getenv("MINIO_ACCESS_KEY"), os.Getenv("MINIO_SECRET_KEY"), ""),
	})
	if err != nil {
		log.Fatal(err)
	}
	r := chi.NewRouter()
	r.Post("/upload/{bucket}/{filename}", uploadHandler)
	r.Get("/download/{bucket}/{filename}", downloadHandler)
	r.Delete("/delete/{bucket}/{filename}", deleteHandler)
	log.Println("Storage API on :3004")
	log.Fatal(http.ListenAndServe(":3004", r))
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	filename := chi.URLParam(r, "filename")
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer file.Close()
	_, err = minioClient.PutObject(context.Background(), bucket, filename, file, -1, minio.PutObjectOptions{})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write([]byte("uploaded"))
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	filename := chi.URLParam(r, "filename")
	obj, err := minioClient.GetObject(context.Background(), bucket, filename, minio.GetObjectOptions{})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer obj.Close()
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	io.Copy(w, obj)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	filename := chi.URLParam(r, "filename")
	err := minioClient.RemoveObject(context.Background(), bucket, filename, minio.RemoveObjectOptions{})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write([]byte("deleted"))
}
