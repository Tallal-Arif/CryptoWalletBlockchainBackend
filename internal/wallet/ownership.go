package wallet

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureWalletOwnedByUser(pool *pgxpool.Pool, walletID string, userID string) error {
	var owner string
	err := pool.QueryRow(context.Background(),
		`SELECT user_id FROM wallets WHERE wallet_id=$1`, walletID).Scan(&owner)
	if err != nil {
		return errors.New("wallet not found")
	}
	if owner != userID {
		return errors.New("forbidden: wallet ownership mismatch")
	}
	return nil
}
