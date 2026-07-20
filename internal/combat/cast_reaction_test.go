package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeHellishRebuke is the reaction-spell archetype: cast time "1 reaction",
// single-target DEX save-for-half fire damage, auto-resolved.
func makeHellishRebuke() refdata.Spell {
	return refdata.Spell{
		ID:             "hellish-rebuke",
		Name:           "Hellish Rebuke",
		Level:          1,
		CastingTime:    "1 reaction",
		RangeType:      "ranged",
		RangeFt:        sql.NullInt32{Int32: 60, Valid: true},
		SaveAbility:    sql.NullString{String: "dex", Valid: true},
		SaveEffect:     sql.NullString{String: "half_damage", Valid: true},
		Damage:         pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"dice":"2d10","type":"fire"}`), Valid: true},
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: false, Valid: true},
	}
}

// TestCast_ReactionSpell_ChargesReactionNotAction locks the fix for the live
// bug: a PC casting a reaction spell (Hellish Rebuke) during an ENEMY's turn
// used to fail with "resource already spent" because Cast charged the active
// creature's action. A reaction spell must instead be charged against the
// caster's reaction, must NOT mutate the active creature's turn, and must
// still deduct the caster's slot + resolve the spell effect.
func TestCast_ReactionSpell_ChargesReactionNotAction(t *testing.T) {
	charID := uuid.New()
	char := makeWarlockCharacter(charID)
	caster := makeSpellCaster(charID)
	enemy := makeSpellTarget() // the creature that damaged the caster
	encID := uuid.New()
	enemyTurnID := uuid.New()

	var turnPersisted bool
	var pactSlotSaved bool
	reactionMarkedRound := int32(-1)

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeHellishRebuke(), nil }
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return enemy, nil
	}
	// Active turn belongs to the ENEMY, round 1, action already spent.
	store.getActiveTurnByEncounterIDFn = func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: enemyTurnID, EncounterID: encID, CombatantID: enemy.ID, RoundNumber: 1, Status: "active", ActionUsed: true}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		turnPersisted = true
		return refdata.Turn{ID: arg.ID}, nil
	}
	store.updateCharacterPactMagicSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterPactMagicSlotsParams) (refdata.Character, error) {
		pactSlotSaved = true
		return refdata.Character{ID: arg.ID}, nil
	}
	store.createReactionDeclarationFn = func(_ context.Context, arg refdata.CreateReactionDeclarationParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: uuid.New(), EncounterID: arg.EncounterID, CombatantID: arg.CombatantID, Description: arg.Description, Status: "active"}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(_ context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		reactionMarkedRound = arg.UsedOnRound.Int32
		return refdata.ReactionDeclaration{ID: arg.ID, Status: "used", UsedOnRound: arg.UsedOnRound}, nil
	}
	store.createPendingSaveFn = func(_ context.Context, _ refdata.CreatePendingSaveParams) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: uuid.New()}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:     "hellish-rebuke",
		CasterID:    caster.ID,
		TargetID:    enemy.ID,
		EncounterID: encID,
		// The active turn belongs to the ENEMY and its action is spent; a
		// reaction cast must not be charged against it (synthetic zero turn in
		// production, an already-spent enemy turn here to prove the point).
		Turn: refdata.Turn{ID: enemyTurnID, CombatantID: enemy.ID, ActionUsed: true},
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.Equal(t, "Hellish Rebuke", result.SpellName)
	assert.True(t, result.UsedPactSlot, "reaction spell should consume the caster's pact slot")
	assert.True(t, pactSlotSaved, "pact slot deduction should persist")
	assert.False(t, turnPersisted, "reaction cast must not mutate the active creature's turn")
	assert.Equal(t, int32(1), reactionMarkedRound, "caster's reaction should be marked used for the round")
}

// TestCast_ReactionSpell_RejectsWhenReactionAlreadyUsed proves the reaction
// economy is enforced: if the caster already spent their reaction this round
// (a used declaration stamped with the current round), the cast is rejected
// and no slot is burned.
func TestCast_ReactionSpell_RejectsWhenReactionAlreadyUsed(t *testing.T) {
	charID := uuid.New()
	char := makeWarlockCharacter(charID)
	caster := makeSpellCaster(charID)
	enemy := makeSpellTarget()
	encID := uuid.New()

	var pactSlotSaved bool

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeHellishRebuke(), nil }
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return enemy, nil
	}
	store.getActiveTurnByEncounterIDFn = func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), EncounterID: encID, CombatantID: enemy.ID, RoundNumber: 2, Status: "active"}, nil
	}
	// Caster already used a reaction this round.
	store.listReactionDeclarationsByCombatantFn = func(_ context.Context, _ refdata.ListReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{{ID: uuid.New(), CombatantID: caster.ID, Status: "used", UsedOnRound: sql.NullInt32{Int32: 2, Valid: true}}}, nil
	}
	store.updateCharacterPactMagicSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterPactMagicSlotsParams) (refdata.Character, error) {
		pactSlotSaved = true
		return refdata.Character{ID: arg.ID}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:     "hellish-rebuke",
		CasterID:    caster.ID,
		TargetID:    enemy.ID,
		EncounterID: encID,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: enemy.ID},
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrReactionAlreadyUsed)
	assert.False(t, pactSlotSaved, "a rejected reaction cast must not burn a slot")
}

// TestCast_ReactionSpell_ReusesPreDeclaredReaction proves a pre-declared
// reaction (e.g. "hellish rebuke if attacked") is the one marked used, rather
// than orphaning it and creating a second declaration.
func TestCast_ReactionSpell_ReusesPreDeclaredReaction(t *testing.T) {
	charID := uuid.New()
	char := makeWarlockCharacter(charID)
	caster := makeSpellCaster(charID)
	enemy := makeSpellTarget()
	encID := uuid.New()
	preDeclID := uuid.New()

	var createdDeclaration bool
	var markedUsedID uuid.UUID

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeHellishRebuke(), nil }
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return enemy, nil
	}
	store.getActiveTurnByEncounterIDFn = func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), EncounterID: encID, CombatantID: enemy.ID, RoundNumber: 1, Status: "active"}, nil
	}
	store.listActiveReactionDeclarationsByCombatantFn = func(_ context.Context, _ refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{{ID: preDeclID, CombatantID: caster.ID, Description: "hellish rebuke if attacked", Status: "active"}}, nil
	}
	store.createReactionDeclarationFn = func(_ context.Context, _ refdata.CreateReactionDeclarationParams) (refdata.ReactionDeclaration, error) {
		createdDeclaration = true
		return refdata.ReactionDeclaration{ID: uuid.New()}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(_ context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		markedUsedID = arg.ID
		return refdata.ReactionDeclaration{ID: arg.ID, Status: "used"}, nil
	}
	store.updateCharacterPactMagicSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterPactMagicSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID}, nil
	}
	store.createPendingSaveFn = func(_ context.Context, _ refdata.CreatePendingSaveParams) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: uuid.New()}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{SpellID: "hellish-rebuke", CasterID: caster.ID, TargetID: enemy.ID, EncounterID: encID, Turn: refdata.Turn{ID: uuid.New(), CombatantID: enemy.ID}}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.False(t, createdDeclaration, "should reuse the pre-declared reaction, not create a new one")
	assert.Equal(t, preDeclID, markedUsedID, "the pre-declared reaction should be the one marked used")
}

// TestCast_ReactionSpell_OutOfCombat_NoReactionTracked proves a reaction spell
// cast with no active turn (exploration) still resolves and does not attempt to
// stamp a reaction — reactions aren't tracked out of combat.
func TestCast_ReactionSpell_OutOfCombat_NoReactionTracked(t *testing.T) {
	charID := uuid.New()
	char := makeWarlockCharacter(charID)
	caster := makeSpellCaster(charID)
	enemy := makeSpellTarget()
	encID := uuid.New()

	var markedUsed bool

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeHellishRebuke(), nil }
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return enemy, nil
	}
	store.getActiveTurnByEncounterIDFn = func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, sql.ErrNoRows
	}
	store.updateReactionDeclarationStatusUsedFn = func(_ context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		markedUsed = true
		return refdata.ReactionDeclaration{ID: arg.ID}, nil
	}
	store.updateCharacterPactMagicSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterPactMagicSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID}, nil
	}
	store.createPendingSaveFn = func(_ context.Context, _ refdata.CreatePendingSaveParams) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: uuid.New()}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{SpellID: "hellish-rebuke", CasterID: caster.ID, TargetID: enemy.ID, EncounterID: encID, Turn: refdata.Turn{}}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.True(t, result.UsedPactSlot, "the spell should still resolve out of combat")
	assert.False(t, markedUsed, "no reaction should be tracked when there is no active turn")
}
