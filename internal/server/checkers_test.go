package server

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
)

// pingDriver is a minimal sql.Driver stub supporting Ping via a Pinger
// connection. The connection can be configured to return an error on ping.
type pingDriver struct{ fail bool }

func (d *pingDriver) Open(_ string) (driver.Conn, error) {
	return &pingConn{fail: d.fail}, nil
}

type pingConn struct {
	fail bool
}

func (c *pingConn) Prepare(_ string) (driver.Stmt, error) { return nil, errors.New("not implemented") }
func (c *pingConn) Close() error                          { return nil }
func (c *pingConn) Begin() (driver.Tx, error)             { return nil, errors.New("not implemented") }

// Ping makes *sql.DB.Ping work without going through a real query.
// Matches driver.Pinger so database/sql will call this directly.
func (c *pingConn) Ping(_ context.Context) error {
	if c.fail {
		return errors.New("db down")
	}
	return nil
}

func TestDBChecker_ConnectedWhenPingOK(t *testing.T) {
	sql.Register("pingdriver-ok", &pingDriver{fail: false})
	db, err := sql.Open("pingdriver-ok", "")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	checker := NewDBChecker(db)
	status, healthy := checker()
	if !healthy {
		t.Errorf("expected healthy, got false")
	}
	if status != "connected" {
		t.Errorf("expected status=connected, got %q", status)
	}
}

func TestDBChecker_DisconnectedWhenPingFails(t *testing.T) {
	sql.Register("pingdriver-fail", &pingDriver{fail: true})
	db, err := sql.Open("pingdriver-fail", "")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	checker := NewDBChecker(db)
	status, healthy := checker()
	if healthy {
		t.Errorf("expected unhealthy")
	}
	if status != "disconnected" {
		t.Errorf("expected status=disconnected, got %q", status)
	}
}

func TestDBChecker_NilDBReportsDisconnected(t *testing.T) {
	checker := NewDBChecker(nil)
	status, healthy := checker()
	if healthy {
		t.Errorf("expected unhealthy when DB is nil")
	}
	if status != "disconnected" {
		t.Errorf("expected status=disconnected, got %q", status)
	}
}

func TestDiscordChecker_ConnectedWhenTrue(t *testing.T) {
	checker := NewDiscordChecker(func() bool { return true })
	status, healthy := checker()
	if !healthy {
		t.Errorf("expected healthy")
	}
	if status != "connected" {
		t.Errorf("expected status=connected, got %q", status)
	}
}

func TestDiscordChecker_DisconnectedWhenFalse(t *testing.T) {
	checker := NewDiscordChecker(func() bool { return false })
	status, healthy := checker()
	if healthy {
		t.Errorf("expected unhealthy")
	}
	if status != "disconnected" {
		t.Errorf("expected status=disconnected, got %q", status)
	}
}

func TestDiscordChecker_NilProbeReportsDisconnected(t *testing.T) {
	checker := NewDiscordChecker(nil)
	status, healthy := checker()
	if healthy {
		t.Errorf("expected unhealthy")
	}
	if status != "disconnected" {
		t.Errorf("expected status=disconnected, got %q", status)
	}
}
