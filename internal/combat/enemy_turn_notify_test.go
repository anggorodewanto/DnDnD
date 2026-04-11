package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// Phase 106b: AdvanceTurn should post a KindEnemyTurnReady dm-queue event
// when the newly-started turn belongs to a DM-controlled (NPC) combatant.

func advanceTurnStoreForKind(combatantID uuid.UUID, displayName string, isNpc bool) *mockStore {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:            id,
			Status:        "active",
			RoundNumber:   1,
			CurrentTurnID: uuid.NullUUID{},
			CampaignID:    uuid.New(),
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{
				ID:              combatantID,
				InitiativeOrder: 1,
				DisplayName:     displayName,
				Conditions:      json.RawMessage(`[]`),
				IsAlive:         true,
				IsNpc:           isNpc,
			},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber, Status: arg.Status}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), GuildID: "g1"}, nil
	}
	return store
}

func TestAdvanceTurn_PostsEnemyTurnReadyForNPC(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := advanceTurnStoreForKind(combatantID, "Goblin G2", true)

	notifier := &fakeDMNotifier{}
	svc := NewService(store)
	svc.SetDMNotifier(notifier)

	_, err := svc.AdvanceTurn(ctx, encounterID)
	require.NoError(t, err)

	require.Len(t, notifier.posts, 1, "expected exactly 1 dm-queue post for NPC turn")
	e := notifier.posts[0]
	assert.Equal(t, dmqueue.KindEnemyTurnReady, e.Kind)
	assert.Equal(t, "Goblin G2", e.PlayerName)
	assert.Equal(t, "is up", e.Summary)
	assert.Equal(t, "g1", e.GuildID)
}

func TestAdvanceTurn_NoDMQueuePostForPC(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:            id,
			Status:        "active",
			RoundNumber:   1,
			CurrentTurnID: uuid.NullUUID{},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{
				ID:              combatantID,
				InitiativeOrder: 1,
				DisplayName:     "Aria",
				Conditions:      json.RawMessage(`[]`),
				IsAlive:         true,
				IsNpc:           false,
				CharacterID:     uuid.NullUUID{UUID: charID, Valid: true},
			},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber, Status: arg.Status}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{ID: charID, SpeedFt: 30, Classes: json.RawMessage(`[{"class":"fighter","level":1}]`)}, nil
	}
	store.getClassFn = func(ctx context.Context, id string) (refdata.Class, error) {
		return refdata.Class{ID: "fighter", AttacksPerAction: json.RawMessage(`{"1":1}`)}, nil
	}

	notifier := &fakeDMNotifier{}
	svc := NewService(store)
	svc.SetDMNotifier(notifier)

	_, err := svc.AdvanceTurn(ctx, encounterID)
	require.NoError(t, err)
	assert.Empty(t, notifier.posts, "no dm-queue post expected for PC turn")
}

func TestAdvanceTurn_NPC_CampaignLookupErrorSwallowed(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := advanceTurnStoreForKind(combatantID, "Goblin", true)
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{}, errFake("no campaign")
	}

	notifier := &fakeDMNotifier{}
	svc := NewService(store)
	svc.SetDMNotifier(notifier)

	_, err := svc.AdvanceTurn(ctx, encounterID)
	require.NoError(t, err)
	assert.Empty(t, notifier.posts, "no notifier post when campaign lookup fails")
}

type errFake string

func (e errFake) Error() string { return string(e) }

func TestAdvanceTurn_NoNotifierWiredStillWorks(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := advanceTurnStoreForKind(combatantID, "Goblin", true)
	svc := NewService(store)

	_, err := svc.AdvanceTurn(ctx, encounterID)
	require.NoError(t, err)
}
