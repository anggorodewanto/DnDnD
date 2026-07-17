package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// The DM dashboard "advance turn" path calls Service.AdvanceTurn directly (no
// /done handler), so before this fix a turn advanced from the dashboard posted
// nothing to #your-turn — the next player sat in silence. AdvanceTurn must fire
// the TurnStartNotifier for the newly-active PC on every advance, not just the
// first turn of combat. (NPCs are intentionally excluded — see
// TestAdvanceTurn_NoYourTurnBannerForNPC.)
func TestAdvanceTurn_FiresTurnStartNotifier(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(_ context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1, CurrentTurnID: uuid.NullUUID{}}, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: combatantID, InitiativeOrder: 1, DisplayName: "Aria", Conditions: json.RawMessage(`[]`), IsAlive: true, IsNpc: false, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(_ context.Context, _ refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.createTurnFn = func(_ context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber, Status: arg.Status}, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{ID: charID, SpeedFt: 30, Classes: json.RawMessage(`[{"class":"fighter","level":1}]`)}, nil
	}
	store.getClassFn = func(_ context.Context, _ string) (refdata.Class, error) {
		return refdata.Class{ID: "fighter", AttacksPerAction: json.RawMessage(`{"1":1}`)}, nil
	}

	notifier := &stubTurnStartNotifier{}
	svc := NewService(store)
	svc.SetTurnStartNotifier(notifier)

	info, err := svc.AdvanceTurn(ctx, encounterID)
	require.NoError(t, err)

	require.Len(t, notifier.calls, 1, "advance must fire the turn-start ping exactly once")
	assert.Equal(t, combatantID, notifier.calls[0].CombatantID)
	assert.Equal(t, info.Turn.ID, notifier.calls[0].Turn.ID)
	assert.Equal(t, int32(1), notifier.calls[0].RoundNumber)
	// No prior completed turn for this combatant → empty impact summary.
	assert.Empty(t, notifier.impacts[0])
}

// A nil TurnStartNotifier must never break a turn advance.
func TestAdvanceTurn_NilTurnStartNotifier_NoOp(t *testing.T) {
	ctx := context.Background()
	store := advanceTurnStoreForKind(uuid.New(), "Goblin", true)

	svc := NewService(store)
	// no SetTurnStartNotifier

	_, err := svc.AdvanceTurn(ctx, uuid.New())
	require.NoError(t, err)
}
