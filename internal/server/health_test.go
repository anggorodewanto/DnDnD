package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthHandler_Returns503WhenSubsystemUnhealthy(t *testing.T) {
	h := NewHealthHandler()
	h.Register("db", func() (string, bool) { return "connected", true })
	h.Register("discord", func() (string, bool) { return "disconnected", false })

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}

	if body["status"] != "degraded" {
		t.Fatalf("expected status degraded, got %v", body["status"])
	}
	if body["db"] != "connected" {
		t.Fatalf("expected db connected, got %v", body["db"])
	}
	if body["discord"] != "disconnected" {
		t.Fatalf("expected discord disconnected, got %v", body["discord"])
	}
}

func TestHealthHandler_ReturnsOKWhenAllSubsystemsHealthy(t *testing.T) {
	h := NewHealthHandler()
	h.Register("db", func() (string, bool) { return "connected", true })

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}

	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body["status"])
	}
	if body["db"] != "connected" {
		t.Fatalf("expected db connected, got %v", body["db"])
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"zero", 0, "0m"},
		{"minutes only", 45 * time.Minute, "45m"},
		{"hours and minutes", 3*time.Hour + 22*time.Minute, "3h 22m"},
		{"days hours minutes", 3*24*time.Hour + 14*time.Hour + 22*time.Minute, "3d 14h 22m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatUptime(tt.duration)
			if got != tt.want {
				t.Fatalf("formatUptime(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestHealthHandler_ReturnsOKWhenNoSubsystems(t *testing.T) {
	h := NewHealthHandler()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", ct)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}

	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body["status"])
	}

	if _, ok := body["uptime"]; !ok {
		t.Fatal("expected uptime field in response")
	}
}

func TestHealthHandler_DegradedWhenDBNotConfigured(t *testing.T) {
	// Finding 3: when DATABASE_URL is empty, nil DB checker reports unhealthy.
	h := NewHealthHandler()
	h.Register("db", NewDBChecker(nil))
	h.Register("discord", NewDiscordChecker(nil))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}
	if body["status"] != "degraded" {
		t.Fatalf("expected status degraded, got %v", body["status"])
	}
	if body["db"] != "disconnected" {
		t.Fatalf("expected db disconnected, got %v", body["db"])
	}
	if body["discord"] != "disconnected" {
		t.Fatalf("expected discord disconnected, got %v", body["discord"])
	}
}
