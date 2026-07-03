// Package metrics exposes Prometheus instruments for the uptime engine.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	checksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "uptime_checks_total",
		Help: "Health checks completed by protocol and result (up|down).",
	}, []string{"protocol", "result"})

	checkDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "uptime_check_duration_seconds",
		Help:    "Health check round-trip latency in seconds.",
		Buckets: prometheus.ExponentialBuckets(0.01, 2, 12),
	}, []string{"protocol"})

	activeMonitors = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "uptime_active_monitors",
		Help: "Number of rows in active_monitors.",
	})

	jobsQueueLength = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "uptime_jobs_queue_length",
		Help: "Pending monitor jobs waiting for workers.",
	})

	resultsQueueLength = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "uptime_results_queue_length",
		Help: "Ping results waiting for the processor goroutine.",
	})

	redisBufferLength = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "uptime_redis_buffer_length",
		Help: "Items in Redis list ping_buffer awaiting flush to Postgres.",
	})

	feederTargetsLast = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "uptime_feeder_targets_last",
		Help: "Targets enqueued on the most recent feeder tick.",
	})

	flushRowsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "uptime_flush_rows_total",
		Help: "Ping rows successfully inserted into Postgres by the flusher.",
	})

	flushErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "uptime_flush_errors_total",
		Help: "Flusher failures (Redis read, SQL, or commit).",
	})

	redisPushErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "uptime_redis_push_errors_total",
		Help: "Failures writing a check result to Redis ping_buffer.",
	})

	alertsSentTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "uptime_alerts_sent_total",
		Help: "SMTP alert emails sent successfully.",
	}, []string{"kind"})

	alertsFailedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "uptime_alerts_failed_total",
		Help: "SMTP alert send failures.",
	})

	retentionDeletedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "uptime_retention_deleted_rows_total",
		Help: "Rows removed from ping_results by the retention job.",
	})

	// Per-site metrics (used by the Grafana operations dashboard).
	siteStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "uptime_site_status",
		Help: "Current site status: 1=up, 0=down.",
	}, []string{"site"})

	responseTimeMs = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "uptime_response_time_ms",
		Help:    "Health check response time in milliseconds.",
		Buckets: []float64{10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
	}, []string{"site"})

	totalChecksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "uptime_total_checks_total",
		Help: "Total health checks completed per site.",
	}, []string{"site"})

	incidentsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "uptime_incidents_total",
		Help: "Total downtime incidents per site (state transitions to down).",
	}, []string{"site"})
)

var (
	jobsQueueDepthFn    func() int
	resultsQueueDepthFn func() int
)

// SetJobsQueueDepthFunc reports len(jobs channel) for scraping.
func SetJobsQueueDepthFunc(fn func() int) {
	jobsQueueDepthFn = fn
}

// SetResultsQueueDepthFunc reports len(results channel) for scraping.
func SetResultsQueueDepthFunc(fn func() int) {
	resultsQueueDepthFn = fn
}

// RecordSiteCheck updates per-site gauges, histogram, and check counter.
func RecordSiteCheck(site string, up bool, latencyMs float64) {
	status := 0.0
	if up {
		status = 1.0
	}
	siteStatus.WithLabelValues(site).Set(status)
	responseTimeMs.WithLabelValues(site).Observe(latencyMs)
	totalChecksTotal.WithLabelValues(site).Inc()
}

// DeleteSiteMetrics deletes the site's metrics from the Prometheus registry vectors.
func DeleteSiteMetrics(site string) {
	siteStatus.DeleteLabelValues(site)
	responseTimeMs.DeleteLabelValues(site)
	totalChecksTotal.DeleteLabelValues(site)
	incidentsTotal.DeleteLabelValues(site)
}

// RecordIncident increments the per-site incident counter (transition to down).
func RecordIncident(site string) {
	incidentsTotal.WithLabelValues(site).Inc()
}

// RecordCheck increments aggregate check counters and observes latency.
func RecordCheck(protocol string, up bool, latency time.Duration) {
	result := "down"
	if up {
		result = "up"
	}
	checksTotal.WithLabelValues(protocol, result).Inc()
	checkDuration.WithLabelValues(protocol).Observe(latency.Seconds())
}

// RecordFeederRun updates the last feeder batch size.
func RecordFeederRun(targetCount int) {
	feederTargetsLast.Set(float64(targetCount))
}

// RecordFlushRows adds successfully flushed rows.
func RecordFlushRows(n int) {
	if n > 0 {
		flushRowsTotal.Add(float64(n))
	}
}

// RecordFlushError increments flush failure counter.
func RecordFlushError() {
	flushErrorsTotal.Inc()
}

// RecordRedisPushError increments Redis write failure counter.
func RecordRedisPushError() {
	redisPushErrorsTotal.Inc()
}

// RecordAlertSent increments successful email alerts (kind: down|recovery).
func RecordAlertSent(kind string) {
	alertsSentTotal.WithLabelValues(kind).Inc()
}

// RecordAlertFailed increments failed email sends.
func RecordAlertFailed() {
	alertsFailedTotal.Inc()
}

// RecordRetentionDeleted adds deleted row count.
func RecordRetentionDeleted(n int64) {
	if n > 0 {
		retentionDeletedTotal.Add(float64(n))
	}
}

// SetActiveMonitors sets the active monitor count gauge.
func SetActiveMonitors(n int) {
	activeMonitors.Set(float64(n))
}

// SetRedisBufferLength sets the Redis ping_buffer length gauge.
func SetRedisBufferLength(n int64) {
	redisBufferLength.Set(float64(n))
}

// UpdateQueueGauges refreshes channel depth gauges from registered callbacks.
func UpdateQueueGauges() {
	if jobsQueueDepthFn != nil {
		jobsQueueLength.Set(float64(jobsQueueDepthFn()))
	}
	if resultsQueueDepthFn != nil {
		resultsQueueLength.Set(float64(resultsQueueDepthFn()))
	}
}
