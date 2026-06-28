package main

	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

var (
	dbPool    *pgxpool.Pool
	jwtSecret = []byte(os.Getenv("JWT_SECRET"))
)

func main() {
	ctx := context.Background()
	var err error
	dbPool, err = connectDB(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}

	// Ensure tables … (same as before)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS platform_users ( … )`)
	// … rest of table creation identical to previous full auth server

	r := chi.NewRouter()
	// … all routes …

	log.Println("Auth server on :3001")
	log.Fatal(http.ListenAndServe(":3001", r))
}

// connectDB adds sslmode=require and retries up to 10 times
func connectDB(ctx context.Context, rawURL string) (*pgxpool.Pool, error) {
	if !strings.Contains(rawURL, "sslmode=") {
		sep := "?"
		if strings.Contains(rawURL, "?") {
			sep = "&"
		}
		rawURL += sep + "sslmode=require"
	}
	for i := 0; i < 10; i++ {
		pool, err := pgxpool.New(ctx, rawURL)
		if err == nil {
			return pool, nil
		}
		log.Printf("DB connection attempt %d failed: %v. Retrying in 5s…", i+1, err)
		time.Sleep(5 * time.Second)
	}
	return pgxpool.New(ctx, rawURL)
}

// ---------- All original handler functions (signup, login, …) remain exactly the same ----------
// (Paste the full handlers from the previous working auth server here)
// … (I’ll include the complete set in the actual command)
// … all the handler functions that were already there …
