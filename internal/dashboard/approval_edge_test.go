package dashboard

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestApproveCharacter_DetailNotFound(t *testing.T) {
	store := &mockApprovalStore{
		detailErr: fmt.Errorf("not found"),
	}
	_, r := setupApprovalTest(store, &mockNotifier{})

	id := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/approve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestApproveCharacter_InvalidID(t *testing.T) {
	store := &mockApprovalStore{}
	_, r := setupApprovalTest(store, &mockNotifier{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/not-uuid/approve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRequestChanges_DetailNotFound(t *testing.T) {
	store := &mockApprovalStore{
		detailErr: fmt.Errorf("not found"),
	}
	_, r := setupApprovalTest(store, &mockNotifier{})

	id := uuid.New()
	body := `{"feedback":"Fix it"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/request-changes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRequestChanges_StoreError(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	store := &mockApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{ID: id, CampaignID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), CharacterName: "Gandalf", DiscordUserID: "player1"},
		},
		requestErr: fmt.Errorf("db error"),
	}
	_, r := setupApprovalTest(store, &mockNotifier{})

	body := `{"feedback":"Fix it"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/request-changes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRequestChanges_InvalidID(t *testing.T) {
	store := &mockApprovalStore{}
	_, r := setupApprovalTest(store, &mockNotifier{})

	body := `{"feedback":"Fix it"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/not-uuid/request-changes", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRequestChanges_InvalidBody(t *testing.T) {
	store := &mockApprovalStore{}
	_, r := setupApprovalTest(store, &mockNotifier{})

	id := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/request-changes", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRejectCharacter_DetailNotFound(t *testing.T) {
	store := &mockApprovalStore{
		detailErr: fmt.Errorf("not found"),
	}
	_, r := setupApprovalTest(store, &mockNotifier{})

	id := uuid.New()
	body := `{"feedback":"No"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/reject", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRejectCharacter_StoreError(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	store := &mockApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{ID: id, CampaignID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), CharacterName: "Gandalf", DiscordUserID: "player1"},
		},
		rejectErr: fmt.Errorf("db error"),
	}
	_, r := setupApprovalTest(store, &mockNotifier{})

	body := `{"feedback":"No"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/reject", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRejectCharacter_InvalidID(t *testing.T) {
	store := &mockApprovalStore{}
	_, r := setupApprovalTest(store, &mockNotifier{})

	body := `{"feedback":"No"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/not-uuid/reject", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRejectCharacter_InvalidBody(t *testing.T) {
	store := &mockApprovalStore{}
	_, r := setupApprovalTest(store, &mockNotifier{})

	id := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/reject", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestApproveCharacter_NilNotifier(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	store := &mockApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{
				ID:            id,
				CampaignID:    uuid.MustParse("00000000-0000-0000-0000-000000000001"),
				CharacterName: "Gandalf",
				DiscordUserID: "player1",
			},
		},
	}
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()
	campaignID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ah := NewApprovalHandler(nil, store, nil, hub, campaignID, nil)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(contextWithUser(r.Context(), "dm-user"))
			next.ServeHTTP(w, r)
		})
	})
	ah.RegisterApprovalRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/approve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestDBApprovalStore_TransitionStatus_GetPCError(t *testing.T) {
	fq := &fakeQueries{
		getPCErr: fmt.Errorf("db error"),
	}
	store := NewDBApprovalStore(fq)
	err := store.ApproveCharacter(t.Context(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting player character")
}

func TestDBApprovalStore_TransitionStatus_UpdateError(t *testing.T) {
	id := uuid.New()
	fq := &fakeQueries{
		getPC:     refdata.PlayerCharacter{ID: id, Status: "pending"},
		updateErr: fmt.Errorf("update failed"),
	}
	store := NewDBApprovalStore(fq)
	err := store.ApproveCharacter(t.Context(), id)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating status")
}

func TestDBApprovalStore_RequestChanges_InvalidTransition(t *testing.T) {
	id := uuid.New()
	fq := &fakeQueries{
		getPC: refdata.PlayerCharacter{ID: id, Status: "approved"},
	}
	store := NewDBApprovalStore(fq)
	err := store.RequestChanges(t.Context(), id, "Fix it")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status transition")
}

func TestDBApprovalStore_RejectCharacter_InvalidTransition(t *testing.T) {
	id := uuid.New()
	fq := &fakeQueries{
		getPC: refdata.PlayerCharacter{ID: id, Status: "rejected"},
	}
	store := NewDBApprovalStore(fq)
	err := store.RejectCharacter(t.Context(), id, "No")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status transition")
}
