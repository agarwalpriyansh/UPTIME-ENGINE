package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"monitor-engine/api"
	"monitor-engine/database"
	"monitor-engine/metrics"
	"monitor-engine/models"
	"monitor-engine/notifications"
	"monitor-engine/worker"
)

func main() {
	_ = godotenv.Load()

	if err := database.InitRedis(); err != nil {
		log.Fatalf("Redis Error: %v", err)
	}
	if err := database.InitPostgres(); err != nil {
		log.Fatalf("Postgres Error: %v", err)
	}

	go worker.StartFlusher()
	go worker.StartRetentionCleaner()
	go metrics.StartCollector()

	jobs := make(chan models.MonitorJob, 5000)
	results := make(chan models.PingResult, 5000)

	metrics.SetJobsQueueDepthFunc(func() int { return len(jobs) })
	metrics.SetResultsQueueDepthFunc(func() int { return len(results) })

	alertManager := buildAlertManager()

	numWorkers := parseWorkerCount()
	log.Printf("Starting %d workers\n", numWorkers)
	for w := 1; w <= numWorkers; w++ {
		go worker.StartWorker(w, jobs, results, alertManager)
	}

	go worker.StartTargetFeeder(jobs)

	go func() {
		for result := range results {
			if result.Timestamp.IsZero() {
				result.Timestamp = time.Now()
			}
			if err := database.SaveResult(result); err != nil {
				log.Printf("[DB ERROR] %v\n", err)
				metrics.RecordRedisPushError()
			} else {
				status := "UP"
				if !result.Up {
					status = "DOWN"
				}
				log.Printf("[PROCESSED] %s is %s (latency: %v)\n", result.Job.Target, status, result.Latency)
			}

			target := result.Job.Target
			metrics.RecordSiteCheck(target, result.Up, float64(result.Latency.Milliseconds()))
		}
	}()

	server := &api.APIServer{
		JobsQueue: jobs,
	}

	http.HandleFunc("/api/monitor", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			server.AddMonitorHandler(w, r)
		case http.MethodDelete:
			server.DeleteMonitorHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/api/status", server.GetStatusHandler)
	http.HandleFunc("/api/targets", server.GetTargetsHandler)
	http.HandleFunc("/api/logs", server.GetLogsHandler)
	http.HandleFunc("/api/incidents", server.GetIncidentsHandler)
	http.HandleFunc("/api/ssl", server.GetSSLHandler)
	http.Handle("/metrics", promhttp.Handler())

	addr := getenvDefault("LISTEN_ADDR", ":8080")
	srv := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
	}

	log.Println("==================================================")
	log.Printf("Uptime Engine API listening on %s\n", addr)
	log.Println("==================================================")

	log.Fatal(srv.ListenAndServe())
}

func parseWorkerCount() int {
	n, err := strconv.Atoi(os.Getenv("WORKER_COUNT"))
	if err != nil || n < 1 {
		return 10
	}
	if n > 256 {
		return 256
	}
	return n
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func buildAlertManager() *notifications.AlertManager {
	var channels []notifications.Notifier
	channels = append(channels, notifications.NewEmailNotifier())
	if strings.TrimSpace(os.Getenv("SLACK_WEBHOOK_URL")) != "" {
		channels = append(channels, notifications.NewSlackNotifier())
	}
	multi := notifications.NewMultiNotifier(channels...)
	cfg := notifications.LoadAlertConfig()
	log.Printf("[ALERT] cooldown=%v failures=%d escalation=%v channels=%d\n",
		cfg.Cooldown, cfg.FailureThreshold, cfg.EscalationThreshold, len(channels))
	return notifications.NewAlertManager(multi, cfg)
}
