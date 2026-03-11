package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nhooyr.io/websocket"
)

func TestHub_RegisterAndBroadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	ch := make(chan []byte, 1)
	client := &Client{
		UserID: "user1",
		Send:   ch,
	}

	hub.Register <- client

	// Give hub time to process
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast <- []byte(`{"type":"snapshot","data":{}}`)

	select {
	case msg := <-ch:
		assert.Contains(t, string(msg), "snapshot")
	case <-time.After(time.Second):
		t.Fatal("expected broadcast message")
	}
}

func TestHub_UnregisterRemovesClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	ch := make(chan []byte, 1)
	client := &Client{
		UserID: "user1",
		Send:   ch,
	}

	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	hub.Unregister <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast should not block or send to unregistered client
	hub.Broadcast <- []byte(`{"type":"test"}`)
	time.Sleep(10 * time.Millisecond)

	select {
	case <-ch:
		t.Fatal("should not receive message after unregister")
	default:
		// expected
	}
}

func TestWebSocketEndpoint_RequiresAuth(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	h := NewHandler(nil)
	h.hub = hub

	req := httptest.NewRequest(http.MethodGet, "/dashboard/ws", nil)
	rec := httptest.NewRecorder()

	h.ServeWebSocket(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestWebSocketEndpoint_AcceptsConnection(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	h := NewHandler(nil)
	h.hub = hub

	// Create a test server with auth context
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(contextWithUser(r.Context(), "dm-user-1"))
		h.ServeWebSocket(w, r)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL+"/dashboard/ws", nil)
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Send a message from hub to verify client registered
	hub.Broadcast <- []byte(`{"type":"snapshot"}`)

	_, msg, err := conn.Read(ctx)
	require.NoError(t, err)
	assert.Contains(t, string(msg), "snapshot")
}

func TestHub_BroadcastToMultipleClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	ch1 := make(chan []byte, 1)
	ch2 := make(chan []byte, 1)
	client1 := &Client{UserID: "user1", Send: ch1}
	client2 := &Client{UserID: "user2", Send: ch2}

	hub.Register <- client1
	hub.Register <- client2
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast <- []byte(`{"type":"update"}`)

	for _, ch := range []chan []byte{ch1, ch2} {
		select {
		case msg := <-ch:
			assert.Contains(t, string(msg), "update")
		case <-time.After(time.Second):
			t.Fatal("expected broadcast message on all clients")
		}
	}
}

func TestHub_SlowClientDropped(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	// Buffer size 0 — client can't keep up
	ch := make(chan []byte)
	client := &Client{UserID: "slow", Send: ch}

	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast should not block even though client can't receive
	done := make(chan struct{})
	go func() {
		hub.Broadcast <- []byte(`{"type":"test"}`)
		close(done)
	}()

	select {
	case <-done:
		// good, broadcast didn't block
	case <-time.After(time.Second):
		t.Fatal("broadcast blocked on slow client")
	}
}
