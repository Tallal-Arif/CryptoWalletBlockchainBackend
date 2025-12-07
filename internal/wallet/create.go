package wallet

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/auth"
	"github.com/Tallal-Arif/CryptoWalletBlockchainBackend/internal/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
)

var dbPool *pgxpool.Pool

func Init(pool *pgxpool.Pool) {
	dbPool = pool
}

func CreateHandler(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, _ := claims["user_id"].(string)

	// ✅ Generate ECDSA keypair
	priv, pub, err := crypto.GenerateKeypair()
	if err != nil {
		http.Error(w, "failed to generate keypair", http.StatusInternalServerError)
		return
	}

	// ✅ Serialize public key
	pubHex := crypto.SerializePublicKey(pub)

	// ✅ Wallet ID = SHA-256 hash of public key
	walletID := crypto.WalletHashFromPublicKeyHex(pubHex)

	// ✅ Serialize private key (D scalar, padded to 32 bytes)
	dBytes := priv.D.Bytes()
	if len(dBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(dBytes):], dBytes)
		dBytes = padded
	}

	// ✅ Encrypt private key with AES-GCM
	// For demo: derive key from userID (replace with PBKDF2/Argon2 in production)
	aesKey := make([]byte, 32)
	copy(aesKey, []byte(userID))
	encPriv, err := crypto.EncryptBytesAESGCM(aesKey, dBytes)
	if err != nil {
		http.Error(w, "encryption error", http.StatusInternalServerError)
		return
	}

	// ✅ Insert into DB
	_, err = dbPool.Exec(context.Background(),
		`INSERT INTO wallets (wallet_id, user_id, public_key, private_key_enc, key_type, wallet_hash, created_at)
         VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		walletID, userID, pubHex, encPriv, "ECDSA_P256", walletID, time.Now())
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	// ✅ Respond with wallet info
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"wallet_id":  walletID,
		"public_key": pubHex,
		"key_type":   "ECDSA_P256",
	})
}
