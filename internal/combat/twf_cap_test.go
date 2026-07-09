package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// ISSUE-062 — once-per-turn cap on the Light-property (off-hand) extra attack.
//
// RAW: one Attack-action attack + one Light extra = 2 swings max. Nick only
// FREES the bonus action; it does not grant a 3rd swing. A second off-hand
// attack requires the Dual Wielder feat (which, with Nick, is the legit path to
// 3 total swings). These tests pin that cap.

// twfCapMockStore wires a mock store for a shortsword-main / dagger-off-hand
// character, threading BonusActionUsed through updateTurnActions and counting
// applied-damage HP writes so a rejected swing (no resolve) is observable.
func twfCapMockStore(char refdata.Character) (*mockStore, *int) {
	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		switch id {
		case "shortsword":
			return makeShortsword(), nil
		case "dagger":
			return makeNickDagger(), nil
		}
		return refdata.Weapon{}, sql.ErrNoRows
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	hpWrites := 0
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpWrites++
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, Conditions: json.RawMessage(`[]`)}, nil
	}
	return ms, &hpWrites
}

// dualWielderChar is a nickChar (shortsword main / Nick dagger off-hand) that
// also carries a stored Dual Wielder feat feature — the exact shape ApplyFeat
// persists (Feature{Name: "Dual Wielder", Source: "feat"}).
func dualWielderChar(charID uuid.UUID) refdata.Character {
	char := nickChar(charID, "shortsword", "dagger", `{"weapon_masteries":["dagger"]}`)
	char.Features = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`[{"name":"Dual Wielder","source":"feat"}]`),
		Valid:      true,
	}
	return char
}

// A (core, RED today): Nick off-hand, no Dual Wielder. First off-hand swing is
// the (free) Light extra; a SECOND off-hand swing the same turn is rejected
// because only Dual Wielder grants it. Nick freeing the bonus action must NOT
// buy a 3rd attack.
func TestOffhandAttack_SecondExtraRejectedWithoutDualWielder(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := nickChar(charID, "shortsword", "dagger", `{"weapon_masteries":["dagger"]}`)
	ms, hpWrites := twfCapMockStore(char)

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15 // hit vs AC 13
		}
		return 3
	})

	mkTurn := func(bonusUsed bool) refdata.Turn {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 0, BonusActionUsed: bonusUsed}
	}

	// First off-hand swing (Nick free) succeeds.
	r1, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: nickAttacker(charID, attackerID, encounterID),
		Target:   nickTarget(targetID, encounterID),
		Turn:     mkTurn(false),
	}, roller)
	require.NoError(t, err)
	require.NotNil(t, r1.RemainingTurn)
	assert.False(t, r1.RemainingTurn.BonusActionUsed, "first Nick off-hand is free")

	// Second off-hand swing same turn (bonus still available) is REJECTED —
	// only Dual Wielder grants a second Light extra.
	_, err = svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: nickAttacker(charID, attackerID, encounterID),
		Target:   nickTarget(targetID, encounterID),
		Turn:     mkTurn(r1.RemainingTurn.BonusActionUsed),
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Dual Wielder")
	assert.Equal(t, 1, *hpWrites, "only the first off-hand swing may resolve")
}

// B: Dual Wielder + Nick → 3 total swings. Nick frees the bonus action on the
// first off-hand swing; the freed bonus action pays for the Dual Wielder extra
// (second off-hand swing). A THIRD off-hand swing is capped out.
func TestOffhandAttack_SecondExtraAllowedWithDualWielder(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := dualWielderChar(charID)
	ms, _ := twfCapMockStore(char)

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 3
	})

	mkTurn := func(bonusUsed bool) refdata.Turn {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 0, BonusActionUsed: bonusUsed}
	}

	// 1st off-hand: Nick free, bonus preserved.
	r1, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: nickAttacker(charID, attackerID, encounterID),
		Target:   nickTarget(targetID, encounterID),
		Turn:     mkTurn(false),
	}, roller)
	require.NoError(t, err)
	require.NotNil(t, r1.RemainingTurn)
	assert.False(t, r1.RemainingTurn.BonusActionUsed, "first Nick off-hand is free")

	// 2nd off-hand: allowed by Dual Wielder, spends the (freed) bonus action.
	r2, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: nickAttacker(charID, attackerID, encounterID),
		Target:   nickTarget(targetID, encounterID),
		Turn:     mkTurn(r1.RemainingTurn.BonusActionUsed),
	}, roller)
	require.NoError(t, err)
	require.NotNil(t, r2.RemainingTurn)
	assert.True(t, r2.RemainingTurn.BonusActionUsed, "Dual Wielder 2nd off-hand costs the bonus action")

	// 3rd off-hand: even with a (contrived) available bonus action, the Dual
	// Wielder extra is once/turn — no 4th swing.
	_, err = svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: nickAttacker(charID, attackerID, encounterID),
		Target:   nickTarget(targetID, encounterID),
		Turn:     mkTurn(false),
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no off-hand attacks remain")
}

// Regression: plain two-weapon fighting (no Nick, no Dual Wielder) still yields
// exactly one off-hand swing. The first spends the bonus action; the second
// fails on the pre-existing ResourceBonusAction gate — the cap changes nothing.
func TestOffhandAttack_PlainTWFStillOneOffhand(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	// Off-hand shortsword: light, no Nick mastery.
	char := nickChar(charID, "shortsword", "shortsword", "")
	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeShortsword(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 3
	})

	mkTurn := func(bonusUsed bool) refdata.Turn {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 0, BonusActionUsed: bonusUsed}
	}

	r1, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: nickAttacker(charID, attackerID, encounterID),
		Target:   nickTarget(targetID, encounterID),
		Turn:     mkTurn(false),
	}, roller)
	require.NoError(t, err)
	require.NotNil(t, r1.RemainingTurn)
	assert.True(t, r1.RemainingTurn.BonusActionUsed, "plain TWF off-hand consumes the bonus action")

	// Second off-hand this turn → rejected by the once-per-turn off-hand cap
	// (ISSUE-062): a plain TWF character gets exactly one off-hand swing, and only
	// the Dual Wielder feat grants a second. The bonus action is also spent, but
	// the cap is the primary, more specific rejection now that Nick's free swing
	// no longer sits behind an up-front bonus-action gate.
	_, err = svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: nickAttacker(charID, attackerID, encounterID),
		Target:   nickTarget(targetID, encounterID),
		Turn:     mkTurn(r1.RemainingTurn.BonusActionUsed),
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already made your off-hand")
}
