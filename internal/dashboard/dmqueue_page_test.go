package dashboard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dmqueue"
)

// stubNotifier lets dashboard tests inspect and drive a Notifier without Discord.
type stubNotifier struct {
	items                       map[string]dmqueue.Item
	resolveErr                  error
	cancelErr                   error
	resolveWhisperErr           error
	resolveSkillCheckNarrErr    error
	resolvedIDs                 []string
	cancelledIDs                []string
	whisperReplies              map[string]string
	skillCheckNarrations        map[string]string
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
	r.Get("/dashboard/queue/{itemID}", h.ServeItem)
	r.Post("/dashboard/queue/{itemID}/resolve", h.HandleResolve)
	r.Post("/dashboard/queue/{itemID}/reply", h.HandleWhisperReply)
	r.Post("/dashboard/queue/{itemID}/narrate", h.HandleSkillCheckNarration)
	return r
}

func requestWithUser(method, path string, body string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req = req.WithContext(contextWithUser(req.Context(), "dm-user-1"))
	return req
}

func TestDMQueuePage_RendersPendingItem(t *testing.T) {
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
	body := rec.Body.String()
	assert.Contains(t, body, "Thorn")
	assert.Contains(t, body, "flip the table")
	assert.Contains(t, body, "Resolve")
	assert.Contains(t, body, `action="/dashboard/queue/abc/resolve"`)
}

func TestDMQueuePage_NotFound(t *testing.T) {
	n := newStubNotifier()
	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodGet, "/dashboard/queue/missing", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDMQueuePage_RequiresAuth(t *testing.T) {
	n := newStubNotifier()
	n.items["abc"] = dmqueue.Item{ID: "abc", Status: dmqueue.StatusPending}
	r := newDMQueueTestRouter(n)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/queue/abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDMQueuePage_ResolveMarksItemResolved(t *testing.T) {
	n := newStubNotifier()
	n.items["abc"] = dmqueue.Item{ID: "abc", Status: dmqueue.StatusPending}
	r := newDMQueueTestRouter(n)

	form := url.Values{}
	form.Set("outcome", "table flipped")
	req := requestWithUser(http.MethodPost, "/dashboard/queue/abc/resolve", form.Encode())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, []string{"abc"}, n.resolvedIDs)
	it, _ := n.Get("abc")
	assert.Equal(t, dmqueue.StatusResolved, it.Status)
	assert.Equal(t, "table flipped", it.Outcome)
}

func TestDMQueuePage_ResolveUnknownItem(t *testing.T) {
	n := newStubNotifier()
	r := newDMQueueTestRouter(n)
	form := url.Values{}
	form.Set("outcome", "x")
	req := requestWithUser(http.MethodPost, "/dashboard/queue/missing/resolve", form.Encode())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDMQueuePage_ResolveRequiresAuth(t *testing.T) {
	n := newStubNotifier()
	r := newDMQueueTestRouter(n)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/queue/x/resolve", strings.NewReader("outcome=x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDMQueuePage_ResolveBackendError(t *testing.T) {
	n := newStubNotifier()
	n.items["x"] = dmqueue.Item{ID: "x", Status: dmqueue.StatusPending}
	n.resolveErr = errors.New("boom")
	r := newDMQueueTestRouter(n)
	form := url.Values{}
	form.Set("outcome", "x")
	req := requestWithUser(http.MethodPost, "/dashboard/queue/x/resolve", form.Encode())
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

func TestDMQueuePage_WhisperItemShowsReplyForm(t *testing.T) {
	n := newStubNotifier()
	n.items["w1"] = dmqueue.Item{
		ID: "w1",
		Event: dmqueue.Event{
			Kind:       dmqueue.KindPlayerWhisper,
			PlayerName: "Aria",
			Summary:    `"pickpocket the merchant"`,
			ExtraMetadata: map[string]string{
				dmqueue.WhisperTargetDiscordUserIDKey: "user-42",
			},
		},
		Status: dmqueue.StatusPending,
	}

	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodGet, "/dashboard/queue/w1", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `action="/dashboard/queue/w1/reply"`)
	assert.Contains(t, body, `name="reply"`)
	// The generic resolve form must not be rendered for whisper items.
	assert.NotContains(t, body, `action="/dashboard/queue/w1/resolve"`)
}

func TestDMQueuePage_HandleWhisperReply_Success(t *testing.T) {
	n := newStubNotifier()
	n.items["w1"] = dmqueue.Item{
		ID: "w1",
		Event: dmqueue.Event{
			Kind:       dmqueue.KindPlayerWhisper,
			PlayerName: "Aria",
			Summary:    `"x"`,
			ExtraMetadata: map[string]string{dmqueue.WhisperTargetDiscordUserIDKey: "user-42"},
		},
		Status: dmqueue.StatusPending,
	}
	r := newDMQueueTestRouter(n)

	form := url.Values{}
	form.Set("reply", "You succeed.")
	req := requestWithUser(http.MethodPost, "/dashboard/queue/w1/reply", form.Encode())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "You succeed.", n.whisperReplies["w1"])
	it, _ := n.Get("w1")
	assert.Equal(t, dmqueue.StatusResolved, it.Status)
	assert.Equal(t, "You succeed.", it.Outcome)
}

func TestDMQueuePage_HandleWhisperReply_RequiresAuth(t *testing.T) {
	n := newStubNotifier()
	r := newDMQueueTestRouter(n)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/queue/w1/reply", strings.NewReader("reply=hi"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDMQueuePage_HandleWhisperReply_WrongKind(t *testing.T) {
	n := newStubNotifier()
	n.items["a1"] = dmqueue.Item{
		ID:     "a1",
		Event:  dmqueue.Event{Kind: dmqueue.KindFreeformAction, PlayerName: "Thorn", Summary: `"flip"`},
		Status: dmqueue.StatusPending,
	}
	r := newDMQueueTestRouter(n)

	form := url.Values{}
	form.Set("reply", "nope")
	req := requestWithUser(http.MethodPost, "/dashboard/queue/a1/reply", form.Encode())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDMQueuePage_HandleWhisperReply_MissingDeliverer(t *testing.T) {
	n := newStubNotifier()
	n.items["w1"] = dmqueue.Item{
		ID: "w1",
		Event: dmqueue.Event{
			Kind:       dmqueue.KindPlayerWhisper,
			PlayerName: "Aria",
			ExtraMetadata: map[string]string{dmqueue.WhisperTargetDiscordUserIDKey: "user-42"},
		},
		Status: dmqueue.StatusPending,
	}
	n.resolveWhisperErr = dmqueue.ErrWhisperDelivererMissing
	r := newDMQueueTestRouter(n)

	form := url.Values{}
	form.Set("reply", "hi")
	req := requestWithUser(http.MethodPost, "/dashboard/queue/w1/reply", form.Encode())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestDMQueuePage_HandleWhisperReply_UnknownItem(t *testing.T) {
	n := newStubNotifier()
	r := newDMQueueTestRouter(n)
	form := url.Values{}
	form.Set("reply", "hi")
	req := requestWithUser(http.MethodPost, "/dashboard/queue/missing/reply", form.Encode())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDMQueuePage_RendersResolvedItem(t *testing.T) {
	n := newStubNotifier()
	n.items["xyz"] = dmqueue.Item{
		ID:     "xyz",
		Event:  dmqueue.Event{Kind: dmqueue.KindSkillCheckNarration, PlayerName: "Aria", Summary: "Athletics 18"},
		Status: dmqueue.StatusResolved,
		Outcome: "climbs cliff",
	}
	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodGet, "/dashboard/queue/xyz", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Resolved")
	assert.Contains(t, body, "climbs cliff")
	// Resolve form should NOT be rendered for already-resolved items.
	assert.NotContains(t, body, "/resolve\"")
}
