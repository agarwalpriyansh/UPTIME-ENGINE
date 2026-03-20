package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"monitor-engine/database"
	"monitor-engine/models"
)

// StartFlusher runs infinitely in the background
func StartFlusher() {
	// Wake up every 10 seconds
	ticker := time.NewTicker(10 * time.Second)
	ctx := context.Background()

	for range ticker.C {
		flushToPostgres(ctx)
	}
}

func flushToPostgres(ctx context.Context) {
	// 1. Atomically pop up to 1000 items from the Redis list
	// LPopCount is perfect because it reads and deletes the data from Redis in one atomic move!
	results, err := database.RedisClient.LPopCount(ctx, "ping_buffer", 1000).Result()
	
	if err != nil && err.Error() != "redis: nil" {
		log.Printf("[FLUSHER ERROR] Failed to read from Redis: %v\n", err)
		return
	}

	if len(results) == 0 {
		return // Nothing to flush right now
	}

	fmt.Printf("[FLUSHER] Scooped %d records from Redis. Writing to PostgreSQL...\n", len(results))

	// 2. Prepare our SQL Bulk Insert statement
	// Using a transaction ensures that if one insert fails, they all roll back safely.
	tx, err := database.DB.Begin()
	if err != nil {
		log.Printf("[FLUSHER ERROR] Failed to start DB transaction: %v\n", err)
		return
	}

	stmt, err := tx.Prepare(`
		INSERT INTO ping_results (target_url, protocol, status_code, latency_ms, is_up, error_msg, checked_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`)
	if err != nil {
		log.Printf("[FLUSHER ERROR] Failed to prepare SQL: %v\n", err)
		return
	}
	defer stmt.Close()

	// 3. Loop through the JSON strings we got from Redis, parse them, and insert them
	for _, jsonStr := range results {
		var ping models.PingResult
		err := json.Unmarshal([]byte(jsonStr), &ping)
		if err != nil {
			log.Printf("[FLUSHER ERROR] Failed to parse JSON: %v\n", err)
			continue
		}

		_, err = stmt.Exec(
			ping.Job.Target, 
			ping.Job.Type, 
			ping.StatusCode, 
			ping.Latency.Milliseconds(), 
			ping.Up, 
			ping.ErrorMsg, 
			ping.Timestamp,
		)
		if err != nil {
			log.Printf("[FLUSHER ERROR] Failed to execute insert: %v\n", err)
		}
	}

	// 4. Commit the transaction to save the data permanently!
	err = tx.Commit()
	if err != nil {
		log.Printf("[FLUSHER ERROR] Failed to commit transaction: %v\n", err)
	} else {
		fmt.Printf("[FLUSHER] Successfully saved %d records to disk.\n", len(results))
	}
}