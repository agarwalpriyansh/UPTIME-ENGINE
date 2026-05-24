package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"monitor-engine/database"
	"monitor-engine/models"
)

// incidentResponse is the JSON shape returned by GET /api/incidents.
type incidentResponse struct {
	ID           int64   `json:"id"`
	IncidentNum  int64   `json:"incident_number"`
	StartedAt    string  `json:"started_at"`
	ResolvedAt   *string `json:"resolved_at"`
	Duration     string  `json:"duration"`
	DurationSecs int64   `json:"duration_seconds"`
	Status       string  `json:"status"` // Active or Resolved
}

func formatIncidentDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalMins := int(d.Minutes())
	h := totalMins / 60
	m := totalMins % 60
	if h > 0 {
		if m > 0 {
			return fmt.Sprintf("%dh %dm", h, m)
		}
		return fmt.Sprintf("%dh", h)
	}
	if m > 0 {
		return fmt.Sprintf("%dm", m)
	}
	secs := int(d.Seconds())
	if secs <= 0 {
		return "0s"
	}
	return fmt.Sprintf("%ds", secs)
}

func incidentToResponse(inc models.Incident, now time.Time) incidentResponse {
	status := "Resolved"
	var end time.Time
	if inc.Status == models.IncidentOpen {
		status = "Active"
		end = now
	} else if inc.ResolvedAt != nil {
		end = *inc.ResolvedAt
	} else {
		end = now
	}

	dur := end.Sub(inc.StartedAt)
	resp := incidentResponse{
		ID:           inc.ID,
		IncidentNum:  inc.ID,
		StartedAt:    inc.StartedAt.UTC().Format(time.RFC3339),
		Duration:     formatIncidentDuration(dur),
		DurationSecs: int64(dur.Seconds()),
		Status:       status,
	}
	if inc.ResolvedAt != nil {
		s := inc.ResolvedAt.UTC().Format(time.RFC3339)
		resp.ResolvedAt = &s
	}
	return resp
}

// GetIncidentsHandler handles GET /api/incidents?siteId=...
func (s *APIServer) GetIncidentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	siteID := r.URL.Query().Get("siteId")
	if siteID == "" {
		http.Error(w, "Missing 'siteId' query parameter", http.StatusBadRequest)
		return
	}

	limit := parseLimit(r.URL.Query().Get("limit"), 50, 200)
	incidents, err := database.GetIncidentHistory(siteID, limit)
	if err != nil {
		http.Error(w, "Failed to fetch incidents", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	out := make([]incidentResponse, 0, len(incidents))
	for _, inc := range incidents {
		out = append(out, incidentToResponse(inc, now))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if out == nil {
		out = []incidentResponse{}
	}
	_ = json.NewEncoder(w).Encode(out)
}
