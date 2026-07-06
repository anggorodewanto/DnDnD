package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// tacticalMasterFeaturesJSON is a Features column carrying the Tactical Master
// feature (Fighter 9), which the override gate detects by name.
var tacticalMasterFeaturesJSON = []byte(`[{"name":"Tactical Master","mechanical_effect":"tactical_master"}]`)

// tacticalMasterMockStore mirrors pushMockStore but equips a SAP mace (a mastery
// the fighter already uses) as the base weapon, so a Tactical Master override to
// "push" is observable as a swapped mastery + a real forced move; without the
// override the weapon's own "sap" fires instead. hasFeature toggles whether the
// fighter carries the feature. It provides both a position sink (for the push
// override) and a condition sink (for the sap control).
func tacticalMasterMockStore(t *testing.T, charID, mapID uuid.UUID, hasFeature bool) (*mockStore, *[]refdata.UpdateCombatantPositionParams, *map[uuid.UUID][]CombatCondition) {
	t.Helper()
	char := makeCharacter(16, 10, 2, "mace")
	char.ID = charID
	char.CharacterData = charDataWithMasteries(`{"weapon_masteries":["mace"]}`)
	if hasFeature {
		char.Features = pqtype.NullRawMessage{RawMessage: tacticalMasterFeaturesJSON, Valid: true}
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) { return makeSapMace(), nil }
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, Conditions: json.RawMessage(`[]`)}, nil
	}
	ms.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, MapID: uuid.NullUUID{UUID: mapID, Valid: true}}, nil
	}
	ms.getMapByIDUncheckedFn = func(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
		return refdata.Map{ID: id, WidthSquares: 10, HeightSquares: 10}, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: id, Size: "Medium"}, nil
	}
	var posWrites []refdata.UpdateCombatantPositionParams
	ms.updateCombatantPositionFn = func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
		posWrites = append(posWrites, arg)
		return refdata.Combatant{ID: arg.ID, PositionCol: arg.PositionCol, PositionRow: arg.PositionRow, Conditions: json.RawMessage(`[]`)}, nil
	}
	condWrites := make(map[uuid.UUID][]CombatCondition)
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		var conds []CombatCondition
		_ = json.Unmarshal(arg.Conditions, &conds)
		condWrites[arg.ID] = conds
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}
	return ms, &posWrites, &condWrites
}

func hitRoller() *dice.Roller {
	return dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18 // hit
		}
		return 6
	})
}

// TestServiceAttack_TacticalMaster_ReplacesMasteryWithPush: a Fighter-9 with the
// feature attacks with a weapon whose own mastery is Sap, opts into
// tactical:"push", and the target is pushed (not sapped).
func TestServiceAttack_TacticalMaster_ReplacesMasteryWithPush(t *testing.T) {
	ctx := context.Background()
	charID, attackerID, targetID := uuid.New(), uuid.New(), uuid.New()
	turnID, encounterID, mapID := uuid.New(), uuid.New(), uuid.New()

	ms, posWrites, _ := tacticalMasterMockStore(t, charID, mapID, true)
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{pushAttacker(charID, attackerID, encounterID), pushTarget(targetID, encounterID, "goblin")}, nil
	}

	svc := NewService(ms)
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(ctx, AttackCommand{
		Attacker:        pushAttacker(charID, attackerID, encounterID),
		Target:          pushTarget(targetID, encounterID, "goblin"),
		Turn:            turn,
		TacticalMastery: "push",
	}, hitRoller())
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "push", result.MasteryProperty, "Tactical Master must swap sap → push")

	require.NotEmpty(t, *posWrites, "expected the pushed target to be moved")
	last := (*posWrites)[len(*posWrites)-1]
	assert.Equal(t, targetID, last.ID)
	assert.Equal(t, "D", last.PositionCol) // col 2 → pushed 2 squares → col 4
	assert.Equal(t, int32(3), last.PositionRow)
}

// TestServiceAttack_TacticalMaster_NoFeatureKeepsWeaponMastery: without the
// feature, tactical:"push" is ignored — the weapon's own Sap fires and nothing
// is pushed.
func TestServiceAttack_TacticalMaster_NoFeatureKeepsWeaponMastery(t *testing.T) {
	ctx := context.Background()
	charID, attackerID, targetID := uuid.New(), uuid.New(), uuid.New()
	turnID, encounterID, mapID := uuid.New(), uuid.New(), uuid.New()

	ms, posWrites, condWrites := tacticalMasterMockStore(t, charID, mapID, false)
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{pushAttacker(charID, attackerID, encounterID), pushTarget(targetID, encounterID, "goblin")}, nil
	}

	svc := NewService(ms)
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(ctx, AttackCommand{
		Attacker:        pushAttacker(charID, attackerID, encounterID),
		Target:          pushTarget(targetID, encounterID, "goblin"),
		Turn:            turn,
		TacticalMastery: "push",
	}, hitRoller())
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "sap", result.MasteryProperty, "no feature → weapon's own sap fires")
	assert.Empty(t, *posWrites, "no feature → target must NOT be pushed")

	conds := (*condWrites)[targetID]
	var sapped bool
	for _, c := range conds {
		if c.Condition == "sap_disadvantage" {
			sapped = true
		}
	}
	assert.True(t, sapped, "no feature → the weapon's own sap disadvantage lands")
}

// TestTacticalMasteryOverride covers the pure gate: a valid Push/Sap/Slow choice
// on a mastery-using attacker who carries the feature returns the slug;
// everything else returns "".
func TestTacticalMasteryOverride(t *testing.T) {
	known := AttackInput{Weapon: makeSapMace(), WeaponMasteries: []string{"mace"}}
	unknown := AttackInput{Weapon: makeSapMace(), WeaponMasteries: []string{"club"}}                // mastery not known
	noMastery := AttackInput{Weapon: refdata.Weapon{ID: "mace"}, WeaponMasteries: []string{"mace"}} // no Mastery set

	withFeat := pqtype.NullRawMessage{RawMessage: tacticalMasterFeaturesJSON, Valid: true}
	noFeat := pqtype.NullRawMessage{}

	tests := []struct {
		name     string
		choice   string
		input    AttackInput
		features pqtype.NullRawMessage
		want     string
	}{
		{"push valid", "push", known, withFeat, "push"},
		{"sap valid", "sap", known, withFeat, "sap"},
		{"slow valid", "slow", known, withFeat, "slow"},
		{"empty choice", "", known, withFeat, ""},
		{"non-substitutable mastery", "topple", known, withFeat, ""},
		{"cleave not allowed", "cleave", known, withFeat, ""},
		{"no feature", "push", known, noFeat, ""},
		{"mastery not known", "push", unknown, withFeat, ""},
		{"weapon has no mastery", "push", noMastery, withFeat, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tacticalMasteryOverride(tt.choice, tt.input, tt.features))
		})
	}
}
