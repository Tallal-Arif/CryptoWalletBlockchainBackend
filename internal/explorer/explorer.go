package explorer

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

var dbPool *pgxpool.Pool

func Init(pool *pgxpool.Pool) { dbPool = pool }

func WalletInfoHandler(w http.ResponseWriter, r *http.Request) {
	walletID := r.URL.Query().Get("wallet_id")
	if walletID == "" {
		http.Error(w, "wallet_id required", http.StatusBadRequest)
		return
	}

	var pubKey string
	err := dbPool.QueryRow(context.Background(),
		`SELECT public_key FROM wallets WHERE wallet_id=$1`, walletID).Scan(&pubKey)
	if err != nil {
		http.Error(w, "wallet not found", http.StatusNotFound)
		return
	}

	var balance int64
	err = dbPool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(amount),0) FROM utxos WHERE wallet_id=$1 AND spent=false`, walletID).
		Scan(&balance)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"wallet_id":  walletID,
		"public_key": pubKey,
		"balance":    balance,
	})
}

func WalletUtxosHandler(w http.ResponseWriter, r *http.Request) {
	walletID := r.URL.Query().Get("wallet_id")
	if walletID == "" {
		http.Error(w, "wallet_id required", http.StatusBadRequest)
		return
	}

	rows, err := dbPool.Query(context.Background(),
		`SELECT utxo_id::text, amount FROM utxos WHERE wallet_id=$1 AND spent=false ORDER BY created_at ASC`,
		walletID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	type U struct {
		UTXOID string `json:"utxo_id"`
		Amount int64  `json:"amount"`
	}
	var list []U
	for rows.Next() {
		var u U
		if err := rows.Scan(&u.UTXOID, &u.Amount); err != nil {
			http.Error(w, "db scan error", http.StatusInternalServerError)
			return
		}
		list = append(list, u)
	}
	rows.Close()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"wallet_id": walletID,
		"utxos":     list,
	})
}

func TxDetailHandler(w http.ResponseWriter, r *http.Request) {
	txID := r.URL.Query().Get("tx_id")
	if txID == "" {
		http.Error(w, "tx_id required", http.StatusBadRequest)
		return
	}

	var from, to, status string
	var amount, fee int64
	err := dbPool.QueryRow(context.Background(),
		`SELECT from_wallet_id, to_wallet_id, amount, fee, status
         FROM transactions WHERE tx_id=$1::uuid`, txID).
		Scan(&from, &to, &amount, &fee, &status)
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
			http.Error(w, "db scan error", http.StatusInternalServerError)
			return
		}
		inputs = append(inputs, i)
	}
	inRows.Close()

	// Outputs
	outRows, err := dbPool.Query(context.Background(),
		`SELECT wallet_id, amount, output_index FROM transaction_outputs WHERE tx_id=$1::uuid ORDER BY output_index`,
		txID)
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
			http.Error(w, "db scan error", http.StatusInternalServerError)
			return
		}
		outputs = append(outputs, o)
	}
	outRows.Close()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tx_id":   txID,
		"from":    from,
		"to":      to,
		"amount":  amount,
		"fee":     fee,
		"status":  status,
		"inputs":  inputs,
		"outputs": outputs,
	})
}

func BlocksListHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := dbPool.Query(context.Background(),
		`SELECT block_id::text, height, prev_hash, block_hash, created_at
         FROM blocks ORDER BY height DESC LIMIT 100`)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	type B struct {
		BlockID   string `json:"block_id"`
		Height    int    `json:"height"`
		PrevHash  string `json:"prev_hash"`
		BlockHash string `json:"block_hash"`
		CreatedAt string `json:"created_at"`
	}
	var list []B
	for rows.Next() {
		var b B
		if err := rows.Scan(&b.BlockID, &b.Height, &b.PrevHash, &b.BlockHash, &b.CreatedAt); err != nil {
			http.Error(w, "db scan error", http.StatusInternalServerError)
			return
		}
		list = append(list, b)
	}
	rows.Close()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"blocks": list,
	})
}
