package main

import (
	"log"
	"net/http"

	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/auth"
	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/db"
	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/wallet"
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

	pool, err := db.ConnectDB()
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	//defer pool.Close() // close when the server exits

	// pass pool into your handlers or initialize your auth package
	auth.Init(pool)
	wallet.Init(pool)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health)
	mux.HandleFunc("/auth/register", auth.RegisterHandler)
	mux.HandleFunc("/auth/verify-otp", auth.VerifyOTPHandler)
	mux.Handle("/wallet/create", auth.JWTMiddleware(http.HandlerFunc(wallet.CreateHandler)))
	log.Println("Server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
