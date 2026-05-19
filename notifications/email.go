package notifications

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
)

// SendEmailAlert sends a plaintext alert via SMTP (defaults to Gmail on port 587).
// Configure with SMTP_USER, SMTP_PASS, and optional SMTP_HOST / SMTP_PORT.
func SendEmailAlert(targetURL string, isUp bool, toEmail string) {
	from := os.Getenv("SMTP_USER")
	password := os.Getenv("SMTP_PASS")
	if from == "" || password == "" || toEmail == "" {
		log.Println("[ALERT] skipping email: missing SMTP_USER, SMTP_PASS, or recipient")
		return
	}

	host := getenvDefault("SMTP_HOST", "smtp.gmail.com")
	port := getenvDefault("SMTP_PORT", "587")
	addr := host + ":" + port

	// Avoid header injection if target ever contained CR/LF.
	safeURL := strings.NewReplacer("\r", "", "\n", "").Replace(targetURL)

	var subj, body string
	if isUp {
		subj = "RECOVERY: " + safeURL + " is back online"
		body = fmt.Sprintf("Your monitor reports %s is responding again.\r\n", safeURL)
	} else {
		subj = "DOWN: " + safeURL + " is not responding"
		body = fmt.Sprintf(
			"Your monitor reports %s is down or not returning a successful HTTP status.\r\nPlease check the service.\r\n",
			safeURL,
		)
	}

	// RFC 5322-style message: headers, blank line, body (CRLF throughout).
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from,
		toEmail,
		subj,
		body,
	)

	auth := smtp.PlainAuth("", from, password, host)
	if err := smtp.SendMail(addr, auth, from, []string{toEmail}, []byte(msg)); err != nil {
		log.Printf("[ALERT] send failed to %s: %v", toEmail, err)
		return
	}
	log.Printf("[ALERT] email sent to %s (%s)", toEmail, subj)
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
