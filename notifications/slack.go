package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"monitor-engine/metrics"
	"monitor-engine/models"
)

// AlertType classifies outbound Slack messages.
type AlertType string

const (
	AlertDown        AlertType = "down"
	AlertRecovery    AlertType = "recovery"
	AlertEscalation  AlertType = "escalation"
)

// Alert is the payload for SlackNotifier.Send.
type Alert struct {
	Type             AlertType
	Site             models.Site
	FailureCount     int
	FirstFailedAt    time.Time
	DowntimeDuration time.Duration
	DetectedAt       time.Time
}

// SlackNotifier posts formatted alerts to a Slack incoming webhook.
type SlackNotifier struct {
	webhookURL string
	client     *http.Client
}

// NewSlackNotifier reads SLACK_WEBHOOK_URL from the environment.
func NewSlackNotifier() *SlackNotifier {
	return &SlackNotifier{
		webhookURL: strings.TrimSpace(os.Getenv("SLACK_WEBHOOK_URL")),
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SlackNotifier) enabled() bool {
	return s.webhookURL != ""
}

// Send formats and delivers a Slack message for the given alert.
func (s *SlackNotifier) Send(alert Alert) error {
	if !s.enabled() {
		return nil
	}
	text := formatSlackMessage(alert)
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return fmt.Errorf("slack marshal: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack webhook status %d", resp.StatusCode)
	}
	metrics.RecordAlertSent(string(alert.Type))
	log.Printf("[ALERT] slack sent (%s) for %s", alert.Type, displayHost(alert.Site.URL))
	return nil
}

func (s *SlackNotifier) SendDownAlert(site models.Site, failureCount int, firstFailedAt time.Time) error {
	return s.Send(Alert{
		Type:          AlertDown,
		Site:          site,
		FailureCount:  failureCount,
		FirstFailedAt: firstFailedAt,
		DetectedAt:    time.Now(),
	})
}

func (s *SlackNotifier) SendRecoveryAlert(site models.Site, downtimeDuration time.Duration) error {
	return s.Send(Alert{
		Type:             AlertRecovery,
		Site:             site,
		DowntimeDuration: downtimeDuration,
		DetectedAt:       time.Now(),
	})
}

func (s *SlackNotifier) SendEscalationAlert(site models.Site, downtimeDuration time.Duration) error {
	return s.Send(Alert{
		Type:             AlertEscalation,
		Site:             site,
		DowntimeDuration: downtimeDuration,
		DetectedAt:       time.Now(),
	})
}

func formatSlackMessage(a Alert) string {
	host := displayHost(a.Site.URL)
	when := a.DetectedAt.Format("3:04 PM")
	if a.Type == AlertDown && !a.FirstFailedAt.IsZero() {
		when = a.FirstFailedAt.Format("3:04 PM")
	}

	switch a.Type {
	case AlertDown:
		return fmt.Sprintf("🔴 %s is DOWN — detected at %s", host, when)
	case AlertRecovery:
		return fmt.Sprintf("🟢 %s is back UP — was down for %s", host, formatDuration(a.DowntimeDuration))
	case AlertEscalation:
		return fmt.Sprintf("🚨 %s still DOWN — 30+ minutes, immediate attention needed", host)
	default:
		return fmt.Sprintf("⚠️ %s alert", host)
	}
}

func displayHost(raw string) string {
	raw = strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://")
	if i := strings.IndexByte(raw, '/'); i >= 0 {
		raw = raw[:i]
	}
	return raw
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	mins := int(d.Minutes())
	if mins < 60 {
		return fmt.Sprintf("%d minutes", mins)
	}
	h := mins / 60
	m := mins % 60
	if m == 0 {
		return fmt.Sprintf("%d hours", h)
	}
	return fmt.Sprintf("%d hours %d minutes", h, m)
}
