package main

import (
  "fmt"
  "log"
  "net/http"
)

func health(w http.ResponseWriter, r *http.Request) {
  w.WriteHeader(http.StatusOK)
  fmt.Fprintln(w, "OK")
}

func main() {
  mux := http.NewServeMux()
  mux.HandleFunc("/health", health)
  log.Println("Server listening on :8080")
  log.Fatal(http.ListenAndServe(":8080", mux))
}
