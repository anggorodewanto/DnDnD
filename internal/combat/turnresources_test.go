package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/refdata"
)

func TestValidateResource_ActionAvailable(t *testing.T) {
	turn := refdata.Turn{ActionUsed: false}
	err := ValidateResource(turn, ResourceAction)
	assert.NoError(t, err)
}

func TestValidateResource_ActionSpent(t *testing.T) {
	turn := refdata.Turn{ActionUsed: true}
	err := ValidateResource(turn, ResourceAction)
	assert.ErrorIs(t, err, ErrResourceSpent)
	assert.Contains(t, err.Error(), "action")
}

func TestValidateResource_AllResourceTypes(t *testing.T) {
	tests := []struct {
		name     string
		turn     refdata.Turn
		resource ResourceType
		wantErr  bool
	}{
		{"bonus_action_available", refdata.Turn{BonusActionUsed: false}, ResourceBonusAction, false},
		{"bonus_action_spent", refdata.Turn{BonusActionUsed: true}, ResourceBonusAction, true},
		{"reaction_available", refdata.Turn{ReactionUsed: false}, ResourceReaction, false},
		{"reaction_spent", refdata.Turn{ReactionUsed: true}, ResourceReaction, true},
		{"free_interact_available", refdata.Turn{FreeInteractUsed: false}, ResourceFreeInteract, false},
		{"free_interact_spent", refdata.Turn{FreeInteractUsed: true}, ResourceFreeInteract, true},
		{"movement_available", refdata.Turn{MovementRemainingFt: 10}, ResourceMovement, false},
		{"movement_spent", refdata.Turn{MovementRemainingFt: 0}, ResourceMovement, true},
		{"attack_available", refdata.Turn{AttacksRemaining: 1}, ResourceAttack, false},
		{"attack_spent", refdata.Turn{AttacksRemaining: 0}, ResourceAttack, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateResource(tc.turn, tc.resource)
			if tc.wantErr {
				assert.ErrorIs(t, err, ErrResourceSpent)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateResource_UnknownType(t *testing.T) {
	turn := refdata.Turn{}
	err := ValidateResource(turn, ResourceType("teleport"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown resource type")
}

func TestUseResource_Action(t *testing.T) {
	turn := refdata.Turn{ActionUsed: false}
	updated, err := UseResource(turn, ResourceAction)
	assert.NoError(t, err)
	assert.True(t, updated.ActionUsed)
}

func TestUseResource_ActionAlreadyUsed(t *testing.T) {
	turn := refdata.Turn{ActionUsed: true}
	_, err := UseResource(turn, ResourceAction)
	assert.ErrorIs(t, err, ErrResourceSpent)
}

func TestUseResource_BonusAction(t *testing.T) {
	turn := refdata.Turn{BonusActionUsed: false}
	updated, err := UseResource(turn, ResourceBonusAction)
	assert.NoError(t, err)
	assert.True(t, updated.BonusActionUsed)
}

func TestUseResource_Reaction(t *testing.T) {
	turn := refdata.Turn{ReactionUsed: false}
	updated, err := UseResource(turn, ResourceReaction)
	assert.NoError(t, err)
	assert.True(t, updated.ReactionUsed)
}

func TestUseResource_FreeInteract(t *testing.T) {
	turn := refdata.Turn{FreeInteractUsed: false}
	updated, err := UseResource(turn, ResourceFreeInteract)
	assert.NoError(t, err)
	assert.True(t, updated.FreeInteractUsed)
}

func TestUseMovement_Success(t *testing.T) {
	turn := refdata.Turn{MovementRemainingFt: 30}
	updated, err := UseMovement(turn, 10)
	assert.NoError(t, err)
	assert.Equal(t, int32(20), updated.MovementRemainingFt)
}

func TestUseMovement_SplitMovement(t *testing.T) {
	turn := refdata.Turn{MovementRemainingFt: 30}
	turn, err := UseMovement(turn, 15)
	assert.NoError(t, err)
	assert.Equal(t, int32(15), turn.MovementRemainingFt)

	turn, err = UseMovement(turn, 10)
	assert.NoError(t, err)
	assert.Equal(t, int32(5), turn.MovementRemainingFt)
}

func TestUseMovement_ExactlyZero(t *testing.T) {
	turn := refdata.Turn{MovementRemainingFt: 10}
	updated, err := UseMovement(turn, 10)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), updated.MovementRemainingFt)
}

func TestUseMovement_NotEnough(t *testing.T) {
	turn := refdata.Turn{MovementRemainingFt: 5}
	_, err := UseMovement(turn, 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enough movement")
}

func TestUseMovement_NoMovement(t *testing.T) {
	turn := refdata.Turn{MovementRemainingFt: 0}
	_, err := UseMovement(turn, 5)
	assert.ErrorIs(t, err, ErrResourceSpent)
}

func TestUseMovement_NegativeFeet(t *testing.T) {
	turn := refdata.Turn{MovementRemainingFt: 30}
	_, err := UseMovement(turn, -5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "movement must be positive")
}

func TestUseMovement_ZeroFeet(t *testing.T) {
	turn := refdata.Turn{MovementRemainingFt: 30}
	_, err := UseMovement(turn, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "movement must be positive")
}

func TestUseAttack_Success(t *testing.T) {
	turn := refdata.Turn{AttacksRemaining: 2}
	updated, err := UseAttack(turn)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), updated.AttacksRemaining)
}

func TestUseAttack_LastAttack(t *testing.T) {
	turn := refdata.Turn{AttacksRemaining: 1}
	updated, err := UseAttack(turn)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), updated.AttacksRemaining)
}

func TestUseAttack_NoAttacks(t *testing.T) {
	turn := refdata.Turn{AttacksRemaining: 0}
	_, err := UseAttack(turn)
	assert.ErrorIs(t, err, ErrResourceSpent)
}

func TestAttacksPerActionForLevel(t *testing.T) {
	tests := []struct {
		name     string
		attacks  map[string]int
		level    int
		expected int
	}{
		{"level_1_fighter", map[string]int{"1": 1, "5": 2, "11": 3, "20": 4}, 1, 1},
		{"level_4_fighter", map[string]int{"1": 1, "5": 2, "11": 3, "20": 4}, 4, 1},
		{"level_5_fighter", map[string]int{"1": 1, "5": 2, "11": 3, "20": 4}, 5, 2},
		{"level_10_fighter", map[string]int{"1": 1, "5": 2, "11": 3, "20": 4}, 10, 2},
		{"level_11_fighter", map[string]int{"1": 1, "5": 2, "11": 3, "20": 4}, 11, 3},
		{"level_20_fighter", map[string]int{"1": 1, "5": 2, "11": 3, "20": 4}, 20, 4},
		{"level_1_wizard", map[string]int{"1": 1}, 1, 1},
		{"level_20_wizard", map[string]int{"1": 1}, 20, 1},
		{"empty_map_defaults_to_1", map[string]int{}, 5, 1},
		{"nil_map_defaults_to_1", nil, 5, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := AttacksPerActionForLevel(tc.attacks, tc.level)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFormatTurnStartPrompt_FullResources(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		ActionUsed:          false,
		BonusActionUsed:     false,
		FreeInteractUsed:    false,
		ReactionUsed:        false,
		AttacksRemaining:    2,
	}
	result := FormatTurnStartPrompt("Rooftop Ambush", 3, "Aria", turn, nil)
	assert.Contains(t, result, "Rooftop Ambush")
	assert.Contains(t, result, "Round 3")
	assert.Contains(t, result, "@Aria")
	assert.Contains(t, result, "30ft move")
	assert.Contains(t, result, "2 attacks")
	assert.Contains(t, result, "Bonus action")
	assert.Contains(t, result, "Free interact")
	assert.Contains(t, result, "Reaction")
}

func TestFormatTurnStartPrompt_AllSpent(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 0,
		ActionUsed:          true,
		BonusActionUsed:     true,
		FreeInteractUsed:    true,
		ReactionUsed:        true,
		AttacksRemaining:    0,
	}
	result := FormatTurnStartPrompt("Test", 1, "Bob", turn, nil)
	assert.Contains(t, result, "All actions spent")
	assert.Contains(t, result, "/done")
}

func TestFormatTurnStartPrompt_SingleAttack(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	}
	result := FormatTurnStartPrompt("Test", 1, "Bob", turn, nil)
	assert.Contains(t, result, "1 attack")
	assert.NotContains(t, result, "attacks") // singular
}

func TestFormatRemainingResources_SomeSpent(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 5,
		ActionUsed:          true,
		BonusActionUsed:     false,
		FreeInteractUsed:    true,
		ReactionUsed:        false,
		AttacksRemaining:    1,
	}
	result := FormatRemainingResources(turn, nil)
	assert.Contains(t, result, "5ft move")
	assert.Contains(t, result, "1 attack")
	assert.Contains(t, result, "Bonus action")
	assert.NotContains(t, result, "Free interact") // spent
	assert.Contains(t, result, "Reaction")
}

func TestFormatRemainingResources_AllSpent(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 0,
		ActionUsed:          true,
		BonusActionUsed:     true,
		FreeInteractUsed:    true,
		ReactionUsed:        true,
		AttacksRemaining:    0,
	}
	result := FormatRemainingResources(turn, nil)
	assert.Contains(t, result, "All actions spent")
	assert.Contains(t, result, "/done")
}

func TestFormatRemainingResources_OnlyMovement(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 15,
		ActionUsed:          true,
		BonusActionUsed:     true,
		FreeInteractUsed:    true,
		ReactionUsed:        true,
		AttacksRemaining:    0,
	}
	result := FormatRemainingResources(turn, nil)
	assert.Contains(t, result, "15ft move")
	assert.NotContains(t, result, "All actions spent")
}

func TestInitializeTurnResources_PCWithExtraAttack(t *testing.T) {
	charID := uuid.New()
	combatant := refdata.Combatant{
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:       false,
	}
	character := refdata.Character{
		ID:      charID,
		SpeedFt: 30,
		Level:   5,
		Classes: json.RawMessage(`[{"class":"fighter","level":5}]`),
	}
	classData := refdata.Class{
		ID:               "fighter",
		AttacksPerAction: json.RawMessage(`{"1":1,"5":2,"11":3,"20":4}`),
	}

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
		assert.Equal(t, charID, id)
		return character, nil
	}
	store.getClassFn = func(_ context.Context, id string) (refdata.Class, error) {
		assert.Equal(t, "fighter", id)
		return classData, nil
	}

	svc := NewService(store)
	speed, attacks, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(30), speed)
	assert.Equal(t, int32(2), attacks)
}

func TestInitializeTurnResources_NPC(t *testing.T) {
	combatant := refdata.Combatant{
		IsNpc: true,
	}
	store := defaultMockStore()
	svc := NewService(store)

	speed, attacks, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(30), speed) // default
	assert.Equal(t, int32(1), attacks)
}

func TestInitializeTurnResources_PCNoClasses(t *testing.T) {
	charID := uuid.New()
	combatant := refdata.Combatant{
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:       false,
	}
	character := refdata.Character{
		ID:      charID,
		SpeedFt: 25,
		Level:   3,
		Classes: json.RawMessage(`[]`),
	}

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
		return character, nil
	}

	svc := NewService(store)
	speed, attacks, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(25), speed)
	assert.Equal(t, int32(1), attacks)
}

func TestAttacksPerActionForLevel_InvalidKey(t *testing.T) {
	result := AttacksPerActionForLevel(map[string]int{"not_a_number": 3}, 5)
	assert.Equal(t, 1, result) // falls back to default
}

func TestResolveTurnResources_CharacterNotFound(t *testing.T) {
	charID := uuid.New()
	combatant := refdata.Combatant{
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:       false,
	}
	store := defaultMockStore()
	// getCharacterFn already returns sql.ErrNoRows by default
	svc := NewService(store)

	_, _, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting character for turn resources")
}

func TestResolveTurnResources_ZeroSpeed(t *testing.T) {
	charID := uuid.New()
	combatant := refdata.Combatant{
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:       false,
	}
	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{SpeedFt: 0, Classes: json.RawMessage(`[]`)}, nil
	}
	svc := NewService(store)
	speed, attacks, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(30), speed) // defaults to 30 when 0
	assert.Equal(t, int32(1), attacks)
}

func TestResolveTurnResources_InvalidClassJSON(t *testing.T) {
	charID := uuid.New()
	combatant := refdata.Combatant{
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:       false,
	}
	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{SpeedFt: 25, Classes: json.RawMessage(`invalid`)}, nil
	}
	svc := NewService(store)
	speed, attacks, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(25), speed)
	assert.Equal(t, int32(1), attacks)
}

func TestResolveTurnResources_ClassLookupFails(t *testing.T) {
	charID := uuid.New()
	combatant := refdata.Combatant{
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:       false,
	}
	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{SpeedFt: 30, Level: 5, Classes: json.RawMessage(`[{"class":"unknown","level":5}]`)}, nil
	}
	// getClassFn returns sql.ErrNoRows by default
	svc := NewService(store)
	speed, attacks, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(30), speed)
	assert.Equal(t, int32(1), attacks) // fallback to 1
}

func TestResolveTurnResources_NoCombatantCharacterID(t *testing.T) {
	combatant := refdata.Combatant{
		CharacterID: uuid.NullUUID{Valid: false},
		IsNpc:       false,
	}
	store := defaultMockStore()
	svc := NewService(store)
	speed, attacks, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(30), speed)
	assert.Equal(t, int32(1), attacks)
}

func TestResolveTurnResources_MulticlassHighestWins(t *testing.T) {
	charID := uuid.New()
	combatant := refdata.Combatant{
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:       false,
	}
	character := refdata.Character{
		ID:      charID,
		SpeedFt: 30,
		Level:   8,
		Classes: json.RawMessage(`[{"class":"fighter","level":5},{"class":"wizard","level":3}]`),
	}

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
		assert.Equal(t, charID, id)
		return character, nil
	}
	store.getClassFn = func(_ context.Context, id string) (refdata.Class, error) {
		switch id {
		case "fighter":
			return refdata.Class{
				ID:               "fighter",
				AttacksPerAction: json.RawMessage(`{"1":1,"5":2,"11":3,"20":4}`),
			}, nil
		case "wizard":
			return refdata.Class{
				ID:               "wizard",
				AttacksPerAction: json.RawMessage(`{"1":1}`),
			}, nil
		default:
			t.Fatalf("unexpected class lookup: %s", id)
			return refdata.Class{}, nil
		}
	}

	svc := NewService(store)
	speed, attacks, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(30), speed)
	assert.Equal(t, int32(2), attacks) // Fighter 5 gives 2 attacks, Wizard 3 gives 1
}

// C-42: ResolveTurnResources must consult ExhaustionLevel when computing speed.
// Level 2+ halves; level 5+ zeroes.
func TestResolveTurnResources_ExhaustionLevel2HalvesSpeed(t *testing.T) {
	charID := uuid.New()
	combatant := refdata.Combatant{
		CharacterID:     uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:           false,
		ExhaustionLevel: 2,
		Conditions:      json.RawMessage(`[]`),
	}
	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{SpeedFt: 30, Classes: json.RawMessage(`[]`)}, nil
	}
	svc := NewService(store)
	speed, _, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(15), speed, "exhaustion 2 must halve max speed")
}

func TestResolveTurnResources_ExhaustionLevel5ZeroesSpeed(t *testing.T) {
	charID := uuid.New()
	combatant := refdata.Combatant{
		CharacterID:     uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:           false,
		ExhaustionLevel: 5,
		Conditions:      json.RawMessage(`[]`),
	}
	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{SpeedFt: 30, Classes: json.RawMessage(`[]`)}, nil
	}
	svc := NewService(store)
	speed, _, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), speed, "exhaustion 5 must zero speed")
}

func TestResolveTurnResources_NPCExhaustionLevel2HalvesSpeed(t *testing.T) {
	combatant := refdata.Combatant{
		IsNpc:           true,
		ExhaustionLevel: 2,
		Conditions:      json.RawMessage(`[]`),
	}
	store := defaultMockStore()
	svc := NewService(store)
	speed, _, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(15), speed, "NPC exhaustion 2 must halve default 30 speed")
}

func TestResolveTurnResources_ExhaustionLevel1Unchanged(t *testing.T) {
	charID := uuid.New()
	combatant := refdata.Combatant{
		CharacterID:     uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:           false,
		ExhaustionLevel: 1,
		Conditions:      json.RawMessage(`[]`),
	}
	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{SpeedFt: 30, Classes: json.RawMessage(`[]`)}, nil
	}
	svc := NewService(store)
	speed, _, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(30), speed, "exhaustion 1 leaves speed unchanged")
}

func TestResolveTurnResources_InvalidAttacksPerActionJSON(t *testing.T) {
	charID := uuid.New()
	combatant := refdata.Combatant{
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:       false,
	}
	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{SpeedFt: 30, Level: 5, Classes: json.RawMessage(`[{"class":"fighter","level":5}]`)}, nil
	}
	store.getClassFn = func(_ context.Context, _ string) (refdata.Class, error) {
		return refdata.Class{AttacksPerAction: json.RawMessage(`invalid`)}, nil
	}
	svc := NewService(store)
	speed, attacks, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(30), speed)
	assert.Equal(t, int32(1), attacks)
}

func TestTurnToUpdateParams(t *testing.T) {
	turnID := uuid.New()
	turn := refdata.Turn{
		ID:                   turnID,
		MovementRemainingFt:  15,
		ActionUsed:           true,
		BonusActionUsed:      false,
		BonusActionSpellCast: true,
		ActionSpellCast:      false,
		ReactionUsed:         true,
		FreeInteractUsed:     false,
		AttacksRemaining:     1,
		HasDisengaged:        true,
		ActionSurged:         false,
		HasStoodThisTurn:     true,
	}
	params := TurnToUpdateParams(turn)
	assert.Equal(t, turnID, params.ID)
	assert.Equal(t, int32(15), params.MovementRemainingFt)
	assert.True(t, params.ActionUsed)
	assert.False(t, params.BonusActionUsed)
	assert.True(t, params.BonusActionSpellCast)
	assert.False(t, params.ActionSpellCast)
	assert.True(t, params.ReactionUsed)
	assert.False(t, params.FreeInteractUsed)
	assert.Equal(t, int32(1), params.AttacksRemaining)
	assert.True(t, params.HasDisengaged)
	assert.False(t, params.ActionSurged)
	assert.True(t, params.HasStoodThisTurn)
}

func TestUseResource_UnsupportedForMovementAndAttack(t *testing.T) {
	turn := refdata.Turn{MovementRemainingFt: 30}
	_, err := UseResource(turn, ResourceMovement)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UseMovement")

	turn2 := refdata.Turn{AttacksRemaining: 2}
	_, err = UseResource(turn2, ResourceAttack)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UseAttack")
}

// --- TDD Cycle: FormatTurnStartPrompt shows Bardic Inspiration when combatant passed ---

func TestFormatTurnStartPrompt_WithBardicInspiration(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	}
	combatant := refdata.Combatant{
		BardicInspirationDie: sql.NullString{String: "d8", Valid: true},
	}
	result := FormatTurnStartPrompt("Test", 1, "Aria", turn, &combatant)
	assert.Contains(t, result, "Bardic Inspiration (d8)")
}

func TestFormatTurnStartPrompt_WithoutCombatant_NoInspiration(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	}
	result := FormatTurnStartPrompt("Test", 1, "Aria", turn, nil)
	assert.NotContains(t, result, "Bardic Inspiration")
}

// --- TDD Cycle: FormatRemainingResources shows Bardic Inspiration when combatant passed ---

func TestFormatRemainingResources_WithBardicInspiration(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	}
	combatant := refdata.Combatant{
		BardicInspirationDie: sql.NullString{String: "d6", Valid: true},
	}
	result := FormatRemainingResources(turn, &combatant)
	assert.Contains(t, result, "Bardic Inspiration (d6)")
}

func TestFormatRemainingResources_WithoutCombatant_NoInspiration(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	}
	result := FormatRemainingResources(turn, nil)
	assert.NotContains(t, result, "Bardic Inspiration")
}

// --- TDD Cycle: RefundResource ---

func TestRefundResource_Action(t *testing.T) {
	turn := refdata.Turn{ActionUsed: true}
	updated := RefundResource(turn, ResourceAction)
	assert.False(t, updated.ActionUsed)
}

func TestRefundResource_BonusAction(t *testing.T) {
	turn := refdata.Turn{BonusActionUsed: true}
	updated := RefundResource(turn, ResourceBonusAction)
	assert.False(t, updated.BonusActionUsed)
}

func TestRefundResource_Reaction(t *testing.T) {
	turn := refdata.Turn{ReactionUsed: true}
	updated := RefundResource(turn, ResourceReaction)
	assert.False(t, updated.ReactionUsed)
}

func TestRefundResource_FreeInteract(t *testing.T) {
	turn := refdata.Turn{FreeInteractUsed: true}
	updated := RefundResource(turn, ResourceFreeInteract)
	assert.False(t, updated.FreeInteractUsed)
}

func TestRefundResource_AlreadyAvailable(t *testing.T) {
	turn := refdata.Turn{ActionUsed: false}
	updated := RefundResource(turn, ResourceAction)
	assert.False(t, updated.ActionUsed) // no-op, still false
}

func TestResolveTurnResources_F20_WildShapeUsesBeastSpeed(t *testing.T) {
	charID := uuid.New()
	combatant := refdata.Combatant{
		CharacterID:          uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:                false,
		IsWildShaped:         true,
		WildShapeCreatureRef: sql.NullString{String: "wolf", Valid: true},
		Conditions:           json.RawMessage(`[]`),
	}
	character := refdata.Character{
		ID:      charID,
		SpeedFt: 30,
		Level:   5,
		Classes: json.RawMessage(`[{"class":"druid","level":5}]`),
	}

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
		return character, nil
	}
	store.getCreatureFn = func(_ context.Context, id string) (refdata.Creature, error) {
		assert.Equal(t, "wolf", id)
		return refdata.Creature{
			ID:    "wolf",
			Speed: json.RawMessage(`{"walk":40}`),
		}, nil
	}

	svc := NewService(store)
	speed, _, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(40), speed, "wild-shaped combatant should use beast walk speed")
}

// F-C02: ResolveTurnResources must apply heavy armor speed penalty when STR is below requirement.
func TestResolveTurnResources_HeavyArmorPenalty_InsufficientSTR(t *testing.T) {
	charID := uuid.New()
	combatant := refdata.Combatant{
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:       false,
		Conditions:  json.RawMessage(`[]`),
	}
	character := refdata.Character{
		ID:            charID,
		SpeedFt:       30,
		Classes:       json.RawMessage(`[]`),
		AbilityScores: json.RawMessage(`{"str":12,"dex":10,"con":14,"int":10,"wis":10,"cha":10}`),
		EquippedArmor: sql.NullString{String: "splint", Valid: true},
	}

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return character, nil
	}
	store.getArmorFn = func(_ context.Context, id string) (refdata.Armor, error) {
		assert.Equal(t, "splint", id)
		return refdata.Armor{
			ID:          "splint",
			Name:        "Splint Armor",
			ArmorType:   "heavy",
			StrengthReq: sql.NullInt32{Int32: 15, Valid: true},
		}, nil
	}

	svc := NewService(store)
	speed, _, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(20), speed, "STR 12 < req 15: speed should be 30-10=20")
}

func TestResolveTurnResources_HeavyArmorPenalty_SufficientSTR(t *testing.T) {
	charID := uuid.New()
	combatant := refdata.Combatant{
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:       false,
		Conditions:  json.RawMessage(`[]`),
	}
	character := refdata.Character{
		ID:            charID,
		SpeedFt:       30,
		Classes:       json.RawMessage(`[]`),
		AbilityScores: json.RawMessage(`{"str":15,"dex":10,"con":14,"int":10,"wis":10,"cha":10}`),
		EquippedArmor: sql.NullString{String: "splint", Valid: true},
	}

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return character, nil
	}
	store.getArmorFn = func(_ context.Context, id string) (refdata.Armor, error) {
		return refdata.Armor{
			ID:          "splint",
			Name:        "Splint Armor",
			ArmorType:   "heavy",
			StrengthReq: sql.NullInt32{Int32: 15, Valid: true},
		}, nil
	}

	svc := NewService(store)
	speed, _, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(30), speed, "STR 15 >= req 15: no penalty")
}
