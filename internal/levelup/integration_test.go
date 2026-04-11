package levelup_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/levelup"
	"github.com/ab/dndnd/internal/refdata"
)

// fakeLevelUpPublisher records PublishEncounterSnapshot calls from the
// integration test. We intentionally reimplement this rather than importing
// the existing internal fakePublisher because the integration test lives in
// the levelup_test package.
type fakeLevelUpPublisher struct {
	mu        sync.Mutex
	published []uuid.UUID
}

func (f *fakeLevelUpPublisher) PublishEncounterSnapshot(_ context.Context, encounterID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published = append(f.published, encounterID)
	return nil
}

func (f *fakeLevelUpPublisher) calls() []uuid.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]uuid.UUID, len(f.published))
	copy(out, f.published)
	return out
}

// dbEncounterLookup mirrors the production encounterLookupAdapter in
// cmd/dndnd/main.go. We duplicate the small wrapper here so the integration
// test does not have to import from the main binary.
type dbEncounterLookup struct {
	queries *refdata.Queries
}

func (a dbEncounterLookup) ActiveEncounterIDForCharacter(ctx context.Context, characterID uuid.UUID) (uuid.UUID, bool, error) {
	encID, err := a.queries.GetActiveEncounterIDByCharacterID(ctx, uuid.NullUUID{UUID: characterID, Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return encID, true, nil
}

// TestIntegration_LevelUpHandler_PublishesSnapshot exercises the full Phase
// 104c wiring: handler → service → DB-backed store → publisher fan-out.
// The fake publisher must record exactly one call with the active
// encounter ID the character was combatting in at the time of the level-up.
func TestIntegration_LevelUpHandler_PublishesSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	_, q := setupLevelUpTestDB(t)
	seedLevelUpClasses(t, q)

	camp := createLevelUpCampaign(t, q, "guild-levelup-int")
	char := createLevelUpCharacter(t, q, camp.ID, "Integra")
	createLevelUpPlayerCharacter(t, q, camp.ID, char.ID, "discord-int")

	enc, err := q.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID: camp.ID,
		Name:       "Levelup Integration",
		Status:     "active",
	})
	require.NoError(t, err)

	_, err = q.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		CharacterID: uuid.NullUUID{UUID: char.ID, Valid: true},
		ShortID:     "P1",
		DisplayName: "Integra",
		HpMax:       44,
		HpCurrent:   44,
		Ac:          18,
		Conditions:  json.RawMessage(`[]`),
		PositionCol: "A",
		PositionRow: 1,
		IsVisible:   true,
		IsAlive:     true,
	})
	require.NoError(t, err)

	charStore := levelup.NewCharacterStoreAdapter(q)
	classStore := levelup.NewClassStoreAdapter(q)
	notifier := levelup.NewNotifierAdapter(nil) // no Discord in tests
	svc := levelup.NewService(charStore, classStore, notifier)

	pub := &fakeLevelUpPublisher{}
	svc.SetPublisher(pub, dbEncounterLookup{queries: q})

	handler := levelup.NewHandler(svc, nil)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	body, _ := json.Marshal(levelup.LevelUpRequest{
		CharacterID: char.ID,
		ClassID:     "fighter",
		NewLevel:    6,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/levelup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	// DB-side effect: character level bumped to 6.
	row, err := q.GetCharacterForLevelUp(ctx, char.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(6), row.Level)

	// Publisher fan-out: exactly one snapshot for the active encounter.
	calls := pub.calls()
	require.Len(t, calls, 1)
	assert.Equal(t, enc.ID, calls[0])
}

// TestIntegration_LevelUpHandler_NotInCombat_NoPublish verifies the
// publisher fan-out is skipped when the character is not in an active
// encounter. The handler must still succeed and persist the level-up.
func TestIntegration_LevelUpHandler_NotInCombat_NoPublish(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	_, q := setupLevelUpTestDB(t)
	seedLevelUpClasses(t, q)

	camp := createLevelUpCampaign(t, q, "guild-levelup-int-nocombat")
	char := createLevelUpCharacter(t, q, camp.ID, "Solo")
	createLevelUpPlayerCharacter(t, q, camp.ID, char.ID, "discord-solo")

	charStore := levelup.NewCharacterStoreAdapter(q)
	classStore := levelup.NewClassStoreAdapter(q)
	svc := levelup.NewService(charStore, classStore, levelup.NewNotifierAdapter(nil))

	pub := &fakeLevelUpPublisher{}
	svc.SetPublisher(pub, dbEncounterLookup{queries: q})

	handler := levelup.NewHandler(svc, nil)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	body, _ := json.Marshal(levelup.LevelUpRequest{
		CharacterID: char.ID,
		ClassID:     "fighter",
		NewLevel:    6,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/levelup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	row, err := q.GetCharacterForLevelUp(ctx, char.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(6), row.Level)

	assert.Empty(t, pub.calls(), "publisher must not fan out when not in combat")
}
