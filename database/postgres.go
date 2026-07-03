// Package database opens PostgreSQL, creates tables, and exposes query helpers.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"monitor-engine/models"

	_ "github.com/lib/pq"
)

var DB *sql.DB

const (
	maxOpenConns    = 25 // Maximum number of open connections to the database.If Worker26 needs the database, it waits until a connection becomes free.
	maxIdleConns    = 5 // Instead of closing every connection immediately, Go keeps 5 idle connections ready.
	connMaxLifetime = 5 * time.Minute
	initPingTimeout = 5 * time.Second
)

// InitPostgres connects to Postgres, configures the pool, creates tables if needed,
// and best-effort widens legacy VARCHAR target_url columns to TEXT.
func InitPostgres() error {
	connStr := os.Getenv("POSTGRES_DSN")
	if connStr == "" {
		connStr = "postgres://admin:password@localhost:5432/monitor_db?sslmode=disable"
	}

	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open postgres: %w", err)
	}

	DB.SetMaxOpenConns(maxOpenConns)
	DB.SetMaxIdleConns(maxIdleConns)
	DB.SetConnMaxLifetime(connMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), initPingTimeout)
	defer cancel()
	if err = DB.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping postgres: %w", err)
	}

	log.Println("[DATABASES] Successfully connected to PostgreSQL!")

	createTableQuery := `
	CREATE TABLE IF NOT EXISTS ping_results (
		id SERIAL PRIMARY KEY,
		target_url TEXT NOT NULL,
		protocol VARCHAR(10) NOT NULL,
		status_code INT,
		latency_ms BIGINT,
		is_up BOOLEAN,
		error_msg TEXT,
		checked_at TIMESTAMP NOT NULL
	);`

	if _, err = DB.Exec(createTableQuery); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	createMonitorsTable := `
	CREATE TABLE IF NOT EXISTS active_monitors (
		id SERIAL PRIMARY KEY,
		protocol VARCHAR(10) NOT NULL,
		target_url TEXT UNIQUE NOT NULL,
		owner_email VARCHAR(255) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err = DB.Exec(createMonitorsTable); err != nil {
		return fmt.Errorf("failed to create active_monitors table: %w", err)
	}

	// Existing deployments may still have VARCHAR(255); widening is safe in Postgres.
	if _, err = DB.Exec(`ALTER TABLE ping_results ALTER COLUMN target_url TYPE TEXT`); err != nil {
		log.Printf("[DATABASES] optional migrate ping_results.target_url: %v", err)
	}
	if _, err = DB.Exec(`ALTER TABLE active_monitors ALTER COLUMN target_url TYPE TEXT`); err != nil {
		log.Printf("[DATABASES] optional migrate active_monitors.target_url: %v", err)
	}

	if _, err = DB.Exec(`CREATE INDEX IF NOT EXISTS idx_ping_results_checked_at ON ping_results (checked_at)`); err != nil {
		log.Printf("[DATABASES] optional index ping_results.checked_at: %v", err)
	}

	createIncidentsTable := `
	CREATE TABLE IF NOT EXISTS incidents (
		id SERIAL PRIMARY KEY,
		site_id TEXT NOT NULL,
		started_at TIMESTAMP NOT NULL,
		resolved_at TIMESTAMP NULL,
		alert_sent_at TIMESTAMP NULL,
		last_escalation_at TIMESTAMP NULL,
		consecutive_failures INT NOT NULL DEFAULT 0,
		status VARCHAR(20) NOT NULL DEFAULT 'open'
	);`
	if _, err = DB.Exec(createIncidentsTable); err != nil {
		return fmt.Errorf("failed to create incidents table: %w", err)
	}
	if _, err = DB.Exec(`CREATE INDEX IF NOT EXISTS idx_incidents_site_status ON incidents (site_id, status)`); err != nil {
		log.Printf("[DATABASES] optional index incidents.site_status: %v", err)
	}

	return nil
}

// DeleteOldPingResults removes rows in ping_results with checked_at before cutoff.
func DeleteOldPingResults(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := DB.ExecContext(ctx, `DELETE FROM ping_results WHERE checked_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old ping results: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return n, nil
}

func scanPingResultRow(rows *sql.Rows) (models.PingResult, error) {
	var target, protocol, errMsg string
	var statusCode int
	var latencyMs int64
	var isUp bool
	var checkedAt time.Time
	var owner sql.NullString

	if err := rows.Scan(&target, &protocol, &statusCode, &latencyMs, &isUp, &errMsg, &checkedAt, &owner); err != nil {
		return models.PingResult{}, err
	}

	return models.PingResult{
		Job: models.MonitorJob{
			Type:       models.Protocol(protocol),
			Target:     target,
			OwnerEmail: owner.String,
		},
		StatusCode: statusCode,
		Latency:    time.Duration(latencyMs) * time.Millisecond,
		Up:         isUp,
		ErrorMsg:   errMsg,
		Timestamp:  checkedAt,
	}, nil
}

// GetRecentResults fetches the latest 'limit' number of ping results from the database.
func GetRecentResults(limit int) ([]models.PingResult, error) {
	query := `
		SELECT pr.target_url, pr.protocol, pr.status_code, pr.latency_ms, pr.is_up, pr.error_msg, pr.checked_at, am.owner_email
		FROM ping_results pr
		LEFT JOIN active_monitors am ON am.target_url = pr.target_url
		ORDER BY pr.checked_at DESC
		LIMIT $1
	`
	rows, err := DB.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to execute select query: %w", err)
	}
	defer rows.Close()

	var results []models.PingResult
	for rows.Next() {
		ping, err := scanPingResultRow(rows)
		if err != nil {
			log.Printf("[DB ERROR] Failed to scan row: %v\n", err)
			continue
		}
		results = append(results, ping)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating ping results: %w", err)
	}

	return results, nil
}

// GetAllTargets fetches all active URLs that we need to monitor.
func GetAllTargets() ([]models.MonitorJob, error) {
	query := `SELECT protocol, target_url, owner_email FROM active_monitors`
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []models.MonitorJob
	for rows.Next() {
		var job models.MonitorJob
		if err := rows.Scan(&job.Type, &job.Target, &job.OwnerEmail); err != nil {
			log.Printf("[DB ERROR] Failed to scan target row: %v\n", err)
			continue
		}
		targets = append(targets, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating targets: %w", err)
	}

	return targets, nil
}

// DeleteTarget removes a URL from our monitoring list.
func DeleteTarget(targetURL string) error {
	cleanTarget := targetURL
	cleanTargetHttp := targetURL
	cleanTargetHttps := targetURL

	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		cleanTargetHttp = "http://" + targetURL
		cleanTargetHttps = "https://" + targetURL
	} else if strings.HasPrefix(targetURL, "http://") {
		cleanTarget = strings.TrimPrefix(targetURL, "http://")
		cleanTargetHttps = "https://" + cleanTarget
	} else if strings.HasPrefix(targetURL, "https://") {
		cleanTarget = strings.TrimPrefix(targetURL, "https://")
		cleanTargetHttp = "http://" + cleanTarget
	}

	_, err := DB.Exec(`DELETE FROM ping_results WHERE target_url IN ($1, $2, $3)`, cleanTarget, cleanTargetHttp, cleanTargetHttps)
	if err != nil {
		return fmt.Errorf("failed to delete historical ping results: %w", err)
	}

	query := `DELETE FROM active_monitors WHERE target_url IN ($1, $2, $3)`
	_, err = DB.Exec(query, cleanTarget, cleanTargetHttp, cleanTargetHttps)
	return err
}

// AddTarget inserts a new URL into our monitoring list.
// inserted is false when target_url already exists (ON CONFLICT DO NOTHING).
func AddTarget(protocol, targetURL, ownerEmail string) (inserted bool, err error) {
	query := `INSERT INTO active_monitors (protocol, target_url, owner_email) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	res, err := DB.Exec(query, protocol, targetURL, ownerEmail)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// GetLogsByTarget fetches the ping history for a specific target URL.
func GetLogsByTarget(targetURL string, limit int) ([]models.PingResult, error) {
	query := `
		SELECT pr.target_url, pr.protocol, pr.status_code, pr.latency_ms, pr.is_up, pr.error_msg, pr.checked_at, am.owner_email
		FROM ping_results pr
		LEFT JOIN active_monitors am ON am.target_url = pr.target_url
		WHERE pr.target_url = $1
		ORDER BY pr.checked_at DESC
		LIMIT $2
	`
	rows, err := DB.Query(query, targetURL, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs for %s: %w", targetURL, err)
	}
	defer rows.Close()

	var results []models.PingResult
	for rows.Next() {
		ping, err := scanPingResultRow(rows)
		if err != nil {
			log.Printf("[DB ERROR] Failed to scan log row: %v\n", err)
			continue
		}
		results = append(results, ping)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating logs for %s: %w", targetURL, err)
	}

	return results, nil
}
