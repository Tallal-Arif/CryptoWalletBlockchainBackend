package auth

import "github.com/jackc/pgx/v5/pgxpool"

var dbPool *pgxpool.Pool

// Init sets the pool once at startup
func Init(pool *pgxpool.Pool) {
	dbPool = pool
}
