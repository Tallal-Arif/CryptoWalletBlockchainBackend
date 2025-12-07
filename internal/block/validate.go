package block

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ValidateResponse struct {
	Valid         bool     `json:"valid"`
	CheckedBlocks int      `json:"checked_blocks"`
	Errors        []string `json:"errors"`
	LastHeight    int      `json:"last_height"`
	LastHash      string   `json:"last_hash"`
}

// ... imports unchanged ...

func ValidateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	rows, err := dbPool.Query(ctx, `
        SELECT block_id::text, height, prev_hash, hash, nonce, created_at, difficulty, merkle_root
        FROM blocks ORDER BY height ASC`)
	if err != nil {
		http.Error(w, "db query blocks error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var errors []string
	valid := true
	lastHash := "0"
	lastHeight := -1
	checked := 0

	for rows.Next() {
		var blockID, prevHash, hash, merkleRoot string
		var height, difficulty int
		var nonce int64
		var createdAt time.Time
		if err := rows.Scan(&blockID, &height, &prevHash, &hash, &nonce, &createdAt, &difficulty, &merkleRoot); err != nil {
			http.Error(w, "db scan block error", http.StatusInternalServerError)
			return
		}

		if checked == 0 {
			if prevHash != "0" {
				valid = false
				errors = append(errors, fmt.Sprintf("genesis block prev_hash should be '0', got '%s'", prevHash))
			}
		} else if prevHash != lastHash {
			valid = false
			errors = append(errors, fmt.Sprintf("block %d prev_hash mismatch", height))
		}

		txRows, err := dbPool.Query(ctx, `SELECT tx_id::text FROM transactions WHERE block_id=$1::uuid ORDER BY created_at ASC`, blockID)
		if err != nil {
			http.Error(w, "db query tx error", http.StatusInternalServerError)
			return
		}
		var txIDs []string
		for txRows.Next() {
			var tid string
			if err := txRows.Scan(&tid); err != nil {
				http.Error(w, "db scan tx error", http.StatusInternalServerError)
				return
			}
			txIDs = append(txIDs, tid)
		}
		txRows.Close()

		recomputedMerkle := ComputeMerkleRoot(txIDs)
		if recomputedMerkle != merkleRoot {
			valid = false
			errors = append(errors, fmt.Sprintf("block %d merkle root mismatch", height))
		}

		ts := createdAt.UTC().Format(time.RFC3339)
		header := fmt.Sprintf("%d|%s|%s|%s|%s|%d", height, prevHash, ts, merkleRoot, strings.Join(txIDs, ","), nonce)
		sum := sha256.Sum256([]byte(header))
		recomputed := hex.EncodeToString(sum[:])

		if recomputed != hash {
			valid = false
			errors = append(errors, fmt.Sprintf("block %d hash mismatch", height))
		}

		prefix := strings.Repeat("0", difficulty)
		if !strings.HasPrefix(hash, prefix) {
			valid = false
			errors = append(errors, fmt.Sprintf("block %d hash does not meet difficulty %d", height, difficulty))
		}

		lastHash = hash
		lastHeight = height
		checked++
	}

	resp := ValidateResponse{
		Valid:         valid,
		CheckedBlocks: checked,
		Errors:        errors,
		LastHeight:    lastHeight,
		LastHash:      lastHash,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
