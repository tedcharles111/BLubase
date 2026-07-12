package main

import (
    "fmt"
    "io"
    "log"
    "net/http"

    "github.com/go-chi/chi/v5"
)

func main() {
    r := chi.NewRouter()

    r.Get("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("edge functions ready"))
    })

    // Invoke a deployed function by name
    r.Post("/invoke/{name}", func(w http.ResponseWriter, r *http.Request) {
        name := chi.URLParam(r, "name")
        body, _ := io.ReadAll(r.Body)
        // For now, just echo back the name and code length (Deno not installed, but you can add it later)
        msg := fmt.Sprintf("Invoked %s with %d bytes of code", name, len(body))
        w.Write([]byte(msg))
    })

    // List available functions (for the UI's /edge/functions endpoint)
    r.Get("/functions", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`["payment-webhooks", "hello-world"]`))
    })

    log.Println("Edge Functions on :3005")
    log.Fatal(http.ListenAndServe(":3005", r))
}
