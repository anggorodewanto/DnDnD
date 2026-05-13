package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// helpers for Action Surge tests

func makeFighterChar(charID uuid.UUID, fighterLevel int, surgeUses int) refdata.Character {
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Fighter", Level: fighterLevel}})
	featureUsesJSON, _ := json.Marshal(map[string]character.FeatureUse{FeatureKeyActionSurge: {Current: surgeUses, Max: surgeUses, Recharge: "long"}})
	return refdata.Character{
		ID:               charID,
		Name:             "Kael",
		Classes:          classesJSON,
		AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":8,"wis":12,"cha":10}`),
		Level:            int32(fighterLevel),
		HpMax:            44,
		HpCurrent:        44,
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
	}
}

func makeFighterCombatant(combatantID, encounterID uuid.UUID, charID uuid.UUID) refdata.Combatant {
	return refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		ShortID:     "KA",
		DisplayName: "Kael",
		HpMax:       44,
		HpCurrent:   44,
		Ac:          18,
		PositionCol: "C",
		PositionRow: 3,
		IsNpc:       false,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}
}

func makeActionSurgeTestSetup() (uuid.UUID, uuid.UUID, uuid.UUID, *mockStore) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	ms := defaultMockStore()
	return encounterID, combatantID, charID, ms
}

// TDD Cycle 1: Happy path — Action Surge resets action and attacks

func TestActionSurge_HappyPath(t *testing.T) {
	encounterID, combatantID, charID, ms := makeActionSurgeTestSetup()

	fighter := makeFighterCombatant(combatantID, encounterID, charID)
	char := makeFighterChar(charID, 5, 1) // level 5 fighter, 1 surge use

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getClassFn = func(ctx context.Context, id string) (refdata.Class, error) {
		return refdata.Class{
			ID:               "Fighter",
			AttacksPerAction: json.RawMessage(`{"1": 1, "5": 2}`),
		}, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{
			ID:               arg.ID,
			ActionUsed:       arg.ActionUsed,
			AttacksRemaining: arg.AttacksRemaining,
			ActionSurged:     arg.ActionSurged,
		}, nil
	}

	svc := NewService(ms)

	turn := refdata.Turn{
		ID:               uuid.New(),
		ActionUsed:       true,  // action was already used
		AttacksRemaining: 0,     // attacks spent
		ActionSurged:     false, // not yet surged
	}

	cmd := ActionSurgeCommand{
		Fighter: fighter,
		Turn:    turn,
	}

	result, err := svc.ActionSurge(context.Background(), cmd)
	require.NoError(t, err)

	assert.False(t, result.Turn.ActionUsed, "action should be reset")
	assert.Equal(t, int32(2), result.Turn.AttacksRemaining, "attacks should be reset to 2 for level 5 fighter")
	assert.True(t, result.Turn.ActionSurged, "action_surged flag should be set")
	assert.Equal(t, 0, result.UsesRemaining, "should have 0 uses remaining")
	assert.Contains(t, result.CombatLog, "Kael")
	assert.Contains(t, result.CombatLog, "Action Surge")
	assert.Contains(t, result.CombatLog, "0 use(s) remaining")
}

// TDD Cycle 2: Double surge prevention

func TestActionSurge_AlreadySurgedThisTurn(t *testing.T) {
	encounterID, combatantID, charID, ms := makeActionSurgeTestSetup()

	fighter := makeFighterCombatant(combatantID, encounterID, charID)
	svc := NewService(ms)

	turn := refdata.Turn{
		ID:           uuid.New(),
		ActionSurged: true, // already surged
	}

	cmd := ActionSurgeCommand{
		Fighter: fighter,
		Turn:    turn,
	}

	_, err := svc.ActionSurge(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already used this turn")
}

// TDD Cycle 3: No uses remaining

func TestActionSurge_NoUsesRemaining(t *testing.T) {
	encounterID, combatantID, charID, ms := makeActionSurgeTestSetup()

	fighter := makeFighterCombatant(combatantID, encounterID, charID)
	char := makeFighterChar(charID, 5, 0) // 0 surge uses

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)

	turn := refdata.Turn{
		ID:           uuid.New(),
		ActionSurged: false,
	}

	cmd := ActionSurgeCommand{
		Fighter: fighter,
		Turn:    turn,
	}

	_, err := svc.ActionSurge(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no Action Surge uses remaining")
}

// TDD Cycle 4: Not a fighter

func TestActionSurge_NotFighter(t *testing.T) {
	encounterID, combatantID, charID, ms := makeActionSurgeTestSetup()

	fighter := makeFighterCombatant(combatantID, encounterID, charID)

	// Create a wizard character (not fighter)
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Wizard", Level: 5}})
	char := refdata.Character{
		ID:          charID,
		Name:        "Gandalf",
		Classes:     classesJSON,
		FeatureUses: pqtype.NullRawMessage{RawMessage: json.RawMessage(`{}`), Valid: true},
	}

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)

	turn := refdata.Turn{
		ID:           uuid.New(),
		ActionSurged: false,
	}

	cmd := ActionSurgeCommand{
		Fighter: fighter,
		Turn:    turn,
	}

	_, err := svc.ActionSurge(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Fighter level 2+")
}

// TDD Cycle 5: Fighter level 1 (too low)

func TestActionSurge_FighterLevel1(t *testing.T) {
	encounterID, combatantID, charID, ms := makeActionSurgeTestSetup()

	fighter := makeFighterCombatant(combatantID, encounterID, charID)
	char := makeFighterChar(charID, 1, 0) // level 1 fighter

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)

	cmd := ActionSurgeCommand{
		Fighter: fighter,
		Turn:    refdata.Turn{ID: uuid.New()},
	}

	_, err := svc.ActionSurge(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Fighter level 2+")
}

// TDD Cycle 6: NPC cannot use Action Surge

func TestActionSurge_NPC(t *testing.T) {
	_, _, _, ms := makeActionSurgeTestSetup()

	npc := refdata.Combatant{
		ID:          uuid.New(),
		IsNpc:       true,
		DisplayName: "NPC Fighter",
		Conditions:  json.RawMessage(`[]`),
	}

	svc := NewService(ms)

	cmd := ActionSurgeCommand{
		Fighter: npc,
		Turn:    refdata.Turn{ID: uuid.New()},
	}

	_, err := svc.ActionSurge(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not NPC")
}

// TDD Cycle 7: Level 17+ fighter gets 2 uses

func TestActionSurge_Level17TwoUses(t *testing.T) {
	encounterID, combatantID, charID, ms := makeActionSurgeTestSetup()

	fighter := makeFighterCombatant(combatantID, encounterID, charID)
	char := makeFighterChar(charID, 17, 2) // level 17, 2 uses

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getClassFn = func(ctx context.Context, id string) (refdata.Class, error) {
		return refdata.Class{
			ID:               "Fighter",
			AttacksPerAction: json.RawMessage(`{"1": 1, "5": 2, "11": 3, "20": 4}`),
		}, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{
			ID:               arg.ID,
			ActionUsed:       arg.ActionUsed,
			AttacksRemaining: arg.AttacksRemaining,
			ActionSurged:     arg.ActionSurged,
		}, nil
	}

	svc := NewService(ms)

	turn := refdata.Turn{
		ID:               uuid.New(),
		ActionUsed:       true,
		AttacksRemaining: 0,
		ActionSurged:     false,
	}

	cmd := ActionSurgeCommand{
		Fighter: fighter,
		Turn:    turn,
	}

	result, err := svc.ActionSurge(context.Background(), cmd)
	require.NoError(t, err)

	assert.Equal(t, int32(3), result.Turn.AttacksRemaining, "level 17 fighter gets 3 attacks")
	assert.Equal(t, 1, result.UsesRemaining, "should have 1 use remaining")
	assert.Contains(t, result.CombatLog, "1 use(s) remaining")
}

// TDD Cycle 8: DB error paths

func TestActionSurge_GetCharacterError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeActionSurgeTestSetup()

	fighter := makeFighterCombatant(combatantID, encounterID, charID)
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)

	cmd := ActionSurgeCommand{
		Fighter: fighter,
		Turn:    refdata.Turn{ID: uuid.New()},
	}

	_, err := svc.ActionSurge(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

func TestActionSurge_DeductFeatureUseError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeActionSurgeTestSetup()

	fighter := makeFighterCombatant(combatantID, encounterID, charID)
	char := makeFighterChar(charID, 5, 1)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)

	cmd := ActionSurgeCommand{
		Fighter: fighter,
		Turn:    refdata.Turn{ID: uuid.New()},
	}

	_, err := svc.ActionSurge(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating feature_uses")
}

func TestActionSurge_UpdateTurnActionsError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeActionSurgeTestSetup()

	fighter := makeFighterCombatant(combatantID, encounterID, charID)
	char := makeFighterChar(charID, 5, 1)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getClassFn = func(ctx context.Context, id string) (refdata.Class, error) {
		return refdata.Class{
			ID:               "Fighter",
			AttacksPerAction: json.RawMessage(`{"1": 1, "5": 2}`),
		}, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)

	cmd := ActionSurgeCommand{
		Fighter: fighter,
		Turn:    refdata.Turn{ID: uuid.New()},
	}

	_, err := svc.ActionSurge(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

func TestActionSurge_ParseFeatureUsesError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeActionSurgeTestSetup()

	fighter := makeFighterCombatant(combatantID, encounterID, charID)
	char := makeFighterChar(charID, 5, 1)
	char.FeatureUses = pqtype.NullRawMessage{RawMessage: json.RawMessage(`invalid`), Valid: true}

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)

	cmd := ActionSurgeCommand{
		Fighter: fighter,
		Turn:    refdata.Turn{ID: uuid.New()},
	}

	_, err := svc.ActionSurge(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing feature_uses")
}

// TDD Cycle 9: Invalid AttacksPerAction JSON defaults to 1 attack

func TestActionSurge_InvalidAttacksPerActionJSON(t *testing.T) {
	encounterID, combatantID, charID, ms := makeActionSurgeTestSetup()

	fighter := makeFighterCombatant(combatantID, encounterID, charID)
	char := makeFighterChar(charID, 5, 1)
	// Override classes with valid fighter but store returns error for class lookup
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getClassFn = func(ctx context.Context, id string) (refdata.Class, error) {
		// Return class with invalid attacks_per_action JSON
		return refdata.Class{
			ID:               "Fighter",
			AttacksPerAction: json.RawMessage(`invalid`),
		}, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{
			ID:               arg.ID,
			ActionUsed:       arg.ActionUsed,
			AttacksRemaining: arg.AttacksRemaining,
			ActionSurged:     arg.ActionSurged,
		}, nil
	}

	svc := NewService(ms)

	cmd := ActionSurgeCommand{
		Fighter: fighter,
		Turn:    refdata.Turn{ID: uuid.New(), ActionUsed: true},
	}

	result, err := svc.ActionSurge(context.Background(), cmd)
	require.NoError(t, err)
	// Falls back to 1 attack when class lookup fails to parse
	assert.Equal(t, int32(1), result.Turn.AttacksRemaining)
}

// TDD Cycle 9b: GetClass returns error — falls back gracefully

func TestResolveAttacksPerAction_GetClassError(t *testing.T) {
	_, _, _, ms := makeActionSurgeTestSetup()

	ms.getClassFn = func(ctx context.Context, id string) (refdata.Class, error) {
		return refdata.Class{}, fmt.Errorf("class not found")
	}

	svc := NewService(ms)

	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Fighter", Level: 5}})
	char := refdata.Character{
		Classes: classesJSON,
	}

	attacks := svc.resolveAttacksPerAction(context.Background(), char)
	assert.Equal(t, 1, attacks, "should default to 1 when GetClass errors")
}

// TDD Cycle 9c: Invalid classes JSON in resolveAttacksPerAction defaults to 1

func TestResolveAttacksPerAction_InvalidClassesJSON(t *testing.T) {
	_, _, _, ms := makeActionSurgeTestSetup()

	svc := NewService(ms)

	char := refdata.Character{
		Classes: json.RawMessage(`invalid`),
	}

	attacks := svc.resolveAttacksPerAction(context.Background(), char)
	assert.Equal(t, 1, attacks, "should default to 1 when classes JSON is invalid")
}

// TDD Cycle 10: Bonus action and reaction NOT reset

func TestActionSurge_DoesNotResetBonusOrReaction(t *testing.T) {
	encounterID, combatantID, charID, ms := makeActionSurgeTestSetup()

	fighter := makeFighterCombatant(combatantID, encounterID, charID)
	char := makeFighterChar(charID, 5, 1)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getClassFn = func(ctx context.Context, id string) (refdata.Class, error) {
		return refdata.Class{
			ID:               "Fighter",
			AttacksPerAction: json.RawMessage(`{"1": 1, "5": 2}`),
		}, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{
			ID:               arg.ID,
			ActionUsed:       arg.ActionUsed,
			BonusActionUsed:  arg.BonusActionUsed,
			ReactionUsed:     arg.ReactionUsed,
			AttacksRemaining: arg.AttacksRemaining,
			ActionSurged:     arg.ActionSurged,
		}, nil
	}

	svc := NewService(ms)

	turn := refdata.Turn{
		ID:               uuid.New(),
		ActionUsed:       true,
		BonusActionUsed:  true, // already used
		ReactionUsed:     true, // already used
		AttacksRemaining: 0,
		ActionSurged:     false,
	}

	cmd := ActionSurgeCommand{
		Fighter: fighter,
		Turn:    turn,
	}

	result, err := svc.ActionSurge(context.Background(), cmd)
	require.NoError(t, err)

	assert.False(t, result.Turn.ActionUsed, "action should be reset")
	assert.True(t, result.Turn.BonusActionUsed, "bonus action should NOT be reset")
	assert.True(t, result.Turn.ReactionUsed, "reaction should NOT be reset")
}
