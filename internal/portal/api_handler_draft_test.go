package portal_test

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/portal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newDraftAPIHandler wires an APIHandler over the given builder store so the
// draft endpoints exercise the full handler -> service -> store path.
func newDraftAPIHandler(store portal.BuilderStore) *portal.APIHandler {
	return portal.NewAPIHandler(slog.Default(), &mockRefDataStore{}, portal.NewBuilderService(store))
}

func getDraftRequest(t *testing.T, userID, campaignID string) *http.Request {
	t.Helper()
	url := "/portal/api/characters/draft"
	if campaignID != "" {
		url += "?campaign_id=" + campaignID
	}
	req := httptest.NewRequest(http.MethodGet, url, nil)
	if userID != "" {
		req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), userID))
	}
	return req
}

func TestAPIHandler_GetCharacterDraft_Present(t *testing.T) {
	stored := json.RawMessage(`{"v":1,"name":"Gimli","race":"dwarf"}`)
	h := newDraftAPIHandler(&mockBuilderStore{loadDraftResult: stored})

	rec := httptest.NewRecorder()
	h.GetCharacterDraft(rec, getDraftRequest(t, "u1", "camp-1"))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Draft json.RawMessage `json:"draft"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.JSONEq(t, string(stored), string(body.Draft))
}

func TestAPIHandler_GetCharacterDraft_Absent(t *testing.T) {
	h := newDraftAPIHandler(&mockBuilderStore{loadDraftResult: nil})

	rec := httptest.NewRecorder()
	h.GetCharacterDraft(rec, getDraftRequest(t, "u1", "camp-1"))

	require.Equal(t, http.StatusOK, rec.Code)
	// No draft must serialize as {"draft":null} so the client treats it as
	// "nothing stored" rather than an error.
	assert.JSONEq(t, `{"draft":null}`, rec.Body.String())
}

func TestAPIHandler_GetCharacterDraft_MissingCampaign(t *testing.T) {
	h := newDraftAPIHandler(&mockBuilderStore{})

	rec := httptest.NewRecorder()
	h.GetCharacterDraft(rec, getDraftRequest(t, "u1", ""))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPIHandler_GetCharacterDraft_Unauthenticated(t *testing.T) {
	h := newDraftAPIHandler(&mockBuilderStore{})

	rec := httptest.NewRecorder()
	h.GetCharacterDraft(rec, getDraftRequest(t, "", "camp-1"))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAPIHandler_GetCharacterDraft_StoreError(t *testing.T) {
	h := newDraftAPIHandler(&mockBuilderStore{loadDraftErr: errors.New("db down")})

	rec := httptest.NewRecorder()
	h.GetCharacterDraft(rec, getDraftRequest(t, "u1", "camp-1"))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

const draftSubmitBody = `{"token":"t","campaign_id":"camp-1","builder_draft":{"v":1,"name":"Test","race":"elf"},` +
	`"name":"Test","race":"elf","background":"sage","class":"wizard",` +
	`"ability_scores":{"str":8,"dex":8,"con":8,"int":8,"wis":8,"cha":8},"skills":[]}`

func submitDraftRequest(t *testing.T, userID, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/portal/api/characters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req.WithContext(auth.ContextWithDiscordUserID(req.Context(), userID))
}

// A submit must persist the builder draft even when the token is expired, so a
// reissued /create-character link rehydrates the work instead of a blank form
// (T11 / Finding 4·b — the lost-work scenario this fix exists for).
func TestAPIHandler_SubmitCharacter_PersistsDraft_OnTokenExpiry(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", validateTokenErr: portal.ErrTokenExpired}
	h := newDraftAPIHandler(store)

	rec := httptest.NewRecorder()
	h.SubmitCharacter(rec, submitDraftRequest(t, "u1", draftSubmitBody))

	// Still the T10 client-error mapping, never a bare 500.
	require.Equal(t, http.StatusGone, rec.Code, rec.Body.String())
	assert.True(t, store.saveDraftCalled, "draft must be saved despite the token failure")
	assert.Equal(t, "camp-1", store.lastDraftCamp)
	assert.Equal(t, "u1", store.lastDraftUser)
	assert.Equal(t, "player", store.lastDraftMode)
	assert.JSONEq(t, `{"v":1,"name":"Test","race":"elf"}`, string(store.lastDraftBlob))
}

func TestAPIHandler_SubmitCharacter_PersistsDraft_OnSuccess(t *testing.T) {
	store := &mockBuilderStore{
		charID:        "c-1",
		pcID:          "pc-1",
		validateToken: &portal.PortalToken{DiscordUserID: "u1"},
	}
	h := newDraftAPIHandler(store)

	rec := httptest.NewRecorder()
	h.SubmitCharacter(rec, submitDraftRequest(t, "u1", draftSubmitBody))

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.True(t, store.saveDraftCalled)
	assert.JSONEq(t, `{"v":1,"name":"Test","race":"elf"}`, string(store.lastDraftBlob))
}

// A submit without a builder_draft field must not touch the draft store: there
// is nothing to preserve, and we avoid writing an empty row.
func TestAPIHandler_SubmitCharacter_NoDraftNoSave(t *testing.T) {
	const body = `{"token":"t","campaign_id":"camp-1","name":"Test","race":"elf","background":"sage",` +
		`"class":"wizard","ability_scores":{"str":8,"dex":8,"con":8,"int":8,"wis":8,"cha":8},"skills":[]}`
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1", validateToken: &portal.PortalToken{DiscordUserID: "u1"}}
	h := newDraftAPIHandler(store)

	rec := httptest.NewRecorder()
	h.SubmitCharacter(rec, submitDraftRequest(t, "u1", body))

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.False(t, store.saveDraftCalled, "no builder_draft means no draft write")
}
