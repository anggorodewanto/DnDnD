package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockApprovalStore implements ApprovalStore for testing.
type mockApprovalStore struct {
	entries       []ApprovalEntry
	detail        *ApprovalDetail
	listErr       error
	detailErr     error
	approveErr    error
	requestErr    error
	rejectErr     error
	approvedID    uuid.UUID
	requestedID   uuid.UUID
	requestedFB   string
	rejectedID    uuid.UUID
	rejectedFB    string
}

func (m *mockApprovalStore) ListPendingApprovals(_ context.Context, _ uuid.UUID) ([]ApprovalEntry, error) {
	return m.entries, m.listErr
}

func (m *mockApprovalStore) GetApprovalDetail(_ context.Context, id uuid.UUID) (*ApprovalDetail, error) {
	return m.detail, m.detailErr
}

func (m *mockApprovalStore) ApproveCharacter(_ context.Context, id uuid.UUID) error {
	m.approvedID = id
	return m.approveErr
}

func (m *mockApprovalStore) RequestChanges(_ context.Context, id uuid.UUID, feedback string) error {
	m.requestedID = id
	m.requestedFB = feedback
	return m.requestErr
}

func (m *mockApprovalStore) RetireCharacter(_ context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockApprovalStore) RejectCharacter(_ context.Context, id uuid.UUID, feedback string) error {
	m.rejectedID = id
	m.rejectedFB = feedback
	return m.rejectErr
}

// mockNotifier implements PlayerNotifier for testing.
type mockNotifier struct {
	approvalCalls       int
	changesCalls        int
	rejectionCalls      int
	lastDiscordUserID   string
	lastCharacterName   string
	lastFeedback        string
}

func (m *mockNotifier) NotifyApproval(_ context.Context, discordUserID, characterName string) error {
	m.approvalCalls++
	m.lastDiscordUserID = discordUserID
	m.lastCharacterName = characterName
	return nil
}

func (m *mockNotifier) NotifyChangesRequested(_ context.Context, discordUserID, characterName, feedback string) error {
	m.changesCalls++
	m.lastDiscordUserID = discordUserID
	m.lastCharacterName = characterName
	m.lastFeedback = feedback
	return nil
}

func (m *mockNotifier) NotifyRejection(_ context.Context, discordUserID, characterName, feedback string) error {
	m.rejectionCalls++
	m.lastDiscordUserID = discordUserID
	m.lastCharacterName = characterName
	m.lastFeedback = feedback
	return nil
}

func setupApprovalTest(store ApprovalStore, notifier PlayerNotifier) (*ApprovalHandler, chi.Router) {
	hub := NewHub()
	go hub.Run()
	campaignID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ah := NewApprovalHandler(nil, store, notifier, hub, campaignID)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(contextWithUser(r.Context(), "dm-user"))
			next.ServeHTTP(w, r)
		})
	})
	ah.RegisterApprovalRoutes(r)
	return ah, r
}

func TestListApprovals_ReturnsJSON(t *testing.T) {
	store := &mockApprovalStore{
		entries: []ApprovalEntry{
			{
				ID:            uuid.MustParse("00000000-0000-0000-0000-000000000010"),
				CharacterName: "Gandalf",
				DiscordUserID: "player1",
				Status:        "pending",
				CreatedVia:    "import",
			},
		},
	}
	_, r := setupApprovalTest(store, &mockNotifier{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/approvals", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	var result []ApprovalEntry
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "Gandalf", result[0].CharacterName)
}

func TestListApprovals_EmptyList(t *testing.T) {
	store := &mockApprovalStore{entries: []ApprovalEntry{}}
	_, r := setupApprovalTest(store, &mockNotifier{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/approvals", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result []ApprovalEntry
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestListApprovals_StoreError(t *testing.T) {
	store := &mockApprovalStore{listErr: fmt.Errorf("db down")}
	_, r := setupApprovalTest(store, &mockNotifier{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/approvals", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestGetApprovalDetail_ReturnsJSON(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	store := &mockApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{
				ID:            id,
				CharacterName: "Gandalf",
				DiscordUserID: "player1",
				Status:        "pending",
				CreatedVia:    "import",
			},
			Race:    "Human",
			Level:   5,
			Classes: `[{"class":"wizard","level":5}]`,
			HpMax:   32,
			Ac:      12,
		},
	}
	_, r := setupApprovalTest(store, &mockNotifier{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/approvals/"+id.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result ApprovalDetail
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "Gandalf", result.CharacterName)
	assert.Equal(t, "Human", result.Race)
}

func TestGetApprovalDetail_NotFound(t *testing.T) {
	store := &mockApprovalStore{detailErr: fmt.Errorf("not found")}
	_, r := setupApprovalTest(store, &mockNotifier{})

	id := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/approvals/"+id.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetApprovalDetail_InvalidID(t *testing.T) {
	store := &mockApprovalStore{}
	_, r := setupApprovalTest(store, &mockNotifier{})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/approvals/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestApproveCharacter_Success(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	store := &mockApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{
				ID:            id,
				CharacterName: "Gandalf",
				DiscordUserID: "player1",
				Status:        "pending",
			},
		},
	}
	notifier := &mockNotifier{}
	_, r := setupApprovalTest(store, notifier)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/approve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, id, store.approvedID)
	assert.Equal(t, 1, notifier.approvalCalls)
}

func TestApproveCharacter_StoreError(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	store := &mockApprovalStore{
		approveErr: fmt.Errorf("transition failed"),
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{ID: id, CharacterName: "Gandalf", DiscordUserID: "player1"},
		},
	}
	_, r := setupApprovalTest(store, &mockNotifier{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/approve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRequestChanges_Success(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	store := &mockApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{
				ID:            id,
				CharacterName: "Gandalf",
				DiscordUserID: "player1",
			},
		},
	}
	notifier := &mockNotifier{}
	_, r := setupApprovalTest(store, notifier)

	body := `{"feedback":"Please fix your HP"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/request-changes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, id, store.requestedID)
	assert.Equal(t, "Please fix your HP", store.requestedFB)
	assert.Equal(t, 1, notifier.changesCalls)
	assert.Equal(t, "Please fix your HP", notifier.lastFeedback)
}

func TestRequestChanges_MissingFeedback(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	store := &mockApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{ID: id, CharacterName: "Gandalf", DiscordUserID: "player1"},
		},
	}
	_, r := setupApprovalTest(store, &mockNotifier{})

	body := `{"feedback":""}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/request-changes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRejectCharacter_Success(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	store := &mockApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{
				ID:            id,
				CharacterName: "Gandalf",
				DiscordUserID: "player1",
			},
		},
	}
	notifier := &mockNotifier{}
	_, r := setupApprovalTest(store, notifier)

	body := `{"feedback":"Character not allowed"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/reject", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, id, store.rejectedID)
	assert.Equal(t, "Character not allowed", store.rejectedFB)
	assert.Equal(t, 1, notifier.rejectionCalls)
}

func TestRejectCharacter_MissingFeedback(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	store := &mockApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{ID: id, CharacterName: "Gandalf", DiscordUserID: "player1"},
		},
	}
	_, r := setupApprovalTest(store, &mockNotifier{})

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/reject", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestApprovalEndpoints_RequireAuth(t *testing.T) {
	store := &mockApprovalStore{}
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()
	campaignID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ah := NewApprovalHandler(nil, store, &mockNotifier{}, hub, campaignID)

	// Router WITHOUT auth middleware
	r := chi.NewRouter()
	ah.RegisterApprovalRoutes(r)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/dashboard/api/approvals"},
		{http.MethodGet, "/dashboard/api/approvals/" + uuid.New().String()},
		{http.MethodPost, "/dashboard/api/approvals/" + uuid.New().String() + "/approve"},
		{http.MethodPost, "/dashboard/api/approvals/" + uuid.New().String() + "/request-changes"},
		{http.MethodPost, "/dashboard/api/approvals/" + uuid.New().String() + "/reject"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}
