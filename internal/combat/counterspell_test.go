package combat

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- TDD Cycle 1: AvailableCounterspellSlots ---

func TestAvailableCounterspellSlots_ReturnsLevels3AndAbove(t *testing.T) {
	slots := map[int]SlotInfo{
		1: {Current: 2, Max: 4},
		2: {Current: 1, Max: 3},
		3: {Current: 2, Max: 2},
		4: {Current: 1, Max: 1},
		5: {Current: 0, Max: 1}, // no slots remaining at 5
	}
	pact := PactMagicSlotState{}

	result := AvailableCounterspellSlots(slots, pact)
	assert.Equal(t, []int{3, 4}, result)
}

func TestAvailableCounterspellSlots_NoSlotsAvailable(t *testing.T) {
	slots := map[int]SlotInfo{
		1: {Current: 2, Max: 4},
		2: {Current: 1, Max: 3},
		3: {Current: 0, Max: 2},
	}
	pact := PactMagicSlotState{}

	result := AvailableCounterspellSlots(slots, pact)
	assert.Empty(t, result)
}

func TestAvailableCounterspellSlots_IncludesPactSlots(t *testing.T) {
	slots := map[int]SlotInfo{
		3: {Current: 0, Max: 2},
	}
	pact := PactMagicSlotState{SlotLevel: 5, Current: 1, Max: 1}

	result := AvailableCounterspellSlots(slots, pact)
	assert.Equal(t, []int{5}, result)
}

func TestAvailableCounterspellSlots_PactSlotBelow3(t *testing.T) {
	slots := map[int]SlotInfo{}
	pact := PactMagicSlotState{SlotLevel: 2, Current: 1, Max: 1}

	result := AvailableCounterspellSlots(slots, pact)
	assert.Empty(t, result)
}

// --- TDD Cycle 2: TriggerCounterspell ---

func testWizardCharacter() refdata.Character {
	slotsJSON, _ := json.Marshal(map[string]SlotInfo{
		"1": {Current: 4, Max: 4},
		"2": {Current: 3, Max: 3},
		"3": {Current: 2, Max: 2},
		"4": {Current: 1, Max: 1},
		"5": {Current: 1, Max: 1},
	})
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "wizard", Level: 9}})
	scoresJSON, _ := json.Marshal(AbilityScores{Str: 10, Dex: 14, Con: 12, Int: 18, Wis: 13, Cha: 8})

	return refdata.Character{
		ID:              uuid.New(),
		Name:            "Gandalf",
		Level:           9,
		ProficiencyBonus: 4,
		Classes:         classesJSON,
		AbilityScores:   scoresJSON,
		SpellSlots:      pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		PactMagicSlots:  pqtype.NullRawMessage{},
	}
}

func TestTriggerCounterspell_Success(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	char := testWizardCharacter()
	char.ID = charID

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:          declID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			Description: "Counterspell if enemy casts",
			Status:      "active",
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          combatantID,
			EncounterID: encounterID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Gandalf",
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.updateReactionDeclarationCounterspellPromptFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellPromptParams) (refdata.ReactionDeclaration, error) {
		assert.Equal(t, declID, arg.ID)
		assert.Equal(t, "Fireball", arg.CounterspellEnemySpell.String)
		assert.Equal(t, int32(5), arg.CounterspellEnemyLevel.Int32)
		return refdata.ReactionDeclaration{
			ID:                     declID,
			CounterspellEnemySpell: sql.NullString{String: "Fireball", Valid: true},
			CounterspellEnemyLevel: sql.NullInt32{Int32: 5, Valid: true},
			CounterspellStatus:     sql.NullString{String: "prompted", Valid: true},
		}, nil
	}

	svc := NewService(store)
	prompt, err := svc.TriggerCounterspell(context.Background(), declID, "Fireball", 5)
	require.NoError(t, err)
	assert.Equal(t, "Fireball", prompt.EnemySpellName)
	assert.Equal(t, []int{3, 4, 5}, prompt.AvailableSlots)
	assert.Equal(t, "Gandalf", prompt.CasterName)
	// Enemy cast level is intentionally not in CounterspellPrompt struct
}

func TestTriggerCounterspell_DeclarationNotActive(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{Status: "used"}, nil
	}

	svc := NewService(store)
	_, err := svc.TriggerCounterspell(context.Background(), uuid.New(), "Fireball", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not active")
}

func TestTriggerCounterspell_NoSlotsAvailable(t *testing.T) {
	declID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	char := testWizardCharacter()
	char.ID = charID
	// Zero out all slots >= 3
	slotsJSON, _ := json.Marshal(map[string]SlotInfo{
		"1": {Current: 4, Max: 4},
		"2": {Current: 3, Max: 3},
		"3": {Current: 0, Max: 2},
		"4": {Current: 0, Max: 1},
		"5": {Current: 0, Max: 1},
	})
	char.SpellSlots = pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true}

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:          declID,
			CombatantID: combatantID,
			Status:      "active",
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          combatantID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(store)
	_, err := svc.TriggerCounterspell(context.Background(), declID, "Fireball", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no spell slots")
}

// --- TDD Cycle 3: ResolveCounterspell - auto counter ---

func TestResolveCounterspell_AutoCounter(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	char := testWizardCharacter()
	char.ID = charID
	turnID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:                     declID,
			EncounterID:            encounterID,
			CombatantID:            combatantID,
			Status:                 "active",
			CounterspellEnemySpell: sql.NullString{String: "Fireball", Valid: true},
			CounterspellEnemyLevel: sql.NullInt32{Int32: 3, Valid: true},
			CounterspellStatus:     sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          combatantID,
			EncounterID: encounterID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Gandalf",
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"},
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, ReactionUsed: true}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, Status: "used"}, nil
	}
	store.updateReactionDeclarationCounterspellResolvedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error) {
		assert.Equal(t, int32(3), arg.CounterspellSlotUsed.Int32)
		assert.Equal(t, "countered", arg.CounterspellStatus.String)
		return refdata.ReactionDeclaration{ID: declID}, nil
	}

	svc := NewService(store)
	result, err := svc.ResolveCounterspell(context.Background(), declID, 3)
	require.NoError(t, err)
	assert.Equal(t, CounterspellCountered, result.Outcome)
	assert.Equal(t, "Gandalf", result.CasterName)
	assert.Equal(t, "Fireball", result.EnemySpellName)
	assert.Equal(t, 3, result.SlotUsed)
}

// --- TDD Cycle 4: ResolveCounterspell - needs ability check ---

func TestResolveCounterspell_NeedsAbilityCheck(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	char := testWizardCharacter()
	char.ID = charID
	turnID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:                     declID,
			EncounterID:            encounterID,
			CombatantID:            combatantID,
			Status:                 "active",
			CounterspellEnemySpell: sql.NullString{String: "Fireball", Valid: true},
			CounterspellEnemyLevel: sql.NullInt32{Int32: 5, Valid: true}, // cast at 5th level
			CounterspellStatus:     sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          combatantID,
			EncounterID: encounterID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Gandalf",
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"},
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, ReactionUsed: true}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, Status: "used"}, nil
	}
	store.updateReactionDeclarationCounterspellResolvedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error) {
		assert.Equal(t, int32(3), arg.CounterspellSlotUsed.Int32)
		assert.Equal(t, "needs_check", arg.CounterspellStatus.String)
		assert.Equal(t, int32(15), arg.CounterspellDc.Int32) // DC = 10 + 5
		return refdata.ReactionDeclaration{ID: declID}, nil
	}

	svc := NewService(store)
	result, err := svc.ResolveCounterspell(context.Background(), declID, 3) // slot 3 < enemy level 5
	require.NoError(t, err)
	assert.Equal(t, CounterspellNeedsCheck, result.Outcome)
	assert.Equal(t, 15, result.DC) // DC = 10 + enemy spell level 5
	assert.Equal(t, 5, result.EnemyCastLevel)
}

// --- TDD Cycle 5: ResolveCounterspellCheck ---

func TestResolveCounterspellCheck_Success(t *testing.T) {
	declID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:                     declID,
			CombatantID:            combatantID,
			CounterspellEnemySpell: sql.NullString{String: "Fireball", Valid: true},
			CounterspellEnemyLevel: sql.NullInt32{Int32: 5, Valid: true},
			CounterspellSlotUsed:   sql.NullInt32{Int32: 3, Valid: true},
			CounterspellStatus:     sql.NullString{String: "needs_check", Valid: true},
			CounterspellDc:         sql.NullInt32{Int32: 15, Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Gandalf", Conditions: json.RawMessage(`[]`)}, nil
	}
	store.updateReactionDeclarationCounterspellResolvedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error) {
		assert.Equal(t, "countered", arg.CounterspellStatus.String)
		return refdata.ReactionDeclaration{ID: declID}, nil
	}

	svc := NewService(store)
	result, err := svc.ResolveCounterspellCheck(context.Background(), declID, 15) // meets DC
	require.NoError(t, err)
	assert.Equal(t, CounterspellCountered, result.Outcome)
	assert.Equal(t, "Gandalf", result.CasterName)
}

func TestResolveCounterspellCheck_Failure(t *testing.T) {
	declID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:                     declID,
			CombatantID:            combatantID,
			CounterspellEnemySpell: sql.NullString{String: "Fireball", Valid: true},
			CounterspellEnemyLevel: sql.NullInt32{Int32: 5, Valid: true},
			CounterspellSlotUsed:   sql.NullInt32{Int32: 3, Valid: true},
			CounterspellStatus:     sql.NullString{String: "needs_check", Valid: true},
			CounterspellDc:         sql.NullInt32{Int32: 15, Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Gandalf", Conditions: json.RawMessage(`[]`)}, nil
	}
	store.updateReactionDeclarationCounterspellResolvedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error) {
		assert.Equal(t, "failed", arg.CounterspellStatus.String)
		return refdata.ReactionDeclaration{ID: declID}, nil
	}

	svc := NewService(store)
	result, err := svc.ResolveCounterspellCheck(context.Background(), declID, 14) // below DC
	require.NoError(t, err)
	assert.Equal(t, CounterspellFailed, result.Outcome)
}

func TestResolveCounterspellCheck_WrongStatus(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			CounterspellStatus: sql.NullString{String: "prompted", Valid: true},
		}, nil
	}

	svc := NewService(store)
	_, err := svc.ResolveCounterspellCheck(context.Background(), uuid.New(), 15)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in needs_check state")
}

// --- TDD Cycle 6: PassCounterspell ---

func TestPassCounterspell_Success(t *testing.T) {
	declID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:                     declID,
			CombatantID:            combatantID,
			Status:                 "active",
			CounterspellEnemySpell: sql.NullString{String: "Fireball", Valid: true},
			CounterspellStatus:     sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Gandalf", Conditions: json.RawMessage(`[]`)}, nil
	}
	store.updateReactionDeclarationCounterspellResolvedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error) {
		assert.Equal(t, "passed", arg.CounterspellStatus.String)
		return refdata.ReactionDeclaration{ID: declID}, nil
	}

	svc := NewService(store)
	result, err := svc.PassCounterspell(context.Background(), declID)
	require.NoError(t, err)
	assert.Equal(t, CounterspellPassed, result.Outcome)
	assert.Equal(t, "Gandalf", result.CasterName)
	assert.Equal(t, "Fireball", result.EnemySpellName)
}

func TestPassCounterspell_WrongStatus(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			CounterspellStatus: sql.NullString{String: "needs_check", Valid: true},
		}, nil
	}

	svc := NewService(store)
	_, err := svc.PassCounterspell(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in prompted state")
}

// --- TDD Cycle 7: ForfeitCounterspell ---

func TestForfeitCounterspell_Success(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:                     declID,
			EncounterID:            encounterID,
			CombatantID:            combatantID,
			Status:                 "active",
			CounterspellEnemySpell: sql.NullString{String: "Fireball", Valid: true},
			CounterspellStatus:     sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Gandalf", Conditions: json.RawMessage(`[]`)}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"},
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, ReactionUsed: true}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, Status: "used"}, nil
	}
	store.updateReactionDeclarationCounterspellResolvedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error) {
		assert.Equal(t, "forfeited", arg.CounterspellStatus.String)
		return refdata.ReactionDeclaration{ID: declID}, nil
	}

	svc := NewService(store)
	result, err := svc.ForfeitCounterspell(context.Background(), declID)
	require.NoError(t, err)
	assert.Equal(t, CounterspellForfeited, result.Outcome)
	assert.Equal(t, "Gandalf", result.CasterName)
}

func TestForfeitCounterspell_WrongStatus(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			CounterspellStatus: sql.NullString{String: "countered", Valid: true},
		}, nil
	}

	svc := NewService(store)
	_, err := svc.ForfeitCounterspell(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in prompted state")
}

// --- TDD Cycle 8: FormatCounterspellLog ---

func TestFormatCounterspellLog_Countered(t *testing.T) {
	result := CounterspellResult{
		Outcome:        CounterspellCountered,
		CasterName:     "Gandalf",
		EnemySpellName: "Fireball",
	}
	log := FormatCounterspellLog(result)
	assert.Contains(t, log, "Gandalf")
	assert.Contains(t, log, "counters")
	assert.Contains(t, log, "Fireball")
}

func TestFormatCounterspellLog_Failed(t *testing.T) {
	result := CounterspellResult{
		Outcome:        CounterspellFailed,
		CasterName:     "Gandalf",
		EnemySpellName: "Fireball",
		DC:             15,
	}
	log := FormatCounterspellLog(result)
	assert.Contains(t, log, "failed to counter")
	assert.Contains(t, log, "DC 15")
}

func TestFormatCounterspellLog_NeedsCheck(t *testing.T) {
	result := CounterspellResult{
		Outcome:        CounterspellNeedsCheck,
		CasterName:     "Gandalf",
		EnemySpellName: "Fireball",
		EnemyCastLevel: 5,
		SlotUsed:       3,
		DC:             15,
	}
	log := FormatCounterspellLog(result)
	assert.Contains(t, log, "attempts to counter")
	assert.Contains(t, log, "DC 15")
	assert.Contains(t, log, "3rd-level")
}

func TestFormatCounterspellLog_Passed(t *testing.T) {
	result := CounterspellResult{
		Outcome:        CounterspellPassed,
		CasterName:     "Gandalf",
		EnemySpellName: "Fireball",
	}
	log := FormatCounterspellLog(result)
	assert.Contains(t, log, "passes on Counterspell")
}

func TestFormatCounterspellLog_Forfeited(t *testing.T) {
	result := CounterspellResult{
		Outcome:        CounterspellForfeited,
		CasterName:     "Gandalf",
		EnemySpellName: "Fireball",
	}
	log := FormatCounterspellLog(result)
	assert.Contains(t, log, "forfeited")
	assert.Contains(t, log, "timeout")
}

func TestFormatCounterspellLog_Unknown(t *testing.T) {
	result := CounterspellResult{
		Outcome: "unknown",
	}
	log := FormatCounterspellLog(result)
	assert.Equal(t, "", log)
}

// --- TDD Cycle 9: HTTP Handler - TriggerCounterspell ---

func TestHandler_TriggerCounterspell_Success(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	char := testWizardCharacter()
	char.ID = charID

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:          declID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			Status:      "active",
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          combatantID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Gandalf",
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.updateReactionDeclarationCounterspellPromptFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellPromptParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID}, nil
	}

	_, r := newTestCombatRouter(store)

	body, _ := json.Marshal(map[string]interface{}{
		"enemy_spell_name": "Fireball",
		"enemy_cast_level": 5,
	})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/reactions/"+declID.String()+"/counterspell/trigger", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp counterspellPromptResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Fireball", resp.EnemySpellName)
	assert.Equal(t, "Gandalf", resp.CasterName)
	assert.NotEmpty(t, resp.AvailableSlots)
}

// --- TDD Cycle 10: HTTP Handler - ResolveCounterspell ---

func TestHandler_ResolveCounterspell_Success(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	char := testWizardCharacter()
	char.ID = charID
	turnID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:                     declID,
			EncounterID:            encounterID,
			CombatantID:            combatantID,
			Status:                 "active",
			CounterspellEnemySpell: sql.NullString{String: "Fireball", Valid: true},
			CounterspellEnemyLevel: sql.NullInt32{Int32: 3, Valid: true},
			CounterspellStatus:     sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          combatantID,
			EncounterID: encounterID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Gandalf",
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"},
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, ReactionUsed: true}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, Status: "used"}, nil
	}
	store.updateReactionDeclarationCounterspellResolvedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID}, nil
	}

	_, r := newTestCombatRouter(store)

	body, _ := json.Marshal(map[string]interface{}{
		"slot_level": 3,
	})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/reactions/"+declID.String()+"/counterspell/resolve", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp counterspellResultResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "countered", resp.Outcome)
}

// --- TDD Cycle 11: HTTP Handler - ResolveCounterspellCheck ---

func TestHandler_ResolveCounterspellCheck_Success(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:                     declID,
			CombatantID:            combatantID,
			CounterspellEnemySpell: sql.NullString{String: "Fireball", Valid: true},
			CounterspellEnemyLevel: sql.NullInt32{Int32: 5, Valid: true},
			CounterspellSlotUsed:   sql.NullInt32{Int32: 3, Valid: true},
			CounterspellStatus:     sql.NullString{String: "needs_check", Valid: true},
			CounterspellDc:         sql.NullInt32{Int32: 15, Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Gandalf", Conditions: json.RawMessage(`[]`)}, nil
	}
	store.updateReactionDeclarationCounterspellResolvedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID}, nil
	}

	_, r := newTestCombatRouter(store)

	body, _ := json.Marshal(map[string]interface{}{
		"check_total": 16,
	})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/reactions/"+declID.String()+"/counterspell/check", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp counterspellResultResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "countered", resp.Outcome)
}

// --- TDD Cycle 12: HTTP Handler - PassCounterspell ---

func TestHandler_PassCounterspell_Success(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:                     declID,
			CombatantID:            combatantID,
			Status:                 "active",
			CounterspellEnemySpell: sql.NullString{String: "Fireball", Valid: true},
			CounterspellStatus:     sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Gandalf", Conditions: json.RawMessage(`[]`)}, nil
	}
	store.updateReactionDeclarationCounterspellResolvedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID}, nil
	}

	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/reactions/"+declID.String()+"/counterspell/pass", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp counterspellResultResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "passed", resp.Outcome)
}

// --- TDD Cycle 13: HTTP Handler - ForfeitCounterspell ---

func TestHandler_ForfeitCounterspell_Success(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:                     declID,
			EncounterID:            encounterID,
			CombatantID:            combatantID,
			Status:                 "active",
			CounterspellEnemySpell: sql.NullString{String: "Fireball", Valid: true},
			CounterspellStatus:     sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Gandalf", Conditions: json.RawMessage(`[]`)}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"},
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, ReactionUsed: true}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, Status: "used"}, nil
	}
	store.updateReactionDeclarationCounterspellResolvedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID}, nil
	}

	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/reactions/"+declID.String()+"/counterspell/forfeit", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp counterspellResultResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "forfeited", resp.Outcome)
}

// --- TDD Cycle 14: Edge cases ---

func TestTriggerCounterspell_GetDeclarationError(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("not found")
	}

	svc := NewService(store)
	_, err := svc.TriggerCounterspell(context.Background(), uuid.New(), "Fireball", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting reaction declaration")
}

func TestTriggerCounterspell_NPCCannotCounterspell(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{Status: "active", CombatantID: uuid.New()}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{CharacterID: uuid.NullUUID{}, Conditions: json.RawMessage(`[]`)}, nil
	}

	svc := NewService(store)
	_, err := svc.TriggerCounterspell(context.Background(), uuid.New(), "Fireball", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only player characters")
}

func TestTriggerCounterspell_GetCombatantError(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{Status: "active", CombatantID: uuid.New()}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("not found")
	}

	svc := NewService(store)
	_, err := svc.TriggerCounterspell(context.Background(), uuid.New(), "Fireball", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting combatant")
}

func TestTriggerCounterspell_GetCharacterError(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{Status: "active", CombatantID: uuid.New()}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`)}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, errors.New("not found")
	}

	svc := NewService(store)
	_, err := svc.TriggerCounterspell(context.Background(), uuid.New(), "Fireball", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

func TestTriggerCounterspell_UpdatePromptError(t *testing.T) {
	charID := uuid.New()
	char := testWizardCharacter()
	char.ID = charID

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{Status: "active", CombatantID: uuid.New()}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`)}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.updateReactionDeclarationCounterspellPromptFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellPromptParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("db error")
	}

	svc := NewService(store)
	_, err := svc.TriggerCounterspell(context.Background(), uuid.New(), "Fireball", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating counterspell prompt")
}

func TestResolveCounterspell_NotPrompted(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			CounterspellStatus: sql.NullString{String: "countered", Valid: true},
		}, nil
	}

	svc := NewService(store)
	_, err := svc.ResolveCounterspell(context.Background(), uuid.New(), 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in prompted state")
}

func TestResolveCounterspell_GetDeclarationError(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("not found")
	}

	svc := NewService(store)
	_, err := svc.ResolveCounterspell(context.Background(), uuid.New(), 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting reaction declaration")
}

func TestResolveCounterspell_NPCCannotCounterspell(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			CombatantID:        uuid.New(),
			CounterspellStatus: sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{CharacterID: uuid.NullUUID{}, Conditions: json.RawMessage(`[]`)}, nil
	}

	svc := NewService(store)
	_, err := svc.ResolveCounterspell(context.Background(), uuid.New(), 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only player characters")
}

func TestResolveCounterspellCheck_GetDeclarationError(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("not found")
	}

	svc := NewService(store)
	_, err := svc.ResolveCounterspellCheck(context.Background(), uuid.New(), 15)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting reaction declaration")
}

func TestResolveCounterspellCheck_GetCombatantError(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			CombatantID:        uuid.New(),
			CounterspellStatus: sql.NullString{String: "needs_check", Valid: true},
			CounterspellDc:     sql.NullInt32{Int32: 15, Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("not found")
	}

	svc := NewService(store)
	_, err := svc.ResolveCounterspellCheck(context.Background(), uuid.New(), 15)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting combatant")
}

func TestResolveCounterspellCheck_UpdateError(t *testing.T) {
	combatantID := uuid.New()
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			CombatantID:        combatantID,
			CounterspellStatus: sql.NullString{String: "needs_check", Valid: true},
			CounterspellDc:     sql.NullInt32{Int32: 15, Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Gandalf", Conditions: json.RawMessage(`[]`)}, nil
	}
	store.updateReactionDeclarationCounterspellResolvedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("db error")
	}

	svc := NewService(store)
	_, err := svc.ResolveCounterspellCheck(context.Background(), uuid.New(), 15)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating counterspell check result")
}

func TestPassCounterspell_GetDeclarationError(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("not found")
	}

	svc := NewService(store)
	_, err := svc.PassCounterspell(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting reaction declaration")
}

func TestPassCounterspell_GetCombatantError(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			CombatantID:        uuid.New(),
			CounterspellStatus: sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("not found")
	}

	svc := NewService(store)
	_, err := svc.PassCounterspell(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting combatant")
}

func TestPassCounterspell_UpdateError(t *testing.T) {
	combatantID := uuid.New()
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			CombatantID:        combatantID,
			CounterspellStatus: sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Gandalf", Conditions: json.RawMessage(`[]`)}, nil
	}
	store.updateReactionDeclarationCounterspellResolvedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("db error")
	}

	svc := NewService(store)
	_, err := svc.PassCounterspell(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating counterspell passed")
}

func TestForfeitCounterspell_GetDeclarationError(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("not found")
	}

	svc := NewService(store)
	_, err := svc.ForfeitCounterspell(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting reaction declaration")
}

func TestForfeitCounterspell_GetCombatantError(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			CombatantID:        uuid.New(),
			Status:             "active",
			CounterspellStatus: sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("not found")
	}

	svc := NewService(store)
	_, err := svc.ForfeitCounterspell(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting combatant")
}

func TestForfeitCounterspell_ResolveReactionError(t *testing.T) {
	combatantID := uuid.New()
	encounterID := uuid.New()
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			CombatantID:        combatantID,
			EncounterID:        encounterID,
			Status:             "active",
			CounterspellStatus: sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Gandalf", Conditions: json.RawMessage(`[]`)}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, errors.New("no active turn")
	}

	svc := NewService(store)
	_, err := svc.ForfeitCounterspell(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolving reaction for forfeit")
}

func TestAvailableCounterspellSlots_PactSlotDuplicateLevel(t *testing.T) {
	slots := map[int]SlotInfo{
		3: {Current: 1, Max: 2},
		5: {Current: 1, Max: 1},
	}
	// Pact slot at level 5 which already exists in regular slots
	pact := PactMagicSlotState{SlotLevel: 5, Current: 1, Max: 1}

	result := AvailableCounterspellSlots(slots, pact)
	assert.Equal(t, []int{3, 5}, result) // no duplicate
}

func TestAvailableCounterspellSlots_PactSlotNoRemaining(t *testing.T) {
	slots := map[int]SlotInfo{
		3: {Current: 1, Max: 2},
	}
	pact := PactMagicSlotState{SlotLevel: 5, Current: 0, Max: 1}

	result := AvailableCounterspellSlots(slots, pact)
	assert.Equal(t, []int{3}, result)
}

// --- Handler edge case: invalid IDs ---

func TestHandler_TriggerCounterspell_InvalidEncounterID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())
	req := httptest.NewRequest("POST", "/api/combat/bad/reactions/"+uuid.New().String()+"/counterspell/trigger", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_TriggerCounterspell_InvalidReactionID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/reactions/bad/counterspell/trigger", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_TriggerCounterspell_InvalidJSON(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/reactions/"+uuid.New().String()+"/counterspell/trigger", bytes.NewReader([]byte("bad")))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ResolveCounterspell_InvalidJSON(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/reactions/"+uuid.New().String()+"/counterspell/resolve", bytes.NewReader([]byte("bad")))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ResolveCounterspellCheck_InvalidJSON(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/reactions/"+uuid.New().String()+"/counterspell/check", bytes.NewReader([]byte("bad")))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ResolveCounterspell_InvalidEncounterID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())
	req := httptest.NewRequest("POST", "/api/combat/bad/reactions/"+uuid.New().String()+"/counterspell/resolve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ResolveCounterspell_InvalidReactionID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/reactions/bad/counterspell/resolve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ResolveCounterspellCheck_InvalidEncounterID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())
	req := httptest.NewRequest("POST", "/api/combat/bad/reactions/"+uuid.New().String()+"/counterspell/check", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ResolveCounterspellCheck_InvalidReactionID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/reactions/bad/counterspell/check", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_PassCounterspell_InvalidEncounterID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())
	req := httptest.NewRequest("POST", "/api/combat/bad/reactions/"+uuid.New().String()+"/counterspell/pass", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_PassCounterspell_InvalidReactionID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/reactions/bad/counterspell/pass", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ForfeitCounterspell_InvalidEncounterID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())
	req := httptest.NewRequest("POST", "/api/combat/bad/reactions/"+uuid.New().String()+"/counterspell/forfeit", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestResolveCounterspell_GetCombatantError(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			CombatantID:        uuid.New(),
			CounterspellStatus: sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("not found")
	}

	svc := NewService(store)
	_, err := svc.ResolveCounterspell(context.Background(), uuid.New(), 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting combatant")
}

func TestResolveCounterspell_GetCharacterError(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			CombatantID:        uuid.New(),
			CounterspellStatus: sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`)}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, errors.New("not found")
	}

	svc := NewService(store)
	_, err := svc.ResolveCounterspell(context.Background(), uuid.New(), 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

func TestResolveCounterspell_UpdateCounterError(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	char := testWizardCharacter()
	char.ID = charID
	turnID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:                     declID,
			EncounterID:            encounterID,
			CombatantID:            combatantID,
			Status:                 "active",
			CounterspellEnemySpell: sql.NullString{String: "Fireball", Valid: true},
			CounterspellEnemyLevel: sql.NullInt32{Int32: 3, Valid: true},
			CounterspellStatus:     sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          combatantID,
			EncounterID: encounterID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Gandalf",
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"},
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, ReactionUsed: true}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, Status: "used"}, nil
	}
	store.updateReactionDeclarationCounterspellResolvedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("db error")
	}

	svc := NewService(store)
	_, err := svc.ResolveCounterspell(context.Background(), declID, 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating counterspell resolved")
}

func TestResolveCounterspell_NeedsCheckUpdateError(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	char := testWizardCharacter()
	char.ID = charID
	turnID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:                     declID,
			EncounterID:            encounterID,
			CombatantID:            combatantID,
			Status:                 "active",
			CounterspellEnemySpell: sql.NullString{String: "Fireball", Valid: true},
			CounterspellEnemyLevel: sql.NullInt32{Int32: 5, Valid: true},
			CounterspellStatus:     sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          combatantID,
			EncounterID: encounterID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Gandalf",
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"},
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, ReactionUsed: true}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, Status: "used"}, nil
	}
	store.updateReactionDeclarationCounterspellResolvedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("db error")
	}

	svc := NewService(store)
	_, err := svc.ResolveCounterspell(context.Background(), declID, 3) // 3 < 5, needs check
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating counterspell needs_check")
}

func TestResolveCounterspell_WithPactSlot(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	char := testWizardCharacter()
	char.ID = charID
	// Add pact magic slots at level 3
	pactJSON, _ := json.Marshal(PactMagicSlotState{SlotLevel: 3, Current: 1, Max: 1})
	char.PactMagicSlots = pqtype.NullRawMessage{RawMessage: pactJSON, Valid: true}
	turnID := uuid.New()

	var pactDeducted bool
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:                     declID,
			EncounterID:            encounterID,
			CombatantID:            combatantID,
			Status:                 "active",
			CounterspellEnemySpell: sql.NullString{String: "Fireball", Valid: true},
			CounterspellEnemyLevel: sql.NullInt32{Int32: 3, Valid: true},
			CounterspellStatus:     sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          combatantID,
			EncounterID: encounterID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Gandalf",
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"},
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, ReactionUsed: true}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, Status: "used"}, nil
	}
	store.updateReactionDeclarationCounterspellResolvedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID}, nil
	}
	store.updateCharacterPactMagicSlotsFn = func(ctx context.Context, arg refdata.UpdateCharacterPactMagicSlotsParams) (refdata.Character, error) {
		pactDeducted = true
		return refdata.Character{ID: charID}, nil
	}

	svc := NewService(store)
	result, err := svc.ResolveCounterspell(context.Background(), declID, 3)
	require.NoError(t, err)
	assert.Equal(t, CounterspellCountered, result.Outcome)
	assert.True(t, pactDeducted)
}

func TestResolveCounterspell_ResolveReactionError(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	char := testWizardCharacter()
	char.ID = charID

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:                     declID,
			EncounterID:            encounterID,
			CombatantID:            combatantID,
			Status:                 "active",
			CounterspellEnemySpell: sql.NullString{String: "Fireball", Valid: true},
			CounterspellEnemyLevel: sql.NullInt32{Int32: 3, Valid: true},
			CounterspellStatus:     sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          combatantID,
			EncounterID: encounterID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Gandalf",
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, errors.New("no active turn")
	}

	svc := NewService(store)
	_, err := svc.ResolveCounterspell(context.Background(), declID, 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolving reaction")
}

func TestResolveCounterspell_DeductSlotError(t *testing.T) {
	charID := uuid.New()
	char := testWizardCharacter()
	char.ID = charID

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			CombatantID:        uuid.New(),
			CounterspellStatus: sql.NullString{String: "prompted", Valid: true},
			CounterspellEnemyLevel: sql.NullInt32{Int32: 3, Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`)}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.updateCharacterSpellSlotsFn = func(ctx context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{}, errors.New("db error")
	}

	svc := NewService(store)
	_, err := svc.ResolveCounterspell(context.Background(), uuid.New(), 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deducting spell slot")
}

func TestResolveCounterspell_DeductPactSlotError(t *testing.T) {
	charID := uuid.New()
	char := testWizardCharacter()
	char.ID = charID
	pactJSON, _ := json.Marshal(PactMagicSlotState{SlotLevel: 3, Current: 1, Max: 1})
	char.PactMagicSlots = pqtype.NullRawMessage{RawMessage: pactJSON, Valid: true}

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			CombatantID:        uuid.New(),
			CounterspellStatus: sql.NullString{String: "prompted", Valid: true},
			CounterspellEnemyLevel: sql.NullInt32{Int32: 3, Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`)}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.updateCharacterPactMagicSlotsFn = func(ctx context.Context, arg refdata.UpdateCharacterPactMagicSlotsParams) (refdata.Character, error) {
		return refdata.Character{}, errors.New("db error")
	}

	svc := NewService(store)
	_, err := svc.ResolveCounterspell(context.Background(), uuid.New(), 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deducting pact slot")
}

func TestResolveCounterspell_InvalidSpellSlots(t *testing.T) {
	charID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			CombatantID:        uuid.New(),
			CounterspellStatus: sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`)}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:         charID,
			SpellSlots: pqtype.NullRawMessage{RawMessage: []byte("invalid"), Valid: true},
		}, nil
	}

	svc := NewService(store)
	_, err := svc.ResolveCounterspell(context.Background(), uuid.New(), 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing spell slots")
}

func TestTriggerCounterspell_InvalidSpellSlots(t *testing.T) {
	charID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{Status: "active", CombatantID: uuid.New()}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`)}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:         charID,
			SpellSlots: pqtype.NullRawMessage{RawMessage: []byte("invalid"), Valid: true},
		}, nil
	}

	svc := NewService(store)
	_, err := svc.TriggerCounterspell(context.Background(), uuid.New(), "Fireball", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing spell slots")
}

func TestForfeitCounterspell_UpdateCounterspellError(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	callCount := 0
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:                     declID,
			EncounterID:            encounterID,
			CombatantID:            combatantID,
			Status:                 "active",
			CounterspellEnemySpell: sql.NullString{String: "Fireball", Valid: true},
			CounterspellStatus:     sql.NullString{String: "prompted", Valid: true},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Gandalf", Conditions: json.RawMessage(`[]`)}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"},
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, ReactionUsed: true}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, Status: "used"}, nil
	}
	store.updateReactionDeclarationCounterspellResolvedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationCounterspellResolvedParams) (refdata.ReactionDeclaration, error) {
		callCount++
		return refdata.ReactionDeclaration{}, errors.New("db error")
	}

	svc := NewService(store)
	_, err := svc.ForfeitCounterspell(context.Background(), declID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating counterspell forfeited")
}

func TestHandler_ForfeitCounterspell_InvalidReactionID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/reactions/bad/counterspell/forfeit", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_TriggerCounterspell_ServiceError(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("not found")
	}

	_, r := newTestCombatRouter(store)
	body, _ := json.Marshal(map[string]interface{}{"enemy_spell_name": "Fireball", "enemy_cast_level": 5})
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/reactions/"+uuid.New().String()+"/counterspell/trigger", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ResolveCounterspell_ServiceError(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("not found")
	}

	_, r := newTestCombatRouter(store)
	body, _ := json.Marshal(map[string]interface{}{"slot_level": 3})
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/reactions/"+uuid.New().String()+"/counterspell/resolve", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ResolveCounterspellCheck_ServiceError(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("not found")
	}

	_, r := newTestCombatRouter(store)
	body, _ := json.Marshal(map[string]interface{}{"check_total": 15})
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/reactions/"+uuid.New().String()+"/counterspell/check", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_PassCounterspell_ServiceError(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("not found")
	}

	_, r := newTestCombatRouter(store)
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/reactions/"+uuid.New().String()+"/counterspell/pass", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ForfeitCounterspell_ServiceError(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("not found")
	}

	_, r := newTestCombatRouter(store)
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/reactions/"+uuid.New().String()+"/counterspell/forfeit", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
