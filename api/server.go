package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"monitor-engine/database"
	"monitor-engine/metrics"
	"monitor-engine/models"
)

// Max JSON body size for POST /api/monitor (defense against huge uploads).
const maxMonitorJSONBody = 1 << 20 // 1 MiB

// APIServer holds our dependencies (like the jobs queue).
type APIServer struct {
	JobsQueue chan<- models.MonitorJob
}

// parseLimit parses a positive integer query param; def when empty/invalid, capped at max.
func parseLimit(s string, def, max int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

// AddMonitorHandler handles POST /api/monitor.
func (s *APIServer) AddMonitorHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var job models.MonitorJob
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxMonitorJSONBody))
	if err := dec.Decode(&job); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	if err := job.Valid(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	job.Normalize()

	inserted, err := database.AddTarget(string(job.Type), job.Target, job.OwnerEmail)
	if err != nil {
		http.Error(w, "Failed to save target to database", http.StatusInternalServerError)
		return
	}
	if !inserted {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("Target is already monitored: %s", job.Target),
		})
		return
	}

	s.JobsQueue <- job

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Successfully queued %s check for %s", job.Type, job.Target),
	})
}

// DeleteMonitorHandler handles DELETE /api/monitor?url=...
func (s *APIServer) DeleteMonitorHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, "Missing 'url' query parameter", http.StatusBadRequest)
		return
	}

	if err := database.DeleteTarget(targetURL); err != nil {
		http.Error(w, "Failed to delete target", http.StatusInternalServerError)
		return
	}
	metrics.DeleteSiteMetrics(targetURL)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Successfully deleted %s from active monitors", targetURL),
	})
}

// GetStatusHandler handles GET /api/status?limit=500
func (s *APIServer) GetStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := parseLimit(r.URL.Query().Get("limit"), 500, 2000)
	results, err := database.GetRecentResults(limit)
	if err != nil {
		http.Error(w, "Failed to fetch status from database", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if results == nil {
		results = []models.PingResult{}
	}

	_ = json.NewEncoder(w).Encode(results)
}

// GetTargetsHandler handles GET /api/targets — returns all active monitors.
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

	_ = json.NewEncoder(w).Encode(targets)
}

// GetLogsHandler handles GET /api/logs?url=...&limit=100
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

	limit := parseLimit(r.URL.Query().Get("limit"), 100, 2000)
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

	_ = json.NewEncoder(w).Encode(results)
}
