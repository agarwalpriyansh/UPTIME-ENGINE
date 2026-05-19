package worker

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"monitor-engine/database"
	"monitor-engine/metrics"
)

// retentionDays is how long ping_results are kept (default 30). Set PING_RETENTION_DAYS; use 0 to disable.
func retentionDays() int {
	s := os.Getenv("PING_RETENTION_DAYS")
	if s == "" {
		return 30
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 30
	}
	return n
}

// retentionCleanupInterval is how often the retention job runs (default 24h). Set RETENTION_CLEANUP_INTERVAL_SEC.
func retentionCleanupInterval() time.Duration {
	s := os.Getenv("RETENTION_CLEANUP_INTERVAL_SEC")
	if s == "" {
		return 24 * time.Hour
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 60 {
		return 24 * time.Hour
	}
	return time.Duration(n) * time.Second
}

func pruneOldPingsOnce(days int) {
	cutoff := time.Now().AddDate(0, 0, -days)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	deleted, err := database.DeleteOldPingResults(ctx, cutoff)
	if err != nil {
		log.Printf("[RETENTION ERROR] Failed to delete old pings: %v\n", err)
		return
	}
	metrics.RecordRetentionDeleted(deleted)
	if deleted > 0 {
		log.Printf("[RETENTION] Deleted %d ping_results older than %d days (before %s)\n",
			deleted, days, cutoff.Format(time.RFC3339))
	}
}

// StartRetentionCleaner periodically deletes ping_results older than PING_RETENTION_DAYS (default 30).
func StartRetentionCleaner() {
	days := retentionDays()
	if days <= 0 {
		log.Println("[RETENTION] Ping retention disabled (PING_RETENTION_DAYS <= 0)")
		return
	}

	log.Printf("[RETENTION] Keeping ping_results for %d days; cleanup every %v\n", days, retentionCleanupInterval())

	pruneOldPingsOnce(days)

	ticker := time.NewTicker(retentionCleanupInterval())
	defer ticker.Stop()
	for range ticker.C {
		pruneOldPingsOnce(days)
	}
}
