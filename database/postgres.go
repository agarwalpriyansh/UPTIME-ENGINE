// Create the connection logic and automatically create our database table if it doesn't exist.
package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq" // The underscore means we import it for its side-effects (registering the driver)
	"monitor-engine/models" // Make sure to import your models!
)

var DB *sql.DB

func InitPostgres() error {
	// 1. Define the connection string (matches our docker-compose settings)
	connStr := "postgres://admin:password@localhost:5432/monitor_db?sslmode=disable"
	
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