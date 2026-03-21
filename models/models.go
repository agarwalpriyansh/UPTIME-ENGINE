package models

import "time"

// MonitorJob defines the type of check and the target URL/IP
type MonitorJob struct {
	Type   string  `json:"type"`// "HTTP" or "TCP"
	Target string  `json:"target"`
}

// PingResult holds the outcome of a completed MonitorJob
type PingResult struct {
	Job        MonitorJob    `json:"job"`
	StatusCode int           `json:"status_code"`
	Latency    time.Duration `json:"latency"`
	Up         bool          `json:"up"`
	ErrorMsg   string        `json:"error_msg,omitempty"` // omitempty means it won't show in JSON if it's empty
	Timestamp  time.Time     `json:"timestamp"`           // NEW: We need to know exactly when this happened!
}