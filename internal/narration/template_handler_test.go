package narration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func newTestTemplateHandler(t *testing.T) (*TemplateHandler, *fakeTemplateStore) {
	t.Helper()
	store := newFakeTemplateStore()
	svc := NewTemplateService(store)
	return NewTemplateHandler(svc), store
}

func TestTemplateHandler_Create_Success(t *testing.T) {
	h, store := newTestTemplateHandler(t)
	camp := uuid.New()
	body := map[string]any{
		"campaign_id": camp.String(),
		"name":        "Tavern",
		"category":    "Locations",
		"body":        "Welcome to {place}.",
	}
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/narration/templates", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if len(store.templates) != 1 {
		t.Fatalf("expected stored template")
	}
}

func TestTemplateHandler_Create_BadJSON(t *testing.T) {
	h, _ := newTestTemplateHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/narration/templates", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTemplateHandler_Create_InvalidCampaignID(t *testing.T) {
	h, _ := newTestTemplateHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/narration/templates", strings.NewReader(`{"campaign_id":"x","name":"n","body":"b"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTemplateHandler_Create_ValidationError(t *testing.T) {
	h, _ := newTestTemplateHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/narration/templates", strings.NewReader(`{"campaign_id":"`+uuid.New().String()+`","name":"","body":"b"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTemplateHandler_List_ReturnsTemplates(t *testing.T) {
	h, store := newTestTemplateHandler(t)
	camp := uuid.New()
	store.templates[uuid.New()] = Template{ID: uuid.New(), CampaignID: camp, Name: "x", Body: "b"}

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/narration/templates?campaign_id="+camp.String()+"&category=Combat&q=foo", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var got []Template
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
	if store.lastFilter.Category != "Combat" || store.lastFilter.Search != "foo" {
		t.Fatalf("filter passthrough wrong: %+v", store.lastFilter)
	}
}

func TestTemplateHandler_List_MissingCampaignID(t *testing.T) {
	h, _ := newTestTemplateHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/narration/templates", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTemplateHandler_List_InvalidCampaignID(t *testing.T) {
	h, _ := newTestTemplateHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/narration/templates?campaign_id=nope", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTemplateHandler_Get_Success(t *testing.T) {
	h, store := newTestTemplateHandler(t)
	id := uuid.New()
	camp := uuid.New()
	store.templates[id] = Template{ID: id, CampaignID: camp, Name: "x", Body: "b"}

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/narration/templates/"+id.String()+"?campaign_id="+camp.String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestTemplateHandler_Get_NotFound(t *testing.T) {
	h, _ := newTestTemplateHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/narration/templates/"+uuid.New().String()+"?campaign_id="+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTemplateHandler_Get_InvalidID(t *testing.T) {
	h, _ := newTestTemplateHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/narration/templates/not-a-uuid", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTemplateHandler_Update_Success(t *testing.T) {
	h, store := newTestTemplateHandler(t)
	id := uuid.New()
	camp := uuid.New()
	store.templates[id] = Template{ID: id, CampaignID: camp, Name: "old", Body: "b"}

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	body := `{"name":"new","category":"c","body":"new body"}`
	req := httptest.NewRequest(http.MethodPut, "/api/narration/templates/"+id.String()+"?campaign_id="+camp.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if store.templates[id].Name != "new" {
		t.Fatalf("not updated")
	}
}

func TestTemplateHandler_Update_BadJSON(t *testing.T) {
	h, _ := newTestTemplateHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPut, "/api/narration/templates/"+uuid.New().String()+"?campaign_id="+uuid.New().String(), strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTemplateHandler_Update_InvalidID(t *testing.T) {
	h, _ := newTestTemplateHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPut, "/api/narration/templates/x", strings.NewReader(`{"name":"n","body":"b"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTemplateHandler_Update_NotFound(t *testing.T) {
	h, _ := newTestTemplateHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPut, "/api/narration/templates/"+uuid.New().String()+"?campaign_id="+uuid.New().String(), strings.NewReader(`{"name":"n","body":"b"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTemplateHandler_Update_ValidationError(t *testing.T) {
	h, _ := newTestTemplateHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPut, "/api/narration/templates/"+uuid.New().String()+"?campaign_id="+uuid.New().String(), strings.NewReader(`{"name":"","body":"b"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTemplateHandler_Delete_Success(t *testing.T) {
	h, store := newTestTemplateHandler(t)
	id := uuid.New()
	camp := uuid.New()
	store.templates[id] = Template{ID: id, CampaignID: camp}

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodDelete, "/api/narration/templates/"+id.String()+"?campaign_id="+camp.String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rr.Code)
	}
	if _, ok := store.templates[id]; ok {
		t.Fatalf("expected deletion")
	}
}

func TestTemplateHandler_Delete_InvalidID(t *testing.T) {
	h, _ := newTestTemplateHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodDelete, "/api/narration/templates/x", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTemplateHandler_Duplicate_Success(t *testing.T) {
	h, store := newTestTemplateHandler(t)
	id := uuid.New()
	camp := uuid.New()
	store.templates[id] = Template{ID: id, CampaignID: camp, Name: "Tavern", Body: "hi"}

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/api/narration/templates/"+id.String()+"/duplicate?campaign_id="+camp.String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if len(store.templates) != 2 {
		t.Fatalf("expected 2 templates")
	}
}

func TestTemplateHandler_Duplicate_NotFound(t *testing.T) {
	h, _ := newTestTemplateHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/api/narration/templates/"+uuid.New().String()+"/duplicate?campaign_id="+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTemplateHandler_Duplicate_InvalidID(t *testing.T) {
	h, _ := newTestTemplateHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/api/narration/templates/x/duplicate", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTemplateHandler_Apply_Success(t *testing.T) {
	h, store := newTestTemplateHandler(t)
	id := uuid.New()
	camp := uuid.New()
	store.templates[id] = Template{ID: id, CampaignID: camp, Name: "n", Body: "Hello {p}, in {l}."}

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	body := `{"values":{"p":"Aragorn","l":"Bree"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/narration/templates/"+id.String()+"/apply?campaign_id="+camp.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Body string `json:"body"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Body != "Hello Aragorn, in Bree." {
		t.Fatalf("apply body = %q", resp.Body)
	}
}

func TestTemplateHandler_Apply_BadJSON(t *testing.T) {
	h, _ := newTestTemplateHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/api/narration/templates/"+uuid.New().String()+"/apply?campaign_id="+uuid.New().String(), strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTemplateHandler_Apply_InvalidID(t *testing.T) {
	h, _ := newTestTemplateHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/api/narration/templates/x/apply", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTemplateHandler_Apply_NotFound(t *testing.T) {
	h, _ := newTestTemplateHandler(t)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/api/narration/templates/"+uuid.New().String()+"/apply?campaign_id="+uuid.New().String(), strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rr.Code)
	}
}
