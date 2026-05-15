package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ab/dndnd/internal/errorlog"
)

func TestPanicRecovery_CatchesPanicAndReturns500(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := PanicRecovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["error"] != "internal server error" {
		t.Fatalf("expected error message, got %v", body["error"])
	}

	// Verify the panic was logged
	if logBuf.Len() == 0 {
		t.Fatal("expected panic to be logged")
	}

	var logEntry map[string]any
	if err := json.NewDecoder(&logBuf).Decode(&logEntry); err != nil {
		t.Fatalf("failed to decode log entry: %v", err)
	}
	if logEntry["msg"] != "panic recovered" {
		t.Fatalf("expected log message 'panic recovered', got %v", logEntry["msg"])
	}
}

func TestPanicRecovery_PassesThroughNormalRequests(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	handler := PanicRecovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestPanicRecoveryWithRecorder_RecordsPanic(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelError}))
	store := errorlog.NewMemoryStore(nil)

	handler := PanicRecoveryWithRecorder(logger, store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("http handler exploded")
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/encounters", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	// Verify the panic was recorded in the error store.
	entries, err := store.ListRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecent failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 recorded entry, got %d", len(entries))
	}
	if entries[0].Command != "POST /api/encounters" {
		t.Fatalf("expected command 'POST /api/encounters', got %q", entries[0].Command)
	}
	if entries[0].Summary != "panic: http handler exploded" {
		t.Fatalf("expected summary 'panic: http handler exploded', got %q", entries[0].Summary)
	}
}
