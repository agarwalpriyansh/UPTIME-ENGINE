// Create the connection logic and automatically create our database table if it doesn't exist.
package database

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq" // The underscore means we import it for its side-effects (registering the driver)
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