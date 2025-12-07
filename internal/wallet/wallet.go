package wallet

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/auth"
)

// ✅ List all wallets for the authenticated user
func ListHandler(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, _ := claims["user_id"].(string)

	rows, err := dbPool.Query(context.Background(),
		`SELECT wallet_id, public_key, created_at FROM wallets WHERE user_id=$1`, userID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type W struct {
		WalletID  string    `json:"wallet_id"`
		PublicKey string    `json:"public_key"`
		CreatedAt time.Time `json:"created_at"`
	}
	var list []W
	for rows.Next() {
		var wallet W
		if err := rows.Scan(&wallet.WalletID, &wallet.PublicKey, &wallet.CreatedAt); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		list = append(list, wallet)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"wallets": list})
}

// ✅ Get details of a specific wallet
func DetailHandler(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, _ := claims["user_id"].(string)

	walletID := r.URL.Query().Get("wallet_id")
	if walletID == "" {
		http.Error(w, "wallet_id required", http.StatusBadRequest)
		return
	}

	var pubKey, owner string
	var created time.Time
	err := dbPool.QueryRow(context.Background(),
		`SELECT user_id, public_key, created_at FROM wallets WHERE wallet_id=$1`, walletID).
		Scan(&owner, &pubKey, &created)
	if err != nil {
		http.Error(w, "wallet not found", http.StatusNotFound)
		return
	}
	if owner != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"wallet_id":  walletID,
		"public_key": pubKey,
		"created_at": created,
	})
}

// ✅ List UTXOs for a wallet
func UtxosHandler(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, _ := claims["user_id"].(string)

	walletID := r.URL.Query().Get("wallet_id")
	if walletID == "" {
		http.Error(w, "wallet_id required", http.StatusBadRequest)
		return
	}

	// Ownership check
	var owner string
	err := dbPool.QueryRow(context.Background(),
		`SELECT user_id FROM wallets WHERE wallet_id=$1`, walletID).Scan(&owner)
	if err != nil {
		http.Error(w, "wallet not found", http.StatusNotFound)
		return
	}
	if owner != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	rows, err := dbPool.Query(context.Background(),
		`SELECT utxo_id::text, amount, spent FROM utxos WHERE wallet_id=$1 ORDER BY created_at ASC`, walletID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type U struct {
		UTXOID string `json:"utxo_id"`
		Amount int64  `json:"amount"`
		Spent  bool   `json:"spent"`
	}
	var list []U
	for rows.Next() {
		var u U
		if err := rows.Scan(&u.UTXOID, &u.Amount, &u.Spent); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		list = append(list, u)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"wallet_id": walletID,
		"utxos":     list,
	})
}

// ✅ List transactions for a wallet
func TxHistoryHandler(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, _ := claims["user_id"].(string)

	walletID := r.URL.Query().Get("wallet_id")
	if walletID == "" {
		http.Error(w, "wallet_id required", http.StatusBadRequest)
		return
	}

	// Ownership check
	var owner string
	err := dbPool.QueryRow(context.Background(),
		`SELECT user_id FROM wallets WHERE wallet_id=$1`, walletID).Scan(&owner)
	if err != nil {
		http.Error(w, "wallet not found", http.StatusNotFound)
		return
	}
	if owner != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	rows, err := dbPool.Query(context.Background(),
		`SELECT tx_id::text, from_wallet_id, to_wallet_id, amount, fee, status, created_at
         FROM transactions
         WHERE from_wallet_id=$1 OR to_wallet_id=$1
         ORDER BY created_at DESC`, walletID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type T struct {
		TxID    string `json:"tx_id"`
		From    string `json:"from_wallet_id"`
		To      string `json:"to_wallet_id"`
		Amount  int64  `json:"amount"`
		Fee     int64  `json:"fee"`
		Status  string `json:"status"`
		Created string `json:"created_at"`
	}
	var list []T
	for rows.Next() {
		var t T
		if err := rows.Scan(&t.TxID, &t.From, &t.To, &t.Amount, &t.Fee, &t.Status, &t.Created); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		list = append(list, t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"wallet_id":    walletID,
		"transactions": list,
	})
}
