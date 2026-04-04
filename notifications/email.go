package notifications

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
)

// SendEmailAlert sends an email using Google's SMTP server
func SendEmailAlert(targetURL string, isUp bool, toEmail string) {
	
	// 1. Fetch credentials from our Environment Variables
	from := os.Getenv("SMTP_USER")     // Your Gmail address
	password := os.Getenv("SMTP_PASS") // Your Gmail App Password
	

	if from == "" || password == "" || toEmail == "" {
		log.Println("[ALERT ERROR] Missing SMTP credentials. Skipping email.")
		return
	}

	// 2. Configure the Gmail SMTP server
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"

	// 3. Build the Email Subject and Body
	var subject, body string
	if isUp {
		subject = "Subject: 🟢 RECOVERY: " + targetURL + " is BACK ONLINE\r\n"
		body = fmt.Sprintf("Good news! Your monitor for %s is responding successfully again.", targetURL)
	} else {
		subject = "Subject: 🔴 CRITICAL: " + targetURL + " is DOWN\r\n"
		body = fmt.Sprintf("Alert! Your monitor for %s has stopped responding. Please check your servers immediately.", targetURL)
	}

	// Combine them into the format SMTP requires
	message := []byte(subject + "\r\n" + body)

	// 4. Authenticate with Google
	auth := smtp.PlainAuth("", from, password, smtpHost)

	// 5. Send the Email
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{toEmail}, message)
	if err != nil {
		log.Printf("[ALERT ERROR] Failed to send email: %v\n", err)
		return
	}
	
	log.Printf("Email alert successfully sent to %s!\n", toEmail)
}