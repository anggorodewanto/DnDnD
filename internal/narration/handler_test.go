package narration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/auth"
)

func newTestHandler(t *testing.T) (*Handler, *fakeStore, *fakePoster) {
	t.Helper()
	store := &fakeStore{}
	poster := &fakePoster{returnIDs: []string{"m-1"}}
	svc := NewService(store, poster, &fakeAttachments{urls: map[uuid.UUID]string{}}, &fakeCampaigns{guildID: "g1"})
	return NewHandler(svc), store, poster
}

func TestHandler_Preview_RendersBody(t *testing.T) {
	h, _, _ := newTestHandler(t)
	body := `{"body":":::read-aloud\nBoxed text.\n:::\nAfter."}`
	req := httptest.NewRequest(http.MethodPost, "/api/narration/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Preview(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var resp DiscordMessage
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Embeds) != 1 || resp.Embeds[0].Description != "Boxed text." {
		t.Fatalf("embeds = %+v", resp.Embeds)
	}
	if !strings.Contains(resp.Body, "After.") {
		t.Fatalf("body missing after: %q", resp.Body)
	}
}

func TestHandler_Preview_BadJSON(t *testing.T) {
	h, _, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/narration/preview", strings.NewReader("{bad"))
	rr := httptest.NewRecorder()
	h.Preview(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_Post_Success(t *testing.T) {
	h, store, poster := newTestHandler(t)
	campID := uuid.New()

	payload := map[string]any{
		"campaign_id":    campID.String(),
		"author_user_id": "dm-1",
		"body":           "hello there",
	}
	buf, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/narration/post", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-1"))
	rr := httptest.NewRecorder()

	h.Post(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if len(store.inserted) != 1 {
		t.Fatalf("expected insert, got %d", len(store.inserted))
	}
	if len(poster.calls) != 1 {
		t.Fatalf("expected 1 poster call")
	}
}

func TestHandler_Post_RejectsInvalidUUID(t *testing.T) {
	h, _, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/narration/post", strings.NewReader(`{"campaign_id":"not-a-uuid","author_user_id":"u","body":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Post(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_Post_ServiceErrorIsBadRequest(t *testing.T) {
	h, _, _ := newTestHandler(t)
	// Empty body triggers ErrInvalidInput
	req := httptest.NewRequest(http.MethodPost, "/api/narration/post", strings.NewReader(`{"campaign_id":"`+uuid.New().String()+`","author_user_id":"u","body":""}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Post(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_History_ReturnsPosts(t *testing.T) {
	store := &fakeStore{list: []Post{
		{ID: uuid.New(), CampaignID: uuid.New(), Body: "a", PostedAt: time.Now(), AttachmentAssetIDs: []uuid.UUID{}, DiscordMessageIDs: []string{"x"}},
	}}
	svc := NewService(store, &fakePoster{}, &fakeAttachments{}, &fakeCampaigns{})
	h := NewHandler(svc)

	campID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/narration/history?campaign_id="+campID.String()+"&limit=5", nil)
	rr := httptest.NewRecorder()
	h.History(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var got []Post
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Body != "a" {
		t.Fatalf("got = %+v", got)
	}
}

func TestHandler_Post_BadJSON(t *testing.T) {
	h, _, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/narration/post", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Post(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_Post_BadAttachmentUUID(t *testing.T) {
	h, _, _ := newTestHandler(t)
	body := `{"campaign_id":"` + uuid.New().String() + `","author_user_id":"u","body":"b","attachment_asset_ids":["nope"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/narration/post", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Post(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandler_Post_WithAttachments(t *testing.T) {
	assetID := uuid.New()
	store := &fakeStore{}
	poster := &fakePoster{returnIDs: []string{"m"}}
	att := &fakeAttachments{urls: map[uuid.UUID]string{assetID: "/api/assets/" + assetID.String()}}
	svc := NewService(store, poster, att, &fakeCampaigns{guildID: "g1"})
	h := NewHandler(svc)

	body := `{"campaign_id":"` + uuid.New().String() + `","author_user_id":"u","body":"b","attachment_asset_ids":["` + assetID.String() + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/narration/post", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "u"))
	rr := httptest.NewRecorder()
	h.Post(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if len(store.inserted) != 1 || len(store.inserted[0].AttachmentAssetIDs) != 1 {
		t.Fatalf("insert mismatch: %+v", store.inserted)
	}
}

func TestHandler_History_InvalidCampaignID(t *testing.T) {
	h, _, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/narration/history?campaign_id=not-a-uuid", nil)
	rr := httptest.NewRecorder()
	h.History(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_History_StoreError(t *testing.T) {
	store := &fakeStore{listErr: errors.New("db down")}
	svc := NewService(store, &fakePoster{}, &fakeAttachments{}, &fakeCampaigns{})
	h := NewHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/narration/history?campaign_id="+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	h.History(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_History_MissingCampaignID(t *testing.T) {
	h, _, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/narration/history", nil)
	rr := httptest.NewRecorder()
	h.History(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_RegisterRoutes_Mounts(t *testing.T) {
	h, _, _ := newTestHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	// Ping preview
	req := httptest.NewRequest(http.MethodPost, "/api/narration/preview", strings.NewReader(`{"body":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
}

// sanity check unused import
var _ = context.Background

func TestHandler_History_ForbiddenWhenNotCampaignDM(t *testing.T) {
	campA := uuid.New()
	campB := uuid.New()

	verifier := &fakeCampaignVerifierN{ownedCampaign: campA.String()}
	store := &fakeStore{list: []Post{{ID: uuid.New(), Body: "x", PostedAt: time.Now(), AttachmentAssetIDs: []uuid.UUID{}, DiscordMessageIDs: []string{"m"}}}}
	svc := NewService(store, &fakePoster{}, &fakeAttachments{}, &fakeCampaigns{})
	h := NewHandler(svc, WithCampaignVerifier(verifier))

	req := httptest.NewRequest(http.MethodGet, "/api/narration/history?campaign_id="+campB.String(), nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-1"))
	rr := httptest.NewRecorder()
	h.History(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandler_History_AllowedWhenCampaignDM(t *testing.T) {
	campA := uuid.New()

	verifier := &fakeCampaignVerifierN{ownedCampaign: campA.String()}
	store := &fakeStore{list: []Post{{ID: uuid.New(), Body: "x", PostedAt: time.Now(), AttachmentAssetIDs: []uuid.UUID{}, DiscordMessageIDs: []string{"m"}}}}
	svc := NewService(store, &fakePoster{}, &fakeAttachments{}, &fakeCampaigns{})
	h := NewHandler(svc, WithCampaignVerifier(verifier))

	req := httptest.NewRequest(http.MethodGet, "/api/narration/history?campaign_id="+campA.String(), nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-1"))
	rr := httptest.NewRecorder()
	h.History(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandler_Post_UsesContextUserIDNotBody(t *testing.T) {
	h, store, _ := newTestHandler(t)
	campID := uuid.New()

	payload := map[string]any{
		"campaign_id":    campID.String(),
		"author_user_id": "fake-user",
		"body":           "hello",
	}
	buf, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/narration/post", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "real-user"))
	rr := httptest.NewRecorder()

	h.Post(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if len(store.inserted) != 1 {
		t.Fatalf("expected 1 insert, got %d", len(store.inserted))
	}
	if store.inserted[0].AuthorUserID != "real-user" {
		t.Fatalf("expected author_user_id='real-user', got %q", store.inserted[0].AuthorUserID)
	}
}

type fakeCampaignVerifierN struct {
	ownedCampaign string
}

func (f *fakeCampaignVerifierN) IsCampaignDM(_ context.Context, _, campaignID string) (bool, error) {
	return f.ownedCampaign == campaignID, nil
}
