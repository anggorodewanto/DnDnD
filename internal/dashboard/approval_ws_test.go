package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupApprovalWSTest(t *testing.T, store *mockApprovalStore) (chi.Router, *Hub) {
	t.Helper()
	hub := NewHub()
	go hub.Run()
	t.Cleanup(hub.Stop)

	campaignID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ah := NewApprovalHandler(nil, store, &mockNotifier{}, hub, campaignID, nil)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(contextWithUser(r.Context(), "dm-user"))
			next.ServeHTTP(w, r)
		})
	})
	ah.RegisterApprovalRoutes(r)
	return r, hub
}

func TestApprove_BroadcastsWebSocketMessage(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	store := &mockApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{
				ID:            id,
				CharacterName: "Gandalf",
				DiscordUserID: "player1",
			},
		},
	}
	r, hub := setupApprovalWSTest(t, store)

	// Register a client to receive broadcast
	ch := make(chan []byte, 1)
	client := &Client{UserID: "dm-user", Send: ch}
	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/approve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	select {
	case msg := <-ch:
		var data map[string]string
		err := json.Unmarshal(msg, &data)
		require.NoError(t, err)
		assert.Equal(t, "approval_updated", data["type"])
		assert.Equal(t, id.String(), data["id"])
	case <-time.After(time.Second):
		t.Fatal("expected broadcast message")
	}
}

func TestReject_BroadcastsWebSocketMessage(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	store := &mockApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{
				ID:            id,
				CharacterName: "Gandalf",
				DiscordUserID: "player1",
			},
		},
	}
	r, hub := setupApprovalWSTest(t, store)

	ch := make(chan []byte, 1)
	client := &Client{UserID: "dm-user", Send: ch}
	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	body := `{"feedback":"Not allowed"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/reject", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	select {
	case msg := <-ch:
		var data map[string]string
		err := json.Unmarshal(msg, &data)
		require.NoError(t, err)
		assert.Equal(t, "approval_updated", data["type"])
	case <-time.After(time.Second):
		t.Fatal("expected broadcast message")
	}
}

func TestRequestChanges_BroadcastsWebSocketMessage(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	store := &mockApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{
				ID:            id,
				CharacterName: "Gandalf",
				DiscordUserID: "player1",
			},
		},
	}
	r, hub := setupApprovalWSTest(t, store)

	ch := make(chan []byte, 1)
	client := &Client{UserID: "dm-user", Send: ch}
	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	body := `{"feedback":"Fix HP"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/request-changes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	select {
	case msg := <-ch:
		var data map[string]string
		err := json.Unmarshal(msg, &data)
		require.NoError(t, err)
		assert.Equal(t, "approval_updated", data["type"])
	case <-time.After(time.Second):
		t.Fatal("expected broadcast message")
	}
}

func TestBroadcastUpdate_NilHub_NoPanic(t *testing.T) {
	campaignID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ah := NewApprovalHandler(nil, &mockApprovalStore{}, &mockNotifier{}, nil, campaignID, nil)
	assert.NotPanics(t, func() {
		ah.broadcastUpdate("test", uuid.New())
	})
}
