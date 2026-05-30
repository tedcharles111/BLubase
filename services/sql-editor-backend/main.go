package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var pool *pgxpool.Pool

func main() {
	var err error
	pool, err = pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	r := chi.NewRouter()
	r.Get("/sql", runSQLHandler)
	log.Println("SQL Editor on :3007")
	log.Fatal(http.ListenAndServe(":3007", r))
}

func runSQLHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	rows, err := pool.Query(context.Background(), query)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer rows.Close()
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
