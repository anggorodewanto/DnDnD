package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HealthChecker is a function that checks a subsystem's health.
// It returns the subsystem status string (e.g. "connected", "disconnected")
// and whether the subsystem is healthy.
type HealthChecker func() (status string, healthy bool)

// HealthHandler serves the GET /health endpoint.
type HealthHandler struct {
	startTime  time.Time
	subsystems map[string]HealthChecker
}

// NewHealthHandler creates a HealthHandler with the given start time set to now.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{
		startTime:  time.Now(),
		subsystems: make(map[string]HealthChecker),
	}
}

// Register adds a named subsystem health checker.
func (h *HealthHandler) Register(name string, checker HealthChecker) {
	h.subsystems[name] = checker
}

// ServeHTTP handles the health check request.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(h.startTime)
	resp := map[string]interface{}{
		"status": "ok",
		"uptime": formatUptime(uptime),
	}

	allHealthy := true
	for name, checker := range h.subsystems {
		status, healthy := checker()
		resp[name] = status
		if !healthy {
			allHealthy = false
		}
	}

	if !allHealthy {
		resp["status"] = "degraded"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(resp)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
