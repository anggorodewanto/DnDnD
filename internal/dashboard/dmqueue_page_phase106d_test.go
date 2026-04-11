package dashboard

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestDMQueuePage_SkillCheckNarrationItemShowsNarrateForm(t *testing.T) {
	n := newStubNotifier()
	it := newSkillCheckItem()
	n.items[it.ID] = it

	r := newDMQueueTestRouter(n)
	req := requestWithUser(http.MethodGet, "/dashboard/queue/sc1", "")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `action="/dashboard/queue/sc1/narrate"`)
	assert.Contains(t, body, `name="narration"`)
	// Generic resolve form should not be rendered for narration items.
	assert.NotContains(t, body, `action="/dashboard/queue/sc1/resolve"`)
}

func TestDMQueuePage_HandleSkillCheckNarration_Success(t *testing.T) {
	n := newStubNotifier()
	it := newSkillCheckItem()
	n.items[it.ID] = it

	r := newDMQueueTestRouter(n)
	form := url.Values{}
	form.Set("narration", "You spot the trap.")
	req := requestWithUser(http.MethodPost, "/dashboard/queue/sc1/narrate", form.Encode())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "You spot the trap.", n.skillCheckNarrations["sc1"])
	got, _ := n.Get("sc1")
	assert.Equal(t, dmqueue.StatusResolved, got.Status)
	assert.Equal(t, "You spot the trap.", got.Outcome)
}

func TestDMQueuePage_HandleSkillCheckNarration_RequiresAuth(t *testing.T) {
	n := newStubNotifier()
	r := newDMQueueTestRouter(n)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/queue/sc1/narrate", strings.NewReader("narration=x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDMQueuePage_HandleSkillCheckNarration_WrongKind(t *testing.T) {
	n := newStubNotifier()
	n.items["a1"] = dmqueue.Item{
		ID:     "a1",
		Event:  dmqueue.Event{Kind: dmqueue.KindFreeformAction, PlayerName: "x", Summary: "y"},
		Status: dmqueue.StatusPending,
	}
	r := newDMQueueTestRouter(n)
	form := url.Values{}
	form.Set("narration", "x")
	req := requestWithUser(http.MethodPost, "/dashboard/queue/a1/narrate", form.Encode())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDMQueuePage_HandleSkillCheckNarration_NotFound(t *testing.T) {
	n := newStubNotifier()
	r := newDMQueueTestRouter(n)
	form := url.Values{}
	form.Set("narration", "x")
	req := requestWithUser(http.MethodPost, "/dashboard/queue/missing/narrate", form.Encode())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDMQueuePage_HandleSkillCheckNarration_BackendError(t *testing.T) {
	n := newStubNotifier()
	it := newSkillCheckItem()
	n.items[it.ID] = it
	n.resolveSkillCheckNarrErr = errors.New("boom")

	r := newDMQueueTestRouter(n)
	form := url.Values{}
	form.Set("narration", "x")
	req := requestWithUser(http.MethodPost, "/dashboard/queue/sc1/narrate", form.Encode())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
