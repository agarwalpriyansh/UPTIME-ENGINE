// Create the connection logic and automatically create our database table if it doesn't exist.
package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"
	"os"

	_ "github.com/lib/pq" // The underscore means we import it for its side-effects (registering the driver)
	"monitor-engine/models" // Make sure to import your models!
)

var DB *sql.DB

func InitPostgres() error {
	// NEW: Read from environment, fallback to localhost if not found
	connStr := os.Getenv("POSTGRES_DSN")
	if connStr == "" {
		connStr = "postgres://admin:password@localhost:5432/monitor_db?sslmode=disable"
	}
	
	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open postgres: %v", err)
	}

	// 2. Ping to ensure the connection is actually alive
	if err = DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping postgres: %v", err)
	}

	fmt.Println("[DATABASES] Successfully connected to PostgreSQL!")

	// 3. Automatically create our table if it doesn't exist
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS ping_results (
		id SERIAL PRIMARY KEY,
		target_url VARCHAR(255) NOT NULL,
		protocol VARCHAR(10) NOT NULL,
		status_code INT,
		latency_ms BIGINT,
		is_up BOOLEAN,
		error_msg TEXT,
		checked_at TIMESTAMP NOT NULL
	);`

	_, err = DB.Exec(createTableQuery)
	if err != nil {
		return fmt.Errorf("failed to create table: %v", err)
	}

	// Create the active_monitors table to store our targets permanently
	createMonitorsTable := `
	CREATE TABLE IF NOT EXISTS active_monitors (
		id SERIAL PRIMARY KEY,
		protocol VARCHAR(10) NOT NULL,
		target_url VARCHAR(255) UNIQUE NOT NULL, -- UNIQUE prevents adding the same site twice!
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	_, err = DB.Exec(createMonitorsTable)
	if err != nil {
		return fmt.Errorf("failed to create active_monitors table: %v", err)
	}

	return nil
}

// GetRecentResults fetches the latest 'limit' number of ping results from the database
func GetRecentResults(limit int) ([]models.PingResult, error) {
	// 1. Query the database, ordering by the newest checks first
	query := `
		SELECT target_url, protocol, status_code, latency_ms, is_up, error_msg, checked_at 
		FROM ping_results 
		ORDER BY checked_at DESC 
		LIMIT $1
	`
	rows, err := DB.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to execute select query: %v", err)
	}
	defer rows.Close()

	var results []models.PingResult

	// 2. Loop through the returned rows
	for rows.Next() {
		var target, protocol, errMsg string
		var statusCode int
		var latencyMs int64
		var isUp bool
		var checkedAt time.Time

		// 3. Scan the SQL columns into our Go variables
		err := rows.Scan(&target, &protocol, &statusCode, &latencyMs, &isUp, &errMsg, &checkedAt)
		if err != nil {
			log.Printf("[DB ERROR] Failed to scan row: %v\n", err)
			continue
		}

		// 4. Reconstruct our PingResult struct
		ping := models.PingResult{
			Job: models.MonitorJob{
				Type:   protocol,
				Target: target,
			},
			StatusCode: statusCode,
			Latency:    time.Duration(latencyMs) * time.Millisecond, // Convert DB integer back to Go Duration
			Up:         isUp,
			ErrorMsg:   errMsg,
			Timestamp:  checkedAt,
		}

		results = append(results, ping)
	}

	return results, nil
}

// GetAllTargets fetches all active URLs that we need to monitor
func GetAllTargets() ([]models.MonitorJob, error) {
	query := `SELECT protocol, target_url FROM active_monitors`
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []models.MonitorJob
	for rows.Next() {
		var job models.MonitorJob
		if err := rows.Scan(&job.Type, &job.Target); err != nil {
			continue
		}
		targets = append(targets, job)
	}
	return targets, nil
}

// DeleteTarget removes a URL from our monitoring list
func DeleteTarget(targetURL string) error {
	query := `DELETE FROM active_monitors WHERE target_url = $1`
	_, err := DB.Exec(query, targetURL)
	return err
}

// AddTarget inserts a new URL into our monitoring list
func AddTarget(protocol, targetURL string) error {
    query := `INSERT INTO active_monitors (protocol, target_url) VALUES ($1, $2) ON CONFLICT DO NOTHING`
    _, err := DB.Exec(query, protocol, targetURL)
    return err
}

// GetLogsByTarget fetches the ping history for a specific target URL
func GetLogsByTarget(targetURL string, limit int) ([]models.PingResult, error) {
	query := `
		SELECT target_url, protocol, status_code, latency_ms, is_up, error_msg, checked_at 
		FROM ping_results 
		WHERE target_url = $1
		ORDER BY checked_at DESC 
		LIMIT $2
	`
	rows, err := DB.Query(query, targetURL, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs for %s: %v", targetURL, err)
	}
	defer rows.Close()

	var results []models.PingResult
	for rows.Next() {
		var target, protocol, errMsg string
		var statusCode int
		var latencyMs int64
		var isUp bool
		var checkedAt time.Time

		err := rows.Scan(&target, &protocol, &statusCode, &latencyMs, &isUp, &errMsg, &checkedAt)
		if err != nil {
			log.Printf("[DB ERROR] Failed to scan log row: %v\n", err)
			continue
		}

		ping := models.PingResult{
			Job: models.MonitorJob{
				Type:   protocol,
				Target: target,
			},
			StatusCode: statusCode,
			Latency:    time.Duration(latencyMs) * time.Millisecond,
			Up:         isUp,
			ErrorMsg:   errMsg,
			Timestamp:  checkedAt,
		}
		results = append(results, ping)
	}
	return results, nil
}