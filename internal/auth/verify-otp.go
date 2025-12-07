package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type VerifyRequest struct {
	Email string `json:"email"`
	OTP   string `json:"otp"`
}

func VerifyOTPHandler(w http.ResponseWriter, r *http.Request) {
	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	var storedOTP string
	var expires time.Time
	var userID string

	err := dbPool.QueryRow(context.Background(),
		`select id, otp_code, otp_expires from users where email=$1`, req.Email).
		Scan(&userID, &storedOTP, &expires)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if storedOTP != req.OTP || time.Now().After(expires) {
		http.Error(w, "invalid or expired OTP", http.StatusUnauthorized)
		return
	}

	// Mark email_verified true
	_, err = dbPool.Exec(context.Background(),
		`update users set email_verified=true, otp_code=null, otp_expires=null where id=$1`, userID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	// Issue JWT
	secret := os.Getenv("JWT_SECRET")
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   req.Email,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secret))
	if err != nil {
		http.Error(w, "token error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": signedToken})
}
