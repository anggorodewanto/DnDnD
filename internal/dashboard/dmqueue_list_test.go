package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dmqueue"
)

// stubCampaignQueueLister is a CampaignQueueLister that returns a fixed
// slice for tests. It also records the campaign id it was called with so
// tests can assert scoping.
type stubCampaignQueueLister struct {
	items   []dmqueue.Item
	gotID   uuid.UUID
	listErr error
}

func (s *stubCampaignQueueLister) ListPendingForCampaign(_ context.Context, campaignID uuid.UUID) ([]dmqueue.Item, error) {
	s.gotID = campaignID
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.items, nil
}

// fixedCampaignLookup returns a fixed (id, status) pair for tests.
type fixedCampaignLookup struct {
	id     string
	status string
	err    error
}

func (f fixedCampaignLookup) LookupActiveCampaign(_ context.Context, _ string) (string, string, error) {
	return f.id, f.status, f.err
}

// newDMQueueListRouter wires the F-12 list endpoint exactly as the
// production RegisterDMQueueRoutes does, minus auth middleware. Tests
// inject the user via requestWithUser.
func newDMQueueListRouter(t *testing.T, lister CampaignQueueLister, lookup CampaignLookup) chi.Router {
	t.Helper()
	r := chi.NewRouter()
	h := NewDMQueueHandler(nil, newStubNotifier())
	if lister != nil {
		h.SetCampaignLister(lister)
	}
	if lookup != nil {
		h.SetCampaignLookup(lookup)
	}
	r.Get("/dashboard/queue", h.ServeList)
	return r
}

func TestDMQueueList_ReturnsPendingItemsForCampaign(t *testing.T) {
	campaignID := uuid.New()
	lister := &stubCampaignQueueLister{
		items: []dmqueue.Item{
			{
				ID: "abc",
				Event: dmqueue.Event{
					Kind:        dmqueue.KindPlayerWhisper,
					PlayerName:  "Aria",
					Summary:     `"can I bribe the guard?"`,
					ResolvePath: "/dashboard/queue/abc",
				},
				Status: dmqueue.StatusPending,
			},
			{
				ID: "def",
				Event: dmqueue.Event{
					Kind:        dmqueue.KindRestRequest,
					PlayerName:  "Kael",
					Summary:     "requests a short rest",
					ResolvePath: "/dashboard/queue/def",
				},
				Status: dmqueue.StatusPending,
			},
		},
	}
	lookup := fixedCampaignLookup{id: campaignID.String(), status: "active"}

	r := newDMQueueListRouter(t, lister, lookup)
	req := requestWithUser(http.MethodGet, "/dashboard/queue", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, campaignID, lister.gotID, "lister scoped to active campaign")

	var got []dmqueueListEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 2)
	assert.Equal(t, "abc", got[0].ID)
	assert.Equal(t, "Player Whisper", got[0].KindLabel)
	assert.Equal(t, "Aria", got[0].PlayerName)
	assert.Equal(t, "/dashboard/queue/abc", got[0].ResolvePath)
	assert.Equal(t, "Rest Request", got[1].KindLabel)
}

func TestDMQueueList_EmptyWhenLookupHasNoCampaign(t *testing.T) {
	lister := &stubCampaignQueueLister{
		items: []dmqueue.Item{{ID: "abc", Status: dmqueue.StatusPending}},
	}
	r := newDMQueueListRouter(t, lister, fixedCampaignLookup{id: ""})

	req := requestWithUser(http.MethodGet, "/dashboard/queue", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got []dmqueueListEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Empty(t, got, "no active campaign -> empty array")
}

func TestDMQueueList_EmptyWhenListerOrLookupUnwired(t *testing.T) {
	r := newDMQueueListRouter(t, nil, nil)

	req := requestWithUser(http.MethodGet, "/dashboard/queue", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got []dmqueueListEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Empty(t, got)
}

func TestDMQueueList_RequiresAuth(t *testing.T) {
	lister := &stubCampaignQueueLister{}
	r := newDMQueueListRouter(t, lister, fixedCampaignLookup{id: uuid.NewString()})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/queue", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDMQueueList_DegradeOnLookupError(t *testing.T) {
	lister := &stubCampaignQueueLister{
		items: []dmqueue.Item{{ID: "abc", Status: dmqueue.StatusPending}},
	}
	lookup := fixedCampaignLookup{err: errors.New("db down")}
	r := newDMQueueListRouter(t, lister, lookup)

	req := requestWithUser(http.MethodGet, "/dashboard/queue", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got []dmqueueListEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Empty(t, got, "lookup error degrades to empty list, not 500")
}

func TestDMQueueList_DegradeOnListerError(t *testing.T) {
	lister := &stubCampaignQueueLister{listErr: errors.New("boom")}
	lookup := fixedCampaignLookup{id: uuid.NewString()}
	r := newDMQueueListRouter(t, lister, lookup)

	req := requestWithUser(http.MethodGet, "/dashboard/queue", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got []dmqueueListEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Empty(t, got)
}

func TestDMQueueList_DegradeOnInvalidCampaignID(t *testing.T) {
	lister := &stubCampaignQueueLister{}
	lookup := fixedCampaignLookup{id: "not-a-uuid"}
	r := newDMQueueListRouter(t, lister, lookup)

	req := requestWithUser(http.MethodGet, "/dashboard/queue", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got []dmqueueListEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Empty(t, got)
}
