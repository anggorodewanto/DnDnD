package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dmqueue"
)

// stubNotifier lets dashboard tests inspect and drive a Notifier without Discord.
type stubNotifier struct {
	items                    map[string]dmqueue.Item
	resolveErr               error
	cancelErr                error
	resolveWhisperErr        error
	resolveSkillCheckNarrErr error
	resolvedIDs              []string
	cancelledIDs             []string
	whisperReplies           map[string]string
	skillCheckNarrations     map[string]string
}

func newStubNotifier() *stubNotifier {
	return &stubNotifier{
		items:                map[string]dmqueue.Item{},
		whisperReplies:       map[string]string{},
		skillCheckNarrations: map[string]string{},
	}
}

func (s *stubNotifier) ResolveSkillCheckNarration(_ context.Context, id, narration string) error {
	if s.resolveSkillCheckNarrErr != nil {
		return s.resolveSkillCheckNarrErr
	}
	it, ok := s.items[id]
	if !ok {
		return dmqueue.ErrItemNotFound
	}
	if it.Event.Kind != dmqueue.KindSkillCheckNarration {
		return dmqueue.ErrNotSkillCheckNarrationItem
	}
	it.Status = dmqueue.StatusResolved
	it.Outcome = narration
	s.items[id] = it
	s.skillCheckNarrations[id] = narration
	s.resolvedIDs = append(s.resolvedIDs, id)
	return nil
}

func (s *stubNotifier) ResolveWhisper(_ context.Context, id, replyText string) error {
	if s.resolveWhisperErr != nil {
		return s.resolveWhisperErr
	}
	it, ok := s.items[id]
	if !ok {
		return dmqueue.ErrItemNotFound
	}
	if it.Event.Kind != dmqueue.KindPlayerWhisper {
		return dmqueue.ErrNotWhisperItem
	}
	it.Status = dmqueue.StatusResolved
	it.Outcome = replyText
	s.items[id] = it
	s.whisperReplies[id] = replyText
	s.resolvedIDs = append(s.resolvedIDs, id)
	return nil
}

func (s *stubNotifier) Post(_ context.Context, _ dmqueue.Event) (string, error) {
	return "", nil
}
func (s *stubNotifier) Cancel(_ context.Context, id, _ string) error {
	if s.cancelErr != nil {
		return s.cancelErr
	}
	it, ok := s.items[id]
	if !ok {
		return dmqueue.ErrItemNotFound
	}
	it.Status = dmqueue.StatusCancelled
	s.items[id] = it
	s.cancelledIDs = append(s.cancelledIDs, id)
	return nil
}
func (s *stubNotifier) Resolve(_ context.Context, id, outcome string) error {
	if s.resolveErr != nil {
		return s.resolveErr
	}
	it, ok := s.items[id]
	if !ok {
		return dmqueue.ErrItemNotFound
	}
	it.Status = dmqueue.StatusResolved
	it.Outcome = outcome
	s.items[id] = it
	s.resolvedIDs = append(s.resolvedIDs, id)
	return nil
}
func (s *stubNotifier) Get(id string) (dmqueue.Item, bool) {
	it, ok := s.items[id]
	return it, ok
}
func (s *stubNotifier) ListPending() []dmqueue.Item {
	var out []dmqueue.Item
	for _, it := range s.items {
		if it.Status == dmqueue.StatusPending {
			out = append(out, it)
		}
	}
	return out
}

func newDMQueueTestRouter(n dmqueue.Notifier) chi.Router {
	r := chi.NewRouter()
	h := NewDMQueueHandler(nil, n)
	r.Get("/dashboard/queue/{itemID}", h.GetItemJSON)
	r.Post("/dashboard/queue/{itemID}/resolve", h.HandleResolve)
	r.Post("/dashboard/queue/{itemID}/reply", h.HandleWhisperReply)
	r.Post("/dashboard/queue/{itemID}/narrate", h.HandleSkillCheckNarration)
	return r
}

// requestWithUser builds an authenticated request. body is sent verbatim
// with Content-Type: application/json; pass "" for GET requests.
func requestWithUser(method, path string, body string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req = req.WithContext(contextWithUser(req.Context(), "dm-user-1"))
	return req
}

func jsonBody(t *testing.T, payload any) string {
	t.Helper()
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	return string(b)
}

func TestDMQueueItem_ReturnsPendingItemAsJSON(t *testing.T) {
	n := newStubNotifier()
	n.items["abc"] = dmqueue.Item{
		ID: "abc",
		Event: dmqueue.Event{
			Kind:       dmqueue.KindFreeformAction,
			PlayerName: "Thorn",
			Summary:    `"flip the table"`,
		},
		Status: dmqueue.StatusPending,
	}

	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodGet, "/dashboard/queue/abc", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var got dmqueueItemDetail
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "abc", got.ID)
	assert.Equal(t, "Thorn", got.PlayerName)
	assert.Equal(t, `"flip the table"`, got.Summary)
	assert.Equal(t, "Freeform Action", got.KindLabel)
	assert.Equal(t, "pending", got.Status)
	assert.True(t, got.IsPending)
	assert.False(t, got.IsResolved)
	assert.False(t, got.IsWhisper)
	assert.False(t, got.IsSkillCheckNarration)
}

func TestDMQueueItem_NotFound(t *testing.T) {
	n := newStubNotifier()
	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodGet, "/dashboard/queue/missing", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDMQueueItem_RequiresAuth(t *testing.T) {
	n := newStubNotifier()
	n.items["abc"] = dmqueue.Item{ID: "abc", Status: dmqueue.StatusPending}
	r := newDMQueueTestRouter(n)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/queue/abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDMQueueItem_WhisperFlagsSet(t *testing.T) {
	n := newStubNotifier()
	n.items["w1"] = dmqueue.Item{
		ID: "w1",
		Event: dmqueue.Event{
			Kind:       dmqueue.KindPlayerWhisper,
			PlayerName: "Aria",
			Summary:    `"pickpocket"`,
		},
		Status: dmqueue.StatusPending,
	}
	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodGet, "/dashboard/queue/w1", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got dmqueueItemDetail
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.True(t, got.IsWhisper)
	assert.False(t, got.IsSkillCheckNarration)
}

func TestDMQueueItem_SkillCheckNarrationFlagsSet(t *testing.T) {
	n := newStubNotifier()
	n.items["sc1"] = dmqueue.Item{
		ID:     "sc1",
		Event:  dmqueue.Event{Kind: dmqueue.KindSkillCheckNarration, PlayerName: "Aria", Summary: "Perception 17"},
		Status: dmqueue.StatusPending,
	}
	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodGet, "/dashboard/queue/sc1", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got dmqueueItemDetail
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.True(t, got.IsSkillCheckNarration)
	assert.False(t, got.IsWhisper)
}

func TestDMQueueItem_ResolvedItemSurfacesOutcome(t *testing.T) {
	n := newStubNotifier()
	n.items["xyz"] = dmqueue.Item{
		ID:      "xyz",
		Event:   dmqueue.Event{Kind: dmqueue.KindSkillCheckNarration, PlayerName: "Aria", Summary: "Athletics 18"},
		Status:  dmqueue.StatusResolved,
		Outcome: "climbs cliff",
	}
	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodGet, "/dashboard/queue/xyz", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got dmqueueItemDetail
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "resolved", got.Status)
	assert.Equal(t, "climbs cliff", got.Outcome)
	assert.True(t, got.IsResolved)
	assert.False(t, got.IsPending)
}

func TestDMQueueItem_CancelledItemFlagged(t *testing.T) {
	n := newStubNotifier()
	n.items["c1"] = dmqueue.Item{
		ID:     "c1",
		Event:  dmqueue.Event{Kind: dmqueue.KindFreeformAction},
		Status: dmqueue.StatusCancelled,
	}
	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodGet, "/dashboard/queue/c1", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got dmqueueItemDetail
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.True(t, got.IsCancelled)
	assert.False(t, got.IsPending)
}

func TestDMQueueResolve_AcceptsJSONAndMarksResolved(t *testing.T) {
	n := newStubNotifier()
	n.items["abc"] = dmqueue.Item{ID: "abc", Status: dmqueue.StatusPending}
	r := newDMQueueTestRouter(n)

	req := requestWithUser(http.MethodPost, "/dashboard/queue/abc/resolve", jsonBody(t, map[string]string{"outcome": "table flipped"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, []string{"abc"}, n.resolvedIDs)
	it, _ := n.Get("abc")
	assert.Equal(t, dmqueue.StatusResolved, it.Status)
	assert.Equal(t, "table flipped", it.Outcome)
}

func TestDMQueueResolve_RejectsInvalidJSON(t *testing.T) {
	n := newStubNotifier()
	n.items["abc"] = dmqueue.Item{ID: "abc", Status: dmqueue.StatusPending}
	r := newDMQueueTestRouter(n)

	req := requestWithUser(http.MethodPost, "/dashboard/queue/abc/resolve", "{not json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, n.resolvedIDs)
}

func TestDMQueueResolve_UnknownItem(t *testing.T) {
	n := newStubNotifier()
	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodPost, "/dashboard/queue/missing/resolve", jsonBody(t, map[string]string{"outcome": "x"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDMQueueResolve_RequiresAuth(t *testing.T) {
	n := newStubNotifier()
	r := newDMQueueTestRouter(n)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/queue/x/resolve", strings.NewReader(`{"outcome":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDMQueueResolve_BackendError(t *testing.T) {
	n := newStubNotifier()
	n.items["x"] = dmqueue.Item{ID: "x", Status: dmqueue.StatusPending}
	n.resolveErr = errors.New("boom")
	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodPost, "/dashboard/queue/x/resolve", jsonBody(t, map[string]string{"outcome": "x"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestKindLabelFor_AllKinds(t *testing.T) {
	cases := []struct {
		kind dmqueue.EventKind
		want string
	}{
		{dmqueue.KindFreeformAction, "Freeform Action"},
		{dmqueue.KindReactionDeclaration, "Reaction Declaration"},
		{dmqueue.KindRestRequest, "Rest Request"},
		{dmqueue.KindSkillCheckNarration, "Skill Check Narration"},
		{dmqueue.KindConsumable, "Consumable Usage"},
		{dmqueue.KindEnemyTurnReady, "Enemy Turn Ready"},
		{dmqueue.KindNarrativeTeleport, "Narrative Teleport"},
		{dmqueue.KindPlayerWhisper, "Player Whisper"},
		{dmqueue.EventKind("mystery"), "Notification"},
	}
	for _, tc := range cases {
		if got := kindLabelFor(tc.kind); got != tc.want {
			t.Errorf("kindLabelFor(%q) = %q want %q", tc.kind, got, tc.want)
		}
	}
}

func TestDMQueueWhisperReply_AcceptsJSON(t *testing.T) {
	n := newStubNotifier()
	n.items["w1"] = dmqueue.Item{
		ID: "w1",
		Event: dmqueue.Event{
			Kind:          dmqueue.KindPlayerWhisper,
			PlayerName:    "Aria",
			Summary:       `"x"`,
			ExtraMetadata: map[string]string{dmqueue.WhisperTargetDiscordUserIDKey: "user-42"},
		},
		Status: dmqueue.StatusPending,
	}
	r := newDMQueueTestRouter(n)

	req := requestWithUser(http.MethodPost, "/dashboard/queue/w1/reply", jsonBody(t, map[string]string{"reply": "You succeed."}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "You succeed.", n.whisperReplies["w1"])
	it, _ := n.Get("w1")
	assert.Equal(t, dmqueue.StatusResolved, it.Status)
	assert.Equal(t, "You succeed.", it.Outcome)
}

func TestDMQueueWhisperReply_RequiresAuth(t *testing.T) {
	n := newStubNotifier()
	r := newDMQueueTestRouter(n)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/queue/w1/reply", strings.NewReader(`{"reply":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDMQueueWhisperReply_WrongKind(t *testing.T) {
	n := newStubNotifier()
	n.items["a1"] = dmqueue.Item{
		ID:     "a1",
		Event:  dmqueue.Event{Kind: dmqueue.KindFreeformAction, PlayerName: "Thorn", Summary: `"flip"`},
		Status: dmqueue.StatusPending,
	}
	r := newDMQueueTestRouter(n)

	req := requestWithUser(http.MethodPost, "/dashboard/queue/a1/reply", jsonBody(t, map[string]string{"reply": "nope"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDMQueueWhisperReply_MissingDeliverer(t *testing.T) {
	n := newStubNotifier()
	n.items["w1"] = dmqueue.Item{
		ID: "w1",
		Event: dmqueue.Event{
			Kind:          dmqueue.KindPlayerWhisper,
			PlayerName:    "Aria",
			ExtraMetadata: map[string]string{dmqueue.WhisperTargetDiscordUserIDKey: "user-42"},
		},
		Status: dmqueue.StatusPending,
	}
	n.resolveWhisperErr = dmqueue.ErrWhisperDelivererMissing
	r := newDMQueueTestRouter(n)

	req := requestWithUser(http.MethodPost, "/dashboard/queue/w1/reply", jsonBody(t, map[string]string{"reply": "hi"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestDMQueueWhisperReply_UnknownItem(t *testing.T) {
	n := newStubNotifier()
	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodPost, "/dashboard/queue/missing/reply", jsonBody(t, map[string]string{"reply": "hi"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDMQueueWhisperReply_RejectsInvalidJSON(t *testing.T) {
	n := newStubNotifier()
	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodPost, "/dashboard/queue/w1/reply", "garbage")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
