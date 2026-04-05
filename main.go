package main

import (
	"fmt"
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
	godotenv.Load()

	// 1. Boot up Infrastructure
	if err := database.InitRedis(); err != nil {
		log.Fatalf("Redis Error: %v", err)
	}
	if err := database.InitPostgres(); err != nil {
		log.Fatalf("Postgres Error: %v", err)
	}
	
	// Start the background flusher to move data from Redis -> Postgres
	go worker.StartFlusher()

	// 2. Setup the Channels
	// We use a buffer of 5000 so the API doesn't block if we get a spike in traffic
	jobs := make(chan models.MonitorJob, 5000)
	results := make(chan models.PingResult, 5000)

	// 3. Boot up the Worker Pool
	workerStr := os.Getenv("WORKER_COUNT")
	numWorkers, _ := strconv.Atoi(workerStr)
	if numWorkers == 0 {
		numWorkers = 10
	}
	
	fmt.Printf("Starting %d Workers...\n", numWorkers)
	for w := 1; w <= numWorkers; w++ {
		go worker.StartWorker(w, jobs, results)
	}
	
	// Start the Target Feeder to automatically generate jobs every 30 seconds!
	go worker.StartTargetFeeder(jobs)

	// 4. Start the Result Processor with the State Machine
	// This runs infinitely in the background, saving data to Redis
	go func() {
		// NEW: The State Machine memory map. 
		// It maps a Target URL (string) to its last known status (bool: true=UP, false=DOWN)
		lastState := make(map[string]bool)

		for result := range results {
			// 1. Existing database save logic
			result.Timestamp = time.Now()
			if err := database.SaveResult(result); err != nil {
				log.Printf("[DB ERROR] %v\n", err)
			} else {
				// Print to terminal so we can see it working!
				status := "UP"
				if !result.Up { status = "DOWN" }
				fmt.Printf("[PROCESSED] %s is %s (Latency: %v)\n", result.Job.Target, status, result.Latency)
			}

			// ---------------------------------------------------------
			// 2. Alerting Logic (The State Machine)
			// ---------------------------------------------------------
			target := result.Job.Target
			
			// Check if we have seen this target before
			prevState, exists := lastState[target]

			if !exists {
				// First time we are checking this site. Record it.
				lastState[target] = result.Up
				
				// IF the site is broken on its very first check, send an alert immediately!
				if !result.Up {
					fmt.Printf("[ALERT Triggered] %s is down on initial check!\n", target)
					go notifications.SendEmailAlert(target, result.Up, result.Job.OwnerEmail)
				}
			} else if prevState != result.Up {
				// STATE CHANGED! (e.g., UP flipped to DOWN)
				fmt.Printf("[ALERT Triggered] %s state changed from %v to %v!\n", target, prevState, result.Up)
				
				// Update our memory map with the new state
				lastState[target] = result.Up
				
				// Fire the EMAIL in the background so it doesn't block the processor!
				go notifications.SendEmailAlert(target, result.Up, result.Job.OwnerEmail)
			}
		}
	}()

	// 5. Start the REST API Server
	server := &api.APIServer{
		JobsQueue: jobs,
	}
	
	http.HandleFunc("/api/monitor", func(w http.ResponseWriter, r *http.Request) {
		// Clean switch statement to handle POST and DELETE
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
	
	// Serve frontend files from the "static" folder!
	http.Handle("/", http.FileServer(http.Dir("./static")))

	fmt.Println("==================================================")
	fmt.Println("Uptime Engine API is LIVE on http://localhost:8080")
	fmt.Println("==================================================")
	
	// ListenAndServe blocks forever, keeping the program alive
	log.Fatal(http.ListenAndServe(":8080", nil)) 
}