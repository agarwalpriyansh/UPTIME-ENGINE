package notifications

import (
	"log"
	"os"
	"strconv"
	"time"

	"monitor-engine/database"
	"monitor-engine/metrics"
	"monitor-engine/models"
)

// AlertConfig holds alerting thresholds loaded from the environment.
type AlertConfig struct {
	Cooldown             time.Duration
	FailureThreshold     int
	EscalationThreshold  time.Duration
}

// LoadAlertConfig reads alerting tuning from environment variables.
func LoadAlertConfig() AlertConfig {
	cooldownMin := envInt("ALERT_COOLDOWN_MINUTES", 10)
	failures := envInt("CONSECUTIVE_FAILURES_THRESHOLD", 3)
	escalationMin := envInt("ESCALATION_THRESHOLD_MINUTES", 30)
	return AlertConfig{
		Cooldown:            time.Duration(cooldownMin) * time.Minute,
		FailureThreshold:    failures,
		EscalationThreshold: time.Duration(escalationMin) * time.Minute,
	}
}

// AlertManager applies production alerting rules and dispatches via Notifier.
//
// Rules:
//   - Down alert only after N consecutive failures (default 3)
//   - Cooldown between alerts for the same site (default 10 minutes)
//   - Recovery alert when a site comes back up after being down
//   - Escalation if still down past escalation threshold (default 30 minutes)
//   - No duplicate down alerts for the same open incident
type AlertManager struct {
	notifier Notifier
	cfg      AlertConfig
}

// NewAlertManager wires a notifier (typically MultiNotifier) with config.
func NewAlertManager(notifier Notifier, cfg AlertConfig) *AlertManager {
	return &AlertManager{notifier: notifier, cfg: cfg}
}

// ShouldAlert reports whether a new DOWN alert may be sent for siteID.
// currentStatus false means the latest check failed.
func (am *AlertManager) ShouldAlert(siteID string, currentStatus bool) bool {
	if currentStatus {
		return false
	}

	state := database.LoadAlertState(siteID)
	if state.ConsecutiveFailures < am.cfg.FailureThreshold {
		return false
	}
	if am.inCooldown(state) {
		return false
	}

	inc, err := database.GetOpenIncident(siteID)
	if err != nil {
		log.Printf("[ALERT] ShouldAlert incident lookup %s: %v", siteID, err)
		return false
	}
	// Never duplicate down alert for an open incident that was already notified.
	if inc != nil && inc.AlertSentAt != nil {
		return false
	}
	return true
}

// ProcessCheck runs the full alerting state machine for one health-check result.
func (am *AlertManager) ProcessCheck(site models.Site, up bool, checkedAt time.Time) {
	siteID := site.ID
	state := database.LoadAlertState(siteID)

	if up {
		am.handleRecovery(site, state, checkedAt)
		return
	}
	am.handleFailure(site, state, checkedAt)
}

func (am *AlertManager) handleRecovery(site models.Site, state models.AlertState, at time.Time) {
	siteID := site.ID
	wasDown := state.IsDown

	state.ConsecutiveFailures = 0
	state.IsDown = false
	if err := database.SaveAlertState(state); err != nil {
		log.Printf("[ALERT] save state recovery %s: %v", siteID, err)
	}

	if !wasDown {
		return
	}

	inc, err := database.GetOpenIncident(siteID)
	if err != nil {
		log.Printf("[ALERT] recovery incident lookup %s: %v", siteID, err)
		return
	}
	if inc == nil {
		return
	}

	downtime := at.Sub(inc.StartedAt)
	if err := am.SendRecoveryAlert(site, downtime); err != nil {
		log.Printf("[ALERT] recovery send %s: %v", siteID, err)
	} else {
		database.SetAlertCooldown(siteID, at)
	}
	if err := database.ResolveIncident(siteID, at); err != nil {
		log.Printf("[ALERT] resolve incident %s: %v", siteID, err)
	}
}

func (am *AlertManager) handleFailure(site models.Site, state models.AlertState, at time.Time) {
	siteID := site.ID

	state.ConsecutiveFailures++
	state.IsDown = true
	if err := database.SaveAlertState(state); err != nil {
		log.Printf("[ALERT] save state failure %s: %v", siteID, err)
	}

	if state.ConsecutiveFailures < am.cfg.FailureThreshold {
		return
	}

	if am.ShouldAlert(siteID, false) {
		inc, err := database.CreateIncident(siteID, at, state.ConsecutiveFailures)
		if err != nil {
			log.Printf("[ALERT] create incident %s: %v", siteID, err)
			return
		}
		if err := am.SendDownAlert(site, state.ConsecutiveFailures, inc.StartedAt); err != nil {
			log.Printf("[ALERT] down send %s: %v", siteID, err)
			return
		}
		if err := database.MarkIncidentAlertSent(inc.ID, at); err != nil {
			log.Printf("[ALERT] mark alert sent %s: %v", siteID, err)
		}
		database.SetAlertCooldown(siteID, at)
		metrics.RecordIncident(siteID)
	}

	am.maybeEscalate(site, at)
}

func (am *AlertManager) maybeEscalate(site models.Site, at time.Time) {
	siteID := site.ID
	inc, err := database.GetOpenIncident(siteID)
	if err != nil || inc == nil || inc.AlertSentAt == nil {
		return
	}

	downtime := at.Sub(inc.StartedAt)
	if downtime < am.cfg.EscalationThreshold {
		return
	}

	state := database.LoadAlertState(siteID)
	if am.inCooldown(state) {
		return
	}
	if inc.LastEscalationAt != nil {
		return
	}

	if err := am.SendEscalationAlert(site, downtime); err != nil {
		log.Printf("[ALERT] escalation send %s: %v", siteID, err)
		return
	}
	if err := database.MarkIncidentEscalation(inc.ID, at); err != nil {
		log.Printf("[ALERT] mark escalation %s: %v", siteID, err)
	}
	database.SetAlertCooldown(siteID, at)
}

func (am *AlertManager) inCooldown(state models.AlertState) bool {
	if state.LastAlertSentAt.IsZero() {
		return false
	}
	return time.Since(state.LastAlertSentAt) < am.cfg.Cooldown
}

// SendDownAlert notifies all channels that a site is down.
func (am *AlertManager) SendDownAlert(site models.Site, failureCount int, firstFailedAt time.Time) error {
	return am.notifier.SendDownAlert(site, failureCount, firstFailedAt)
}

// SendRecoveryAlert notifies all channels that a site recovered.
func (am *AlertManager) SendRecoveryAlert(site models.Site, downtimeDuration time.Duration) error {
	return am.notifier.SendRecoveryAlert(site, downtimeDuration)
}

// SendEscalationAlert notifies all channels that a site has been down too long.
func (am *AlertManager) SendEscalationAlert(site models.Site, downtimeDuration time.Duration) error {
	return am.notifier.SendEscalationAlert(site, downtimeDuration)
}

func envInt(key string, def int) int {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}
