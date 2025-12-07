package block

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// ✅ Get latest block
func LatestHandler(w http.ResponseWriter, r *http.Request) {
	var blockID, prevHash, blockHash string
	var height int
	var created time.Time
	err := dbPool.QueryRow(context.Background(),
		`SELECT block_id::text, height, prev_hash, block_hash, created_at
         FROM blocks ORDER BY height DESC LIMIT 1`).
		Scan(&blockID, &height, &prevHash, &blockHash, &created)
	if err != nil {
		http.Error(w, "no blocks found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"block_id":   blockID,
		"height":     height,
		"prev_hash":  prevHash,
		"block_hash": blockHash,
		"created_at": created,
	})
}

// ✅ Get block details
func DetailHandler(w http.ResponseWriter, r *http.Request) {
	blockID := r.URL.Query().Get("block_id")
	if blockID == "" {
		http.Error(w, "block_id required", http.StatusBadRequest)
		return
	}

	var height int
	var prevHash, blockHash string
	var created time.Time
	err := dbPool.QueryRow(context.Background(),
		`SELECT height, prev_hash, block_hash, created_at
         FROM blocks WHERE block_id=$1::uuid`, blockID).
		Scan(&height, &prevHash, &blockHash, &created)
	if err != nil {
		http.Error(w, "block not found", http.StatusNotFound)
		return
	}

	rows, err := dbPool.Query(context.Background(),
		`SELECT tx_id::text FROM transactions WHERE block_id=$1::uuid`, blockID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	var txIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		txIDs = append(txIDs, id)
	}
	rows.Close()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"block_id":   blockID,
		"height":     height,
		"prev_hash":  prevHash,
		"block_hash": blockHash,
		"created_at": created,
		"tx_ids":     txIDs,
	})
}
