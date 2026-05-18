# Uptime Engine

A distributed, multi-tenant server monitoring system built to track website health, measure latency, and dispatch real-time email alerts upon state changes. 

The architecture leverages a Go-based backend for high-throughput concurrent processing, a Redis message broker for task queuing, PostgreSQL for persistent storage, and a modern Next.js React frontend for real-time visualization.

## Architecture & Data Flow

1. **Client Interface:** Users interact with the Next.js single-page application to add or remove target URLs and specify alert emails.
2. **REST API:** The Go backend receives the request and persists the monitoring job to PostgreSQL.
3. **The Feeder:** A background Go Cron job reads active targets from the database every 30 seconds and publishes them to a Redis message queue.
4. **Worker Pool:** A pool of concurrent Go workers pulls jobs from Redis, executes HTTP/TCP pings, and calculates latency.
5. **State Machine:** Results are passed via Go Channels to a central processor. An in-memory state machine tracks the historical status of each URL.
6. **Alerting:** If a state change is detected (e.g., UP to DOWN), the system asynchronously dispatches an SMTP email alert to the specific owner of that target.

## Tech Stack

* **Backend:** Go (Golang)
* **Frontend:** Next.js (React), Tailwind CSS, TypeScript
* **Database:** PostgreSQL
* **Message Broker:** Redis
* **Deployment:** Docker, Docker Compose, AWS EC2

## Project Structure

```text
uptime-engine/
├── api/                  # REST API handlers and routing
├── database/             # PostgreSQL and Redis connection and query logic
├── frontend/             # Next.js React application
├── models/               # Shared Go structs and data definitions
├── notifications/        # SMTP email alerting system
├── worker/               # Goroutine worker pool, HTTP ping logic, and Redis queuing
├── main.go               # Application entry point and state machine processor
├── docker-compose.yml    # Multi-container orchestration
└── .dockerignore         # Build optimization rules
