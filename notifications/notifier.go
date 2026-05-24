package notifications

import (
	"errors"
	"log"
	"time"

	"monitor-engine/models"
)

// Notifier delivers downtime notifications on a single channel (email, Slack, etc.).
type Notifier interface {
	SendDownAlert(site models.Site, failureCount int, firstFailedAt time.Time) error
	SendRecoveryAlert(site models.Site, downtimeDuration time.Duration) error
	SendEscalationAlert(site models.Site, downtimeDuration time.Duration) error
}

// MultiNotifier fans out alerts to every configured notifier.
// Individual channel failures are logged; other channels still run.
type MultiNotifier struct {
	notifiers []Notifier
}

// NewMultiNotifier wraps one or more Notifier implementations.
func NewMultiNotifier(notifiers ...Notifier) *MultiNotifier {
	var active []Notifier
	for _, n := range notifiers {
		if n != nil {
			active = append(active, n)
		}
	}
	return &MultiNotifier{notifiers: active}
}

func (m *MultiNotifier) SendDownAlert(site models.Site, failureCount int, firstFailedAt time.Time) error {
	return m.broadcast(func(n Notifier) error {
		return n.SendDownAlert(site, failureCount, firstFailedAt)
	}, "down", site.ID)
}

func (m *MultiNotifier) SendRecoveryAlert(site models.Site, downtimeDuration time.Duration) error {
	return m.broadcast(func(n Notifier) error {
		return n.SendRecoveryAlert(site, downtimeDuration)
	}, "recovery", site.ID)
}

func (m *MultiNotifier) SendEscalationAlert(site models.Site, downtimeDuration time.Duration) error {
	return m.broadcast(func(n Notifier) error {
		return n.SendEscalationAlert(site, downtimeDuration)
	}, "escalation", site.ID)
}

func (m *MultiNotifier) broadcast(fn func(Notifier) error, kind, siteID string) error {
	if len(m.notifiers) == 0 {
		log.Printf("[ALERT] no notifiers configured; skipping %s for %s", kind, siteID)
		return nil
	}
	var errs []error
	for _, n := range m.notifiers {
		if err := fn(n); err != nil {
			log.Printf("[ALERT] %s notify failed for %s: %v", kind, siteID, err)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
