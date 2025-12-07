package db

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ConnectDB() (*pgxpool.Pool, error) {
	dbURL := os.Getenv("DATABASE_URL")
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		return nil, err
	}

	// Test the connection
	var version string
	if err := pool.QueryRow(context.Background(), "SELECT version()").Scan(&version); err != nil {
		return nil, err
	}
	log.Println("Connected to:", version)

	return pool, nil
}
