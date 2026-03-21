package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"monitor-engine/models"
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