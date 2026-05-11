package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- TDD Cycle 1: ReadyAction costs action and creates readied action declaration ---

func TestReadyAction_Success(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	declID := uuid.New()
	turnID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Aragorn",
		Conditions:  []byte(`[]`),
	}
	turn := refdata.Turn{
		ID:          turnID,
		EncounterID: encounterID,
		CombatantID: combatantID,
		ActionUsed:  false,
	}

	store := defaultMockStore()
	store.createReadiedActionDeclarationFn = func(ctx context.Context, arg refdata.CreateReadiedActionDeclarationParams) (refdata.ReactionDeclaration, error) {
		assert.Equal(t, encounterID, arg.EncounterID)
		assert.Equal(t, combatantID, arg.CombatantID)
		assert.Equal(t, "I attack when the goblin moves past me", arg.Description)
		assert.False(t, arg.SpellName.Valid)
		assert.False(t, arg.SpellSlotLevel.Valid)
		return refdata.ReactionDeclaration{
			ID:              declID,
			EncounterID:     encounterID,
			CombatantID:     combatantID,
			Description:     "I attack when the goblin moves past me",
			Status:          "active",
			IsReadiedAction: true,
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		assert.True(t, arg.ActionUsed)
		return refdata.Turn{ID: turnID, ActionUsed: true}, nil
	}

	svc := NewService(store)
	result, err := svc.ReadyAction(context.Background(), ReadyActionCommand{
		Combatant:   combatant,
		Turn:        turn,
		Description: "I attack when the goblin moves past me",
	})
	require.NoError(t, err)
	assert.Equal(t, declID, result.Declaration.ID)
	assert.True(t, result.Declaration.IsReadiedAction)
	assert.True(t, result.Turn.ActionUsed)
	assert.Contains(t, result.CombatLog, "Aragorn")
	assert.Contains(t, result.CombatLog, "readies an action")
}

// --- TDD Cycle 2: ReadyAction rejects when action already used ---

func TestReadyAction_ActionAlreadyUsed(t *testing.T) {
	store := defaultMockStore()
	svc := NewService(store)

	_, err := svc.ReadyAction(context.Background(), ReadyActionCommand{
		Combatant:   refdata.Combatant{Conditions: []byte(`[]`)},
		Turn:        refdata.Turn{ActionUsed: true},
		Description: "attack when goblin moves",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrResourceSpent)
}

// --- TDD Cycle 3: ReadyAction rejects empty description ---

func TestReadyAction_EmptyDescription(t *testing.T) {
	store := defaultMockStore()
	svc := NewService(store)

	_, err := svc.ReadyAction(context.Background(), ReadyActionCommand{
		Combatant:   refdata.Combatant{Conditions: []byte(`[]`)},
		Turn:        refdata.Turn{},
		Description: "   ",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description must not be empty")
}

// --- TDD Cycle 4: ReadyAction rejects when incapacitated ---

func TestReadyAction_Incapacitated(t *testing.T) {
	store := defaultMockStore()
	svc := NewService(store)

	_, err := svc.ReadyAction(context.Background(), ReadyActionCommand{
		Combatant:   refdata.Combatant{Conditions: []byte(`[{"condition":"stunned"}]`)},
		Turn:        refdata.Turn{},
		Description: "attack",
	})
	require.Error(t, err)
}

// --- TDD Cycle 5: ReadyAction with spell stores spell info ---

func TestReadyAction_WithSpell(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	declID := uuid.New()

	store := defaultMockStore()
	store.createReadiedActionDeclarationFn = func(ctx context.Context, arg refdata.CreateReadiedActionDeclarationParams) (refdata.ReactionDeclaration, error) {
		assert.True(t, arg.SpellName.Valid)
		assert.Equal(t, "Hold Person", arg.SpellName.String)
		assert.True(t, arg.SpellSlotLevel.Valid)
		assert.Equal(t, int32(2), arg.SpellSlotLevel.Int32)
		return refdata.ReactionDeclaration{
			ID:              declID,
			EncounterID:     encounterID,
			CombatantID:     combatantID,
			Description:     "Cast Hold Person if shaman moves",
			Status:          "active",
			IsReadiedAction: true,
			SpellName:       sql.NullString{String: "Hold Person", Valid: true},
			SpellSlotLevel:  sql.NullInt32{Int32: 2, Valid: true},
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ActionUsed: true}, nil
	}

	svc := NewService(store)
	result, err := svc.ReadyAction(context.Background(), ReadyActionCommand{
		Combatant:      refdata.Combatant{ID: combatantID, EncounterID: encounterID, DisplayName: "Gandalf", Conditions: []byte(`[]`)},
		Turn:           refdata.Turn{},
		Description:    "Cast Hold Person if shaman moves",
		SpellName:      "Hold Person",
		SpellSlotLevel: 2,
	})
	require.NoError(t, err)
	assert.True(t, result.Declaration.SpellName.Valid)
	assert.Equal(t, "Hold Person", result.Declaration.SpellName.String)
}

// --- TDD Cycle 6: ReadyAction DB error ---

func TestReadyAction_CreateDeclarationError(t *testing.T) {
	store := defaultMockStore()
	store.createReadiedActionDeclarationFn = func(ctx context.Context, arg refdata.CreateReadiedActionDeclarationParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("db error")
	}
	svc := NewService(store)

	_, err := svc.ReadyAction(context.Background(), ReadyActionCommand{
		Combatant:   refdata.Combatant{Conditions: []byte(`[]`)},
		Turn:        refdata.Turn{},
		Description: "attack",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating readied action declaration")
}

func TestReadyAction_UpdateTurnError(t *testing.T) {
	store := defaultMockStore()
	store.createReadiedActionDeclarationFn = func(ctx context.Context, arg refdata.CreateReadiedActionDeclarationParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{Status: "active", IsReadiedAction: true}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, errors.New("db error")
	}
	svc := NewService(store)

	_, err := svc.ReadyAction(context.Background(), ReadyActionCommand{
		Combatant:   refdata.Combatant{Conditions: []byte(`[]`)},
		Turn:        refdata.Turn{},
		Description: "attack",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// --- TDD Cycle 7: ExpireReadiedActions at turn start ---

func TestExpireReadiedActions_ExpiresActive(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	declID := uuid.New()

	store := defaultMockStore()
	store.listActiveReactionDeclarationsByCombatantFn = func(ctx context.Context, arg refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{
			{
				ID:              declID,
				EncounterID:     encounterID,
				CombatantID:     combatantID,
				Description:     "I attack when the goblin moves past me",
				Status:          "active",
				IsReadiedAction: true,
			},
		}, nil
	}
	store.cancelReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		assert.Equal(t, declID, id)
		return refdata.ReactionDeclaration{ID: declID, Status: "cancelled"}, nil
	}

	svc := NewService(store)
	notices, err := svc.ExpireReadiedActions(context.Background(), combatantID, encounterID)
	require.NoError(t, err)
	require.Len(t, notices, 1)
	assert.Contains(t, notices[0], "Your readied action expired unused")
	assert.Contains(t, notices[0], "I attack when the goblin moves past me")
}

func TestExpireReadiedActions_NoReadiedActions(t *testing.T) {
	store := defaultMockStore()
	store.listActiveReactionDeclarationsByCombatantFn = func(ctx context.Context, arg refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{
			{Description: "Shield if I get hit", Status: "active", IsReadiedAction: false},
		}, nil
	}

	svc := NewService(store)
	notices, err := svc.ExpireReadiedActions(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, notices)
}

func TestExpireReadiedActions_SpellExpiry(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	declID := uuid.New()

	store := defaultMockStore()
	store.listActiveReactionDeclarationsByCombatantFn = func(ctx context.Context, arg refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{
			{
				ID:              declID,
				EncounterID:     encounterID,
				CombatantID:     combatantID,
				Description:     "Cast Hold Person if shaman moves",
				Status:          "active",
				IsReadiedAction: true,
				SpellName:       sql.NullString{String: "Hold Person", Valid: true},
				SpellSlotLevel:  sql.NullInt32{Int32: 2, Valid: true},
			},
		}, nil
	}
	store.cancelReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, Status: "cancelled"}, nil
	}

	svc := NewService(store)
	notices, err := svc.ExpireReadiedActions(context.Background(), combatantID, encounterID)
	require.NoError(t, err)
	require.Len(t, notices, 1)
	assert.Contains(t, notices[0], "Your readied action expired unused")
	assert.Contains(t, notices[0], "Hold Person")
	assert.Contains(t, notices[0], "spell slot lost")
}

// --- TDD Cycle 8: FormatTurnStartPrompt includes expiry notices ---

func TestFormatTurnStartPromptWithExpiry(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	}
	notices := []string{
		"\u23f3 Your readied action expired unused: \"I attack when the goblin moves past me\"",
	}

	prompt := FormatTurnStartPromptWithExpiry("Battle of Helms Deep", 2, "Aragorn", turn, nil, notices)
	assert.Contains(t, prompt, "readied action expired unused")
	assert.Contains(t, prompt, "Battle of Helms Deep")
	assert.Contains(t, prompt, "@Aragorn")
}

func TestFormatTurnStartPromptWithExpiry_NoNotices(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	}

	prompt := FormatTurnStartPromptWithExpiry("Battle", 1, "Aragorn", turn, nil, nil)
	// Should be the same as regular prompt
	regularPrompt := FormatTurnStartPrompt("Battle", 1, "Aragorn", turn, nil)
	assert.Equal(t, regularPrompt, prompt)
}

// --- TDD Cycle 8b: FormatReadiedActionsStatus ---

func TestFormatReadiedActionsStatus_Empty(t *testing.T) {
	result := FormatReadiedActionsStatus(nil)
	assert.Empty(t, result)
}

func TestFormatReadiedActionsStatus_NonSpell(t *testing.T) {
	readied := []refdata.ReactionDeclaration{
		{Description: "Attack when goblin moves", IsReadiedAction: true},
	}
	result := FormatReadiedActionsStatus(readied)
	assert.Contains(t, result, "Readied Actions")
	assert.Contains(t, result, "Attack when goblin moves")
}

func TestFormatReadiedActionsStatus_WithSpell(t *testing.T) {
	readied := []refdata.ReactionDeclaration{
		{
			Description:    "Cast Fireball when clustered",
			IsReadiedAction: true,
			SpellName:      sql.NullString{String: "Fireball", Valid: true},
			SpellSlotLevel: sql.NullInt32{Int32: 3, Valid: true},
		},
	}
	result := FormatReadiedActionsStatus(readied)
	assert.Contains(t, result, "Fireball")
	assert.Contains(t, result, "3rd-level")
}

func TestFormatOrdinalSlotLevel(t *testing.T) {
	assert.Equal(t, "1st-level", formatOrdinalSlotLevel(1))
	assert.Equal(t, "2nd-level", formatOrdinalSlotLevel(2))
	assert.Equal(t, "3rd-level", formatOrdinalSlotLevel(3))
	assert.Equal(t, "4th-level", formatOrdinalSlotLevel(4))
	assert.Equal(t, "5th-level", formatOrdinalSlotLevel(5))
	assert.Equal(t, "9th-level", formatOrdinalSlotLevel(9))
}

// --- TDD Cycle 8c: ExpireReadiedActions error paths ---

func TestExpireReadiedActions_ListError(t *testing.T) {
	store := defaultMockStore()
	store.listActiveReactionDeclarationsByCombatantFn = func(ctx context.Context, arg refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return nil, errors.New("db error")
	}

	svc := NewService(store)
	_, err := svc.ExpireReadiedActions(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing active reactions for expiry")
}

func TestExpireReadiedActions_CancelError(t *testing.T) {
	store := defaultMockStore()
	store.listActiveReactionDeclarationsByCombatantFn = func(ctx context.Context, arg refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{
			{ID: uuid.New(), IsReadiedAction: true, Description: "attack"},
		}, nil
	}
	store.cancelReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("cancel failed")
	}

	svc := NewService(store)
	_, err := svc.ExpireReadiedActions(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelling expired readied action")
}

func TestListReadiedActions_Error(t *testing.T) {
	store := defaultMockStore()
	store.listActiveReactionDeclarationsByCombatantFn = func(ctx context.Context, arg refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return nil, errors.New("db error")
	}

	svc := NewService(store)
	_, err := svc.ListReadiedActions(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing active reactions")
}

// --- TDD Cycle 9: ListReadiedActions for status display ---

func TestListReadiedActions(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.listActiveReactionDeclarationsByCombatantFn = func(ctx context.Context, arg refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{
			{Description: "Shield if I get hit", Status: "active", IsReadiedAction: false},
			{Description: "I attack when the goblin moves", Status: "active", IsReadiedAction: true},
			{Description: "Cast Fireball when clustered", Status: "active", IsReadiedAction: true,
				SpellName: sql.NullString{String: "Fireball", Valid: true}},
		}, nil
	}

	svc := NewService(store)
	readied, err := svc.ListReadiedActions(context.Background(), combatantID, encounterID)
	require.NoError(t, err)
	assert.Len(t, readied, 2)
	assert.True(t, readied[0].IsReadiedAction)
	assert.True(t, readied[1].IsReadiedAction)
}

// --- med-28 / Phase 71: ReadyAction deducts slot + sets concentration ---

func TestReadyAction_WithSpell_DeductsSlot(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	char := makeWizardCharacter(charID)

	slotDeducted := false
	concentrationSet := false
	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
		assert.Equal(t, charID, id)
		return char, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		slotDeducted = true
		// The 2nd-level slot should drop from 3 to 2.
		var slots map[string]SlotInfo
		require.NoError(t, json.Unmarshal(arg.SpellSlots.RawMessage, &slots))
		assert.Equal(t, 2, slots["2"].Current)
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}
	store.setCombatantConcentrationFn = func(_ context.Context, arg refdata.SetCombatantConcentrationParams) error {
		concentrationSet = true
		assert.Equal(t, combatantID, arg.ID)
		assert.True(t, arg.ConcentrationSpellName.Valid)
		assert.Equal(t, "Hold Person", arg.ConcentrationSpellName.String)
		return nil
	}
	store.createReadiedActionDeclarationFn = func(_ context.Context, arg refdata.CreateReadiedActionDeclarationParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID: uuid.New(), EncounterID: arg.EncounterID, CombatantID: arg.CombatantID,
			Description: arg.Description, Status: "active", IsReadiedAction: true,
			SpellName: arg.SpellName, SpellSlotLevel: arg.SpellSlotLevel,
		}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, _ refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ActionUsed: true}, nil
	}

	svc := NewService(store)
	_, err := svc.ReadyAction(context.Background(), ReadyActionCommand{
		Combatant: refdata.Combatant{
			ID:          combatantID,
			EncounterID: encounterID,
			DisplayName: "Gandalf",
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  []byte(`[]`),
		},
		Turn:           refdata.Turn{},
		Description:    "Cast Hold Person if shaman moves",
		SpellName:      "Hold Person",
		SpellSlotLevel: 2,
	})
	require.NoError(t, err)
	assert.True(t, slotDeducted, "spell slot must be deducted at ready time")
	assert.True(t, concentrationSet, "concentration must be set on the readied spell")
}

// TestReadyAction_NonSpell_SkipsSlotAndConcentration verifies the non-spell
// path doesn't try to deduct a slot or set concentration.
func TestReadyAction_NonSpell_SkipsSlotAndConcentration(t *testing.T) {
	store := defaultMockStore()
	concentrationCalls := 0
	slotCalls := 0
	store.setCombatantConcentrationFn = func(_ context.Context, _ refdata.SetCombatantConcentrationParams) error {
		concentrationCalls++
		return nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		slotCalls++
		return refdata.Character{ID: arg.ID}, nil
	}
	store.createReadiedActionDeclarationFn = func(_ context.Context, arg refdata.CreateReadiedActionDeclarationParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: uuid.New(), Status: "active", IsReadiedAction: true}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, _ refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ActionUsed: true}, nil
	}

	svc := NewService(store)
	_, err := svc.ReadyAction(context.Background(), ReadyActionCommand{
		Combatant:   refdata.Combatant{ID: uuid.New(), DisplayName: "Frodo", Conditions: []byte(`[]`)},
		Turn:        refdata.Turn{},
		Description: "swing my sword if the door opens",
	})
	require.NoError(t, err)
	assert.Zero(t, slotCalls, "non-spell readied action must not deduct any slot")
	assert.Zero(t, concentrationCalls, "non-spell readied action must not touch concentration")
}
