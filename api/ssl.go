package api

import (
	"encoding/json"
	"net/http"

	"monitor-engine/ssl"
)

// GetSSLHandler handles GET /api/ssl?siteId=...
func (s *APIServer) GetSSLHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	siteID := r.URL.Query().Get("siteId")
	if siteID == "" {
		http.Error(w, "Missing 'siteId' query parameter", http.StatusBadRequest)
		return
	}

	info := ssl.Check(siteID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(info)
}
