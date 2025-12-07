package wallet

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

var dbPool *pgxpool.Pool

func Init(pool *pgxpool.Pool) {
	dbPool = pool
}

func CreateHandler(w http.ResponseWriter, r *http.Request) {
	// ✅ Extract claims from JWT
	claims := auth.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, _ := claims["user_id"].(string)

	// ✅ Generate RSA keypair
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		http.Error(w, "failed to generate keypair", http.StatusInternalServerError)
		return
	}

	// Public key (simplified as modulus for demo)
	pubKeyBytes := privKey.PublicKey.N.Bytes()
	pubKey := base64.StdEncoding.EncodeToString(pubKeyBytes)

	// Encrypt private key (demo: base64; in production use AES)
	privKeyEnc := base64.StdEncoding.EncodeToString(privKey.D.Bytes())

	// Wallet ID (simple derivation from public key)
	walletID := base64.StdEncoding.EncodeToString(pubKeyBytes[:16])

	// ✅ Insert into DB
	_, err = dbPool.Exec(context.Background(),
		`insert into wallets (wallet_id, user_id, public_key, private_key_enc, created_at)
         values ($1, $2, $3, $4, $5)`,
		walletID, userID, pubKey, privKeyEnc, time.Now())
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	// ✅ Respond with wallet info
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"wallet_id":  walletID,
		"public_key": pubKey,
	})
}
