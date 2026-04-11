package dashboard

import (
	"context"
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
	items       map[string]dmqueue.Item
	resolveErr  error
	cancelErr   error
	resolvedIDs []string
	cancelledIDs []string
}

func newStubNotifier() *stubNotifier {
	return &stubNotifier{items: map[string]dmqueue.Item{}}
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
