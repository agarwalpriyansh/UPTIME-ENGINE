package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"monitor-engine/database"
	"monitor-engine/models"
	"monitor-engine/ssl"

	"github.com/DATA-DOG/go-sqlmock"
)

func setupTestDB(t *testing.T) (sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %s", err)
	}

	oldDB := database.DB
	database.DB = db

	cleanup := func() {
		db.Close()
		database.DB = oldDB
	}

	return mock, cleanup
}

func TestAddMonitorHandler(t *testing.T) {
	mock, cleanup := setupTestDB(t)
	defer cleanup()

	jobsCh := make(chan models.MonitorJob, 10)
	server := &APIServer{JobsQueue: jobsCh}

	// 1. Method not allowed
	t.Run("MethodNotAllowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/monitor", nil)
		w := httptest.NewRecorder()
		server.AddMonitorHandler(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	// 2. Invalid JSON
	t.Run("InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/monitor", bytes.NewBufferString("{invalid}"))
		w := httptest.NewRecorder()
		server.AddMonitorHandler(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	// 3. Validation failure (e.g. invalid type or invalid url)
	t.Run("ValidationFailure", func(t *testing.T) {
		job := models.MonitorJob{
			Type:       "invalid_protocol",
			Target:     "not-a-valid-url",
			OwnerEmail: "owner@test.com",
		}
		body, _ := json.Marshal(job)
		req := httptest.NewRequest(http.MethodPost, "/api/monitor", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		server.AddMonitorHandler(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	// 4. Successful add (inserted = true)
	t.Run("SuccessAdd", func(t *testing.T) {
		job := models.MonitorJob{
			Type:       models.ProtocolHTTP,
			Target:     "https://example.com",
			OwnerEmail: "owner@test.com",
		}
		body, _ := json.Marshal(job)
		req := httptest.NewRequest(http.MethodPost, "/api/monitor", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		mock.ExpectExec("INSERT INTO active_monitors").
			WithArgs("HTTP", "https://example.com", "owner@test.com").
			WillReturnResult(sqlmock.NewResult(1, 1))

		server.AddMonitorHandler(w, req)

		if w.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d", w.Code)
		}

		select {
		case j := <-jobsCh:
			if j.Target != "https://example.com" {
				t.Errorf("expected job target 'https://example.com', got '%s'", j.Target)
			}
		default:
			t.Error("expected job to be queued, but queue was empty")
		}
	})

	// 5. Duplicate add (inserted = false)
	t.Run("DuplicateAdd", func(t *testing.T) {
		job := models.MonitorJob{
			Type:       models.ProtocolHTTP,
			Target:     "https://example.com",
			OwnerEmail: "owner@test.com",
		}
		body, _ := json.Marshal(job)
		req := httptest.NewRequest(http.MethodPost, "/api/monitor", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		mock.ExpectExec("INSERT INTO active_monitors").
			WithArgs("HTTP", "https://example.com", "owner@test.com").
			WillReturnResult(sqlmock.NewResult(0, 0))

		server.AddMonitorHandler(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("expected 409, got %d", w.Code)
		}
	})

	// 6. DB Error
	t.Run("DBError", func(t *testing.T) {
		job := models.MonitorJob{
			Type:       models.ProtocolHTTP,
			Target:     "https://example.com",
			OwnerEmail: "owner@test.com",
		}
		body, _ := json.Marshal(job)
		req := httptest.NewRequest(http.MethodPost, "/api/monitor", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		mock.ExpectExec("INSERT INTO active_monitors").
			WithArgs("HTTP", "https://example.com", "owner@test.com").
			WillReturnError(errors.New("db disconnect"))

		server.AddMonitorHandler(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})
}

func TestDeleteMonitorHandler(t *testing.T) {
	mock, cleanup := setupTestDB(t)
	defer cleanup()

	server := &APIServer{}

	t.Run("MethodNotAllowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/monitor", nil)
		w := httptest.NewRecorder()
		server.DeleteMonitorHandler(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("MissingURL", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/monitor", nil)
		w := httptest.NewRecorder()
		server.DeleteMonitorHandler(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("SuccessDelete", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/monitor?url=example.com", nil)
		w := httptest.NewRecorder()

		mock.ExpectExec("DELETE FROM ping_results").
			WithArgs("example.com", "http://example.com", "https://example.com").
			WillReturnResult(sqlmock.NewResult(0, 5))

		mock.ExpectExec("DELETE FROM active_monitors").
			WithArgs("example.com", "http://example.com", "https://example.com").
			WillReturnResult(sqlmock.NewResult(0, 1))

		server.DeleteMonitorHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("DBError", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/monitor?url=example.com", nil)
		w := httptest.NewRecorder()

		mock.ExpectExec("DELETE FROM ping_results").
			WillReturnError(errors.New("db fail"))

		server.DeleteMonitorHandler(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})
}

func TestGetStatusHandler(t *testing.T) {
	mock, cleanup := setupTestDB(t)
	defer cleanup()

	server := &APIServer{}

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/status?limit=10", nil)
		w := httptest.NewRecorder()

		columns := []string{"target_url", "protocol", "status_code", "latency_ms", "is_up", "error_msg", "checked_at", "owner_email"}
		rows := sqlmock.NewRows(columns).
			AddRow("https://example.com", "http", 200, 150, true, "", time.Now(), "test@example.com")

		mock.ExpectQuery("SELECT pr.target_url").WithArgs(10).WillReturnRows(rows)

		server.GetStatusHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var resp []models.PingResult
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp) != 1 || resp[0].Job.Target != "https://example.com" {
			t.Errorf("unexpected payload: %+v", resp)
		}
	})

	t.Run("DBError", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
		w := httptest.NewRecorder()

		mock.ExpectQuery("SELECT pr.target_url").WillReturnError(errors.New("db error"))

		server.GetStatusHandler(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})
}

func TestGetTargetsHandler(t *testing.T) {
	mock, cleanup := setupTestDB(t)
	defer cleanup()

	server := &APIServer{}

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/targets", nil)
		w := httptest.NewRecorder()

		columns := []string{"protocol", "target_url", "owner_email"}
		rows := sqlmock.NewRows(columns).
			AddRow("http", "https://example.com", "owner@example.com")

		mock.ExpectQuery("SELECT protocol, target_url, owner_email FROM active_monitors").WillReturnRows(rows)

		server.GetTargetsHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var resp []models.MonitorJob
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp) != 1 || resp[0].Target != "https://example.com" {
			t.Errorf("unexpected payload: %+v", resp)
		}
	})
}

func TestGetLogsHandler(t *testing.T) {
	mock, cleanup := setupTestDB(t)
	defer cleanup()

	server := &APIServer{}

	t.Run("MissingURL", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
		w := httptest.NewRecorder()
		server.GetLogsHandler(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/logs?url=https://example.com&limit=5", nil)
		w := httptest.NewRecorder()

		columns := []string{"target_url", "protocol", "status_code", "latency_ms", "is_up", "error_msg", "checked_at", "owner_email"}
		rows := sqlmock.NewRows(columns).
			AddRow("https://example.com", "http", 200, 150, true, "", time.Now(), "test@example.com")

		mock.ExpectQuery("SELECT pr.target_url").WithArgs("https://example.com", 5).WillReturnRows(rows)

		server.GetLogsHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})
}

func TestGetSSLHandler(t *testing.T) {
	server := &APIServer{}

	t.Run("MissingSiteId", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ssl", nil)
		w := httptest.NewRecorder()
		server.GetSSLHandler(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("SuccessCheck", func(t *testing.T) {
		oldTLSConfig := ssl.TLSConfig
		ssl.TLSConfig = &tls.Config{InsecureSkipVerify: true}
		defer func() {
			ssl.TLSConfig = oldTLSConfig
		}()

		// Use a non-existent port to force immediate dial failure in background check,
		// or mock it. The handler directly calls ssl.Check(siteID).
		// Dialing an invalid host yields a response with status "unavailable" and Host: "invalid-site".
		req := httptest.NewRequest(http.MethodGet, "/api/ssl?siteId=https://invalid-site.local", nil)
		w := httptest.NewRecorder()

		server.GetSSLHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var resp ssl.Info
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Status != "unavailable" || resp.Host != "invalid-site.local" {
			t.Errorf("unexpected payload: %+v", resp)
		}
	})
}

func TestGetIncidentsHandler(t *testing.T) {
	mock, cleanup := setupTestDB(t)
	defer cleanup()

	server := &APIServer{}

	t.Run("MissingSiteId", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/incidents", nil)
		w := httptest.NewRecorder()
		server.GetIncidentsHandler(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/incidents?siteId=https://example.com&limit=10", nil)
		w := httptest.NewRecorder()

		columns := []string{"id", "site_id", "started_at", "resolved_at", "alert_sent_at", "last_escalation_at", "consecutive_failures", "status"}
		rows := sqlmock.NewRows(columns).
			AddRow(int64(123), "https://example.com", time.Now().Add(-1*time.Hour), time.Now(), time.Now().Add(-1*time.Hour), nil, 3, "resolved")

		mock.ExpectQuery("SELECT id, site_id, started_at").WithArgs("https://example.com", 10).WillReturnRows(rows)

		server.GetIncidentsHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var resp []incidentResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if len(resp) != 1 || resp[0].ID != 123 || resp[0].Status != "Resolved" {
			t.Errorf("unexpected payload: %+v", resp)
		}
	})
}
