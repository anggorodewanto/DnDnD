package dashboard

import (
	"context"
	"net/http"
	"time"

	"nhooyr.io/websocket"

	"github.com/ab/dndnd/internal/auth"
)

// Client represents a connected WebSocket client.
type Client struct {
	UserID string
	Send   chan []byte
}

// Hub manages WebSocket client connections and message broadcasting.
type Hub struct {
	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan []byte
	clients    map[*Client]bool
	stop       chan struct{}
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan []byte),
		clients:    make(map[*Client]bool),
		stop:       make(chan struct{}),
	}
}

// Run starts the hub's main loop.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.clients[client] = true
		case client := <-h.Unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
		case msg := <-h.Broadcast:
			for client := range h.clients {
				select {
				case client.Send <- msg:
				default:
					// Slow client — drop and unregister
					delete(h.clients, client)
					close(client.Send)
				}
			}
		case <-h.stop:
			return
		}
	}
}

// Stop signals the hub to shut down.
func (h *Hub) Stop() {
	close(h.stop)
}

// ServeWebSocket handles WebSocket upgrade and message pushing.
func (h *Handler) ServeWebSocket(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Allow connections from any origin in dev
	})
	if err != nil {
		h.logger.Error("websocket accept failed", "error", err)
		return
	}

	client := &Client{
		UserID: userID,
		Send:   make(chan []byte, 16),
	}

	h.hub.Register <- client

	// Writer goroutine: sends messages from hub to client
	go func() {
		defer conn.Close(websocket.StatusNormalClosure, "")

		for msg := range client.Send {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			err := conn.Write(ctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				h.logger.Debug("websocket write failed", "error", err, "user_id", userID)
				return
			}
		}
	}()

	// Reader loop: keeps connection alive by reading (discards messages).
	// When the connection closes, unregister from the hub which closes client.Send,
	// which in turn terminates the writer goroutine.
	for {
		_, _, err := conn.Read(r.Context())
		if err != nil {
			h.logger.Debug("websocket read closed", "error", err, "user_id", userID)
			h.hub.Unregister <- client
			return
		}
	}
}
