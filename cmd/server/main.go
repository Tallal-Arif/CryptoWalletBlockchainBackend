package main

import (
	"log"
	"net/http"

	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/db"
	"github.com/joho/godotenv"
)

func health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	// Load .env file into environment
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment")
	}

	db.ConnectDB()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", health)

	log.Println("Server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
