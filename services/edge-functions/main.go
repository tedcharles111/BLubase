package main

import (
    "io"
    "log"
    "net/http"
    "os/exec"
    "strings"

    "github.com/go-chi/chi/v5"
)

func main() {
    r := chi.NewRouter()

    r.Get("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("edge functions ready"))
    })

    r.Post("/invoke/{name}", func(w http.ResponseWriter, r *http.Request) {
        name := chi.URLParam(r, "name")
        body, _ := io.ReadAll(r.Body)
        // Execute Deno (must be installed in the container)
        cmd := exec.Command("deno", "eval", string(body))
        output, err := cmd.CombinedOutput()
        if err != nil {
            w.WriteHeader(500)
            w.Write([]byte(err.Error()))
            return
        }
        w.Write(output)
    })

    log.Println("Edge Functions on :3005")
    log.Fatal(http.ListenAndServe(":3005", r))
}
