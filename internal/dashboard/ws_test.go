package dashboard

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
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

	// After unregister, the hub closes the Send channel
	_, open := <-ch
	assert.False(t, open, "Send channel should be closed after unregister")

	// Broadcast should not block or panic with no clients
	hub.Broadcast <- []byte(`{"type":"test"}`)
	time.Sleep(10 * time.Millisecond)
}

func TestWebSocketEndpoint_RequiresAuth(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	h := NewHandler(nil, hub)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/ws", nil)
	rec := httptest.NewRecorder()

	h.ServeWebSocket(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestWebSocketEndpoint_AcceptsConnection(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	h := NewHandler(nil, hub)

	// Create a test server with auth context
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(contextWithUser(r.Context(), "dm-user-1"))
		h.ServeWebSocket(w, r)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL+"/dashboard/ws", nil)
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Allow time for the client to be registered with the hub
	time.Sleep(50 * time.Millisecond)

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

func TestHub_BroadcastAfterUnregister_NoPanic(t *testing.T) {
	// This test verifies that broadcasting after a client has been unregistered
	// does not panic (e.g., by sending on a closed channel).
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	client := &Client{UserID: "user1", Send: make(chan []byte, 1)}
	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	// Unregister the client (hub should close the Send channel)
	hub.Unregister <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast should not panic even though client.Send was closed by hub
	assert.NotPanics(t, func() {
		hub.Broadcast <- []byte(`{"type":"test"}`)
		time.Sleep(10 * time.Millisecond)
	})
}

func TestHub_ConcurrentUnregisterAndBroadcast_NoPanic(t *testing.T) {
	// Stress test: concurrent broadcast and unregister should not cause a panic
	// from sending on a closed channel. Run with -race to verify.
	for i := 0; i < 50; i++ {
		hub := NewHub()
		go hub.Run()

		client := &Client{UserID: "user1", Send: make(chan []byte, 1)}
		hub.Register <- client
		time.Sleep(1 * time.Millisecond)

		// Race: broadcast and unregister concurrently
		go func() { hub.Broadcast <- []byte(`{"type":"test"}`) }()
		go func() { hub.Unregister <- client }()

		time.Sleep(5 * time.Millisecond)
		hub.Stop()
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

func TestHub_BroadcastEncounter_OnlyMatchingClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	chA := make(chan []byte, 1)
	chB := make(chan []byte, 1)
	chGlobal := make(chan []byte, 1)
	clientA := &Client{UserID: "userA", EncounterID: "enc-1", Send: chA}
	clientB := &Client{UserID: "userB", EncounterID: "enc-2", Send: chB}
	clientGlobal := &Client{UserID: "userG", EncounterID: "", Send: chGlobal}

	hub.Register <- clientA
	hub.Register <- clientB
	hub.Register <- clientGlobal
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastEncounter("enc-1", []byte(`{"type":"encounter_snapshot","encounter_id":"enc-1"}`))

	select {
	case msg := <-chA:
		assert.Contains(t, string(msg), "enc-1")
	case <-time.After(time.Second):
		t.Fatal("expected encounter-scoped message to clientA")
	}

	select {
	case msg := <-chB:
		t.Fatalf("clientB should not receive msg for enc-1, got %s", string(msg))
	case <-time.After(50 * time.Millisecond):
	}

	select {
	case msg := <-chGlobal:
		t.Fatalf("global client should not receive encounter-scoped msg, got %s", string(msg))
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHub_BroadcastEncounter_EmptyID_NoDelivery(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	ch := make(chan []byte, 1)
	client := &Client{UserID: "u", EncounterID: "", Send: ch}
	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastEncounter("", []byte(`{"type":"snapshot"}`))
	select {
	case msg := <-ch:
		t.Fatalf("empty encounterID should not deliver, got %s", string(msg))
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHub_BroadcastEncounter_SlowClientDropped(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	ch := make(chan []byte) // unbuffered
	client := &Client{UserID: "slow", EncounterID: "enc-1", Send: ch}
	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		hub.BroadcastEncounter("enc-1", []byte(`{"type":"snapshot"}`))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("BroadcastEncounter blocked on slow client")
	}
}

func TestHub_BroadcastGlobal_StillDeliversToEncounterClients(t *testing.T) {
	// Back-compat: existing global Broadcast should still reach all clients
	// regardless of encounter subscription (approvals etc.).
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	chA := make(chan []byte, 1)
	chG := make(chan []byte, 1)
	hub.Register <- &Client{UserID: "a", EncounterID: "enc-1", Send: chA}
	hub.Register <- &Client{UserID: "g", EncounterID: "", Send: chG}
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast <- []byte(`{"type":"approval_updated"}`)

	for _, ch := range []chan []byte{chA, chG} {
		select {
		case msg := <-ch:
			assert.Contains(t, string(msg), "approval_updated")
		case <-time.After(time.Second):
			t.Fatal("expected global broadcast")
		}
	}
}

// SR-016: WS Origin verification must reject foreign origins in prod and
// accept them in dev. Same-origin must always be accepted.
//
// Prod mode = SetWebSocketOriginPolicy([]string{"dashboard.example.com"}, false).
// Dev mode  = SetWebSocketOriginPolicy(nil, true) (current default behaviour).
//
// We can't drive the WS upgrade via httptest because httptest's loopback host
// changes per run; we instead exercise ServeWebSocket directly with a crafted
// http.Request carrying the Sec-WebSocket-* handshake headers + a deliberate
// Origin. nhooyr/websocket's Accept writes HTTP 403 on origin failure, so we
// can assert on the recorded response status code without completing a full
// upgrade.
func newWSHandshakeRequest(t *testing.T, originHeader, hostHeader string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/ws", nil)
	req.Host = hostHeader
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	if originHeader != "" {
		req.Header.Set("Origin", originHeader)
	}
	r := req.WithContext(contextWithUser(req.Context(), "dm-user-1"))
	return r
}

// hijackableRecorder is a httptest.ResponseRecorder that implements
// http.Hijacker so nhooyr/websocket's Accept doesn't bail out with 501.
type hijackableRecorder struct {
	*httptest.ResponseRecorder
}

func (h *hijackableRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	// We never want the upgrade to succeed because we don't have a real
	// underlying TCP conn — returning an error after the origin check has
	// passed is fine: the test only cares about the recorded HTTP status
	// (403 on origin failure, anything-not-403 if origin check passed).
	return nil, nil, errHijackNotSupported
}

var errHijackNotSupported = errHijack("hijack not supported in test")

type errHijack string

func (e errHijack) Error() string { return string(e) }

func TestWebSocketEndpoint_ProdMode_RejectsForeignOrigin(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	h := NewHandler(nil, hub)
	h.SetWebSocketOriginPolicy([]string{"dashboard.example.com"}, false)

	rec := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	req := newWSHandshakeRequest(t, "https://evil.example.com", "dashboard.example.com")

	h.ServeWebSocket(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code,
		"foreign Origin in prod mode must yield 403, got %d (body=%q)",
		rec.Code, rec.Body.String())
}

func TestWebSocketEndpoint_DevMode_AcceptsForeignOrigin(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	h := NewHandler(nil, hub)
	h.SetWebSocketOriginPolicy(nil, true) // dev: skip origin verification

	rec := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	req := newWSHandshakeRequest(t, "https://evil.example.com", "dashboard.example.com")

	h.ServeWebSocket(rec, req)

	// In dev mode, origin check is skipped; the upgrade then proceeds and
	// fails on our fake hijacker. The key assertion is that we do NOT get
	// HTTP 403 (the origin-rejection status).
	assert.NotEqual(t, http.StatusForbidden, rec.Code,
		"foreign Origin in dev mode must NOT be rejected; got %d (body=%q)",
		rec.Code, rec.Body.String())
}

func TestWebSocketEndpoint_SameOriginAccepted_BothModes(t *testing.T) {
	cases := []struct {
		name            string
		allowed         []string
		insecureSkipVer bool
	}{
		{"prod", []string{"dashboard.example.com"}, false},
		{"dev", nil, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			hub := NewHub()
			go hub.Run()
			defer hub.Stop()

			h := NewHandler(nil, hub)
			h.SetWebSocketOriginPolicy(tc.allowed, tc.insecureSkipVer)

			rec := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
			req := newWSHandshakeRequest(t,
				"https://dashboard.example.com",
				"dashboard.example.com",
			)

			h.ServeWebSocket(rec, req)

			assert.NotEqual(t, http.StatusForbidden, rec.Code,
				"same-origin upgrade in %s mode must be accepted; got %d",
				tc.name, rec.Code)
		})
	}
}

func TestWebSocketEndpoint_RejectsEncounterFromOtherCampaign(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	// DM's active campaign is campaign-A.
	dmCampaignID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	otherCampaignID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	encounterID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")

	lookup := &stubCampaignLookup{id: dmCampaignID.String(), status: "active"}
	resolver := &stubEncounterCampaignResolver{
		campaignID: otherCampaignID, // encounter belongs to a different campaign
	}

	h := NewHandler(nil, hub)
	h.SetCampaignLookup(lookup)
	h.SetEncounterCampaignResolver(resolver)

	rec := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	req := newWSHandshakeRequest(t, "", "localhost")
	req.URL.RawQuery = "encounter_id=" + encounterID.String()

	h.ServeWebSocket(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code,
		"DM connecting with encounter from another campaign must get 403")
}

func TestWebSocketEndpoint_AcceptsEncounterFromOwnCampaign(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	campaignID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	encounterID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")

	lookup := &stubCampaignLookup{id: campaignID.String(), status: "active"}
	resolver := &stubEncounterCampaignResolver{campaignID: campaignID}

	h := NewHandler(nil, hub)
	h.SetCampaignLookup(lookup)
	h.SetEncounterCampaignResolver(resolver)

	rec := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	req := newWSHandshakeRequest(t, "", "localhost")
	req.URL.RawQuery = "encounter_id=" + encounterID.String()

	h.ServeWebSocket(rec, req)

	// Should NOT get 403 — the encounter belongs to the DM's campaign.
	assert.NotEqual(t, http.StatusForbidden, rec.Code,
		"DM connecting with own campaign's encounter must not get 403")
}

// stubEncounterCampaignResolver satisfies EncounterCampaignResolver for tests.
type stubEncounterCampaignResolver struct {
	campaignID uuid.UUID
	err        error
}

func (s *stubEncounterCampaignResolver) GetEncounterCampaignID(_ context.Context, _ uuid.UUID) (uuid.UUID, error) {
	return s.campaignID, s.err
}

func TestWebSocketEndpoint_SubscribeToEncounter(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	h := NewHandler(nil, hub)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(contextWithUser(r.Context(), "dm-user-1"))
		h.ServeWebSocket(w, r)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL+"/dashboard/ws?encounter_id=enc-42", nil)
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "")

	time.Sleep(50 * time.Millisecond)

	hub.BroadcastEncounter("enc-42", []byte(`{"type":"encounter_snapshot","encounter_id":"enc-42"}`))

	_, msg, err := conn.Read(ctx)
	require.NoError(t, err)
	assert.Contains(t, string(msg), "enc-42")
}

func TestWebSocketEndpoint_DefaultRejectsForeignOrigin(t *testing.T) {
	// A-H03: A fresh Handler without SetWebSocketOriginPolicy must default to
	// rejecting foreign-origin WS upgrades (wsInsecureSkipVerify=false).
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	h := NewHandler(nil, hub) // no SetWebSocketOriginPolicy call

	rec := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	req := newWSHandshakeRequest(t, "https://evil.example.com", "dashboard.example.com")

	h.ServeWebSocket(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code,
		"default Handler must reject foreign Origin; got %d (body=%q)",
		rec.Code, rec.Body.String())
}
