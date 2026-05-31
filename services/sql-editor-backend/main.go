package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var pool *pgxpool.Pool

func main() {
	var err error
	pool, err = pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil { log.Fatal(err) }

	// ensure history table
	pool.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS sql_history (
		id SERIAL PRIMARY KEY,
		user_id TEXT,
		query TEXT,
		created_at TIMESTAMPTZ DEFAULT now()
	)`)

	r := chi.NewRouter()
	r.Get("/sql", runSQLHandler)
	r.Get("/history", historyHandler)
	log.Println("SQL Editor on :3007")
	log.Fatal(http.ListenAndServe(":3007", r))
}

func extractUserID(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

func runSQLHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	userID := extractUserID(r)
	rows, err := pool.Query(context.Background(), query)
	if err != nil {
		// log failure
		pool.Exec(context.Background(), `INSERT INTO sql_history (user_id, query) VALUES ($1,$2)`, userID, query)
		http.Error(w, err.Error(), 400)
		return
	}
	defer rows.Close()

	// log success
	pool.Exec(context.Background(), `INSERT INTO sql_history (user_id, query) VALUES ($1,$2)`, userID, query)

	cols := rows.FieldDescriptions()
	result := []map[string]interface{}{}
	for rows.Next() {
		vals, _ := rows.Values()
		rowMap := map[string]interface{}{}
		for i, col := range cols {
			rowMap[string(col.Name)] = vals[i]
		}
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
