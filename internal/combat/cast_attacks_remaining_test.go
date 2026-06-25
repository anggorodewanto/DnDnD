package combat

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// makeBonusCantrip is a minimal self-range bonus-action cantrip fixture used to
// prove the negative case: a bonus-action cast must NOT touch attacks_remaining
// (it leaves the Attack action — and its attacks — available this turn).
func makeBonusCantrip() refdata.Spell {
	return refdata.Spell{
		ID:             "bonus-cantrip",
		Name:           "Bonus Cantrip",
		Level:          0,
		CastingTime:    "1 bonus action",
		RangeType:      "self",
		Components:     []string{"V"},
		Duration:       "Instantaneous",
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: false, Valid: true},
	}
}

// Casting a spell with your ACTION is the Cast-a-Spell action, not the Attack
// action, so no weapon attack remains. The engine must zero the seeded
// attacks_remaining — otherwise /done and the resource summary report a phantom
// "1 attack" (live repro: Vale cast Hold Person, /done warned of an unused
// attack she never had).
func TestCast_ActionSpell_ZeroesAttacksRemaining(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeClericCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeDyingTarget(t, 1, 2)

	var persisted refdata.UpdateTurnActionsParams
	ms := defaultMockStore()
	ms.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeSpareTheDying(), nil }
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return target, nil
	}
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		persisted = arg
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.updateCombatantDeathSavesFn = func(_ context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, DeathSaves: arg.DeathSaves}, nil
	}

	svc := NewService(ms)
	_, err := svc.Cast(ctx, CastCommand{
		SpellID:  SpareTheDyingSpellID,
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID, AttacksRemaining: 1},
	}, testRoller())
	require.NoError(t, err)

	assert.True(t, persisted.ActionUsed, "an action spell consumes the action")
	assert.Equal(t, int32(0), persisted.AttacksRemaining, "action-cast must leave no phantom attack")
}

func TestCast_BonusActionSpell_LeavesAttacksRemaining(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeClericCharacter(charID)
	caster := makeSpellCaster(charID)

	var persisted refdata.UpdateTurnActionsParams
	ms := defaultMockStore()
	ms.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeBonusCantrip(), nil }
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) { return caster, nil }
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		persisted = arg
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	_, err := svc.Cast(ctx, CastCommand{
		SpellID:  "bonus-cantrip",
		CasterID: caster.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID, AttacksRemaining: 1},
	}, testRoller())
	require.NoError(t, err)

	assert.True(t, persisted.BonusActionUsed, "a bonus-action spell consumes the bonus action")
	assert.Equal(t, int32(1), persisted.AttacksRemaining, "a bonus-action cast leaves the Attack action (and its attacks) intact")
}
