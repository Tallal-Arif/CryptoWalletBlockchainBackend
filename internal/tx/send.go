package tx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/auth"
	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/wallet"
)

var dbPool *pgxpool.Pool

func Init(pool *pgxpool.Pool) { dbPool = pool }

type SendRequest struct {
	FromWalletID string `json:"from_wallet_id"`
	ToWalletID   string `json:"to_wallet_id"`
	Amount       int64  `json:"amount"` // smallest units
	Nonce        string `json:"nonce"`  // client-provided idempotency key
}

type SendResponse struct {
	TxID    string   `json:"tx_id"`
	Status  string   `json:"status"`
	Inputs  []string `json:"inputs"`
	Outputs []struct {
		WalletID string `json:"wallet_id"`
		Amount   int64  `json:"amount"`
		Index    int    `json:"index"`
	} `json:"outputs"`
}

// Simple deterministic fee policy: 1% capped at 1000 units (adjust as needed)
func calcFee(amount int64) int64 {
	fee := amount / 100
	if fee > 1000 {
		fee = 1000
	}
	return fee
}

func SendHandler(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, _ := claims["user_id"].(string)

	var req SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.FromWalletID == "" || req.ToWalletID == "" || req.Amount <= 0 || req.Nonce == "" {
		http.Error(w, "missing or invalid fields", http.StatusBadRequest)
		return
	}
	if req.FromWalletID == req.ToWalletID {
		http.Error(w, "sender and recipient cannot be same for this endpoint", http.StatusBadRequest)
		return
	}

	// Ownership check
	if err := wallet.EnsureWalletOwnedByUser(dbPool, req.FromWalletID, userID); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	fee := calcFee(req.Amount)
	total := req.Amount + fee

	ctx := context.Background()
	tx, err := dbPool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		http.Error(w, "db begin error", http.StatusInternalServerError)
		return
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// Idempotency: nonce unique per from_wallet_id
	var existingTxID string
	err = tx.QueryRow(ctx,
		`SELECT tx_id::text FROM transactions WHERE from_wallet_id=$1 AND nonce=$2`,
		req.FromWalletID, req.Nonce).Scan(&existingTxID)
	if err == nil && existingTxID != "" {
		// Return existing response for idempotent requests
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"tx_id":  existingTxID,
			"status": "pending",
		})
		_ = tx.Rollback(ctx)
		return
	}

	// Gather unspent UTXOs
	rows, err := tx.Query(ctx,
		`SELECT utxo_id::text, amount
         FROM utxos
         WHERE wallet_id=$1 AND spent=false
         ORDER BY created_at ASC`, req.FromWalletID)
	if err != nil {
		http.Error(w, "db utxo query error", http.StatusInternalServerError)
		return
	}

	var selected []struct {
		UTXOID string
		Amount int64
	}
	var sum int64
	for rows.Next() {
		var id string
		var amt int64
		if err := rows.Scan(&id, &amt); err != nil {
			http.Error(w, "db scan error", http.StatusInternalServerError)
			return
		}
		selected = append(selected, struct {
			UTXOID string
			Amount int64
		}{UTXOID: id, Amount: amt})
		sum += amt
		if sum >= total {
			break
		}
	}
	rows.Close()

	if sum < total {
		http.Error(w, "insufficient funds", http.StatusBadRequest)
		return
	}

	// Create transaction row (pending)
	var newTxID string
	// Basic hash for signature preimage (can be expanded)
	preimage := fmt.Sprintf("%s|%s|%d|%d|%s", req.FromWalletID, req.ToWalletID, req.Amount, fee, req.Nonce)
	digest := sha256.Sum256([]byte(preimage))
	signature := hex.EncodeToString(digest[:]) // placeholder signature

	err = tx.QueryRow(ctx,
		`INSERT INTO transactions (from_wallet_id, to_wallet_id, amount, fee, nonce, signature, status)
         VALUES ($1,$2,$3,$4,$5,$6,'pending')
         RETURNING tx_id::text`,
		req.FromWalletID, req.ToWalletID, req.Amount, fee, req.Nonce, signature).
		Scan(&newTxID)
	if err != nil {
		http.Error(w, "db insert transaction error", http.StatusInternalServerError)
		return
	}

	// Mark selected UTXOs spent, create transaction inputs
	for _, s := range selected {
		_, err = tx.Exec(ctx, `UPDATE utxos SET spent=true WHERE utxo_id=$1`, s.UTXOID)
		if err != nil {
			http.Error(w, "db update utxo error", http.StatusInternalServerError)
			return
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO transaction_inputs (tx_id, utxo_id) VALUES ($1,$2)`,
			newTxID, s.UTXOID)
		if err != nil {
			http.Error(w, "db insert input error", http.StatusInternalServerError)
			return
		}
	}

	// Create outputs: recipient and change (if any)
	change := sum - total
	// output_index 0 → recipient
	_, err = tx.Exec(ctx,
		`INSERT INTO transaction_outputs (tx_id, wallet_id, amount, output_index)
         VALUES ($1,$2,$3,0)`,
		newTxID, req.ToWalletID, req.Amount)
	if err != nil {
		http.Error(w, "db insert output error", http.StatusInternalServerError)
		return
	}
	// output_index 1 → change (optional)
	if change > 0 {
		_, err = tx.Exec(ctx,
			`INSERT INTO transaction_outputs (tx_id, wallet_id, amount, output_index)
             VALUES ($1,$2,$3,1)`,
			newTxID, req.FromWalletID, change)
		if err != nil {
			http.Error(w, "db insert change output error", http.StatusInternalServerError)
			return
		}
	}

	// Materialize outputs into UTXOs
	_, err = tx.Exec(ctx,
		`INSERT INTO utxos (wallet_id, tx_id, output_index, amount, spent)
         SELECT wallet_id, tx_id, output_index, amount, false
         FROM transaction_outputs
         WHERE tx_id=$1`,
		newTxID)
	if err != nil {
		http.Error(w, "db insert utxos error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "db commit error", http.StatusInternalServerError)
		return
	}
	tx = nil

	// Build response
	resp := SendResponse{
		TxID:   newTxID,
		Status: "pending",
	}
	for _, s := range selected {
		resp.Inputs = append(resp.Inputs, s.UTXOID)
	}
	resp.Outputs = append(resp.Outputs, struct {
		WalletID string "json:\"wallet_id\""
		Amount   int64  "json:\"amount\""
		Index    int    "json:\"index\""
	}{WalletID: req.ToWalletID, Amount: req.Amount, Index: 0})
	if change > 0 {
		resp.Outputs = append(resp.Outputs, struct {
			WalletID string "json:\"wallet_id\""
			Amount   int64  "json:\"amount\""
			Index    int    "json:\"index\""
		}{WalletID: req.FromWalletID, Amount: change, Index: 1})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
