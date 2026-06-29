package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var pool *pgxpool.Pool

func main() {
	ctx := context.Background()
	var err error
	pool, err = connectDB(ctx, os.Getenv("DATABASE_URL"))
	if err != nil { log.Fatal(err) }

	// Ensure history table
	pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS sql_history (
		id SERIAL PRIMARY KEY, user_id TEXT, query TEXT,
		created_at TIMESTAMPTZ DEFAULT now()
	)`)

	r := chi.NewRouter()
	r.Get("/sql", runSQLHandler)
	r.Post("/sql", runSQLHandler)
	r.Get("/history", historyHandler)
	r.Post("/import", importHandler)
	log.Println("SQL Editor on :3007")
	log.Fatal(http.ListenAndServe(":3007", r))
}

func connectDB(ctx context.Context, rawURL string) (*pgxpool.Pool, error) {
	if !strings.Contains(rawURL, "sslmode=") {
		sep := "?"
		if strings.Contains(rawURL, "?") { sep = "&" }
		rawURL += sep + "sslmode=require"
	}
	for i := 0; i < 10; i++ {
		pool, err := pgxpool.New(ctx, rawURL)
		if err == nil { return pool, nil }
		log.Printf("DB connection attempt %d failed: %v. Retrying in 5s...", i+1, err)
		time.Sleep(5 * time.Second)
	}
	return pgxpool.New(ctx, rawURL)
}

func extractUserID(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") { return "" }
	return strings.TrimPrefix(auth, "Bearer ")
}

func runSQLHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" { 
		var req struct{ Query string `json:"query"` } 
		json.NewDecoder(r.Body).Decode(&req) 
		query = req.Query 
	}
	userID := extractUserID(r)
	rows, err := pool.Query(context.Background(), query)
	if err != nil {
		pool.Exec(context.Background(), `INSERT INTO sql_history (user_id, query) VALUES ($1,$2)`, userID, query)
		http.Error(w, err.Error(), 400)
		return
	}
	defer rows.Close()
	pool.Exec(context.Background(), `INSERT INTO sql_history (user_id, query) VALUES ($1,$2)`, userID, query)

	cols := rows.FieldDescriptions()
	result := []map[string]interface{}{}
	for rows.Next() {
		vals, _ := rows.Values()
		rowMap := map[string]interface{}{}
		for i, col := range cols { rowMap[string(col.Name)] = vals[i] }
		result = append(result, rowMap)
	}
	json.NewEncoder(w).Encode(result)
}

func historyHandler(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	limit := r.URL.Query().Get("limit")
	if limit == "" { limit = "10" }
	rows, _ := pool.Query(context.Background(),
		`SELECT query, created_at FROM sql_history WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2`, userID, limit)
	defer rows.Close()
	var history []map[string]interface{}
	for rows.Next() {
		var query string
		var createdAt time.Time
		rows.Scan(&query, &createdAt)
		history = append(history, map[string]interface{}{"query": query, "created_at": createdAt})
	}
	json.NewEncoder(w).Encode(history)
}

func importHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(10 << 20)
	file, _, err := r.FormFile("sqlfile")
	if err != nil { http.Error(w, `{"error":"missing sqlfile"}`, 400); return }
	defer file.Close()
	data, _ := io.ReadAll(file)
	cmd := exec.Command("psql", os.Getenv("DATABASE_URL"), "-c", string(data))
	out, err := cmd.CombinedOutput()
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.Write(out)
		w.WriteHeader(500)
		return
	}
	w.Write([]byte(`{"status":"migration successful"}`))
}
