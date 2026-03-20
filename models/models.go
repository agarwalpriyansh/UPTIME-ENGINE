package models

import "time"

// MonitorJob defines the type of check and the target URL/IP
type MonitorJob struct {
	Type   string // "HTTP" or "TCP"
	Target string
}

// PingResult holds the outcome of a completed MonitorJob
type PingResult struct {
	Job        MonitorJob
	StatusCode int
	Latency    time.Duration
	Up         bool
	ErrorMsg   string
}