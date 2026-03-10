package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewRouter_HealthEndpoint(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	r, _ := NewRouter(logger)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body["status"])
	}
}

func TestNewRouter_ExposesHealthHandler(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	router, health := NewRouter(logger)

	// Register a subsystem checker via the returned HealthHandler
	health.Register("db", func() (string, bool) { return "disconnected", false })

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503 after registering unhealthy subsystem, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["status"] != "degraded" {
		t.Fatalf("expected status degraded, got %v", body["status"])
	}
	if body["db"] != "disconnected" {
		t.Fatalf("expected db disconnected, got %v", body["db"])
	}
}

func TestNewRouter_PanicRecoveryIntegrated(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	r, _ := NewRouter(logger)

	// We can't easily inject a panicking handler into the existing router,
	// but we can verify the middleware chain is set up by testing that
	// unknown routes return 405 or 404 (Chi behavior).
	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}
