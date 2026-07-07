package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("edge functions ready"))
	})
	log.Println("Edge Functions on :3005")
	log.Fatal(http.ListenAndServe(":3005", r))
}
