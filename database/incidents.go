package database

import (
	"database/sql"
	"fmt"
	"time"

	"monitor-engine/models"
)

func scanIncident(row interface {
	Scan(dest ...any) error
}) (*models.Incident, error) {
	var inc models.Incident
	var resolvedAt, alertSentAt, lastEscalationAt sql.NullTime
	var status string

	if err := row.Scan(
		&inc.ID,
		&inc.SiteID,
		&inc.StartedAt,
		&resolvedAt,
		&alertSentAt,
		&lastEscalationAt,
		&inc.ConsecutiveFailures,
		&status,
	); err != nil {
		return nil, err
	}

	if resolvedAt.Valid {
		inc.ResolvedAt = &resolvedAt.Time
	}
	if alertSentAt.Valid {
		inc.AlertSentAt = &alertSentAt.Time
	}
	if lastEscalationAt.Valid {
		inc.LastEscalationAt = &lastEscalationAt.Time
	}
	inc.Status = models.IncidentStatus(status)
	return &inc, nil
}

// CreateIncident opens a new incident row for a site (only if none is open).
func CreateIncident(siteID string, startedAt time.Time, consecutiveFailures int) (*models.Incident, error) {
	open, err := GetOpenIncident(siteID)
	if err != nil {
		return nil, err
	}
	if open != nil {
		return open, nil
	}

	query := `
		INSERT INTO incidents (site_id, started_at, consecutive_failures, status)
		VALUES ($1, $2, $3, 'open')
		RETURNING id, site_id, started_at, resolved_at, alert_sent_at, last_escalation_at, consecutive_failures, status
	`
	row := DB.QueryRow(query, siteID, startedAt, consecutiveFailures)
	inc, err := scanIncident(row)
	if err != nil {
		return nil, fmt.Errorf("create incident: %w", err)
	}
	return inc, nil
}

// MarkIncidentAlertSent records that the initial down alert was delivered.
func MarkIncidentAlertSent(incidentID int64, at time.Time) error {
	_, err := DB.Exec(
		`UPDATE incidents SET alert_sent_at = $1, consecutive_failures = GREATEST(consecutive_failures, 1) WHERE id = $2`,
		at, incidentID,
	)
	return err
}

// MarkIncidentEscalation records the last escalation notification time.
func MarkIncidentEscalation(incidentID int64, at time.Time) error {
	_, err := DB.Exec(`UPDATE incidents SET last_escalation_at = $1 WHERE id = $2`, at, incidentID)
	return err
}

// ResolveIncident closes the open incident for a site.
func ResolveIncident(siteID string, resolvedAt time.Time) error {
	_, err := DB.Exec(
		`UPDATE incidents SET resolved_at = $1, status = 'resolved' WHERE site_id = $2 AND status = 'open'`,
		resolvedAt, siteID,
	)
	return err
}

// GetOpenIncident returns the current open incident for a site, or nil.
func GetOpenIncident(siteID string) (*models.Incident, error) {
	query := `
		SELECT id, site_id, started_at, resolved_at, alert_sent_at, last_escalation_at, consecutive_failures, status
		FROM incidents
		WHERE site_id = $1 AND status = 'open'
		ORDER BY started_at DESC
		LIMIT 1
	`
	row := DB.QueryRow(query, siteID)
	inc, err := scanIncident(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get open incident: %w", err)
	}
	return inc, nil
}

// GetIncidentHistory returns recent incidents for a site.
func GetIncidentHistory(siteID string, limit int) ([]models.Incident, error) {
	if limit < 1 {
		limit = 50
	}
	query := `
		SELECT id, site_id, started_at, resolved_at, alert_sent_at, last_escalation_at, consecutive_failures, status
		FROM incidents
		WHERE site_id = $1
		ORDER BY started_at DESC
		LIMIT $2
	`
	rows, err := DB.Query(query, siteID, limit)
	if err != nil {
		return nil, fmt.Errorf("incident history: %w", err)
	}
	defer rows.Close()

	var list []models.Incident
	for rows.Next() {
		inc, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *inc)
	}
	return list, rows.Err()
}
