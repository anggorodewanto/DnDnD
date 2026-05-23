package dashboard

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dmqueue"
)

func newSkillCheckItem() dmqueue.Item {
	return dmqueue.Item{
		ID: "sc1",
		Event: dmqueue.Event{
			Kind:       dmqueue.KindSkillCheckNarration,
			PlayerName: "Aria",
			Summary:    "Perception check (rolled 17)",
			ExtraMetadata: map[string]string{
				dmqueue.SkillCheckChannelIDKey:       "chan-7",
				dmqueue.SkillCheckPlayerDiscordIDKey: "user-42",
				dmqueue.SkillCheckSkillLabelKey:      "Perception",
				dmqueue.SkillCheckTotalKey:           "17",
			},
		},
		Status: dmqueue.StatusPending,
	}
}

func TestDMQueueNarrate_AcceptsJSONAndMarksResolved(t *testing.T) {
	n := newStubNotifier()
	it := newSkillCheckItem()
	n.items[it.ID] = it

	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodPost, "/dashboard/queue/sc1/narrate", jsonBody(t, map[string]string{"narration": "You spot the trap."}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "You spot the trap.", n.skillCheckNarrations["sc1"])
	got, _ := n.Get("sc1")
	assert.Equal(t, dmqueue.StatusResolved, got.Status)
	assert.Equal(t, "You spot the trap.", got.Outcome)
}

func TestDMQueueNarrate_RequiresAuth(t *testing.T) {
	n := newStubNotifier()
	r := newDMQueueTestRouter(n)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/queue/sc1/narrate", strings.NewReader(`{"narration":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDMQueueNarrate_WrongKind(t *testing.T) {
	n := newStubNotifier()
	n.items["a1"] = dmqueue.Item{
		ID:     "a1",
		Event:  dmqueue.Event{Kind: dmqueue.KindFreeformAction, PlayerName: "x", Summary: "y"},
		Status: dmqueue.StatusPending,
	}
	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodPost, "/dashboard/queue/a1/narrate", jsonBody(t, map[string]string{"narration": "x"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDMQueueNarrate_NotFound(t *testing.T) {
	n := newStubNotifier()
	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodPost, "/dashboard/queue/missing/narrate", jsonBody(t, map[string]string{"narration": "x"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDMQueueNarrate_BackendError(t *testing.T) {
	n := newStubNotifier()
	it := newSkillCheckItem()
	n.items[it.ID] = it
	n.resolveSkillCheckNarrErr = errors.New("boom")

	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodPost, "/dashboard/queue/sc1/narrate", jsonBody(t, map[string]string{"narration": "x"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestDMQueueNarrate_RejectsInvalidJSON(t *testing.T) {
	n := newStubNotifier()
	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodPost, "/dashboard/queue/sc1/narrate", "not json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
