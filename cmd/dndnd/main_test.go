package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

// getFreePort asks the OS for a free port and returns it as a "host:port" string.
func getFreePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
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
	var entry map[string]any
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

	addr := getFreePort(t)
	var logBuf bytes.Buffer

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, &logBuf, addr)
	}()

	// Wait for server to be ready (poll after failure, not before)
	var resp *http.Response
	for range 20 {
		var err error
		resp, err = http.Get(fmt.Sprintf("http://%s/health", addr))
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if resp == nil {
		t.Fatal("server did not start in time")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
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
	ctx, cancel := context.WithCancel(context.Background())

	addr := getFreePort(t)
	var logBuf bytes.Buffer

	// Set ADDR env so run() picks it up as default
	t.Setenv("ADDR", addr)

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, &logBuf, "")
	}()

	// Wait for server to be ready
	var resp *http.Response
	for range 20 {
		var err error
		resp, err = http.Get(fmt.Sprintf("http://%s/health", addr))
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
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
