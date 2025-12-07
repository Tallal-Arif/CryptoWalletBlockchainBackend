package block

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/auth"
)

var dbPool *pgxpool.Pool

func Init(pool *pgxpool.Pool) { dbPool = pool }

type CommitRequest struct {
	MaxTx int `json:"max_tx"` // optional limit; 0 means all pending
}

type CommitResponse struct {
	BlockID   string   `json:"block_id"`
	Height    int      `json:"height"`
	PrevHash  string   `json:"prev_hash"`
	BlockHash string   `json:"block_hash"`
	TxIDs     []string `json:"tx_ids"`
	Count     int      `json:"count"`
}

func CommitHandler(w http.ResponseWriter, r *http.Request) {
	// Optional: require admin via claim like role=admin
	claims := auth.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	// If you have roles: role, _ := claims["role"].(string); if role != "admin" { ... }

	var req CommitRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.MaxTx < 0 {
		req.MaxTx = 0
	}

	ctx := context.Background()
	tx, err := dbPool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		http.Error(w, "db begin error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Get latest block for prev_hash and height
	var prevHash string
	var latestHeight int
	err = tx.QueryRow(ctx,
		`SELECT block_hash, height FROM blocks ORDER BY height DESC LIMIT 1`).
		Scan(&prevHash, &latestHeight)
	if err != nil {
		// No blocks yet: genesis
		prevHash = "0"
		latestHeight = -1
	}

	// Collect pending transactions
	q := `SELECT tx_id::text FROM transactions WHERE status='pending' ORDER BY created_at ASC`
	if req.MaxTx > 0 {
		q += fmt.Sprintf(" LIMIT %d", req.MaxTx)
	}
	rows, err := tx.Query(ctx, q)
	if err != nil {
		http.Error(w, "db query pending tx error", http.StatusInternalServerError)
		return
	}

	var txIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			http.Error(w, "db scan error", http.StatusInternalServerError)
			return
		}
		txIDs = append(txIDs, id)
	}
	rows.Close()

	if len(txIDs) == 0 {
		http.Error(w, "no pending transactions", http.StatusBadRequest)
		return
	}

	// Compute block hash: hash(height+1 | prev_hash | timestamp | tx_ids joined)
	nextHeight := latestHeight + 1
	header := fmt.Sprintf("%d|%s|%d|%v", nextHeight, prevHash, time.Now().Unix(), txIDs)
	digest := sha256.Sum256([]byte(header))
	blockHash := hex.EncodeToString(digest[:])

	// Insert block
	var blockID string
	err = tx.QueryRow(ctx,
		`INSERT INTO blocks (height, prev_hash, block_hash)
         VALUES ($1,$2,$3)
         RETURNING block_id::text`,
		nextHeight, prevHash, blockHash).Scan(&blockID)
	if err != nil {
		http.Error(w, "db insert block error", http.StatusInternalServerError)
		return
	}

	// Attach transactions to block and mark committed
	_, err = tx.Exec(ctx,
		`UPDATE transactions
         SET status='committed', block_id=$1
         WHERE tx_id = ANY($2::uuid[])`,
		blockID, txIDs)
	if err != nil {
		http.Error(w, "db update tx error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "db commit error", http.StatusInternalServerError)
		return
	}

	resp := CommitResponse{
		BlockID:   blockID,
		Height:    nextHeight,
		PrevHash:  prevHash,
		BlockHash: blockHash,
		TxIDs:     txIDs,
		Count:     len(txIDs),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
