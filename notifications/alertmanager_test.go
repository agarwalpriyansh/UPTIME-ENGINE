package notifications

import (
	"database/sql"
	"testing"
	"time"

	"monitor-engine/database"
	"monitor-engine/models"

	"github.com/DATA-DOG/go-sqlmock"
)

type MockNotifier struct {
	downAlerts       []models.Site
	recoveryAlerts   []models.Site
	escalationAlerts []models.Site
}

func (m *MockNotifier) SendDownAlert(site models.Site, failureCount int, firstFailedAt time.Time) error {
	m.downAlerts = append(m.downAlerts, site)
	return nil
}

func (m *MockNotifier) SendRecoveryAlert(site models.Site, downtimeDuration time.Duration) error {
	m.recoveryAlerts = append(m.recoveryAlerts, site)
	return nil
}

func (m *MockNotifier) SendEscalationAlert(site models.Site, downtimeDuration time.Duration) error {
	m.escalationAlerts = append(m.escalationAlerts, site)
	return nil
}

func setupAlertTestDB(t *testing.T) (sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %s", err)
	}

	oldDB := database.DB
	database.DB = db

	cleanup := func() {
		db.Close()
		database.DB = oldDB
	}

	return mock, cleanup
}

func TestAlertManager_Lifecycle(t *testing.T) {
	mock, cleanup := setupAlertTestDB(t)
	defer cleanup()

	cfg := AlertConfig{
		Cooldown:            5 * time.Minute,
		FailureThreshold:    3,
		EscalationThreshold: 30 * time.Minute,
	}

	notifier := &MockNotifier{}
	am := NewAlertManager(notifier, cfg)

	site := models.Site{
		ID:         "test-site-id",
		URL:        "https://testsite.com",
		OwnerEmail: "owner@testsite.com",
	}

	// Reset database state for test-site-id
	_ = database.SaveAlertState(models.AlertState{
		SiteID:              site.ID,
		ConsecutiveFailures: 0,
		IsDown:              false,
	})

	now := time.Now()

	// 1st Failure (should not alert)
	t.Run("FirstFailure", func(t *testing.T) {
		am.ProcessCheck(site, false, now)
		if len(notifier.downAlerts) != 0 {
			t.Errorf("expected 0 down alerts, got %d", len(notifier.downAlerts))
		}
		state := database.LoadAlertState(site.ID)
		if state.ConsecutiveFailures != 1 || !state.IsDown {
			t.Errorf("unexpected state: %+v", state)
		}
	})

	// 2nd Failure (should not alert)
	t.Run("SecondFailure", func(t *testing.T) {
		am.ProcessCheck(site, false, now.Add(1*time.Minute))
		if len(notifier.downAlerts) != 0 {
			t.Errorf("expected 0 down alerts, got %d", len(notifier.downAlerts))
		}
		state := database.LoadAlertState(site.ID)
		if state.ConsecutiveFailures != 2 || !state.IsDown {
			t.Errorf("unexpected state: %+v", state)
		}
	})

	// 3rd Failure (reaches threshold of 3, should alert and create incident)
	t.Run("ThirdFailure_Alerts", func(t *testing.T) {
		// Expect queries:
		// 1. ShouldAlert -> GetOpenIncident (returns no rows)
		mock.ExpectQuery("SELECT id, site_id, started_at").
			WithArgs(site.ID).
			WillReturnError(sql.ErrNoRows)

		// 2. CreateIncident -> GetOpenIncident (returns no rows)
		mock.ExpectQuery("SELECT id, site_id, started_at").
			WithArgs(site.ID).
			WillReturnError(sql.ErrNoRows)

		// 3. CreateIncident -> INSERT INTO incidents
		incidentCols := []string{"id", "site_id", "started_at", "resolved_at", "alert_sent_at", "last_escalation_at", "consecutive_failures", "status"}
		mock.ExpectQuery("INSERT INTO incidents").
			WithArgs(site.ID, now.Add(2*time.Minute), 3).
			WillReturnRows(sqlmock.NewRows(incidentCols).AddRow(int64(10), site.ID, now.Add(2*time.Minute), nil, nil, nil, 3, "open"))

		// 4. MarkIncidentAlertSent -> UPDATE incidents
		mock.ExpectExec("UPDATE incidents SET alert_sent_at").
			WithArgs(now.Add(2*time.Minute), int64(10)).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// 5. maybeEscalate -> GetOpenIncident (returns the incident we just created)
		mock.ExpectQuery("SELECT id, site_id, started_at").
			WithArgs(site.ID).
			WillReturnRows(sqlmock.NewRows(incidentCols).AddRow(int64(10), site.ID, now.Add(2*time.Minute), nil, now.Add(2*time.Minute), nil, 3, "open"))

		am.ProcessCheck(site, false, now.Add(2*time.Minute))

		if len(notifier.downAlerts) != 1 {
			t.Errorf("expected 1 down alert, got %d", len(notifier.downAlerts))
		}
		if notifier.downAlerts[0].ID != site.ID {
			t.Errorf("expected alert for site %s, got %s", site.ID, notifier.downAlerts[0].ID)
		}

		state := database.LoadAlertState(site.ID)
		if state.ConsecutiveFailures != 3 || !state.IsDown {
			t.Errorf("unexpected state: %+v", state)
		}
	})

	// 4th Failure (in cooldown, should not alert)
	t.Run("FourthFailure_Cooldown", func(t *testing.T) {
		am.ProcessCheck(site, false, now.Add(3*time.Minute))
		if len(notifier.downAlerts) != 1 {
			t.Errorf("expected down alerts count to remain 1, got %d", len(notifier.downAlerts))
		}
	})

	// 5th Failure (outside cooldown, but open incident already notified)
	t.Run("Failure_OutsideCooldown_AlreadyNotified", func(t *testing.T) {
		// Move alert state past cooldown to test
		state := database.LoadAlertState(site.ID)
		state.LastAlertSentAt = now.Add(-10 * time.Minute)
		_ = database.SaveAlertState(state)

		incidentCols := []string{"id", "site_id", "started_at", "resolved_at", "alert_sent_at", "last_escalation_at", "consecutive_failures", "status"}

		// ShouldAlert -> GetOpenIncident (returns incident with alert_sent_at set)
		mock.ExpectQuery("SELECT id, site_id, started_at").
			WithArgs(site.ID).
			WillReturnRows(sqlmock.NewRows(incidentCols).AddRow(int64(10), site.ID, now.Add(2*time.Minute), nil, now.Add(2*time.Minute), nil, 3, "open"))

		// maybeEscalate -> GetOpenIncident (returns incident, downtime is 12 mins which is < 30 mins)
		mock.ExpectQuery("SELECT id, site_id, started_at").
			WithArgs(site.ID).
			WillReturnRows(sqlmock.NewRows(incidentCols).AddRow(int64(10), site.ID, now.Add(2*time.Minute), nil, now.Add(2*time.Minute), nil, 3, "open"))

		am.ProcessCheck(site, false, now.Add(14*time.Minute))

		if len(notifier.downAlerts) != 1 {
			t.Errorf("expected down alerts count to remain 1, got %d", len(notifier.downAlerts))
		}
	})

	// Escalation (downtime > 30 minutes, should trigger escalation alert)
	t.Run("EscalationAlerts", func(t *testing.T) {
		// Reset cooldown so it isn't in cooldown
		state := database.LoadAlertState(site.ID)
		state.LastAlertSentAt = now.Add(-10 * time.Minute)
		_ = database.SaveAlertState(state)

		incidentCols := []string{"id", "site_id", "started_at", "resolved_at", "alert_sent_at", "last_escalation_at", "consecutive_failures", "status"}

		// ShouldAlert -> GetOpenIncident (returns already notified incident)
		mock.ExpectQuery("SELECT id, site_id, started_at").
			WithArgs(site.ID).
			WillReturnRows(sqlmock.NewRows(incidentCols).AddRow(int64(10), site.ID, now.Add(2*time.Minute), nil, now.Add(2*time.Minute), nil, 3, "open"))

		// maybeEscalate -> GetOpenIncident (returns incident, downtime is 40 mins which is > 30 mins)
		mock.ExpectQuery("SELECT id, site_id, started_at").
			WithArgs(site.ID).
			WillReturnRows(sqlmock.NewRows(incidentCols).AddRow(int64(10), site.ID, now.Add(2*time.Minute), nil, now.Add(2*time.Minute), nil, 3, "open"))

		// database.MarkIncidentEscalation -> UPDATE incidents
		mock.ExpectExec("UPDATE incidents SET last_escalation_at").
			WithArgs(now.Add(42*time.Minute), int64(10)).
			WillReturnResult(sqlmock.NewResult(1, 1))

		am.ProcessCheck(site, false, now.Add(42*time.Minute))

		if len(notifier.escalationAlerts) != 1 {
			t.Errorf("expected 1 escalation alert, got %d", len(notifier.escalationAlerts))
		}
	})

	// Recovery (should trigger recovery alert and resolve incident)
	t.Run("RecoveryAlerts", func(t *testing.T) {
		incidentCols := []string{"id", "site_id", "started_at", "resolved_at", "alert_sent_at", "last_escalation_at", "consecutive_failures", "status"}

		// handleRecovery -> GetOpenIncident (returns open incident)
		mock.ExpectQuery("SELECT id, site_id, started_at").
			WithArgs(site.ID).
			WillReturnRows(sqlmock.NewRows(incidentCols).AddRow(int64(10), site.ID, now.Add(2*time.Minute), nil, now.Add(2*time.Minute), nil, 3, "open"))

		// ResolveIncident -> UPDATE incidents
		mock.ExpectExec("UPDATE incidents SET resolved_at").
			WithArgs(now.Add(50*time.Minute), site.ID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		am.ProcessCheck(site, true, now.Add(50*time.Minute))

		if len(notifier.recoveryAlerts) != 1 {
			t.Errorf("expected 1 recovery alert, got %d", len(notifier.recoveryAlerts))
		}

		state := database.LoadAlertState(site.ID)
		if state.ConsecutiveFailures != 0 || state.IsDown {
			t.Errorf("expected reset alert state, got %+v", state)
		}
	})
}
