package metrics

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"monitor-engine/database"
)

func collectorInterval() time.Duration {
	s := os.Getenv("METRICS_COLLECT_INTERVAL_SEC")
	if s == "" {
		return 15 * time.Second
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 5 {
		return 15 * time.Second
	}
	return time.Duration(n) * time.Second
}

// StartCollector periodically updates gauges that require DB/Redis queries.
func StartCollector() {
	tick := collectorInterval()
	log.Printf("[METRICS] Collector interval %v\n", tick)

	collectOnce()

	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for range ticker.C {
		collectOnce()
	}
}

func collectOnce() {
	UpdateQueueGauges()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if database.RedisClient != nil {
		n, err := database.RedisClient.LLen(ctx, database.PingBufferKey).Result()
		if err != nil {
			log.Printf("[METRICS] redis buffer length: %v", err)
		} else {
			SetRedisBufferLength(n)
		}
	}

	targets, err := database.GetAllTargets()
	if err != nil {
		log.Printf("[METRICS] active monitors: %v", err)
		return
	}
	SetActiveMonitors(len(targets))
}
