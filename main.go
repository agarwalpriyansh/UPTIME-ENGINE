package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"monitor-engine/models"
	"monitor-engine/worker"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using default environment variables")
	}

	// 2. Read the worker count dynamically, default to 10 if not found
	workerStr := os.Getenv("WORKER_COUNT")
	numWorkers, err := strconv.Atoi(workerStr)
	if err != nil || numWorkers == 0 {
		numWorkers = 10 
	}

	fmt.Printf("Starting Engine with %d Workers (Loaded from Env)...\n", numWorkers)
	var jobsList []models.MonitorJob

	// 1. SYNTHETIC LOAD GENERATION
	fmt.Println("Generating 1,000 Synthetic Jobs...")
	for i := 0; i < 500; i++ {
		jobsList = append(jobsList, models.MonitorJob{Type: "HTTP", Target: "https://google.com"})
		jobsList = append(jobsList, models.MonitorJob{Type: "TCP", Target: "8.8.8.8:53"})
	}

	
	numJobs := len(jobsList)

	// Notice how we use models.MonitorJob now
	jobs := make(chan models.MonitorJob, numJobs)
	results := make(chan models.PingResult, numJobs)

	fmt.Printf("Starting Stress Test with %d Workers...\n", numWorkers)
	fmt.Println("--------------------------------------------------")
	programStart := time.Now()

	// 2. Boot up the Worker Pool using our new worker package
	for w := 1; w <= numWorkers; w++ {
		go worker.StartWorker(w, jobs, results)
	}

	// 3. Load the Queue
	for _, j := range jobsList {
		jobs <- j
	}
	close(jobs)

	// 4. Collect Results
	successCount := 0
	failCount := 0

	for i := 1; i <= numJobs; i++ {
		result := <-results
		if result.Up {
			successCount++
		} else {
			failCount++
		}
	}

	// 5. Print Final Analytics
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Total execution time: %v\n", time.Since(programStart))
	
	seconds := time.Since(programStart).Seconds()
	if seconds > 0 {
		fmt.Printf("Throughput: ~%d requests per second\n", int(float64(numJobs)/seconds))
	}
	
	fmt.Printf("Successful Checks: %d\n", successCount)
	fmt.Printf("Failed Checks: %d\n", failCount)
}