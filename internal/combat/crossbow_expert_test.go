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

func TestIsHandCrossbow(t *testing.T) {
	assert.True(t, IsHandCrossbow(makeHandCrossbow()), "the hand crossbow qualifies")
	assert.False(t, IsHandCrossbow(makeHeavyCrossbowFull()), "a heavy crossbow is two-handed, not a hand crossbow")
	assert.False(t, IsHandCrossbow(makeShortsword()), "a shortsword is not a hand crossbow")
	assert.False(t, IsHandCrossbow(makeLongbow()), "a longbow is not a hand crossbow")
}

// makeHandCrossbow builds the seeded hand crossbow: a martial ranged weapon
// (1d6 piercing) with the ammunition/light/loading properties and a bolt FK.
func makeHandCrossbow() refdata.Weapon {
	return refdata.Weapon{
		ID:            "hand-crossbow",
		Name:          "Hand Crossbow",
		Damage:        "1d6",
		DamageType:    "piercing",
		WeaponType:    "martial_ranged",
		Properties:    []string{"ammunition", "light", "loading"},
		RangeNormalFt: sql.NullInt32{Int32: 30, Valid: true},
		RangeLongFt:   sql.NullInt32{Int32: 120, Valid: true},
		AmmunitionID:  sql.NullString{String: "crossbow-bolt", Valid: true},
	}
}

// crossbowExpertChar builds a DEX-16 (+3) fighter with the Crossbow Expert feat,
// a hand crossbow in the main hand, and a stack of bolts in inventory.
func crossbowExpertChar() refdata.Character {
	feats := []CharacterFeature{{Name: "Crossbow Expert", MechanicalEffect: `[{"effect_type":"bonus_action_hand_crossbow"},{"effect_type":"ignore_loading_crossbow"},{"effect_type":"no_disadvantage_ranged_5ft"}]`}}
	classes := []CharacterClass{{Class: "Fighter", Level: 5}}
	char := makeCharacterWithFeats(10, 16, 3, "hand-crossbow", feats, classes)
	inv, _ := json.Marshal([]character.InventoryItem{{ItemID: "crossbow-bolt", Name: "Bolts", Quantity: 20, Type: "ammunition"}})
	char.Inventory = nullRawMessage(inv)
	return char
}

func crossbowExpertMockStore(char refdata.Character) *mockStore {
	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, id string) (refdata.Weapon, error) {
		switch id {
		case "hand-crossbow":
			return makeHandCrossbow(), nil
		case "shortsword":
			return makeShortsword(), nil
		}
		return refdata.Weapon{}, sql.ErrNoRows
	}
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining, BonusActionUsed: arg.BonusActionUsed}, nil
	}
	return ms
}

// crossbowExpertCombatants mirrors gwmCombatants: an attacker+target 5 ft apart
// with the attacker's Attack action already spent (AttacksRemaining 0), so the
// bonus attack's "you must have attacked this turn" gate passes.
func crossbowExpertCombatants(charID uuid.UUID) (refdata.Combatant, refdata.Combatant, refdata.Turn) {
	encounterID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	attacker := refdata.Combatant{
		ID: attackerID, EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Vex", PositionCol: "A", PositionRow: 1,
		IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID: targetID, EncounterID: encounterID,
		DisplayName: "Grix", PositionCol: "B", PositionRow: 1, Ac: 13,
		IsAlive: true, IsNpc: true, IsVisible: true, HpCurrent: 30, HpMax: 30,
		Conditions: json.RawMessage(`[]`),
	}
	turn := refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 0}
	return attacker, target, turn
}

// d20=15 hits AC 13; d6=3. DEX 16 (+3). Damage = 1d6(3)+DEX(3) = 6.
func crossbowExpertRoller() *dice.Roller {
	return dice.NewRoller(func(maxN int) int {
		switch maxN {
		case 20:
			return 15
		case 6:
			return 3
		default:
			return 3
		}
	})
}

func TestServiceCrossbowExpertBonusAttack_HappyPath(t *testing.T) {
	ctx := context.Background()
	char := crossbowExpertChar()
	char.ID = uuid.New()
	ms := crossbowExpertMockStore(char)

	var persistedInv json.RawMessage
	ms.updateCharacterInventoryFn = func(_ context.Context, _ uuid.UUID, inv pqtype.NullRawMessage) error {
		persistedInv = inv.RawMessage
		return nil
	}

	svc := NewService(ms)
	attacker, target, turn := crossbowExpertCombatants(char.ID)

	result, err := svc.CrossbowExpertBonusAttack(ctx, CrossbowExpertBonusAttackCommand{
		Attacker: attacker, Target: target, Turn: turn,
	}, crossbowExpertRoller())
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, "Hand Crossbow", result.WeaponName)
	assert.Equal(t, 6, result.DamageTotal, "1d6(3)+DEX(3) = 6")
	assert.Equal(t, "piercing", result.DamageType)

	// A bolt must be consumed: the persisted inventory drops from 20 to 19.
	require.NotNil(t, persistedInv, "inventory must be persisted after firing a bolt")
	var items []character.InventoryItem
	require.NoError(t, json.Unmarshal(persistedInv, &items))
	require.Len(t, items, 1)
	assert.Equal(t, 19, items[0].Quantity, "one bolt spent")
}

// The bolt spent on the bonus shot must be tracked for post-combat half-recovery,
// exactly like a bolt fired on the main /attack (the deduction and recovery
// tracking are coupled in deductWeaponAmmunition).
func TestServiceCrossbowExpertBonusAttack_TracksAmmoForRecovery(t *testing.T) {
	ctx := context.Background()
	char := crossbowExpertChar()
	char.ID = uuid.New()
	svc := NewService(crossbowExpertMockStore(char))
	attacker, target, turn := crossbowExpertCombatants(char.ID)

	_, err := svc.CrossbowExpertBonusAttack(ctx, CrossbowExpertBonusAttackCommand{
		Attacker: attacker, Target: target, Turn: turn,
	}, crossbowExpertRoller())
	require.NoError(t, err)

	snap := svc.ammoTracker.Snapshot(attacker.EncounterID)
	total := 0
	for _, byAmmo := range snap[attacker.ID] {
		total += byAmmo
	}
	assert.Equal(t, 1, total, "one bolt spent on the CE bonus shot must be tracked for recovery")
}

// A hand crossbow held in the OFF hand (main hand a shortsword) still qualifies —
// Crossbow Expert fires "a hand crossbow you are holding", either hand.
func TestServiceCrossbowExpertBonusAttack_OffHandCrossbow(t *testing.T) {
	ctx := context.Background()
	char := crossbowExpertChar()
	char.ID = uuid.New()
	char.EquippedMainHand = sql.NullString{String: "shortsword", Valid: true}
	char.EquippedOffHand = sql.NullString{String: "hand-crossbow", Valid: true}
	ms := crossbowExpertMockStore(char)

	svc := NewService(ms)
	attacker, target, turn := crossbowExpertCombatants(char.ID)

	result, err := svc.CrossbowExpertBonusAttack(ctx, CrossbowExpertBonusAttackCommand{
		Attacker: attacker, Target: target, Turn: turn,
	}, crossbowExpertRoller())
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, "Hand Crossbow", result.WeaponName, "the off-hand hand crossbow is used")
}

func TestServiceCrossbowExpertBonusAttack_RequiresFeat(t *testing.T) {
	ctx := context.Background()
	classes := []CharacterClass{{Class: "Fighter", Level: 5}}
	char := makeCharacterWithFeats(10, 16, 3, "hand-crossbow", nil, classes)
	inv, _ := json.Marshal([]character.InventoryItem{{ItemID: "crossbow-bolt", Name: "Bolts", Quantity: 20, Type: "ammunition"}})
	char.Inventory = nullRawMessage(inv)
	char.ID = uuid.New()
	svc := NewService(crossbowExpertMockStore(char))
	attacker, target, turn := crossbowExpertCombatants(char.ID)

	_, err := svc.CrossbowExpertBonusAttack(ctx, CrossbowExpertBonusAttackCommand{
		Attacker: attacker, Target: target, Turn: turn,
	}, crossbowExpertRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "feat")
}

func TestServiceCrossbowExpertBonusAttack_RequiresAttackFirst(t *testing.T) {
	ctx := context.Background()
	char := crossbowExpertChar()
	char.ID = uuid.New()
	svc := NewService(crossbowExpertMockStore(char))
	attacker, target, turn := crossbowExpertCombatants(char.ID)
	turn.AttacksRemaining = 1 // no attack made yet this turn (max is 1)

	_, err := svc.CrossbowExpertBonusAttack(ctx, CrossbowExpertBonusAttackCommand{
		Attacker: attacker, Target: target, Turn: turn,
	}, crossbowExpertRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "attack")
}

func TestServiceCrossbowExpertBonusAttack_RequiresHandCrossbow(t *testing.T) {
	ctx := context.Background()
	char := crossbowExpertChar()
	char.EquippedMainHand = sql.NullString{String: "shortsword", Valid: true} // no hand crossbow in either hand
	char.ID = uuid.New()
	svc := NewService(crossbowExpertMockStore(char))
	attacker, target, turn := crossbowExpertCombatants(char.ID)

	_, err := svc.CrossbowExpertBonusAttack(ctx, CrossbowExpertBonusAttackCommand{
		Attacker: attacker, Target: target, Turn: turn,
	}, crossbowExpertRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hand crossbow")
}

func TestServiceCrossbowExpertBonusAttack_NotACharacter(t *testing.T) {
	ctx := context.Background()
	char := crossbowExpertChar()
	char.ID = uuid.New()
	svc := NewService(crossbowExpertMockStore(char))
	attacker, target, turn := crossbowExpertCombatants(char.ID)
	attacker.CharacterID = uuid.NullUUID{} // NPC

	_, err := svc.CrossbowExpertBonusAttack(ctx, CrossbowExpertBonusAttackCommand{
		Attacker: attacker, Target: target, Turn: turn,
	}, crossbowExpertRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "character")
}

func TestServiceCrossbowExpertBonusAttack_BonusActionAlreadyUsed(t *testing.T) {
	ctx := context.Background()
	char := crossbowExpertChar()
	char.ID = uuid.New()
	svc := NewService(crossbowExpertMockStore(char))
	attacker, target, turn := crossbowExpertCombatants(char.ID)
	turn.BonusActionUsed = true

	_, err := svc.CrossbowExpertBonusAttack(ctx, CrossbowExpertBonusAttackCommand{
		Attacker: attacker, Target: target, Turn: turn,
	}, crossbowExpertRoller())
	require.Error(t, err)
}

// Firing with an empty quiver returns NoAmmunitionError and does NOT spend the
// bonus action (the ammo check precedes the resource spend).
func TestServiceCrossbowExpertBonusAttack_OutOfAmmo(t *testing.T) {
	ctx := context.Background()
	char := crossbowExpertChar()
	inv, _ := json.Marshal([]character.InventoryItem{{ItemID: "crossbow-bolt", Name: "Bolts", Quantity: 0, Type: "ammunition"}})
	char.Inventory = nullRawMessage(inv)
	char.ID = uuid.New()

	bonusSpent := false
	ms := crossbowExpertMockStore(char)
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		if arg.BonusActionUsed {
			bonusSpent = true
		}
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}

	svc := NewService(ms)
	attacker, target, turn := crossbowExpertCombatants(char.ID)

	_, err := svc.CrossbowExpertBonusAttack(ctx, CrossbowExpertBonusAttackCommand{
		Attacker: attacker, Target: target, Turn: turn,
	}, crossbowExpertRoller())
	require.Error(t, err)
	var noAmmo NoAmmunitionError
	assert.ErrorAs(t, err, &noAmmo, "out of bolts must surface NoAmmunitionError")
	assert.False(t, bonusSpent, "an out-of-ammo attempt must not spend the bonus action")
}
