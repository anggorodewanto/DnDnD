package campaign

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// handlerMockStore is a minimal store to exercise the HTTP handler paths.
type handlerMockStore struct {
	getByID      func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error)
	updateStatus func(ctx context.Context, arg refdata.UpdateCampaignStatusParams) (refdata.Campaign, error)
}

func (h *handlerMockStore) CreateCampaign(ctx context.Context, arg refdata.CreateCampaignParams) (refdata.Campaign, error) {
	return refdata.Campaign{}, nil
}
func (h *handlerMockStore) GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error) {
	return refdata.Campaign{}, nil
}
func (h *handlerMockStore) GetCampaignByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
	return h.getByID(ctx, id)
}
func (h *handlerMockStore) UpdateCampaignStatus(ctx context.Context, arg refdata.UpdateCampaignStatusParams) (refdata.Campaign, error) {
	return h.updateStatus(ctx, arg)
}
func (h *handlerMockStore) UpdateCampaignSettings(ctx context.Context, arg refdata.UpdateCampaignSettingsParams) (refdata.Campaign, error) {
	return refdata.Campaign{}, nil
}
func (h *handlerMockStore) UpdateCampaignName(ctx context.Context, arg refdata.UpdateCampaignNameParams) (refdata.Campaign, error) {
	return refdata.Campaign{}, nil
}
func (h *handlerMockStore) ListCampaigns(ctx context.Context) ([]refdata.Campaign, error) {
	return nil, nil
}

func newHandlerTestRouter(svc *Service) chi.Router {
	h := NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

func TestHandler_Pause_Success(t *testing.T) {
	id := uuid.New()
	store := &handlerMockStore{
		getByID: func(ctx context.Context, got uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: got, GuildID: "g1", Status: StatusActive}, nil
		},
		updateStatus: func(ctx context.Context, arg refdata.UpdateCampaignStatusParams) (refdata.Campaign, error) {
			return refdata.Campaign{ID: arg.ID, GuildID: "g1", Status: arg.Status}, nil
		},
	}
	svc := NewService(store, nil)
	r := newHandlerTestRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/campaigns/"+id.String()+"/pause", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var out struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Status != StatusPaused {
		t.Fatalf("status = %q, want paused", out.Status)
	}
}

func TestHandler_Resume_Success(t *testing.T) {
	id := uuid.New()
	store := &handlerMockStore{
		getByID: func(ctx context.Context, got uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: got, GuildID: "g1", Status: StatusPaused}, nil
		},
		updateStatus: func(ctx context.Context, arg refdata.UpdateCampaignStatusParams) (refdata.Campaign, error) {
			return refdata.Campaign{ID: arg.ID, GuildID: "g1", Status: arg.Status}, nil
		},
	}
	svc := NewService(store, nil)
	r := newHandlerTestRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/campaigns/"+id.String()+"/resume", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var out struct {
		Status string `json:"status"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&out)
	if out.Status != StatusActive {
		t.Fatalf("status = %q, want active", out.Status)
	}
}

func TestHandler_Pause_InvalidID(t *testing.T) {
	svc := NewService(&handlerMockStore{}, nil)
	r := newHandlerTestRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/campaigns/not-a-uuid/pause", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_Resume_InvalidID(t *testing.T) {
	svc := NewService(&handlerMockStore{}, nil)
	r := newHandlerTestRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/campaigns/bogus/resume", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_Pause_ServiceError(t *testing.T) {
	id := uuid.New()
	store := &handlerMockStore{
		getByID: func(ctx context.Context, got uuid.UUID) (refdata.Campaign, error) {
			// Already-paused campaign will cause PauseCampaign to return an error.
			return refdata.Campaign{ID: got, GuildID: "g1", Status: StatusPaused}, nil
		},
	}
	svc := NewService(store, nil)
	r := newHandlerTestRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/campaigns/"+id.String()+"/pause", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatalf("expected non-200, got %d", rr.Code)
	}
}

func TestHandler_Resume_ServiceError(t *testing.T) {
	id := uuid.New()
	store := &handlerMockStore{
		getByID: func(ctx context.Context, got uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{ID: got, GuildID: "g1", Status: StatusActive}, nil
		},
	}
	svc := NewService(store, nil)
	r := newHandlerTestRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/campaigns/"+id.String()+"/resume", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatalf("expected non-200, got %d", rr.Code)
	}
}
