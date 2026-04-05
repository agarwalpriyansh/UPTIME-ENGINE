package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"monitor-engine/models"
	"monitor-engine/database"
)

// APIServer holds our dependencies (like the jobs queue)
type APIServer struct {
	JobsQueue chan<- models.MonitorJob
}

// AddMonitorHandler handles POST /api/monitor
func (s *APIServer) AddMonitorHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 2. Parse the JSON body sent by the user
	var job models.MonitorJob
	err := json.NewDecoder(r.Body).Decode(&job)
	if err != nil || job.Target == "" || job.Type == "" {
		http.Error(w, "Invalid JSON payload. Require 'type' and 'target'", http.StatusBadRequest)
		return
	}

	// Normalize target and protocol
	job.Type = strings.ToUpper(job.Type)
	proto := ""
	if job.Type == "HTTPS" {
		job.Type = "HTTP"
		proto = "https://"
	} else if job.Type == "HTTP" {
		proto = "http://"
	}

	if proto != "" && !strings.HasPrefix(strings.ToLower(job.Target), "http://") && !strings.HasPrefix(strings.ToLower(job.Target), "https://") {
		job.Target = proto + job.Target
	}

	// Save it to PostgreSQL permanently!
	err = database.AddTarget(job.Type, job.Target, job.OwnerEmail)
	if err != nil {
		http.Error(w, "Failed to save target to database", http.StatusInternalServerError)
		return
	}
	// 3. Push the new job directly into the Worker Pool's queue!
	// (This is exactly how a Task Queue Broker works)
	s.JobsQueue <- job

	// 4. Respond to the user
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Successfully queued %s check for %s", job.Type, job.Target),
	})
}
// DeleteMonitorHandler handles DELETE /api/monitor?url=...
func (s *APIServer) DeleteMonitorHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract the URL from the query string (e.g., ?url=https://google.com)
	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, "Missing 'url' query parameter", http.StatusBadRequest)
		return
	}

	// Tell PostgreSQL to delete it
	err := database.DeleteTarget(targetURL)
	if err != nil {
		http.Error(w, "Failed to delete target", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Successfully deleted %s from active monitors", targetURL),
	})
}

// GetStatusHandler handles GET /api/status
func (s *APIServer) GetStatusHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 2. Fetch the 500 most recent results from PostgreSQL
	results, err := database.GetRecentResults(500)
	if err != nil {
		http.Error(w, "Failed to fetch status from database", http.StatusInternalServerError)
		return
	}

	// 3. Convert the Go slice into JSON and send it to the user!
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	// If the database is completely empty, return an empty array instead of null
	if results == nil {
		results = []models.PingResult{}
	}
	
	json.NewEncoder(w).Encode(results)
}

// GetTargetsHandler handles GET /api/targets — returns all active monitors
func (s *APIServer) GetTargetsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	targets, err := database.GetAllTargets()
	if err != nil {
		http.Error(w, "Failed to fetch targets", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if targets == nil {
		targets = []models.MonitorJob{}
	}

	json.NewEncoder(w).Encode(targets)
}

// GetLogsHandler handles GET /api/logs?url=...&limit=100 — returns per-site ping history
func (s *APIServer) GetLogsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, "Missing 'url' query parameter", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if n, err := fmt.Sscanf(limitStr, "%d", &limit); n == 0 || err != nil {
			limit = 100
		}
	}

	results, err := database.GetLogsByTarget(targetURL, limit)
	if err != nil {
		http.Error(w, "Failed to fetch logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if results == nil {
		results = []models.PingResult{}
	}

	json.NewEncoder(w).Encode(results)
}