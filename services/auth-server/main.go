package main

import (
    "context"
    "crypto/rand"
    "encoding/base64"
    "encoding/json"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/cors"
    "github.com/golang-jwt/jwt/v5"
    "github.com/gorilla/sessions"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"
    "golang.org/x/oauth2"
    "golang.org/x/oauth2/github"
    "golang.org/x/oauth2/google"
)

var (
    dbPool  *pgxpool.Pool
    redisClient *redis.Client
    store   = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))
    jwtSecret = []byte(os.Getenv("JWT_SECRET"))
    oauthConfigs = map[string]*oauth2.Config{}
)

func main() {
    // Initialize DB, Redis, OAuth configs (from env or DB)
    initInfra()
    initOAuthProviders()

    r := chi.NewRouter()
    r.Use(cors.Handler(cors.Options{...}))

    // Auth endpoints
    r.Post("/signup", signupHandler)
    r.Post("/login", loginHandler)
    r.Get("/authorize", oauthAuthorize)
    r.Post("/token", oauthToken)
    r.Get("/user", authenticate(userInfoHandler))

    // Social login callbacks
    r.Get("/auth/github/callback", githubCallback)
    r.Get("/auth/google/callback", googleCallback)
    // ... others

    // OAuth2 Client management (for developers to register apps)
    r.Post("/admin/clients", authenticate(createOAuthClient))

    log.Fatal(http.ListenAndServe(":3001", r))
}

func initInfra() {
    var err error
    dbPool, err = pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
    if err != nil { log.Fatal(err) }
    redisClient = redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_URL")})
}

func initOAuthProviders() {
    oauthConfigs["github"] = &oauth2.Config{
        ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
        ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
        RedirectURL:  "http://localhost:3001/auth/github/callback",
        Scopes:       []string{"user:email"},
        Endpoint:     github.Endpoint,
    }
    // ... similarly for Google, GitLab, Figma, Facebook
}

// --- Signup / Login (email+password) ---
func signupHandler(w http.ResponseWriter, r *http.Request) { /* ... */ }
func loginHandler(w http.ResponseWriter, r *http.Request) { /* ... */ }

// --- OAuth2 Server: authorize & token ---
func oauthAuthorize(w http.ResponseWriter, r *http.Request) {
    // Validate client_id, redirect_uri, scope. Create session, show consent.
    // After consent, redirect with code.
}

func oauthToken(w http.ResponseWriter, r *http.Request) {
    // Exchange authorization code for access/refresh JWT.
}

// --- Social Callbacks ---
func githubCallback(w http.ResponseWriter, r *http.Request) {
    // Exchange code for token, get GitHub user, upsert in DB, issue blubase JWT, redirect.
}

// --- JWT Middleware ---
func authenticate(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        tokenStr := extractBearerToken(r)
        token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
            return jwtSecret, nil
        })
        // Set userID in context
        next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), "userID", claims.Subject)))
    }
}

// ... helper functions
