package zakat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var dbPool *pgxpool.Pool
var zakatWalletID string // system zakat wallet

func Init(pool *pgxpool.Pool, zakatWallet string) {
	dbPool = pool
	zakatWalletID = zakatWallet
	go startScheduler()
}

func startScheduler() {
	// Run once a month (every 30 days for demo)
	ticker := time.NewTicker(30 * 24 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		runZakatDeduction()
	}
}

func runZakatDeduction() {
	ctx := context.Background()

	// Collect zakat transactions
	var txIDs []string
	rows, err := dbPool.Query(ctx, `SELECT wallet_id FROM wallets`)
	if err != nil {
		fmt.Println("zakat query wallets error:", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var wid string
		if err := rows.Scan(&wid); err != nil {
			continue
		}

		var balance int64
		err = dbPool.QueryRow(ctx,
			`SELECT COALESCE(SUM(amount),0) FROM utxos WHERE wallet_id=$1 AND spent=false`,
			wid).Scan(&balance)
		if err != nil || balance <= 0 {
			continue
		}

		zakatAmt := balance * 25 / 1000 // 2.5%
		if zakatAmt == 0 {
			continue
		}

		var txID string
		err = dbPool.QueryRow(ctx,
			`INSERT INTO transactions (from_wallet_id, to_wallet_id, amount, fee, nonce, note, timestamp, status)
             VALUES ($1,$2,$3,0,$4,$5,NOW(),'pending')
             RETURNING tx_id::text`,
			wid, zakatWalletID, zakatAmt, fmt.Sprintf("zakat-%d", time.Now().Unix()), "zakat deduction").
			Scan(&txID)
		if err != nil {
			fmt.Println("zakat insert tx error:", err)
			continue
		}
		txIDs = append(txIDs, txID)

		// Log event
		_, _ = dbPool.Exec(ctx,
			`INSERT INTO system_logs (id, type, message, metadata, timestamp)
             VALUES (gen_random_uuid(),'zakat',$1,$2,NOW())`,
			fmt.Sprintf("Zakat deducted from wallet %s", wid),
			fmt.Sprintf(`{"tx_id":"%s","amount":%d}`, txID, zakatAmt))
	}

	if len(txIDs) == 0 {
		fmt.Println("No zakat transactions created")
		return
	}

	// Mine block immediately
	mineBlock(ctx, txIDs)
}

func mineBlock(ctx context.Context, txIDs []string) {
	var prevHash string
	var latestHeight int
	err := dbPool.QueryRow(ctx, `SELECT hash, height FROM blocks ORDER BY height DESC LIMIT 1`).Scan(&prevHash, &latestHeight)
	if err != nil {
		prevHash = "0"
		latestHeight = -1
	}

	nextHeight := latestHeight + 1
	timestamp := time.Now().UTC().Format(time.RFC3339)
	joined := strings.Join(txIDs, ",")
	nonce := int64(0)
	difficulty := 5
	target := strings.Repeat("0", difficulty)
	var finalHash string

	for {
		header := fmt.Sprintf("%d|%s|%s|%s|%d", nextHeight, prevHash, timestamp, joined, nonce)
		sum := sha256.Sum256([]byte(header))
		hashHex := hex.EncodeToString(sum[:])
		if strings.HasPrefix(hashHex, target) {
			finalHash = hashHex
			break
		}
		nonce++
	}

	var blockID string
	err = dbPool.QueryRow(ctx,
		`INSERT INTO blocks (height, prev_hash, hash, nonce, difficulty, created_at)
         VALUES ($1,$2,$3,$4,$5,NOW())
         RETURNING block_id::text`,
		nextHeight, prevHash, finalHash, nonce, difficulty).Scan(&blockID)
	if err != nil {
		fmt.Println("zakat insert block error:", err)
		return
	}

	_, err = dbPool.Exec(ctx,
		`UPDATE transactions SET status='committed', block_id=$1 WHERE tx_id = ANY($2::uuid[])`,
		blockID, txIDs)
	if err != nil {
		fmt.Println("zakat update tx error:", err)
		return
	}

	_, _ = dbPool.Exec(ctx,
		`INSERT INTO system_logs (id, type, message, metadata, timestamp)
         VALUES (gen_random_uuid(),'block',$1,$2,NOW())`,
		fmt.Sprintf("Zakat block mined at height %d", nextHeight),
		fmt.Sprintf(`{"block_id":"%s","hash":"%s","tx_count":%d}`, blockID, finalHash, len(txIDs)))
}
