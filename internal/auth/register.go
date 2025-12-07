package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"
)

type RegisterRequest struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	CNIC     string `json:"cnic"`
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Generate 6-digit OTP
	rand.Seed(time.Now().UnixNano())
	otp := fmt.Sprintf("%06d", rand.Intn(1000000))
	expires := time.Now().Add(5 * time.Minute)

	// Insert or update user record
	_, err := dbPool.Exec(context.Background(),
		`insert into users (full_name, email, cnic, otp_code, otp_expires)
         values ($1, $2, $3, $4, $5)
         on conflict (email) do update
         set full_name=$1, cnic=$3, otp_code=$4, otp_expires=$5`,
		req.FullName, req.Email, req.CNIC, otp, expires)
	if err != nil {
		log.Printf("DB error: %v", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if err := sendEmailSMTP(req.Email, otp); err != nil {
		log.Printf("Failed to send email: %v", err)
		http.Error(w, "failed to send OTP email", http.StatusInternalServerError)
		return
	}

	// TODO: send OTP via email provider
	log.Printf("Generated OTP for %s: %s", req.Email, otp)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "OTP sent"})
}
