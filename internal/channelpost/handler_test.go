package channelpost

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func testRouter(svc *Service) *chi.Mux {
	r := chi.NewRouter()
	NewHandler(svc).RegisterRoutes(r)
	return r
}

func TestHandler_Post_Created(t *testing.T) {
	poster := &fakePoster{ids: []string{"m-1"}}
	svc := newSvc(map[string]string{"in-character": "c-1"}, poster)
	body, _ := json.Marshal(map[string]string{
		"campaign_id": uuid.New().String(), "channel": "in-character", "body": "OOC test",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/channel/post", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	testRouter(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("code = %d body=%s", rec.Code, rec.Body.String())
	}
	var res PostResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.ChannelID != "c-1" || res.Channel != "in-character" {
		t.Fatalf("res = %+v", res)
	}
}

func TestHandler_Post_InvalidJSON(t *testing.T) {
	svc := newSvc(map[string]string{"x": "c"}, &fakePoster{})
	req := httptest.NewRequest(http.MethodPost, "/api/channel/post", strings.NewReader("{"))
	rec := httptest.NewRecorder()
	testRouter(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestHandler_Post_InvalidCampaignID(t *testing.T) {
	svc := newSvc(map[string]string{"x": "c"}, &fakePoster{})
	body, _ := json.Marshal(map[string]string{"campaign_id": "nope", "channel": "x", "body": "hi"})
	req := httptest.NewRequest(http.MethodPost, "/api/channel/post", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	testRouter(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestHandler_Post_UnknownChannel400(t *testing.T) {
	svc := newSvc(map[string]string{"in-character": "c-1"}, &fakePoster{})
	body, _ := json.Marshal(map[string]string{"campaign_id": uuid.New().String(), "channel": "combat-log", "body": "hi"})
	req := httptest.NewRequest(http.MethodPost, "/api/channel/post", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	testRouter(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_List_OK(t *testing.T) {
	svc := newSvc(map[string]string{"the-story": "c1", "in-character": "c2"}, &fakePoster{})
	req := httptest.NewRequest(http.MethodGet, "/api/channel/list?campaign_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	testRouter(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var out struct {
		Channels []string `json:"channels"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Channels) != 2 || out.Channels[0] != "in-character" {
		t.Fatalf("channels = %v", out.Channels)
	}
}

func TestHandler_List_MissingCampaignID(t *testing.T) {
	svc := newSvc(map[string]string{"x": "c"}, &fakePoster{})
	req := httptest.NewRequest(http.MethodGet, "/api/channel/list", nil)
	rec := httptest.NewRecorder()
	testRouter(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestHandler_List_InvalidCampaignID(t *testing.T) {
	svc := newSvc(map[string]string{"x": "c"}, &fakePoster{})
	req := httptest.NewRequest(http.MethodGet, "/api/channel/list?campaign_id=nope", nil)
	rec := httptest.NewRecorder()
	testRouter(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestHandler_List_InternalError(t *testing.T) {
	svc := NewService(fakeLookup{err: errors.New("db down")}, &fakePoster{})
	req := httptest.NewRequest(http.MethodGet, "/api/channel/list?campaign_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	testRouter(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d", rec.Code)
	}
}
