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

// NPC-turn flow: when an NPC's turn ends, players should see the resulting board
// (Fix 1 — post the combat map after an NPC turn), and the newly-active
// combatant's detailed #your-turn banner should NOT fire for NPCs (Fix 2 — that
// prompt is for players; the DM is cued separately via the enemy_turn_ready
// dm-queue post).

// advanceTurnStoreOutgoing builds an active encounter whose CURRENT turn belongs
// to `outgoingID` (already acted this round) and whose next candidate is a PC.
// The current turn is marked ActionUsed so the NPC "run enemy turn first" guard
// passes when the outgoing combatant is an NPC.
func advanceTurnStoreOutgoing(outgoingID uuid.UUID, outgoingIsNPC bool) *mockStore {
	turnID := uuid.New()
	incomingID := uuid.New()
	charID := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(_ context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:            id,
			Status:        "active",
			RoundNumber:   1,
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			CampaignID:    uuid.New(),
		}, nil
	}
	store.getTurnFn = func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, CombatantID: outgoingID, Status: "active", ActionUsed: true}, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, IsNpc: outgoingIsNPC, IsAlive: true, Conditions: json.RawMessage(`[]`)}, nil
	}
	store.completeTurnFn = func(_ context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: id, CombatantID: outgoingID, Status: "completed"}, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: outgoingID, InitiativeOrder: 1, DisplayName: "Follower", Conditions: json.RawMessage(`[]`), IsAlive: true, IsNpc: outgoingIsNPC},
			{ID: incomingID, InitiativeOrder: 2, DisplayName: "Aria", Conditions: json.RawMessage(`[]`), IsAlive: true, IsNpc: false, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(_ context.Context, _ refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{{CombatantID: outgoingID}}, nil
	}
	store.createTurnFn = func(_ context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber, Status: arg.Status}, nil
	}
	store.updateEncounterCurrentTurnFn = func(_ context.Context, arg refdata.UpdateEncounterCurrentTurnParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID}, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{ID: charID, SpeedFt: 30, Classes: json.RawMessage(`[{"class":"fighter","level":1}]`)}, nil
	}
	store.getClassFn = func(_ context.Context, _ string) (refdata.Class, error) {
		return refdata.Class{ID: "fighter", AttacksPerAction: json.RawMessage(`{"1":1}`)}, nil
	}
	store.getCampaignByEncounterIDFn = func(_ context.Context, _ uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), GuildID: "g1"}, nil
	}
	return store
}

// Fix 1: advancing past an NPC's completed turn posts the fresh board to
// #combat-map, so players see the result of the enemy's move even when the DM
// advances via the dashboard (which otherwise posts no map).
func TestAdvanceTurn_PostsCombatMapAfterNPCTurn(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()

	store := advanceTurnStoreOutgoing(uuid.New(), true)
	notifier := &stubCombatMapNotifier{}
	svc := NewService(store)
	svc.SetCombatMapNotifier(notifier)

	_, err := svc.AdvanceTurn(ctx, encounterID)
	require.NoError(t, err)

	require.Len(t, notifier.calls, 1, "advancing past an NPC turn must repost the combat map exactly once")
	assert.Equal(t, encounterID, notifier.calls[0])
}

// Fix 1 (negative): a PC's turn already posts its map from the /done handler, so
// AdvanceTurn must NOT post an extra one when the outgoing combatant is a PC.
func TestAdvanceTurn_NoCombatMapAfterPCTurn(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()

	store := advanceTurnStoreOutgoing(uuid.New(), false)
	notifier := &stubCombatMapNotifier{}
	svc := NewService(store)
	svc.SetCombatMapNotifier(notifier)

	_, err := svc.AdvanceTurn(ctx, encounterID)
	require.NoError(t, err)

	assert.Empty(t, notifier.calls, "no combat-map repost expected after a PC turn (the /done handler posts it)")
}

// Fix 1 (first turn): the first turn of combat has no outgoing combatant, so
// AdvanceTurn must not post a map — the opening board is StartCombat's job and
// would otherwise be duplicated.
func TestAdvanceTurn_FirstTurn_NoCombatMapRepost(t *testing.T) {
	ctx := context.Background()

	store := advanceTurnStoreForKind(uuid.New(), "Goblin", true) // CurrentTurnID invalid (first seat)
	notifier := &stubCombatMapNotifier{}
	svc := NewService(store)
	svc.SetCombatMapNotifier(notifier)

	_, err := svc.AdvanceTurn(ctx, uuid.New())
	require.NoError(t, err)

	assert.Empty(t, notifier.calls, "the first turn has no outgoing combatant, so no repost")
}

// Fix 2: an NPC becoming active must NOT fire the detailed #your-turn banner.
func TestAdvanceTurn_NoYourTurnBannerForNPC(t *testing.T) {
	ctx := context.Background()

	store := advanceTurnStoreForKind(uuid.New(), "Goblin G2", true) // incoming NPC
	notifier := &stubTurnStartNotifier{}
	svc := NewService(store)
	svc.SetTurnStartNotifier(notifier)

	_, err := svc.AdvanceTurn(ctx, uuid.New())
	require.NoError(t, err)

	assert.Empty(t, notifier.calls, "NPC turn-start must not post the detailed #your-turn banner")
}
