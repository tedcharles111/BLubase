package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
	dbPool, err = pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}

	// Ensure tables (platform‑level, still needed for admin, etc.)
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS platform_users (…)`) // shortened for brevity
	dbPool.Exec(ctx, `CREATE TABLE IF NOT EXISTS project_oauth_providers (
		project_ref TEXT NOT NULL,
		provider TEXT NOT NULL,
		client_id TEXT,
		client_secret TEXT,
		enabled BOOLEAN DEFAULT false,
		PRIMARY KEY (project_ref, provider)
	)`)

	r := chi.NewRouter()

	// Platform auth
	r.Post("/signup", signupHandler)
	r.Post("/login", loginHandler)
	r.Post("/forgot-password", forgotPasswordHandler)
	r.Post("/reset-password", resetPasswordHandler)
	r.Get("/auth/{provider}/login", projectOAuthLoginHandler)
	r.Get("/auth/{provider}/callback", projectOAuthCallbackHandler)


	// OAuth flow (project‑scoped)

	// Project‑scoped OAuth admin (requires x‑project‑ref header)
	r.Get("/admin/oauth-providers", listProjectOAuthProvidersHandler)
	r.Post("/admin/oauth-providers", createProjectOAuthProviderHandler)
	r.Put("/admin/oauth-providers/{provider}", updateProjectOAuthProviderHandler)
	r.Delete("/admin/oauth-providers/{provider}", deleteProjectOAuthProviderHandler)

	// … rest of admin endpoints (platform users, templates, etc.)

	log.Println("Auth server on :3001")
	log.Fatal(http.ListenAndServe(":3001", r))
}

// ---------- Helper: extract project ref from header ----------
func getProjectRef(r *http.Request) string {
	return r.Header.Get("x-project-ref")
}

// ---------- OAuth login per project ----------
func projectOAuthLoginHandler(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	projectRef := getProjectRef(r)
	if projectRef == "" {
		http.Error(w, "missing x-project-ref header", 400)
		return
	}

	var cid, csecret string
	var enabled bool
	err := dbPool.QueryRow(context.Background(),
		`SELECT client_id, client_secret, enabled FROM project_oauth_providers
		 WHERE project_ref=$1 AND provider=$2`, projectRef, provider).
		Scan(&cid, &csecret, &enabled)
	if err != nil || !enabled {
		http.Error(w, "provider not configured", 404)
		return
	}

	config := &oauth2.Config{
		ClientID:     cid,
		ClientSecret: csecret,
		RedirectURL:  fmt.Sprintf("%s/auth/%s/callback", os.Getenv("RENDER_EXTERNAL_URL"), provider),
		Scopes:       []string{"user:email"},
		Endpoint:     github.Endpoint, // default, switch below
	}
	switch provider {
	case "google":
		config.Scopes = []string{"https://www.googleapis.com/auth/userinfo.email"}
		config.Endpoint = google.Endpoint
	case "github":
		config.Scopes = []string{"user:email"}
		config.Endpoint = github.Endpoint
	// add other providers as needed
	}

	state := make([]byte, 16)
	rand.Read(state)
	stateStr := base64.URLEncoding.EncodeToString(state)
	dbPool.Exec(context.Background(), `INSERT INTO oauth_states (state, provider, project_ref) VALUES ($1,$2,$3)`,
		stateStr, provider, projectRef)

	url := config.AuthCodeURL(stateStr)
	http.Redirect(w, r, url, http.StatusFound)
}

func projectOAuthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	projectRef := r.URL.Query().Get("state") // state contains project ref? We need a better way.
	// For simplicity, we'll retrieve project_ref from oauth_states table.
	state := r.URL.Query().Get("state")
	var prov, pref string
	err := dbPool.QueryRow(context.Background(),
		`SELECT provider, project_ref FROM oauth_states WHERE state=$1`, state).
		Scan(&prov, &pref)
	if err != nil || prov != provider {
		http.Error(w, "invalid state", 400)
		return
	}
	dbPool.Exec(context.Background(), `DELETE FROM oauth_states WHERE state=$1`, state)

	// Exchange code for token (same as before, using the provider's config loaded from DB)
	// … (code omitted for brevity, identical to previous callback but loads per‑project config)
	// After obtaining email, insert user and return JWT.
}

// ---------- Project‑scoped admin handlers ----------
func listProjectOAuthProvidersHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	if ref == "" { http.Error(w, "missing x-project-ref", 400); return }
	rows, _ := dbPool.Query(context.Background(),
		`SELECT provider, client_id, client_secret, enabled FROM project_oauth_providers WHERE project_ref=$1`, ref)
	defer rows.Close()
	var providers []map[string]interface{}
	for rows.Next() {
		var p, cid, csec string
		var en bool
		rows.Scan(&p, &cid, &csec, &en)
		providers = append(providers, map[string]interface{}{
			"provider": p, "client_id": cid, "client_secret": "***", "enabled": en,
		})
	}
	json.NewEncoder(w).Encode(providers)
}

func createProjectOAuthProviderHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	if ref == "" { http.Error(w, "missing x-project-ref", 400); return }
	var req struct{ Provider, ClientID, ClientSecret string; Enabled bool }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Provider == "" || req.ClientID == "" || req.ClientSecret == "" {
		http.Error(w, `{"error":"provider, clientId, clientSecret required"}`, 400)
		return
	}
	dbPool.Exec(context.Background(),
		`INSERT INTO project_oauth_providers (project_ref, provider, client_id, client_secret, enabled)
		 VALUES ($1,$2,$3,$4,$5) ON CONFLICT (project_ref, provider) DO UPDATE SET client_id=$3, client_secret=$4, enabled=$5`,
		ref, req.Provider, req.ClientID, req.ClientSecret, req.Enabled)
	w.Write([]byte(`{"status":"created"}`))
}

func updateProjectOAuthProviderHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	provider := chi.URLParam(r, "provider")
	var req struct{ ClientID, ClientSecret string; Enabled *bool }
	json.NewDecoder(r.Body).Decode(&req)
	if req.ClientID == "" || req.ClientSecret == "" {
		http.Error(w, `{"error":"clientId and clientSecret required"}`, 400)
		return
	}
	enabled := true
	if req.Enabled != nil { enabled = *req.Enabled }
	dbPool.Exec(context.Background(),
		`UPDATE project_oauth_providers SET client_id=$1, client_secret=$2, enabled=$3
		 WHERE project_ref=$4 AND provider=$5`,
		req.ClientID, req.ClientSecret, enabled, ref, provider)
	w.Write([]byte(`{"status":"updated"}`))
}

func deleteProjectOAuthProviderHandler(w http.ResponseWriter, r *http.Request) {
	ref := getProjectRef(r)
	provider := chi.URLParam(r, "provider")
	dbPool.Exec(context.Background(),
		`DELETE FROM project_oauth_providers WHERE project_ref=$1 AND provider=$2`, ref, provider)
	w.Write([]byte(`{"status":"deleted"}`))
}

// … rest of the handlers (signup, login, forgot-password, etc.) remain the same as the nuclear rewrite.
// They are omitted here for space, but will be included in the actual file.


func getProjectRef(r *http.Request) string { return r.Header.Get("x-project-ref") }

func projectOAuthLoginHandler(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	projectRef := getProjectRef(r)
	if projectRef == "" { http.Error(w, "missing x-project-ref header", 400); return }
	var cid, csecret string
	var enabled bool
	err := dbPool.QueryRow(context.Background(),
		`SELECT client_id, client_secret, enabled FROM project_oauth_providers WHERE project_ref=$1 AND provider=$2`,
		projectRef, provider).Scan(&cid, &csecret, &enabled)
	if err != nil || !enabled { http.Error(w, "provider not configured", 404); return }

	cfg := &oauth2.Config{ClientID: cid, ClientSecret: csecret, RedirectURL: fmt.Sprintf("%s/auth/%s/callback", os.Getenv("RENDER_EXTERNAL_URL"), provider)}
	switch provider {
	case "google": cfg.Scopes = []string{"https://www.googleapis.com/auth/userinfo.email"}; cfg.Endpoint = google.Endpoint
	case "github": cfg.Scopes = []string{"user:email"}; cfg.Endpoint = github.Endpoint
	default: http.Error(w, "unsupported provider", 400); return
	}
	state := make([]byte, 16); rand.Read(state); stateStr := base64.URLEncoding.EncodeToString(state)
	dbPool.Exec(context.Background(), `INSERT INTO oauth_states (state, provider, project_ref) VALUES ($1,$2,$3)`, stateStr, provider, projectRef)
	http.Redirect(w, r, cfg.AuthCodeURL(stateStr), http.StatusFound)
}

func projectOAuthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	var prov, pref string
	err := dbPool.QueryRow(context.Background(), `SELECT provider, project_ref FROM oauth_states WHERE state=$1`, state).Scan(&prov, &pref)
	if err != nil || prov != provider { http.Error(w, "invalid state", 400); return }
	dbPool.Exec(context.Background(), `DELETE FROM oauth_states WHERE state=$1`, state)

	// Load project config
	var cid, csecret string
	dbPool.QueryRow(context.Background(), `SELECT client_id, client_secret FROM project_oauth_providers WHERE project_ref=$1 AND provider=$2`, pref, provider).Scan(&cid, &csecret)
	cfg := &oauth2.Config{ClientID: cid, ClientSecret: csecret, Endpoint: google.Endpoint}
	if provider == "github" { cfg.Endpoint = github.Endpoint }
	token, err := cfg.Exchange(context.Background(), code)
	if err != nil { http.Error(w, "token exchange failed", 500); return }

	// Fetch email (same as before, omitted for brevity)
	// … insert user, return JWT
	json.NewEncoder(w).Encode(map[string]string{"token": "jwt", "userId": "uuid"})
}
