package notifications

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
	"time"

	"monitor-engine/metrics"
	"monitor-engine/models"
)

// EmailNotifier sends alerts via SMTP using environment configuration.
type EmailNotifier struct {
	host     string
	port     string
	user     string
	password string
	from     string
}

// NewEmailNotifier loads SMTP settings from the environment.
func NewEmailNotifier() *EmailNotifier {
	from := os.Getenv("ALERT_FROM_EMAIL")
	if from == "" {
		from = os.Getenv("SMTP_USER")
	}
	return &EmailNotifier{
		host:     getenvDefault("SMTP_HOST", "smtp.gmail.com"),
		port:     getenvDefault("SMTP_PORT", "587"),
		user:     os.Getenv("SMTP_USER"),
		password: os.Getenv("SMTP_PASS"),
		from:     from,
	}
}

func (e *EmailNotifier) SendDownAlert(site models.Site, failureCount int, firstFailedAt time.Time) error {
	subj := "DOWN: " + displayHost(site.URL) + " is not responding"
	body := fmt.Sprintf(
		"Site: %s\r\nConsecutive failures: %d\r\nFirst failure detected: %s\r\n\r\nThe monitor could not reach this target or received a non-success status.\r\n",
		safeHeader(site.URL),
		failureCount,
		firstFailedAt.Format(time.RFC1123),
	)
	return e.send(site.OwnerEmail, subj, body, "down")
}

func (e *EmailNotifier) SendRecoveryAlert(site models.Site, downtimeDuration time.Duration) error {
	subj := "RECOVERY: " + displayHost(site.URL) + " is back online"
	body := fmt.Sprintf(
		"Site: %s\r\nWas down for: %s\r\nRecovered at: %s\r\n",
		safeHeader(site.URL),
		formatDuration(downtimeDuration),
		time.Now().Format(time.RFC1123),
	)
	return e.send(site.OwnerEmail, subj, body, "recovery")
}

func (e *EmailNotifier) SendEscalationAlert(site models.Site, downtimeDuration time.Duration) error {
	subj := "ESCALATION: " + displayHost(site.URL) + " still down"
	body := fmt.Sprintf(
		"Site: %s\r\nDown for: %s (30+ minutes)\r\nImmediate attention is required.\r\n",
		safeHeader(site.URL),
		formatDuration(downtimeDuration),
	)
	return e.send(site.OwnerEmail, subj, body, "escalation")
}

func (e *EmailNotifier) send(to, subject, body, metricKind string) error {
	if e.user == "" || e.password == "" || to == "" {
		log.Printf("[ALERT] skipping email: missing SMTP credentials or recipient")
		return nil
	}
	if e.from == "" {
		return fmt.Errorf("email: ALERT_FROM_EMAIL or SMTP_USER required")
	}

	addr := e.host + ":" + e.port
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		e.from,
		safeHeader(to),
		safeHeader(subject),
		body,
	)
	auth := smtp.PlainAuth("", e.user, e.password, e.host)
	if err := smtp.SendMail(addr, auth, e.from, []string{to}, []byte(msg)); err != nil {
		metrics.RecordAlertFailed()
		return err
	}
	metrics.RecordAlertSent(metricKind)
	log.Printf("[ALERT] email sent to %s (%s)", to, subject)
	return nil
}

func safeHeader(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}
