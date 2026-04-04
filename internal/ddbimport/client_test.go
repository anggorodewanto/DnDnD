package ddbimport

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDDBClient_FetchCharacter_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/character/v5/character/12345" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("unexpected Accept header: %s", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"name":"Test"}}`)
	}))
	defer server.Close()

	client := NewDDBClient(WithBaseURL(server.URL))
	data, err := client.FetchCharacter(context.Background(), "12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(data), `"name":"Test"`) {
		t.Errorf("unexpected response: %s", string(data))
	}
}

func TestDDBClient_FetchCharacter_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewDDBClient(WithBaseURL(server.URL))
	_, err := client.FetchCharacter(context.Background(), "99999")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404: %v", err)
	}
}

func TestDDBClient_FetchCharacter_RateLimitRetry(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if attempt <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"name":"Retry"}}`)
	}))
	defer server.Close()

	client := NewDDBClient(
		WithBaseURL(server.URL),
		WithBackoff(10*time.Millisecond, 100*time.Millisecond, 3),
	)
	data, err := client.FetchCharacter(context.Background(), "12345")
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if !strings.Contains(string(data), `"name":"Retry"`) {
		t.Errorf("unexpected response: %s", string(data))
	}
	if got := attempts.Load(); got != 3 {
		t.Errorf("expected 3 attempts, got %d", got)
	}
}

func TestDDBClient_FetchCharacter_RateLimitExhausted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewDDBClient(
		WithBaseURL(server.URL),
		WithBackoff(10*time.Millisecond, 100*time.Millisecond, 2),
	)
	_, err := client.FetchCharacter(context.Background(), "12345")
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("error should mention rate limit: %v", err)
	}
}

func TestDDBClient_FetchCharacter_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	client := NewDDBClient(WithBaseURL(server.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.FetchCharacter(ctx, "12345")
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestDDBClient_FetchCharacter_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client := NewDDBClient(WithBaseURL(server.URL))
	_, err := client.FetchCharacter(context.Background(), "12345")
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !strings.Contains(err.Error(), "public") {
		t.Errorf("error should suggest public sharing: %v", err)
	}
}
