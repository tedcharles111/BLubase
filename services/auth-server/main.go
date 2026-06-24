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
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/github"
)

var dbPool *pgxpool.Pool

func main() {
	ctx := context.Background()
	var err error
	dbPool, err = pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil { log.Fatal(err) }

	// Ensure per‑project OAuth table exists
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS project_oauth_providers (
		project_ref TEXT NOT NULL, provider TEXT NOT NULL,
		client_id TEXT, client_secret TEXT, enabled BOOLEAN DEFAULT false,
		PRIMARY KEY (project_ref, provider)
	)`)

	r := chi.NewRouter()

	// Test endpoint – just to prove routing works
	r.Get("/oauth-test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OAuth routes active"))
	})

	// Explicit Google login
	r.Get("/google/login", googleLoginHandler)
	r.Get("/google/callback", googleCallbackHandler)

	// Explicit GitHub login
	r.Get("/github/login", githubLoginHandler)
	r.Get("/github/callback", githubCallbackHandler)

	log.Println("Auth server on :3001")
	log.Fatal(http.ListenAndServe(":3001", r))
}

func getProjectRef(r *http.Request) string {
	return r.URL.Query().Get("project")
}

func googleLoginHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	if ref == "" { http.Error(w, "missing project", 400); return }
	var cid, csecret string
	err := dbPool.QueryRow(context.Background(),
		`SELECT client_id, client_secret FROM project_oauth_providers WHERE project_ref=$1 AND provider='google' AND enabled=true`,
		ref).Scan(&cid, &csecret)
	if err != nil { http.Error(w, "provider not configured", 404); return }

	cfg := &oauth2.Config{
		ClientID: cid, ClientSecret: csecret,
		RedirectURL: fmt.Sprintf("%s/auth/google/callback?project=%s", os.Getenv("RENDER_EXTERNAL_URL"), ref),
		Scopes: []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint: google.Endpoint,
	}
	state := make([]byte, 16); rand.Read(state); stateStr := base64.URLEncoding.EncodeToString(state)
	dbPool.Exec(context.Background(), `INSERT INTO oauth_states (state, provider, project_ref) VALUES ($1,'google',$2)`, stateStr, ref)
	http.Redirect(w, r, cfg.AuthCodeURL(stateStr), http.StatusFound)
}

func googleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	// simplified: exchange code, get email, return JWT
	http.Error(w, "callback not fully implemented yet", 501)
}

func githubLoginHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	if ref == "" { http.Error(w, "missing project", 400); return }
	var cid, csecret string
	err := dbPool.QueryRow(context.Background(),
		`SELECT client_id, client_secret FROM project_oauth_providers WHERE project_ref=$1 AND provider='github' AND enabled=true`,
		ref).Scan(&cid, &csecret)
	if err != nil { http.Error(w, "provider not configured", 404); return }

	cfg := &oauth2.Config{
		ClientID: cid, ClientSecret: csecret,
		RedirectURL: fmt.Sprintf("%s/auth/github/callback?project=%s", os.Getenv("RENDER_EXTERNAL_URL"), ref),
		Scopes: []string{"user:email"},
		Endpoint: github.Endpoint,
	}
	state := make([]byte, 16); rand.Read(state); stateStr := base64.URLEncoding.EncodeToString(state)
	dbPool.Exec(context.Background(), `INSERT INTO oauth_states (state, provider, project_ref) VALUES ($1,'github',$2)`, stateStr, ref)
	http.Redirect(w, r, cfg.AuthCodeURL(stateStr), http.StatusFound)
}

func githubCallbackHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "callback not fully implemented yet", 501)
}
