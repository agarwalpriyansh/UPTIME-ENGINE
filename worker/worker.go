package worker

import (
	"context"
	"net"
	"net/http"
	"time"

	// Import your custom models package
	"monitor-engine/models"
)

// StartWorker pulls jobs from the channel and executes the correct protocol check
func StartWorker(id int, jobs <-chan models.MonitorJob, results chan<- models.PingResult) {
	for job := range jobs {
		
		if job.Type == "HTTP" {
			startTime := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, job.Target, nil)
			if err != nil {
				results <- models.PingResult{Job: job, Up: false, Latency: time.Since(startTime), ErrorMsg: err.Error()}
				cancel()
				continue
			}

			resp, err := http.DefaultClient.Do(req)
			latency := time.Since(startTime)

			if err != nil {
				results <- models.PingResult{Job: job, Up: false, Latency: latency, ErrorMsg: err.Error()}
				cancel()
				continue
			}

			results <- models.PingResult{Job: job, StatusCode: resp.StatusCode, Latency: latency, Up: resp.StatusCode == 200}
			resp.Body.Close()
			cancel()

		} else if job.Type == "TCP" {
			startTime := time.Now()
			conn, err := net.DialTimeout("tcp", job.Target, 5*time.Second)
			latency := time.Since(startTime)

			if err != nil {
				results <- models.PingResult{Job: job, Up: false, Latency: latency, ErrorMsg: err.Error()}
				continue
			}
			conn.Close()
			results <- models.PingResult{Job: job, Up: true, Latency: latency}
		}
	}
}