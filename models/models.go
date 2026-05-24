package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Protocol is the check type stored in the DB and passed through the worker pool.
type Protocol string

const (
	ProtocolHTTP  Protocol = "HTTP"
	ProtocolHTTPS Protocol = "HTTPS" // accepted on input; normalized to HTTP
	ProtocolTCP   Protocol = "TCP"
)

// MonitorJob defines the type of check, target URL/host, and alert recipient.
type MonitorJob struct {
	Type       Protocol `json:"type"`
	Target     string   `json:"target"`
	OwnerEmail string   `json:"owner_email"`
}

// Valid reports whether the job has required fields and a supported protocol.
func (j MonitorJob) Valid() error {
	if j.Target == "" {
		return errors.New("target is required")
	}
	if j.Type == "" {
		return errors.New("type is required")
	}
	switch Protocol(strings.ToUpper(string(j.Type))) {
	case ProtocolHTTP, ProtocolHTTPS, ProtocolTCP:
		return nil
	default:
		return fmt.Errorf("unsupported type: %s (use HTTP, HTTPS, or TCP)", j.Type)
	}
}

// Normalize uppercases the protocol, maps HTTPS → HTTP, and prefixes http(s):// when missing.
func (j *MonitorJob) Normalize() {
	j.Type = Protocol(strings.ToUpper(string(j.Type)))

	var proto string
	switch j.Type {
	case ProtocolHTTPS:
		j.Type = ProtocolHTTP
		proto = "https://"
	case ProtocolHTTP:
		proto = "http://"
	}

	if proto == "" {
		return
	}

	lower := strings.ToLower(j.Target)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		j.Target = proto + j.Target
	}
}

// PingResult holds the outcome of a completed MonitorJob.
type PingResult struct {
	Job        MonitorJob    `json:"job"`
	StatusCode int           `json:"status_code"`
	Latency    time.Duration `json:"-"` // internal; API exposes latency_ms
	Up         bool          `json:"up"`
	ErrorMsg   string        `json:"error_msg,omitempty"`
	Timestamp  time.Time     `json:"timestamp"`
}

// MarshalJSON exposes latency as milliseconds for clients (not nanoseconds).
func (p PingResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Job        MonitorJob `json:"job"`
		StatusCode int        `json:"status_code"`
		LatencyMs  int64      `json:"latency_ms"`
		Up         bool       `json:"up"`
		ErrorMsg   string     `json:"error_msg,omitempty"`
		Timestamp  time.Time  `json:"timestamp"`
	}{
		Job:        p.Job,
		StatusCode: p.StatusCode,
		LatencyMs:  p.Latency.Milliseconds(),
		Up:         p.Up,
		ErrorMsg:   p.ErrorMsg,
		Timestamp:  p.Timestamp,
	})
}

// UnmarshalJSON supports latency_ms and legacy nanosecond "latency" from older Redis entries.
func (p *PingResult) UnmarshalJSON(data []byte) error {
	var wire struct {
		Job        MonitorJob `json:"job"`
		StatusCode int        `json:"status_code"`
		Latency    int64      `json:"latency"`
		LatencyMs  int64      `json:"latency_ms"`
		Up         bool       `json:"up"`
		ErrorMsg   string     `json:"error_msg"`
		Timestamp  time.Time  `json:"timestamp"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	p.Job = wire.Job
	p.StatusCode = wire.StatusCode
	p.Up = wire.Up
	p.ErrorMsg = wire.ErrorMsg
	p.Timestamp = wire.Timestamp
	switch {
	case wire.LatencyMs > 0:
		p.Latency = time.Duration(wire.LatencyMs) * time.Millisecond
	case wire.Latency > 0:
		p.Latency = time.Duration(wire.Latency)
	}
	return nil
}

// Site is a monitored target used by the alerting system.
type Site struct {
	ID         string // stable identifier (target URL)
	URL        string
	OwnerEmail string
}

// SiteFromJob builds a Site from a completed monitor job.
func SiteFromJob(job MonitorJob) Site {
	return Site{
		ID:         job.Target,
		URL:        job.Target,
		OwnerEmail: job.OwnerEmail,
	}
}

// IncidentStatus is the lifecycle state of a downtime incident.
type IncidentStatus string

const (
	IncidentOpen     IncidentStatus = "open"
	IncidentResolved IncidentStatus = "resolved"
)

// Incident records one downtime period for a site in PostgreSQL.
type Incident struct {
	ID                   int64
	SiteID               string
	StartedAt            time.Time
	ResolvedAt           *time.Time
	AlertSentAt          *time.Time
	LastEscalationAt     *time.Time
	ConsecutiveFailures  int
	Status               IncidentStatus
}

// AlertState tracks per-site failure streaks and cooldowns (Redis + memory fallback).
type AlertState struct {
	SiteID              string
	LastAlertSentAt     time.Time
	ConsecutiveFailures int
	IsDown              bool
}
