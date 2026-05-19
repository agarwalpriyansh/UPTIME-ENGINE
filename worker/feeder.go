package worker

import (
	"log"
	"os"
	"strconv"
	"time"

	"monitor-engine/database"
	"monitor-engine/models"
)

// feedInterval returns how often the feeder re-queues all monitors (default 30s).
// Set FEED_INTERVAL_SEC in the environment (seconds, minimum 1).
func feedInterval() time.Duration {
	s := os.Getenv("FEED_INTERVAL_SEC")
	if s == "" {
		return 30 * time.Second
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 30 * time.Second
	}
	return time.Duration(n) * time.Second
}

// feedTargetsOnce loads active monitors from Postgres and enqueues one job per target.
func feedTargetsOnce(jobsQueue chan<- models.MonitorJob) {
	targets, err := database.GetAllTargets()
	if err != nil {
		log.Printf("[FEEDER ERROR] Failed to fetch targets: %v\n", err)
		return
	}
	if len(targets) == 0 {
		return
	}

	log.Printf("[FEEDER] Feeding %d targets to the worker pool\n", len(targets))
	for _, target := range targets {
		jobsQueue <- target
	}
}

// StartTargetFeeder runs forever: feeds all targets once at startup, then on every tick.
func StartTargetFeeder(jobsQueue chan<- models.MonitorJob) {
	ticker := time.NewTicker(feedInterval())
	defer ticker.Stop()

	feedTargetsOnce(jobsQueue)

	for range ticker.C {
		feedTargetsOnce(jobsQueue)
	}
}
