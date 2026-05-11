package dashboard

import (
	"context"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock for CharacterCardPoster ---

type mockCardPoster struct {
	postCalls      int
	lastCharID     uuid.UUID
	lastCharName   string
	lastDiscordUID string
	postErr        error

	retireCalls      int
	retireCharID     uuid.UUID
	retireCharName   string
	retireDiscordUID string
	retireErr        error
}

func (m *mockCardPoster) PostCharacterCard(_ context.Context, characterID uuid.UUID, characterName, discordUserID string) error {
	m.postCalls++
	m.lastCharID = characterID
	m.lastCharName = characterName
	m.lastDiscordUID = discordUserID
	return m.postErr
}

func (m *mockCardPoster) UpdateCardRetired(_ context.Context, characterID uuid.UUID, characterName, discordUserID string) error {
	m.retireCalls++
	m.retireCharID = characterID
	m.retireCharName = characterName
	m.retireDiscordUID = discordUserID
	return m.retireErr
}

func (m *mockCardPoster) OnCharacterUpdated(_ context.Context, _ uuid.UUID) error {
	return nil
}

// --- Mock notifier that returns errors ---

type errorNotifier struct {
	approvalErr error
	changesErr  error
	rejectErr   error
}

func (m *errorNotifier) NotifyApproval(_ context.Context, _, _ string) error {
	return m.approvalErr
}

func (m *errorNotifier) NotifyChangesRequested(_ context.Context, _, _, _ string) error {
	return m.changesErr
}

func (m *errorNotifier) NotifyRejection(_ context.Context, _, _, _ string) error {
	return m.rejectErr
}

// setupApprovalTestWithCardPoster is like setupApprovalTest but also sets the card poster.
func setupApprovalTestWithCardPoster(store ApprovalStore, notifier PlayerNotifier, cardPoster CharacterCardPoster) (*ApprovalHandler, chi.Router) {
	hub := NewHub()
	go hub.Run()
	campaignID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ah := NewApprovalHandler(nil, store, notifier, hub, campaignID, cardPoster)
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

// ===== Must-Fix 1: CharacterCardPoster called on approve =====

func TestApprove_PostsCharacterCard(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	charID := uuid.MustParse("00000000-0000-0000-0000-000000000020")
	store := &mockApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{
				ID:            id,
				CharacterID:   charID,
				CharacterName: "Gandalf",
				DiscordUserID: "player1",
				CreatedVia:    "import",
			},
		},
	}
	cardPoster := &mockCardPoster{}
	_, r := setupApprovalTestWithCardPoster(store, &mockNotifier{}, cardPoster)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/approve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, cardPoster.postCalls, "PostCharacterCard should be called on approve")
	assert.Equal(t, charID, cardPoster.lastCharID)
	assert.Equal(t, "Gandalf", cardPoster.lastCharName)
	assert.Equal(t, "player1", cardPoster.lastDiscordUID)
}

func TestApprove_CardPosterError_StillSucceeds(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	store := &mockApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{
				ID:            id,
				CharacterName: "Gandalf",
				DiscordUserID: "player1",
				CreatedVia:    "import",
			},
		},
	}
	cardPoster := &mockCardPoster{postErr: errors.New("discord down")}
	_, r := setupApprovalTestWithCardPoster(store, &mockNotifier{}, cardPoster)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/approve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Should still return OK even if card posting fails
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, cardPoster.postCalls)
}

func TestApprove_NilCardPoster_NoPanic(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	store := &mockApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{
				ID:            id,
				CharacterName: "Gandalf",
				DiscordUserID: "player1",
				CreatedVia:    "import",
			},
		},
	}
	// No card poster set
	_, r := setupApprovalTest(store, &mockNotifier{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/approve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// ===== Must-Fix 3: Retirement approval transitions to "retired" + calls UpdateCardRetired =====

func TestApprove_RetireSubmission_TransitionsToRetired(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	charID := uuid.MustParse("00000000-0000-0000-0000-000000000020")

	retireStore := &mockRetireApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{
				ID:            id,
				CharacterID:   charID,
				CharacterName: "Gandalf",
				DiscordUserID: "player1",
				CreatedVia:    "retire",
			},
		},
	}
	cardPoster := &mockCardPoster{}
	_, r := setupApprovalTestWithCardPoster(retireStore, &mockNotifier{}, cardPoster)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/approve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// Should call RetireCharacter, not ApproveCharacter
	assert.Equal(t, id, retireStore.retiredID, "should call RetireCharacter for retire submissions")
	assert.Equal(t, uuid.UUID{}, retireStore.approvedID, "should NOT call ApproveCharacter for retire submissions")

	// Should call UpdateCardRetired, not PostCharacterCard
	assert.Equal(t, 1, cardPoster.retireCalls, "should call UpdateCardRetired for retire submissions")
	assert.Equal(t, 0, cardPoster.postCalls, "should NOT call PostCharacterCard for retire submissions")
	assert.Equal(t, charID, cardPoster.retireCharID)
}

func TestApprove_RetireSubmission_ReturnsRetiredStatus(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	retireStore := &mockRetireApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{
				ID:            id,
				CharacterName: "Gandalf",
				DiscordUserID: "player1",
				CreatedVia:    "retire",
			},
		},
	}
	_, r := setupApprovalTestWithCardPoster(retireStore, &mockNotifier{}, &mockCardPoster{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/approve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"retired"`)
}

// mockRetireApprovalStore tracks which method was called.
type mockRetireApprovalStore struct {
	detail     *ApprovalDetail
	detailErr  error
	approvedID uuid.UUID
	retiredID  uuid.UUID
	approveErr error
	retireErr  error
}

func (m *mockRetireApprovalStore) ListPendingApprovals(_ context.Context, _ uuid.UUID) ([]ApprovalEntry, error) {
	return nil, nil
}

func (m *mockRetireApprovalStore) GetApprovalDetail(_ context.Context, _ uuid.UUID) (*ApprovalDetail, error) {
	return m.detail, m.detailErr
}

func (m *mockRetireApprovalStore) ApproveCharacter(_ context.Context, id uuid.UUID) error {
	m.approvedID = id
	return m.approveErr
}

func (m *mockRetireApprovalStore) RequestChanges(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

func (m *mockRetireApprovalStore) RejectCharacter(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

func (m *mockRetireApprovalStore) RetireCharacter(_ context.Context, id uuid.UUID) error {
	m.retiredID = id
	return m.retireErr
}

// ===== Must-Fix 4: ServeApprovalPage template error branch =====

func TestServeApprovalPage_TemplateError(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()
	campaignID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	ah := NewApprovalHandler(nil, &mockApprovalStore{}, &mockNotifier{}, hub, campaignID, nil)
	// Inject a broken template that will always fail
	ah.approvalTmpl = template.Must(template.New("broken").Parse("{{ .NonExistent.Method }}"))

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(contextWithUser(r.Context(), "dm-user"))
			next.ServeHTTP(w, r)
		})
	})
	ah.RegisterApprovalRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/approvals", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// ===== Must-Fix 5: Notification errors are logged =====

func TestApprove_NotificationError_IsLogged(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	store := &mockApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{
				ID:            id,
				CharacterName: "Gandalf",
				DiscordUserID: "player1",
				CreatedVia:    "import",
			},
		},
	}
	notifier := &errorNotifier{approvalErr: errors.New("discord unavailable")}

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	hub := NewHub()
	go hub.Run()
	defer hub.Stop()
	campaignID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ah := NewApprovalHandler(logger, store, notifier, hub, campaignID, &mockCardPoster{})

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
	assert.Contains(t, buf.String(), "discord unavailable", "notification error should be logged")
}

func TestRequestChanges_NotificationError_IsLogged(t *testing.T) {
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
	notifier := &errorNotifier{changesErr: errors.New("discord unavailable")}

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	hub := NewHub()
	go hub.Run()
	defer hub.Stop()
	campaignID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ah := NewApprovalHandler(logger, store, notifier, hub, campaignID, nil)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(contextWithUser(r.Context(), "dm-user"))
			next.ServeHTTP(w, r)
		})
	})
	ah.RegisterApprovalRoutes(r)

	body := `{"feedback":"Fix HP"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/request-changes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, buf.String(), "discord unavailable", "notification error should be logged")
}

func TestReject_NotificationError_IsLogged(t *testing.T) {
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
	notifier := &errorNotifier{rejectErr: errors.New("discord unavailable")}

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	hub := NewHub()
	go hub.Run()
	defer hub.Stop()
	campaignID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ah := NewApprovalHandler(logger, store, notifier, hub, campaignID, nil)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(contextWithUser(r.Context(), "dm-user"))
			next.ServeHTTP(w, r)
		})
	})
	ah.RegisterApprovalRoutes(r)

	body := `{"feedback":"Not allowed"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/reject", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, buf.String(), "discord unavailable", "notification error should be logged")
}

func TestApprove_RetireSubmission_StoreError(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	retireStore := &mockRetireApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{
				ID:            id,
				CharacterName: "Gandalf",
				DiscordUserID: "player1",
				CreatedVia:    "retire",
			},
		},
		retireErr: errors.New("db error"),
	}
	_, r := setupApprovalTestWithCardPoster(retireStore, &mockNotifier{}, &mockCardPoster{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/approve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestApprove_RetireSubmission_CardUpdateError_StillSucceeds(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	retireStore := &mockRetireApprovalStore{
		detail: &ApprovalDetail{
			ApprovalEntry: ApprovalEntry{
				ID:            id,
				CharacterName: "Gandalf",
				DiscordUserID: "player1",
				CreatedVia:    "retire",
			},
		},
	}
	cardPoster := &mockCardPoster{retireErr: errors.New("discord down")}
	_, r := setupApprovalTestWithCardPoster(retireStore, &mockNotifier{}, cardPoster)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/approvals/"+id.String()+"/approve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, cardPoster.retireCalls)
}

// ===== Must-Fix 3 (store level): RetireCharacter in store =====

func TestDBApprovalStore_RetireCharacter(t *testing.T) {
	id := uuid.New()
	fq := &fakeQueries{
		getPC: refdata.PlayerCharacter{
			ID:     id,
			Status: "pending",
		},
		updatePC: refdata.PlayerCharacter{
			ID:     id,
			Status: "retired",
		},
	}

	store := NewDBApprovalStore(fq)
	err := store.RetireCharacter(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "retired", fq.updateParams.Status)
}

// A-08-retire-approved-transition: validApprovalTransitions["approved"] must
// permit "retired". The realistic /retire flow tags the row created_via=
// 'retire' while leaving status='approved'; the DM then approves through the
// existing /approve endpoint, which calls RetireCharacter and requires this
// transition.
func TestDBApprovalStore_RetireCharacter_FromApproved(t *testing.T) {
	id := uuid.New()
	fq := &fakeQueries{
		getPC: refdata.PlayerCharacter{
			ID:     id,
			Status: "approved",
		},
		updatePC: refdata.PlayerCharacter{
			ID:     id,
			Status: "retired",
		},
	}

	store := NewDBApprovalStore(fq)
	err := store.RetireCharacter(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "retired", fq.updateParams.Status)
}
