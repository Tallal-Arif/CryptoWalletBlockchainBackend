package auth

import (
	"fmt"
	"net/smtp"
	"os"
	"strings"
)

// sendEmailSMTP sends an OTP email using Gmail SMTP and app password
func sendEmailSMTP(to, otp string) error {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASSWORD")
	from := os.Getenv("SMTP_FROM")

	addr := fmt.Sprintf("%s:%s", host, port)

	// Email message
	msg := []byte(strings.Join([]string{
		"Subject: Your OTP Code",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=\"utf-8\"",
		"",
		fmt.Sprintf("Your OTP is: %s\nIt expires in 5 minutes.", otp),
	}, "\r\n"))

	// Authenticate and send
	auth := smtp.PlainAuth("", user, pass, host)
	return smtp.SendMail(addr, auth, from, []string{to}, msg)
}
