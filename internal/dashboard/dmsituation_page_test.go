package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/situation"
)

// stubSituationBuilder returns a fixed Situation and records the campaign id it
// was built for, so tests can assert per-request campaign scoping.
type stubSituationBuilder struct {
	sit      situation.Situation
	buildErr error
	gotID    string
}

func (s *stubSituationBuilder) Build(_ context.Context, campaignID string) (situation.Situation, error) {
	s.gotID = campaignID
	return s.sit, s.buildErr
}

// newSituationRequest builds a GET request carrying an authenticated DM user.
func newSituationRequest(userID string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/dm/situation", nil)
	if userID != "" {
		req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), userID))
	}
	return req
}

func decodeSituation(t *testing.T, rec *httptest.ResponseRecorder) situation.Situation {
	t.Helper()
	var got situation.Situation
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	return got
}

func TestServeSituation_ReturnsAggregatedViewForCampaign(t *testing.T) {
	builder := &stubSituationBuilder{
		sit: situation.Situation{
			Pending: []situation.PendingItem{
				{ID: "q1", Source: situation.SourceQueue, Label: "Player Whisper", Player: "Forge"},
			},
			State:    situation.StateView{HasEncounter: true, Round: 1, CurrentTurn: "Forge"},
			NextStep: "Resolve Player Whisper from Forge.",
		},
	}
	h := NewDMSituationHandler(nil)
	h.SetBuilder(builder)
	h.SetCampaignLookup(fixedCampaignLookup{id: "camp-9"})

	rec := httptest.NewRecorder()
	h.ServeSituation(rec, newSituationRequest("dm-1"))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, "camp-9", builder.gotID, "builder must be scoped to the DM's active campaign")

	got := decodeSituation(t, rec)
	require.Len(t, got.Pending, 1)
	assert.Equal(t, "Forge", got.Pending[0].Player)
	assert.True(t, got.State.HasEncounter)
	assert.Equal(t, "Resolve Player Whisper from Forge.", got.NextStep)
}

func TestServeSituation_Unauthorized(t *testing.T) {
	h := NewDMSituationHandler(nil)
	h.SetBuilder(&stubSituationBuilder{})
	h.SetCampaignLookup(fixedCampaignLookup{id: "camp-9"})

	rec := httptest.NewRecorder()
	h.ServeSituation(rec, newSituationRequest("")) // no auth user

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestServeSituation_EmptyWhenUnwired(t *testing.T) {
	h := NewDMSituationHandler(nil) // no builder, no lookup
	rec := httptest.NewRecorder()
	h.ServeSituation(rec, newSituationRequest("dm-1"))

	require.Equal(t, http.StatusOK, rec.Code)
	got := decodeSituation(t, rec)
	assert.NotNil(t, got.Pending, "Pending must serialize as [] not null")
	assert.NotNil(t, got.Timeline, "Timeline must serialize as [] not null")
	assert.False(t, got.State.HasEncounter)
}

func TestServeSituation_EmptyWhenNoActiveCampaign(t *testing.T) {
	h := NewDMSituationHandler(nil)
	h.SetBuilder(&stubSituationBuilder{sit: situation.Situation{NextStep: "should not appear"}})
	h.SetCampaignLookup(fixedCampaignLookup{id: ""}) // DM has no active campaign

	rec := httptest.NewRecorder()
	h.ServeSituation(rec, newSituationRequest("dm-1"))

	require.Equal(t, http.StatusOK, rec.Code)
	got := decodeSituation(t, rec)
	assert.Empty(t, got.NextStep, "no active campaign → empty situation, builder not consulted")
}

func TestServeSituation_LookupErrorDegradesToEmpty(t *testing.T) {
	h := NewDMSituationHandler(nil)
	h.SetBuilder(&stubSituationBuilder{sit: situation.Situation{NextStep: "should not appear"}})
	h.SetCampaignLookup(fixedCampaignLookup{err: errors.New("db down")})

	rec := httptest.NewRecorder()
	h.ServeSituation(rec, newSituationRequest("dm-1"))

	require.Equal(t, http.StatusOK, rec.Code)
	got := decodeSituation(t, rec)
	assert.Empty(t, got.NextStep)
}

func TestServeSituation_PartialBuildErrorStillServes(t *testing.T) {
	builder := &stubSituationBuilder{
		sit:      situation.Situation{Pending: []situation.PendingItem{{ID: "q1", Player: "Forge"}}},
		buildErr: errors.New("one source failed"),
	}
	h := NewDMSituationHandler(nil)
	h.SetBuilder(builder)
	h.SetCampaignLookup(fixedCampaignLookup{id: "camp-9"})

	rec := httptest.NewRecorder()
	h.ServeSituation(rec, newSituationRequest("dm-1"))

	require.Equal(t, http.StatusOK, rec.Code, "partial build error must still serve the partial view")
	got := decodeSituation(t, rec)
	require.Len(t, got.Pending, 1)
}

// TestRegisterDMSituationRoutes_MountsBehindAuth verifies the route is wired at
// the documented path and runs the provided auth middleware.
func TestRegisterDMSituationRoutes_MountsBehindAuth(t *testing.T) {
	r := chi.NewRouter()
	authRan := false
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			authRan = true
			ctx := auth.ContextWithDiscordUserID(req.Context(), "dm-1")
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
	h := RegisterDMSituationRoutes(r, nil, mw)
	h.SetBuilder(&stubSituationBuilder{})
	h.SetCampaignLookup(fixedCampaignLookup{id: "camp-1"})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/dm/situation", nil))

	assert.True(t, authRan, "auth middleware must run")
	assert.Equal(t, http.StatusOK, rec.Code)
}
