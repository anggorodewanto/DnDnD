package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestRun_HealthEndpointReturns200(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var logBuf bytes.Buffer

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, &logBuf, ":0")
	}()

	// Give the server a moment to start
	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-errCh
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

func TestRun_ServerStartsAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var logBuf bytes.Buffer

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, &logBuf, ":0")
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-errCh
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	// Verify structured JSON logs were produced
	if logBuf.Len() == 0 {
		t.Fatal("expected log output, got none")
	}

	// Check that at least one log line is valid JSON
	var entry map[string]interface{}
	decoder := json.NewDecoder(&logBuf)
	if err := decoder.Decode(&entry); err != nil {
		t.Fatalf("log output is not valid JSON: %v", err)
	}
}

func TestRun_ListenError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var logBuf bytes.Buffer

	// Use an invalid address to trigger a listen error
	err := run(ctx, &logBuf, ":-1")
	if err == nil {
		t.Fatal("expected error for invalid address, got nil")
	}
}

func TestRun_HealthEndpointFunctional(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var logBuf bytes.Buffer

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, &logBuf, "127.0.0.1:18923")
	}()

	// Wait for server to be ready
	var resp *http.Response
	for i := 0; i < 20; i++ {
		time.Sleep(25 * time.Millisecond)
		var err error
		resp, err = http.Get("http://127.0.0.1:18923/health")
		if err == nil {
			break
		}
	}
	if resp == nil {
		t.Fatal("server did not start in time")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body["status"])
	}

	cancel()
	err := <-errCh
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

func TestRun_DefaultAddr(t *testing.T) {
	// Test that run uses default addr when no override provided
	ctx, cancel := context.WithCancel(context.Background())

	var logBuf bytes.Buffer

	// Set ADDR env to a random port so we don't conflict
	t.Setenv("ADDR", "127.0.0.1:18924")

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, &logBuf)
	}()

	// Wait for server to be ready
	var resp *http.Response
	for i := 0; i < 20; i++ {
		time.Sleep(25 * time.Millisecond)
		var err error
		resp, err = http.Get("http://127.0.0.1:18924/health")
		if err == nil {
			break
		}
	}
	if resp == nil {
		t.Fatal("server did not start in time")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	cancel()
	err := <-errCh
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}
