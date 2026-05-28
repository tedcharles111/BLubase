package main

import (
	"bytes"
	"log"
	"net/http"
	"os/exec"

	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()
	r.Post("/invoke/{name}", invokeFunction)
	log.Println("Edge Function Manager on :3005")
	log.Fatal(http.ListenAndServe(":3005", r))
}

func invokeFunction(w http.ResponseWriter, r *http.Request) {
	funcName := chi.URLParam(r, "name")
	cmd := exec.Command("deno", "eval", "console.log('Edge function "+funcName+" executed')")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		http.Error(w, out.String(), 500)
		return
	}
	w.Write(out.Bytes())
}
