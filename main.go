package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"net"
)

// 1. We create a Job struct so we can mix HTTP and TCP tasks in the same queue
type MonitorJob struct {
	Type   string // "HTTP" or "TCP"
	Target string // e.g., "https://dsdryfruits.in" OR "redis.mycompany.com:6379"
}

// 2. We update PingResult to include the Job details
type PingResult struct {
	Job        MonitorJob
	StatusCode int // We leave this as 0 for TCP checks
	Latency    time.Duration
	Up         bool
	ErrorMsg   string
}

// 3. The Worker now processes MonitorJobs
func worker(id int, jobs <-chan MonitorJob, results chan<- PingResult) {
	for job := range jobs {
		
		// --- IF IT IS AN HTTP JOB ---
		if job.Type == "HTTP" {
			startTime := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, job.Target, nil)
			if err != nil {
				results <- PingResult{Job: job, Up: false, Latency: time.Since(startTime), ErrorMsg: err.Error()}
				cancel()
				continue
			}

			resp, err := http.DefaultClient.Do(req)
			latency := time.Since(startTime)

			if err != nil {
				results <- PingResult{Job: job, Up: false, Latency: latency, ErrorMsg: err.Error()}
				cancel()
				continue
			}

			results <- PingResult{Job: job, StatusCode: resp.StatusCode, Latency: latency, Up: resp.StatusCode == 200}
			resp.Body.Close()
			cancel()
			
		// --- IF IT IS A TCP JOB (NEW!) ---
		} else if job.Type == "TCP" {
			startTime := time.Now()
			
			// net.DialTimeout attempts to open a raw TCP connection to an IP and Port
			conn, err := net.DialTimeout("tcp", job.Target, 5*time.Second)
			latency := time.Since(startTime)

			if err != nil {
				// The port is closed, or the server is down
				results <- PingResult{Job: job, Up: false, Latency: latency, ErrorMsg: err.Error()}
				continue
			}
			
			// If we connect successfully, we MUST close the connection immediately
			// so we don't leave ghost connections on the target server!
			conn.Close() 
			results <- PingResult{Job: job, Up: true, Latency: latency}
		}
	}
}

func main() {
	var jobsList []MonitorJob

	// 1. SYNTHETIC LOAD GENERATION
	// We are going to generate 1,000 total jobs (500 HTTP, 500 TCP)
	fmt.Println("Generating 1,000 Synthetic Jobs...")
	for i := 0; i < 500; i++ {
		jobsList = append(jobsList, MonitorJob{Type: "HTTP", Target: "https://google.com"})
		jobsList = append(jobsList, MonitorJob{Type: "TCP", Target: "8.8.8.8:53"})
	}
	numWorkers := 50
	numJobs := len(jobsList)

	jobs := make(chan MonitorJob, numJobs)
	results := make(chan PingResult, numJobs)

	fmt.Printf("Starting Stress Test with %d Workers...\n", numWorkers)
	fmt.Println("--------------------------------------------------")
	programStart := time.Now()

	// We start 3 goroutines. They will immediately pause because the 'jobs' channel is empty.
	for w := 1; w <= numWorkers; w++ {
		go worker(w, jobs, results)
	}

	// We rapidly send all jobs into the jobs channel
	for _, j := range jobsList {
		jobs <- j
	}
	
	// CRITICAL: We close the jobs channel. This tells the workers: "No more jobs are coming, 
	// when the queue is empty, you can stop running."
	close(jobs)

	

	for i := 1; i <= numJobs; i++ {
		result := <-results
		if result.Up {
			if result.Job.Type == "HTTP" {
				fmt.Printf("[HTTP SUCCESS] %s is UP (Status: %d, Latency: %v)\n", result.Job.Target, result.StatusCode, result.Latency)
			} else {
				fmt.Printf("[TCP SUCCESS]  %s port is OPEN (Latency: %v)\n", result.Job.Target, result.Latency)
			}
		} else {
			fmt.Printf("[DOWN] %s failed: %s\n", result.Job.Target, result.ErrorMsg)
		}
	}

	fmt.Println("--------------------------------------------------")
	fmt.Printf("Total execution time: %v\n", time.Since(programStart))
}