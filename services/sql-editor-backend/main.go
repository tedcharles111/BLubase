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
	"github.com/jackc/pgx/v5"
)

var pool *pgxpool.Pool

func main() {
	var err error
	config, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	// CRITICAL: disable prepared statement caching – prevents "prepared statement already in use"
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	pool, err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatal(err)
	}

	pool.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS sql_history (
		id SERIAL PRIMARY KEY,
		user_id TEXT,
		query TEXT,
		created_at TIMESTAMPTZ DEFAULT now()
	)`)

	r := chi.NewRouter()
	r.Get("/sql", runSQLHandler)
	r.Post("/sql", runSQLHandler) // <-- allow POST as well
	r.Get("/history", historyHandler)
	r.Post("/import", importHandler)

	log.Println("SQL Editor on :3007 – prepared statements DISABLED")
	log.Fatal(http.ListenAndServe(":3007", r))
}

func runSQLHandler(w http.ResponseWriter, r *http.Request) {
	// Helper to convert byte arrays to UUID strings
	toUUID := func(val interface{}) interface{} {
		if b, ok := val.([]byte); ok && len(b) == 16 {
			return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
		}
		return val
	}

	var query string
	if r.Method == "POST" {
		var req struct{ Query string `json:"query"` }
		json.NewDecoder(r.Body).Decode(&req)
		query = req.Query
	} else {
		query = r.URL.Query().Get("query")
	}
	if query == "" {
		http.Error(w, `{"error":"query required"}`, 400)
		return
	}

	userID := r.Header.Get("Authorization")
	if strings.HasPrefix(userID, "Bearer ") {
		userID = userID[7:]
	} else {
		userID = "anonymous"
	}

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
		for i, col := range cols {
			rowMap[string(col.Name)] = toUUID(vals[i])
		}
		result = append(result, rowMap)
	}
	json.NewEncoder(w).Encode(result)
}

func historyHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Authorization")
	if strings.HasPrefix(userID, "Bearer ") {
		userID = userID[7:]
	}
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "10"
	}
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
	if err != nil {
		http.Error(w, `{"error":"missing sqlfile"}`, 400)
		return
	}
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
