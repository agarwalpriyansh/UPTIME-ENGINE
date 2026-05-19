package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"monitor-engine/database"
	"monitor-engine/models"
)

// flushTickInterval is how often we try to drain Redis (default 10s). Set FLUSH_INTERVAL_SEC.
func flushTickInterval() time.Duration {
	s := os.Getenv("FLUSH_INTERVAL_SEC")
	if s == "" {
		return 10 * time.Second
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 10 * time.Second
	}
	return time.Duration(n) * time.Second
}

// requeuePingBatch pushes popped JSON strings back onto the head of ping_buffer (reverse order preserves FIFO).
func requeuePingBatch(ctx context.Context, items []string) {
	if len(items) == 0 {
		return
	}
	pipe := database.RedisClient.Pipeline()
	for i := len(items) - 1; i >= 0; i-- {
		pipe.LPush(ctx, database.PingBufferKey, items[i])
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("[FLUSHER ERROR] failed to re-queue %d pings to Redis: %v", len(items), err)
	}
}

// StartFlusher periodically moves batches from Redis into Postgres.
func StartFlusher() {
	ticker := time.NewTicker(flushTickInterval())
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		flushToPostgres(ctx)
		cancel()
	}
}

func flushToPostgres(ctx context.Context) {
	results, err := database.RedisClient.LPopCount(ctx, database.PingBufferKey, 1000).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		log.Printf("[FLUSHER ERROR] Failed to read from Redis: %v\n", err)
		return
	}
	if len(results) == 0 {
		return
	}

	log.Printf("[FLUSHER] Scooped %d records from Redis, writing to PostgreSQL...\n", len(results))

	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("[FLUSHER ERROR] Failed to start DB transaction: %v\n", err)
		requeuePingBatch(ctx, results)
		return
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ping_results (target_url, protocol, status_code, latency_ms, is_up, error_msg, checked_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`)
	if err != nil {
		log.Printf("[FLUSHER ERROR] Failed to prepare SQL: %v\n", err)
		requeuePingBatch(ctx, results)
		return
	}
	defer stmt.Close()

	for _, jsonStr := range results {
		var ping models.PingResult
		if err := json.Unmarshal([]byte(jsonStr), &ping); err != nil {
			log.Printf("[FLUSHER ERROR] Failed to parse JSON: %v\n", err)
			continue
		}

		if _, err := stmt.ExecContext(ctx,
			ping.Job.Target,
			string(ping.Job.Type),
			ping.StatusCode,
			ping.Latency.Milliseconds(),
			ping.Up,
			ping.ErrorMsg,
			ping.Timestamp,
		); err != nil {
			log.Printf("[FLUSHER ERROR] Failed to execute insert: %v\n", err)
			requeuePingBatch(ctx, results)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[FLUSHER ERROR] Failed to commit transaction: %v\n", err)
		requeuePingBatch(ctx, results)
		return
	}

	log.Printf("[FLUSHER] Successfully saved %d records.\n", len(results))
}
