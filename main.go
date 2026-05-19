package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"monitor-engine/api"
	"monitor-engine/database"
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

	jobs := make(chan models.MonitorJob, 5000)
	results := make(chan models.PingResult, 5000)

	numWorkers := parseWorkerCount()
	log.Printf("Starting %d workers\n", numWorkers)
	for w := 1; w <= numWorkers; w++ {
		go worker.StartWorker(w, jobs, results)
	}

	go worker.StartTargetFeeder(jobs)

	go func() {
		lastState := make(map[string]bool)

		for result := range results {
			if result.Timestamp.IsZero() {
				result.Timestamp = time.Now()
			}
			if err := database.SaveResult(result); err != nil {
				log.Printf("[DB ERROR] %v\n", err)
			} else {
				status := "UP"
				if !result.Up {
					status = "DOWN"
				}
				log.Printf("[PROCESSED] %s is %s (latency: %v)\n", result.Job.Target, status, result.Latency)
			}

			target := result.Job.Target
			prevState, exists := lastState[target]

			if !exists {
				lastState[target] = result.Up
				if !result.Up {
					log.Printf("[ALERT] %s down on initial check\n", target)
					go notifications.SendEmailAlert(target, result.Up, result.Job.OwnerEmail)
				}
			} else if prevState != result.Up {
				log.Printf("[ALERT] %s state changed: %v -> %v\n", target, prevState, result.Up)
				lastState[target] = result.Up
				go notifications.SendEmailAlert(target, result.Up, result.Job.OwnerEmail)
			}
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
