package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)


func main() {
	ctx := context.Background()
	var err error
	db, err = connectDB(ctx, os.Getenv("DATABASE_URL"))
	if err != nil { log.Fatal(err) }

	_, _ = db.Exec(ctx, `CREATE TABLE IF NOT EXISTS edge_functions (name TEXT PRIMARY KEY, code TEXT NOT NULL, updated_at TIMESTAMPTZ DEFAULT now())`)
	_, _ = db.Exec(ctx, `CREATE TABLE IF NOT EXISTS edge_secrets (function_name TEXT NOT NULL, key TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY (function_name, key))`)

	r := chi.NewRouter()
	r.Post("/functions", createFunctionHandler)
	r.Get("/functions", listFunctionsHandler)
	r.Delete("/functions/{name}", deleteFunctionHandler)
	r.Post("/secrets", addSecretHandler)
	r.Get("/secrets/{function}", listSecretsHandler)
	r.Delete("/secrets/{function}/{key}", deleteSecretHandler)
	r.Post("/invoke/{name}", invokeHandler)
	log.Println("Edge Functions Manager on :3005")
	log.Fatal(http.ListenAndServe(":3005", r))
}

	if !strings.Contains(rawURL, "sslmode=") {
		sep := "?"
		if strings.Contains(rawURL, "?") { sep = "&" }
		rawURL += sep + "sslmode=require"
	}
	for i := 0; i < 10; i++ {
		if err == nil { return pool, nil }
		log.Printf("DB connection attempt %d failed: %v. Retrying in 5s...", i+1, err)
		time.Sleep(5 * time.Second)
	}
}

func createFunctionHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name, Code string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" || req.Code == "" { http.Error(w, `{"error":"name and code required"}`, 400); return }
	_, err := db.Exec(context.Background(),
		`INSERT INTO edge_functions (name, code) VALUES ($1,$2) ON CONFLICT (name) DO UPDATE SET code=$2, updated_at=now()`,
		req.Name, req.Code)
	if err != nil { http.Error(w, `{"error":"database error"}`, 500); return }
	w.Write([]byte(`{"status":"created"}`))
}

func listFunctionsHandler(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query(context.Background(), `SELECT name, code, updated_at FROM edge_functions ORDER BY updated_at DESC`)
	defer rows.Close()
	var funcs []map[string]interface{}
	for rows.Next() {
		var name, code, updated string
		rows.Scan(&name, &code, &updated)
		funcs = append(funcs, map[string]interface{}{"name": name, "code": code, "updated_at": updated})
	}
	json.NewEncoder(w).Encode(funcs)
}

func deleteFunctionHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	db.Exec(context.Background(), `DELETE FROM edge_functions WHERE name=$1`, name)
	db.Exec(context.Background(), `DELETE FROM edge_secrets WHERE function_name=$1`, name)
	w.Write([]byte(`{"status":"deleted"}`))
}

func addSecretHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Function, Key, Value string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Function == "" || req.Key == "" || req.Value == "" {
		http.Error(w, `{"error":"function, key, value required"}`, 400); return
	}
	_, err := db.Exec(context.Background(),
		`INSERT INTO edge_secrets (function_name, key, value) VALUES ($1,$2,$3) ON CONFLICT (function_name, key) DO UPDATE SET value=$3`,
		req.Function, req.Key, req.Value)
	if err != nil { http.Error(w, `{"error":"database error"}`, 500); return }
	w.Write([]byte(`{"status":"created"}`))
}

func listSecretsHandler(w http.ResponseWriter, r *http.Request) {
	function := chi.URLParam(r, "function")
	rows, _ := db.Query(context.Background(), `SELECT key, value FROM edge_secrets WHERE function_name=$1`, function)
	defer rows.Close()
	secrets := map[string]string{}
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		secrets[k] = v
	}
	json.NewEncoder(w).Encode(secrets)
}

func deleteSecretHandler(w http.ResponseWriter, r *http.Request) {
	function := chi.URLParam(r, "function")
	key := chi.URLParam(r, "key")
	db.Exec(context.Background(), `DELETE FROM edge_secrets WHERE function_name=$1 AND key=$2`, function, key)
	w.Write([]byte(`{"status":"deleted"}`))
}

func invokeHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var code string
	err := db.QueryRow(context.Background(), `SELECT code FROM edge_functions WHERE name=$1`, name).Scan(&code)
	if err != nil { http.Error(w, `{"error":"function not found"}`, 404); return }

	rows, _ := db.Query(context.Background(), `SELECT key, value FROM edge_secrets WHERE function_name=$1`, name)
	envVars := []string{}
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		envVars = append(envVars, k+"="+v)
	}

	cmd := exec.Command("deno", "eval", code)
	cmd.Env = append(os.Environ(), envVars...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.Write(out.Bytes())
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write(out.Bytes())
}
