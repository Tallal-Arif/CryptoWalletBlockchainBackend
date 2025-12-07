package tx

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/auth"
)

// ✅ Get details of a specific transaction
func DetailHandler(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	txID := r.URL.Query().Get("tx_id")
	if txID == "" {
		http.Error(w, "tx_id required", http.StatusBadRequest)
		return
	}

	var from, to, status, senderPub, sigR, sigS, note, ts string
	var amount, fee int64
	err := dbPool.QueryRow(context.Background(),
		`SELECT from_wallet_id, to_wallet_id, amount, fee, status,
                sender_public_key, signature_r, signature_s, note, timestamp
         FROM transactions WHERE tx_id=$1::uuid`, txID).
		Scan(&from, &to, &amount, &fee, &status, &senderPub, &sigR, &sigS, &note, &ts)
	if err != nil {
		http.Error(w, "transaction not found", http.StatusNotFound)
		return
	}

	// Inputs
	inRows, err := dbPool.Query(context.Background(),
		`SELECT ti.utxo_id::text, u.amount
         FROM transaction_inputs ti
         JOIN utxos u ON u.utxo_id = ti.utxo_id
         WHERE ti.tx_id=$1::uuid`, txID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	type In struct {
		UTXOID string `json:"utxo_id"`
		Amount int64  `json:"amount"`
	}
	var inputs []In
	for inRows.Next() {
		var i In
		if err := inRows.Scan(&i.UTXOID, &i.Amount); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		inputs = append(inputs, i)
	}
	inRows.Close()

	// Outputs
	outRows, err := dbPool.Query(context.Background(),
		`SELECT wallet_id, amount, output_index
         FROM transaction_outputs WHERE tx_id=$1::uuid ORDER BY output_index`, txID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	type Out struct {
		WalletID string `json:"wallet_id"`
		Amount   int64  `json:"amount"`
		Index    int    `json:"index"`
	}
	var outputs []Out
	for outRows.Next() {
		var o Out
		if err := outRows.Scan(&o.WalletID, &o.Amount, &o.Index); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		outputs = append(outputs, o)
	}
	outRows.Close()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tx_id":             txID,
		"from":              from,
		"to":                to,
		"amount":            amount,
		"fee":               fee,
		"status":            status,
		"sender_public_key": senderPub,
		"signature_r":       sigR,
		"signature_s":       sigS,
		"note":              note,
		"timestamp":         ts,
		"inputs":            inputs,
		"outputs":           outputs,
	})
}

// ✅ List transactions for a specific wallet
func WalletTxsHandler(w http.ResponseWriter, r *http.Request) {
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
		`SELECT tx_id::text, from_wallet_id, to_wallet_id, amount, fee, status, note, timestamp, created_at
         FROM transactions
         WHERE from_wallet_id=$1 OR to_wallet_id=$1
         ORDER BY created_at DESC`, walletID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type T struct {
		TxID      string `json:"tx_id"`
		From      string `json:"from_wallet_id"`
		To        string `json:"to_wallet_id"`
		Amount    int64  `json:"amount"`
		Fee       int64  `json:"fee"`
		Status    string `json:"status"`
		Note      string `json:"note"`
		Timestamp string `json:"timestamp"`
		Created   string `json:"created_at"`
	}
	var list []T
	for rows.Next() {
		var t T
		if err := rows.Scan(&t.TxID, &t.From, &t.To, &t.Amount, &t.Fee, &t.Status, &t.Note, &t.Timestamp, &t.Created); err != nil {
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
