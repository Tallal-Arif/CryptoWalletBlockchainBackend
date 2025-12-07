package main

import (
	"log"
	"net/http"

	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/auth"
	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/block"
	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/db"
	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/explorer"
	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/tx"
	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/wallet"
	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/zakat"
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
	block.Init(pool)
	explorer.Init(pool)
	zakat.Init(pool, "<zakat_wallet_id>")
	mux := http.NewServeMux()
	//Check API health
	mux.HandleFunc("/health", health)
	//Auth routes
	mux.HandleFunc("/auth/register", auth.RegisterHandler)
	mux.HandleFunc("/auth/verify-otp", auth.VerifyOTPHandler)
	//Wallet routes
	mux.Handle("/wallet/create", auth.JWTMiddleware(http.HandlerFunc(wallet.CreateHandler)))
	mux.Handle("/wallet/list", auth.JWTMiddleware(http.HandlerFunc(wallet.ListHandler)))
	mux.Handle("/wallet/detail", auth.JWTMiddleware(http.HandlerFunc(wallet.DetailHandler)))
	mux.Handle("/wallet/utxos", auth.JWTMiddleware(http.HandlerFunc(wallet.UtxosHandler)))
	mux.Handle("/wallet/txs", auth.JWTMiddleware(http.HandlerFunc(wallet.TxHistoryHandler)))
	//Transaction routes
	mux.Handle("/tx/send", auth.JWTMiddleware(http.HandlerFunc(tx.SendHandler)))
	mux.Handle("/tx/detail", auth.JWTMiddleware(http.HandlerFunc(tx.DetailHandler)))
	mux.Handle("/tx/wallet", auth.JWTMiddleware(http.HandlerFunc(tx.WalletTxsHandler)))
	//Block routes
	mux.Handle("/blocks/commit", auth.JWTMiddleware(http.HandlerFunc(block.CommitHandler)))
	mux.HandleFunc("/blocks/latest", block.LatestHandler)
	mux.HandleFunc("/blocks/detail", block.DetailHandler)

	//Explorer routes
	mux.HandleFunc("/explorer/wallet/info", explorer.WalletInfoHandler)
	mux.HandleFunc("/explorer/wallet/utxos", explorer.WalletUtxosHandler)
	mux.HandleFunc("/explorer/tx/detail", explorer.TxDetailHandler)
	mux.HandleFunc("/explorer/blocks", explorer.BlocksListHandler)
	mux.HandleFunc("/explorer/block/detail", explorer.BlockDetailHandler)

	log.Println("Server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
