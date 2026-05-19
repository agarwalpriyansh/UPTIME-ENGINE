package worker

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"monitor-engine/models"
)

// httpCheckClient is tuned for health checks: bounded dial/TLS/header waits and connection reuse.
var httpCheckClient = &http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:       true,
		MaxIdleConns:            100,
		IdleConnTimeout:         90 * time.Second,
		TLSHandshakeTimeout:     5 * time.Second,
		ExpectContinueTimeout:   1 * time.Second,
		ResponseHeaderTimeout:   5 * time.Second,
	},
}

// StartWorker pulls jobs from the channel and executes the correct protocol check.
func StartWorker(id int, jobs <-chan models.MonitorJob, results chan<- models.PingResult) {
	for job := range jobs {
		switch job.Type {
		case models.ProtocolHTTP:
			results <- httpCheck(id, job)
		case models.ProtocolTCP:
			results <- tcpCheck(id, job)
		default:
			results <- models.PingResult{
				Job:      job,
				Up:       false,
				ErrorMsg: fmt.Sprintf("unknown protocol: %q", job.Type),
			}
		}
	}
}

func httpCheck(id int, job models.MonitorJob) models.PingResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, job.Target, nil)
	if err != nil {
		return models.PingResult{Job: job, Up: false, Latency: time.Since(start), ErrorMsg: err.Error()}
	}

	resp, err := httpCheckClient.Do(req)
	if err != nil {
		log.Printf("[worker %d] HTTP check failed %s: %v", id, job.Target, err)
		return models.PingResult{Job: job, Up: false, Latency: time.Since(start), ErrorMsg: err.Error()}
	}

	_, copyErr := io.Copy(io.Discard, resp.Body)
	closeErr := resp.Body.Close()
	latency := time.Since(start)

	code := resp.StatusCode
	up := code >= 200 && code < 300
	var errMsg string
	if copyErr != nil {
		errMsg = copyErr.Error()
		up = false
	} else if closeErr != nil {
		errMsg = closeErr.Error()
		up = false
	}

	return models.PingResult{
		Job:        job,
		StatusCode: code,
		Latency:    latency,
		Up:         up,
		ErrorMsg:   errMsg,
	}
}

func tcpCheck(id int, job models.MonitorJob) models.PingResult {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", job.Target, 5*time.Second)
	latency := time.Since(start)
	if err != nil {
		log.Printf("[worker %d] TCP check failed %s: %v", id, job.Target, err)
		return models.PingResult{Job: job, Up: false, Latency: latency, ErrorMsg: err.Error()}
	}
	_ = conn.Close()
	return models.PingResult{Job: job, Up: true, Latency: latency}
}
