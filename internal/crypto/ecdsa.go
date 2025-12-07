package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
)

// GenerateKeypair creates a new ECDSA P-256 keypair
func GenerateKeypair() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return priv, &priv.PublicKey, nil
}

// SerializePublicKey encodes an ECDSA public key in uncompressed hex format (65 bytes: 0x04 + X + Y)
func SerializePublicKey(pub *ecdsa.PublicKey) string {
	xb := pub.X.Bytes()
	yb := pub.Y.Bytes()

	// pad to 32 bytes for P-256
	pad := func(b []byte) []byte {
		if len(b) >= 32 {
			return b
		}
		out := make([]byte, 32)
		copy(out[32-len(b):], b)
		return out
	}
	xb = pad(xb)
	yb = pad(yb)

	raw := append([]byte{0x04}, append(xb, yb...)...)
	return hex.EncodeToString(raw)
}

// DeserializePublicKey decodes a hex-encoded uncompressed public key back into an ECDSA public key
func DeserializePublicKey(hexStr string) (*ecdsa.PublicKey, error) {
	b, err := hex.DecodeString(hexStr)
	if err != nil || len(b) != 65 || b[0] != 0x04 {
		return nil, errors.New("invalid public key encoding")
	}
	x := new(big.Int).SetBytes(b[1:33])
	y := new(big.Int).SetBytes(b[33:])
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
}

// WalletHashFromPublicKeyHex derives a wallet ID by hashing the public key with SHA-256
func WalletHashFromPublicKeyHex(pubHex string) string {
	b, _ := hex.DecodeString(pubHex)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// SignPayload signs a payload using the private key, returning r and s as hex strings
func SignPayload(priv *ecdsa.PrivateKey, payload []byte) (string, string, error) {
	h := sha256.Sum256(payload)
	r, s, err := ecdsa.Sign(rand.Reader, priv, h[:])
	if err != nil {
		return "", "", err
	}
	return r.Text(16), s.Text(16), nil
}

// VerifySignature verifies a signature (r,s hex) against a payload and public key
func VerifySignature(pub *ecdsa.PublicKey, payload []byte, rHex, sHex string) bool {
	h := sha256.Sum256(payload)
	r := new(big.Int)
	s := new(big.Int)
	_, rOk := r.SetString(rHex, 16)
	_, sOk := s.SetString(sHex, 16)
	if !rOk || !sOk {
		return false
	}
	return ecdsa.Verify(pub, h[:], r, s)
}
