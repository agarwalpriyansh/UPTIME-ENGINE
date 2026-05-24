package ssl

import (
	"crypto/tls"
	"net"
	"net/url"
	"strings"
	"time"
)

// Info holds TLS certificate details for a monitored site.
type Info struct {
	Valid          bool       `json:"valid"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	DaysRemaining  int        `json:"days_remaining"`
	Status         string     `json:"status"` // valid, warning, expired, unavailable
	Host           string     `json:"host,omitempty"`
	Error          string     `json:"error,omitempty"`
}

const warningDays = 30

// Check inspects the TLS certificate for an HTTPS target URL.
func Check(targetURL string) Info {
	u, err := url.Parse(targetURL)
	if err != nil || u.Host == "" {
		return Info{Status: "unavailable", Error: "invalid URL"}
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" {
		if scheme == "http" || scheme == "" {
			return Info{Status: "unavailable", Error: "not HTTPS"}
		}
	}

	host := u.Hostname()
	if host == "" {
		return Info{Status: "unavailable", Error: "missing host"}
	}

	addr := net.JoinHostPort(host, "443")
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return Info{
			Status: "unavailable",
			Host:   host,
			Error:  err.Error(),
		}
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return Info{Status: "unavailable", Host: host, Error: "no certificate"}
	}

	expiresAt := certs[0].NotAfter
	now := time.Now()
	days := int(expiresAt.Sub(now).Hours() / 24)

	info := Info{
		Host:          host,
		ExpiresAt:     &expiresAt,
		DaysRemaining: days,
	}

	if now.After(expiresAt) {
		info.Valid = false
		info.Status = "expired"
		return info
	}

	info.Valid = true
	if days < warningDays {
		info.Status = "warning"
	} else {
		info.Status = "valid"
	}
	return info
}
