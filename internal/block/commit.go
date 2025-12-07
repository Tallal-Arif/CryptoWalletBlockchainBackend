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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/auth"
)

var dbPool *pgxpool.Pool

func Init(pool *pgxpool.Pool) { dbPool = pool }

type CommitRequest struct {
	MaxTx      int `json:"max_tx"`     // 0 means all pending
	Difficulty int `json:"difficulty"` // optional; default 5 (produces "00000..." prefix)
}

type CommitResponse struct {
	BlockID    string   `json:"block_id"`
	Height     int      `json:"height"`
	PrevHash   string   `json:"prev_hash"`
	Hash       string   `json:"hash"`
	Nonce      int64    `json:"nonce"`
	Difficulty int      `json:"difficulty"`
	TxIDs      []string `json:"tx_ids"`
	Count      int      `json:"count"`
	Timestamp  string   `json:"timestamp"`
}

func CommitHandler(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req CommitRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.MaxTx < 0 {
		req.MaxTx = 0
	}
	if req.Difficulty <= 0 {
		req.Difficulty = 5
	}

	ctx := context.Background()
	tx, err := dbPool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		http.Error(w, "db begin error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var prevHash string
	var latestHeight int
	err = tx.QueryRow(ctx, `SELECT hash, height FROM blocks ORDER BY height DESC LIMIT 1`).Scan(&prevHash, &latestHeight)
	if err != nil {
		prevHash = "0"
		latestHeight = -1
	}

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

	nextHeight := latestHeight + 1
	timestamp := time.Now().UTC().Format(time.RFC3339)
	merkleRoot := ComputeMerkleRoot(txIDs)

	nonce := int64(0)
	target := strings.Repeat("0", req.Difficulty)
	var finalHash string

	for {
		header := fmt.Sprintf("%d|%s|%s|%s|%s|%d", nextHeight, prevHash, timestamp, merkleRoot, strings.Join(txIDs, ","), nonce)
		sum := sha256.Sum256([]byte(header))
		hashHex := hex.EncodeToString(sum[:])
		if strings.HasPrefix(hashHex, target) {
			finalHash = hashHex
			break
		}
		nonce++
	}

	var blockID string
	err = tx.QueryRow(ctx,
		`INSERT INTO blocks (height, prev_hash, hash, nonce, difficulty, merkle_root, created_at)
         VALUES ($1,$2,$3,$4,$5,$6,NOW())
         RETURNING block_id::text`,
		nextHeight, prevHash, finalHash, nonce, req.Difficulty, merkleRoot).Scan(&blockID)
	if err != nil {
		http.Error(w, "db insert block error", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec(ctx,
		`UPDATE transactions SET status='committed', block_id=$1 WHERE tx_id = ANY($2::uuid[])`,
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
		BlockID:    blockID,
		Height:     nextHeight,
		PrevHash:   prevHash,
		Hash:       finalHash,
		Nonce:      nonce,
		Difficulty: req.Difficulty,
		TxIDs:      txIDs,
		Count:      len(txIDs),
		Timestamp:  timestamp,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
