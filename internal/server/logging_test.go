package server

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestNewLogger_ProducesJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, false)

	logger.Info("test message", "key", "value")

	var entry map[string]interface{}
	if err := json.NewDecoder(&buf).Decode(&entry); err != nil {
		t.Fatalf("log output is not valid JSON: %v", err)
	}
	if entry["msg"] != "test message" {
		t.Fatalf("expected msg 'test message', got %v", entry["msg"])
	}
	if entry["key"] != "value" {
		t.Fatalf("expected key 'value', got %v", entry["key"])
	}
}

func TestNewLogger_DebugLevelInDevMode(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, true)

	logger.Debug("debug message")

	if buf.Len() == 0 {
		t.Fatal("expected debug message to be logged in dev mode")
	}
}

func TestNewLogger_NoDebugInProdMode(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, false)

	logger.Debug("debug message")

	if buf.Len() != 0 {
		t.Fatal("expected debug message to be suppressed in prod mode")
	}
}
