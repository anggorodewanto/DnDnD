package combat_test

import (
	"context"
	"testing"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Integration: ReadyAction costs action and creates readied declaration ---

func TestIntegration_ReadyAction_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	encounterID, combatantID := createTestEncounterAndCombatant(t, db, queries)
	svc := combat.NewService(combat.NewStoreAdapter(queries))

	// Create active turn
	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:         encounterID,
		CombatantID:         combatantID,
		RoundNumber:         1,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	})
	require.NoError(t, err)

	_, err = queries.UpdateEncounterCurrentTurn(ctx, refdata.UpdateEncounterCurrentTurnParams{
		ID:            encounterID,
		CurrentTurnID: uuid.NullUUID{UUID: turn.ID, Valid: true},
	})
	require.NoError(t, err)

	combatant, err := queries.GetCombatant(ctx, combatantID)
	require.NoError(t, err)

	result, err := svc.ReadyAction(ctx, combat.ReadyActionCommand{
		Combatant:   combatant,
		Turn:        turn,
		Description: "I attack when the goblin moves past me",
	})
	require.NoError(t, err)
	assert.True(t, result.Turn.ActionUsed)
	assert.True(t, result.Declaration.IsReadiedAction)
	assert.Equal(t, "active", result.Declaration.Status)
	assert.Equal(t, "I attack when the goblin moves past me", result.Declaration.Description)
	assert.Contains(t, result.CombatLog, "readies an action")

	// Verify it shows up in active reactions
	active, err := svc.ListActiveReactions(ctx, encounterID)
	require.NoError(t, err)
	assert.Len(t, active, 1)
	assert.True(t, active[0].IsReadiedAction)
}

// --- Integration: ReadyAction with spell info ---

func TestIntegration_ReadyAction_WithSpell(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	encounterID, combatantID := createTestEncounterAndCombatant(t, db, queries)
	svc := combat.NewService(combat.NewStoreAdapter(queries))

	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:         encounterID,
		CombatantID:         combatantID,
		RoundNumber:         1,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	})
	require.NoError(t, err)

	combatant, err := queries.GetCombatant(ctx, combatantID)
	require.NoError(t, err)

	result, err := svc.ReadyAction(ctx, combat.ReadyActionCommand{
		Combatant:      combatant,
		Turn:           turn,
		Description:    "Cast Hold Person if shaman moves",
		SpellName:      "Hold Person",
		SpellSlotLevel: 2,
	})
	require.NoError(t, err)
	assert.True(t, result.Declaration.IsReadiedAction)
	assert.True(t, result.Declaration.SpellName.Valid)
	assert.Equal(t, "Hold Person", result.Declaration.SpellName.String)
	assert.True(t, result.Declaration.SpellSlotLevel.Valid)
	assert.Equal(t, int32(2), result.Declaration.SpellSlotLevel.Int32)
}

// --- Integration: ExpireReadiedActions at turn start ---

func TestIntegration_ExpireReadiedActions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	encounterID, combatantID := createTestEncounterAndCombatant(t, db, queries)
	svc := combat.NewService(combat.NewStoreAdapter(queries))

	// Create turn and ready an action
	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:         encounterID,
		CombatantID:         combatantID,
		RoundNumber:         1,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	})
	require.NoError(t, err)

	combatant, err := queries.GetCombatant(ctx, combatantID)
	require.NoError(t, err)

	_, err = svc.ReadyAction(ctx, combat.ReadyActionCommand{
		Combatant:   combatant,
		Turn:        turn,
		Description: "I attack when the goblin moves past me",
	})
	require.NoError(t, err)

	// Expire readied actions (simulates turn start of next round)
	notices, err := svc.ExpireReadiedActions(ctx, combatantID, encounterID)
	require.NoError(t, err)
	require.Len(t, notices, 1)
	assert.Contains(t, notices[0], "readied action expired unused")
	assert.Contains(t, notices[0], "I attack when the goblin moves past me")

	// Verify no active readied actions remain
	readied, err := svc.ListReadiedActions(ctx, combatantID, encounterID)
	require.NoError(t, err)
	assert.Empty(t, readied)
}

// --- Integration: ExpireReadiedActions with spell ---

func TestIntegration_ExpireReadiedActions_WithSpell(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	encounterID, combatantID := createTestEncounterAndCombatant(t, db, queries)
	svc := combat.NewService(combat.NewStoreAdapter(queries))

	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:         encounterID,
		CombatantID:         combatantID,
		RoundNumber:         1,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	})
	require.NoError(t, err)

	combatant, err := queries.GetCombatant(ctx, combatantID)
	require.NoError(t, err)

	_, err = svc.ReadyAction(ctx, combat.ReadyActionCommand{
		Combatant:      combatant,
		Turn:           turn,
		Description:    "Cast Hold Person if shaman moves",
		SpellName:      "Hold Person",
		SpellSlotLevel: 2,
	})
	require.NoError(t, err)

	notices, err := svc.ExpireReadiedActions(ctx, combatantID, encounterID)
	require.NoError(t, err)
	require.Len(t, notices, 1)
	assert.Contains(t, notices[0], "Hold Person")
	assert.Contains(t, notices[0], "spell slot lost")
}

// --- Integration: Readied action fires via ResolveReaction (DM resolution) ---

func TestIntegration_ReadiedAction_ResolvedByDM(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	encounterID, combatantID := createTestEncounterAndCombatant(t, db, queries)
	svc := combat.NewService(combat.NewStoreAdapter(queries))

	// Create turn
	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:         encounterID,
		CombatantID:         combatantID,
		RoundNumber:         1,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	})
	require.NoError(t, err)

	_, err = queries.UpdateEncounterCurrentTurn(ctx, refdata.UpdateEncounterCurrentTurnParams{
		ID:            encounterID,
		CurrentTurnID: uuid.NullUUID{UUID: turn.ID, Valid: true},
	})
	require.NoError(t, err)

	combatant, err := queries.GetCombatant(ctx, combatantID)
	require.NoError(t, err)

	// Ready an action
	result, err := svc.ReadyAction(ctx, combat.ReadyActionCommand{
		Combatant:   combatant,
		Turn:        turn,
		Description: "I attack when the goblin moves past me",
	})
	require.NoError(t, err)

	// DM resolves the readied action (fires it)
	resolved, err := svc.ResolveReaction(ctx, result.Declaration.ID)
	require.NoError(t, err)
	assert.Equal(t, "used", resolved.Status)

	// Verify reaction_used is set on the turn
	updatedTurn, err := queries.GetTurn(ctx, turn.ID)
	require.NoError(t, err)
	assert.True(t, updatedTurn.ReactionUsed)
}

// --- Integration: ListReadiedActions shows only readied, not regular reactions ---

func TestIntegration_ListReadiedActions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	encounterID, combatantID := createTestEncounterAndCombatant(t, db, queries)
	svc := combat.NewService(combat.NewStoreAdapter(queries))

	// Create turn
	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:         encounterID,
		CombatantID:         combatantID,
		RoundNumber:         1,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	})
	require.NoError(t, err)

	combatant, err := queries.GetCombatant(ctx, combatantID)
	require.NoError(t, err)

	// Declare a regular reaction
	_, err = svc.DeclareReaction(ctx, encounterID, combatantID, "Shield if I get hit")
	require.NoError(t, err)

	// Ready an action
	_, err = svc.ReadyAction(ctx, combat.ReadyActionCommand{
		Combatant:   combatant,
		Turn:        turn,
		Description: "I attack when the goblin moves past me",
	})
	require.NoError(t, err)

	// ListReadiedActions should only return the readied one
	readied, err := svc.ListReadiedActions(ctx, combatantID, encounterID)
	require.NoError(t, err)
	assert.Len(t, readied, 1)
	assert.True(t, readied[0].IsReadiedAction)
	assert.Equal(t, "I attack when the goblin moves past me", readied[0].Description)

	// ListActiveReactions should return both
	active, err := svc.ListActiveReactions(ctx, encounterID)
	require.NoError(t, err)
	assert.Len(t, active, 2)
}

// --- Integration: FormatTurnStartPromptWithExpiry ---

func TestIntegration_FormatTurnStartPromptWithExpiry(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	}
	notices := []string{
		"\u23f3 Your readied action expired unused: \"I attack when the goblin moves past me\"",
	}

	prompt := combat.FormatTurnStartPromptWithExpiry("Battle of Helms Deep", 2, "Aragorn", turn, nil, notices)
	assert.Contains(t, prompt, "readied action expired unused")
	assert.Contains(t, prompt, "Battle of Helms Deep")
	assert.Contains(t, prompt, "@Aragorn")
	assert.Contains(t, prompt, "Round 2")

	// Spell expiry notice
	spellNotices := []string{
		"\u23f3 Your readied action expired unused: \"Cast Hold Person if shaman moves\"\n   \u2192 Concentration on Hold Person ended. 2nd-level spell slot lost.",
	}
	spellPrompt := combat.FormatTurnStartPromptWithExpiry("Battle", 3, "Gandalf", turn, nil, spellNotices)
	assert.Contains(t, spellPrompt, "Hold Person")
	assert.Contains(t, spellPrompt, "spell slot lost")
}

// --- Integration: ReadyAction rejects when action already used ---

func TestIntegration_ReadyAction_ActionUsed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	encounterID, combatantID := createTestEncounterAndCombatant(t, db, queries)
	svc := combat.NewService(combat.NewStoreAdapter(queries))

	turn := refdata.Turn{
		EncounterID:         encounterID,
		CombatantID:         combatantID,
		ActionUsed:          true,
		MovementRemainingFt: 30,
	}

	combatant, err := queries.GetCombatant(ctx, combatantID)
	require.NoError(t, err)

	_, err = svc.ReadyAction(ctx, combat.ReadyActionCommand{
		Combatant:   combatant,
		Turn:        turn,
		Description: "attack when goblin moves",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "action")
}

// --- Integration: Status display shows readied actions ---

func TestIntegration_StatusDisplayShowsReadiedActions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	encounterID, combatantID := createTestEncounterAndCombatant(t, db, queries)
	svc := combat.NewService(combat.NewStoreAdapter(queries))

	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:         encounterID,
		CombatantID:         combatantID,
		RoundNumber:         1,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	})
	require.NoError(t, err)

	combatant, err := queries.GetCombatant(ctx, combatantID)
	require.NoError(t, err)

	// Ready two actions (need separate turns, but for test just use fresh turn)
	_, err = svc.ReadyAction(ctx, combat.ReadyActionCommand{
		Combatant:      combatant,
		Turn:           turn,
		Description:    "Cast Fireball when they cluster",
		SpellName:      "Fireball",
		SpellSlotLevel: 3,
	})
	require.NoError(t, err)

	// Format status display
	readied, err := svc.ListReadiedActions(ctx, combatantID, encounterID)
	require.NoError(t, err)
	assert.Len(t, readied, 1)

	statusLines := combat.FormatReadiedActionsStatus(readied)
	assert.Contains(t, statusLines, "Fireball")
	assert.Contains(t, statusLines, "Cast Fireball when they cluster")
}
