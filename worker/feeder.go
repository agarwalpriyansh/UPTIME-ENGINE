package worker

import (
	"fmt"
	"log"
	"time"

	"monitor-engine/database"
	"monitor-engine/models"
)

// StartTargetFeeder runs infinitely, feeding URLs to the workers
func StartTargetFeeder(jobsQueue chan<- models.MonitorJob) {
	// Wake up every 30 seconds
	ticker := time.NewTicker(30 * time.Second)

	for range ticker.C {
		// 1. Get all active targets from PostgreSQL
		targets, err := database.GetAllTargets()
		if err != nil {
			log.Printf("[FEEDER ERROR] Failed to fetch targets: %v\n", err)
			continue
		}

		// 2. If the database is empty, just wait for the next tick
		if len(targets) == 0 {
			continue
		}

		fmt.Printf("[FEEDER] Waking up! Feeding %d targets to the worker pool...\n", len(targets))

		// 3. Push every target into the worker queue!
		for _, target := range targets {
			jobsQueue <- target
		}
	}
}