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

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// ISSUE-014: player combat actions (spell casts, freeform actions, attacks)
// posted to #combat-log but were never persisted to action_log, so the DM
// Console timeline (GET /api/dm/situation) was blind to them. These tests pin
// the new behaviour: every player-driven combat path best-effort records an
// action_log row so it surfaces in the Console.

// captureActionLog wires the mock to record every CreateActionLog call and
// returns a pointer to the captured slice.
func captureActionLog(ms *mockStore) *[]refdata.CreateActionLogParams {
	logged := &[]refdata.CreateActionLogParams{}
	ms.createActionLogFn = func(_ context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
		*logged = append(*logged, arg)
		return refdata.ActionLog{ID: uuid.New()}, nil
	}
	return logged
}

func TestDescribeHelpers_AllBranches(t *testing.T) {
	assert.Equal(t, "Vale cast Mage Hand", describeCast("Vale", "Mage Hand", ""))
	assert.Equal(t, "Vale cast Hold Person on Ghoul", describeCast("Vale", "Hold Person", "Ghoul"))

	assert.Equal(t, "Vale cast Fireball", describeAoECast("Vale", "Fireball", nil))
	assert.Equal(t, "Vale cast Fireball on Ghoul, Kobold", describeAoECast("Vale", "Fireball", []string{"Ghoul", "Kobold"}))

	assert.Contains(t, describeAttack(AttackResult{AttackerName: "Aria", TargetName: "Goblin", WeaponName: "Longsword", Hit: true, DamageTotal: 9}), "hit for 9")
	assert.Contains(t, describeAttack(AttackResult{AttackerName: "Aria", TargetName: "Goblin", WeaponName: "Longsword", Hit: true, CriticalHit: true, DamageTotal: 18}), "CRIT for 18")
	// AutoCrit (e.g. melee vs a paralyzed target) without CriticalHit set.
	assert.Contains(t, describeAttack(AttackResult{AttackerName: "Forge", TargetName: "Wretch", WeaponName: "Handaxe", Hit: true, AutoCrit: true, DamageTotal: 18}), "CRIT for 18")
	assert.Contains(t, describeAttack(AttackResult{AttackerName: "Aria", TargetName: "Goblin", WeaponName: "Longsword"}), "missed")
	assert.Equal(t, "Aria attacked Goblin — missed", describeAttack(AttackResult{AttackerName: "Aria", TargetName: "Goblin"}))

	assert.Equal(t, uuid.NullUUID{}, nullableCombatantID(uuid.Nil))
	id := uuid.New()
	assert.Equal(t, uuid.NullUUID{UUID: id, Valid: true}, nullableCombatantID(id))
}

func TestRecordCombatAction_WritesWhenParentsPresent(t *testing.T) {
	ms := defaultMockStore()
	logged := captureActionLog(ms)
	svc := NewService(ms)

	turnID, encID, actorID := uuid.New(), uuid.New(), uuid.New()
	target := uuid.NullUUID{UUID: uuid.New(), Valid: true}
	svc.recordCombatAction(context.Background(), turnID, encID, actorID, target, "cast", "Vale cast Hold Person on Ghoul")

	require.Len(t, *logged, 1)
	got := (*logged)[0]
	assert.Equal(t, turnID, got.TurnID)
	assert.Equal(t, encID, got.EncounterID)
	assert.Equal(t, actorID, got.ActorID)
	assert.Equal(t, target, got.TargetID)
	assert.Equal(t, "cast", got.ActionType)
	require.True(t, got.Description.Valid)
	assert.Contains(t, got.Description.String, "Hold Person")
}

// Regression for the ISSUE-014 *silent* failure: action_log.before_state and
// after_state are NOT NULL, but recordCombatAction never set them, so every
// player-action insert violated the constraint — and because the write is
// best-effort (error swallowed), the row was dropped without a trace. The DM
// Console timeline went blind to every player action for days while the unit
// suite stayed green, because the mock store happily accepts nil columns the
// real Postgres rejects. Pin that a recorded action carries valid (non-nil)
// JSON state so the insert survives the NOT-NULL constraint.
func TestRecordCombatAction_PopulatesNonNullState(t *testing.T) {
	ms := defaultMockStore()
	logged := captureActionLog(ms)
	svc := NewService(ms)

	svc.recordCombatAction(context.Background(), uuid.New(), uuid.New(), uuid.New(), uuid.NullUUID{}, "cast", "Vale cast Chill Touch on Ghoul")

	require.Len(t, *logged, 1)
	got := (*logged)[0]
	require.NotEmpty(t, got.BeforeState, "before_state is NOT NULL in the DB; a nil here is silently dropped in prod")
	require.NotEmpty(t, got.AfterState, "after_state is NOT NULL in the DB; a nil here is silently dropped in prod")
	assert.True(t, json.Valid(got.BeforeState), "before_state must be valid JSON")
	assert.True(t, json.Valid(got.AfterState), "after_state must be valid JSON")
}

// A missing NOT-NULL parent (turn/encounter/actor) must skip the write rather
// than attempt an insert that would violate the constraint.
func TestRecordCombatAction_SkipsWhenParentMissing(t *testing.T) {
	ms := defaultMockStore()
	logged := captureActionLog(ms)
	svc := NewService(ms)

	good := uuid.New()
	svc.recordCombatAction(context.Background(), uuid.Nil, good, good, uuid.NullUUID{}, "cast", "x")
	svc.recordCombatAction(context.Background(), good, uuid.Nil, good, uuid.NullUUID{}, "cast", "x")
	svc.recordCombatAction(context.Background(), good, good, uuid.Nil, uuid.NullUUID{}, "cast", "x")

	assert.Empty(t, *logged, "writes with a nil parent id must be skipped")
}

// A best-effort write failure must never abort the caller.
func TestRecordCombatAction_SwallowsStoreError(t *testing.T) {
	ms := defaultMockStore()
	ms.createActionLogFn = func(_ context.Context, _ refdata.CreateActionLogParams) (refdata.ActionLog, error) {
		return refdata.ActionLog{}, assert.AnError
	}
	svc := NewService(ms)
	// Must not panic / must return cleanly despite the store error.
	svc.recordCombatAction(context.Background(), uuid.New(), uuid.New(), uuid.New(), uuid.NullUUID{}, "cast", "x")
}

func TestFreeformAction_RecordsActionLog(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()
	logged := captureActionLog(ms)

	ms.createPendingActionFn = func(_ context.Context, arg refdata.CreatePendingActionParams) (refdata.PendingAction, error) {
		return refdata.PendingAction{ID: uuid.New(), Status: "pending", ActionText: arg.ActionText}, nil
	}

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.EncounterID = encounterID
	turn.CombatantID = combatantID

	svc := NewService(ms)
	_, err := svc.FreeformAction(context.Background(), FreeformActionCommand{
		Combatant:  combatant,
		Turn:       turn,
		ActionText: "flip the table",
	})
	require.NoError(t, err)

	require.Len(t, *logged, 1, "freeform action must record exactly one action_log row")
	got := (*logged)[0]
	assert.Equal(t, "freeform_action", got.ActionType)
	assert.Equal(t, combatantID, got.ActorID)
	assert.Equal(t, turn.ID, got.TurnID)
	assert.Equal(t, encounterID, got.EncounterID)
	require.True(t, got.Description.Valid)
	assert.Contains(t, got.Description.String, "Thorn")
	assert.Contains(t, got.Description.String, "flip the table")
}

func TestCast_RecordsActionLog(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeClericCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeDyingTarget(t, 1, 2)

	ms := defaultMockStore()
	logged := captureActionLog(ms)
	ms.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeSpareTheDying(), nil }
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return target, nil
	}
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed, ActionSpellCast: arg.ActionSpellCast}, nil
	}
	ms.updateCombatantDeathSavesFn = func(_ context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, DeathSaves: arg.DeathSaves}, nil
	}

	svc := NewService(ms)
	turnID := uuid.New()
	encID := uuid.New()
	_, err := svc.Cast(ctx, CastCommand{
		SpellID:     SpareTheDyingSpellID,
		CasterID:    caster.ID,
		TargetID:    target.ID,
		EncounterID: encID,
		Turn:        refdata.Turn{ID: turnID, EncounterID: encID, CombatantID: caster.ID},
	}, testRoller())
	require.NoError(t, err)

	require.Len(t, *logged, 1, "a resolved cast must record exactly one action_log row")
	got := (*logged)[0]
	assert.Equal(t, "cast", got.ActionType)
	assert.Equal(t, caster.ID, got.ActorID)
	assert.Equal(t, turnID, got.TurnID)
	require.True(t, got.Description.Valid)
	assert.Contains(t, got.Description.String, "Spare the Dying")
}

func TestAttack_RecordsActionLog(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	classes := []CharacterClass{{Class: "Fighter", Level: 1}}
	char := makeCharacterWithFeats(16, 10, 2, "longsword", nil, classes)
	char.ID = charID
	inv, err := json.Marshal([]character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Type: "weapon", Equipped: true},
	})
	require.NoError(t, err)
	char.Inventory = pqtype.NullRawMessage{RawMessage: inv, Valid: true}

	ms := defaultMockStore()
	logged := captureActionLog(ms)
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) { return makeLongsword(), nil }
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	svc := NewService(ms)

	roller := dice.NewRoller(func(maxN int) int {
		if maxN == 20 {
			return 10
		}
		return 5
	})

	attacker := refdata.Combatant{
		ID: attackerID, EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Aria", PositionCol: "A", PositionRow: 1,
		IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	tgt := refdata.Combatant{
		ID: targetID, EncounterID: encounterID,
		DisplayName: "Goblin", PositionCol: "B", PositionRow: 1, Ac: 10,
		IsAlive: true, IsNpc: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}

	_, err = svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: tgt, Turn: turn}, roller)
	require.NoError(t, err)

	require.Len(t, *logged, 1, "an attack must record exactly one action_log row")
	got := (*logged)[0]
	assert.Equal(t, "attack", got.ActionType)
	assert.Equal(t, attackerID, got.ActorID)
	assert.Equal(t, uuid.NullUUID{UUID: targetID, Valid: true}, got.TargetID)
	require.True(t, got.Description.Valid)
	assert.Contains(t, got.Description.String, "Aria")
	assert.Contains(t, got.Description.String, "Goblin")
}

func TestOffhandAttack_RecordsActionLog(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 14, 2, "shortsword")
	char.ID = charID
	char.EquippedOffHand = sql.NullString{String: "dagger", Valid: true}

	ms := defaultMockStore()
	logged := captureActionLog(ms)
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, id string) (refdata.Weapon, error) {
		switch id {
		case "shortsword":
			return makeShortsword(), nil
		case "dagger":
			return makeDagger(), nil
		}
		return refdata.Weapon{}, sql.ErrNoRows
	}
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(maxN int) int {
		if maxN == 20 {
			return 15
		}
		return 3
	})

	_, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, EncounterID: encounterID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, EncounterID: encounterID, DisplayName: "Goblin", PositionCol: "B", PositionRow: 1, Ac: 13, IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 0},
	}, roller)
	require.NoError(t, err)

	require.Len(t, *logged, 1, "an off-hand attack must record exactly one action_log row")
	got := (*logged)[0]
	assert.Equal(t, "attack", got.ActionType)
	assert.Equal(t, attackerID, got.ActorID)
	assert.Equal(t, turnID, got.TurnID)
	assert.Equal(t, uuid.NullUUID{UUID: targetID, Valid: true}, got.TargetID)
}
