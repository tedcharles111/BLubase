package main

import (
    "context"
    "crypto/rand"
    "fmt"
    "net/http"
    "os"

    "github.com/go-chi/chi/v5"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
)

var (
    controlDB   *pgxpool.Pool
    minioClient *minio.Client
    jwtSignKey  []byte
)

func main() {
    // init DB, MinIO, JWT key
    controlDB, _ = pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
    minioClient, _ = minio.New(os.Getenv("MINIO_ENDPOINT"), &minio.Options{
        Creds: credentials.NewStaticV4(os.Getenv("MINIO_ACCESS_KEY"), os.Getenv("MINIO_SECRET_KEY"), ""),
    })
    jwtSignKey = []byte(os.Getenv("JWT_SECRET"))

    r := chi.NewRouter()
    r.Use(authMiddleware) // from shared JWT middleware (extracts user from auth-server JWT)

    r.Post("/projects", createProjectHandler)
    r.Get("/projects", listProjectsHandler)
    r.Delete("/projects/{id}", deleteProjectHandler)
    // regenerate keys, etc.

    http.ListenAndServe(":3002", r)
}

func createProjectHandler(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(string)
    var req struct{ Name string }
    json.NewDecoder(r.Body).Decode(&req)

    // Generate unique project ref (e.g., short random string)
    ref := generateRef(10)

    // Create new PostgreSQL schema named project_<ref> in the target DB (or spawn new DB)
    targetDB, _ := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL")+"&options=-c%20search_path%3D"+ref)
    // Actually, better to create schema in the same cluster used for project data
    _, err := controlDB.Exec(context.Background(), fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", ref))
    if err != nil { /* handle */ }

    // Create MinIO bucket
    bucketName := fmt.Sprintf("project-%s", ref)
    minioClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})

    // Generate anon key (public JWT with project ref, no user)
    anonToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "ref": ref,
        "role": "anon",
        "iat": time.Now().Unix(),
    })
    anonKey, _ := anonToken.SignedString(jwtSignKey)

    // Insert into projects table
    var projectID string
    err = controlDB.QueryRow(context.Background(),
        `INSERT INTO projects (name, ref, owner_id, anon_key, bucket_name) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
        req.Name, ref, userID, anonKey, bucketName).Scan(&projectID)

    // Return project URL and keys
    json.NewEncoder(w).Encode(map[string]interface{}{
        "id": projectID,
        "ref": ref,
        "url": fmt.Sprintf("https://%s.blubase.local", ref),
        "anon_key": anonKey,
        // service_role key generated similarly but with role "service_role"
    })
}
