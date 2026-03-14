package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// makeTestCombatant creates a minimal combatant at the given position.
func makeTestCombatant(name, col string, row int32) refdata.Combatant {
	return refdata.Combatant{
		ID:          uuid.New(),
		EncounterID: uuid.New(),
		ShortID:     name[0:1],
		DisplayName: name,
		PositionCol: col,
		PositionRow: row,
		HpMax:       20,
		HpCurrent:   20,
		Ac:          15,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
}

// TestServiceAttack_ObscurementFromZones_AttackerInDarkness verifies that when
// the attacker is inside a darkness zone without darkvision, the attack gets
// disadvantage from heavy obscurement.
func TestServiceAttack_ObscurementFromZones_AttackerInDarkness(t *testing.T) {
	encounterID := uuid.New()
	attacker := makeTestCombatant("Thorn", "C", 3) // In the darkness zone
	attacker.EncounterID = encounterID
	target := makeTestCombatant("Goblin", "D", 3) // Adjacent, outside the zone
	target.EncounterID = encounterID

	charID := uuid.New()
	attacker.CharacterID = uuid.NullUUID{UUID: charID, Valid: true}

	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			AbilityScores:    json.RawMessage(`{"str":16,"dex":12,"con":14,"int":10,"wis":12,"cha":8}`),
			ProficiencyBonus: 2,
			EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
			Classes:          json.RawMessage(`[{"class":"fighter","level":5}]`),
		}, nil
	}
	store.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return refdata.Weapon{
			ID:         "longsword",
			Name:       "Longsword",
			Damage:     "1d8",
			DamageType: "slashing",
			WeaponType: "martial_melee",
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID}, nil
	}
	// Darkness zone covering only the attacker's position (C3)
	store.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		return []refdata.EncounterZone{
			{
				ID:          uuid.New(),
				EncounterID: encounterID,
				ZoneType:    "darkness",
				Shape:       "square",
				OriginCol:   "C",
				OriginRow:   3,
				Dimensions:  json.RawMessage(`{"side_ft":5}`),
				AnchorMode:  "static",
			},
		}, nil
	}

	svc := NewService(store)
	roller := dice.NewRoller(func(n int) int { return n - 1 }) // always max

	turn := refdata.Turn{
		ID:                uuid.New(),
		EncounterID:       encounterID,
		Status:            "active",
		AttacksRemaining:  1,
		BonusActionUsed:   false,
	}

	result, err := svc.Attack(context.Background(), AttackCommand{
		Attacker:       attacker,
		Target:         target,
		Turn:           turn,
		AttackerSize:   "Medium",
	}, roller)

	require.NoError(t, err)
	assert.Equal(t, dice.Disadvantage, result.RollMode)
	assert.Contains(t, result.DisadvantageReasons, "heavily obscured (blinded)")
}

// TestServiceAttack_ObscurementFromZones_TargetInDarkness verifies that when
// the target is in a darkness zone, the attacker gets advantage.
func TestServiceAttack_ObscurementFromZones_TargetInDarkness(t *testing.T) {
	encounterID := uuid.New()
	attacker := makeTestCombatant("Aria", "C", 4) // Adjacent but outside 5ft zone at C3
	attacker.EncounterID = encounterID
	target := makeTestCombatant("Goblin", "C", 3) // In the darkness zone
	target.EncounterID = encounterID

	charID := uuid.New()
	attacker.CharacterID = uuid.NullUUID{UUID: charID, Valid: true}

	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			AbilityScores:    json.RawMessage(`{"str":16,"dex":12,"con":14,"int":10,"wis":12,"cha":8}`),
			ProficiencyBonus: 2,
			EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
			Classes:          json.RawMessage(`[{"class":"fighter","level":5}]`),
		}, nil
	}
	store.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return refdata.Weapon{
			ID:         "longsword",
			Name:       "Longsword",
			Damage:     "1d8",
			DamageType: "slashing",
			WeaponType: "martial_melee",
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID}, nil
	}
	// Darkness zone covering only the target's position (C3), not the attacker (C4)
	store.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		return []refdata.EncounterZone{
			{
				ID:          uuid.New(),
				EncounterID: encounterID,
				ZoneType:    "darkness",
				Shape:       "square",
				OriginCol:   "C",
				OriginRow:   3,
				Dimensions:  json.RawMessage(`{"side_ft":5}`), // Single tile at C3
				AnchorMode:  "static",
			},
		}, nil
	}

	svc := NewService(store)
	roller := dice.NewRoller(func(n int) int { return n - 1 })

	turn := refdata.Turn{
		ID:                uuid.New(),
		EncounterID:       encounterID,
		Status:            "active",
		AttacksRemaining:  1,
		BonusActionUsed:   false,
	}

	result, err := svc.Attack(context.Background(), AttackCommand{
		Attacker:     attacker,
		Target:       target,
		Turn:         turn,
		AttackerSize: "Medium",
	}, roller)

	require.NoError(t, err)
	assert.Equal(t, dice.Advantage, result.RollMode)
	assert.Contains(t, result.AdvantageReasons, "target heavily obscured (blinded)")
}

// TestServiceAttack_ObscurementFromZones_DarkvisionNegates verifies that a
// combatant with darkvision in a dim_light zone gets no attack penalty.
func TestServiceAttack_ObscurementFromZones_DarkvisionNegates(t *testing.T) {
	encounterID := uuid.New()
	attacker := makeTestCombatant("Aria", "C", 3)
	attacker.EncounterID = encounterID
	target := makeTestCombatant("Goblin", "C", 4) // Adjacent
	target.EncounterID = encounterID

	charID := uuid.New()
	attacker.CharacterID = uuid.NullUUID{UUID: charID, Valid: true}

	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			AbilityScores:    json.RawMessage(`{"str":16,"dex":12,"con":14,"int":10,"wis":12,"cha":8}`),
			ProficiencyBonus: 2,
			EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
			Classes:          json.RawMessage(`[{"class":"fighter","level":5}]`),
		}, nil
	}
	store.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return refdata.Weapon{
			ID:         "longsword",
			Name:       "Longsword",
			Damage:     "1d8",
			DamageType: "slashing",
			WeaponType: "martial_melee",
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID}, nil
	}
	// Dim light zone covering both positions
	store.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		return []refdata.EncounterZone{
			{
				ID:          uuid.New(),
				EncounterID: encounterID,
				ZoneType:    "dim_light",
				Shape:       "circle",
				OriginCol:   "C",
				OriginRow:   3,
				Dimensions:  json.RawMessage(`{"radius_ft":15}`),
				AnchorMode:  "static",
			},
		}, nil
	}

	svc := NewService(store)
	roller := dice.NewRoller(func(n int) int { return n - 1 })

	turn := refdata.Turn{
		ID:                uuid.New(),
		EncounterID:       encounterID,
		Status:            "active",
		AttacksRemaining:  1,
		BonusActionUsed:   false,
	}

	result, err := svc.Attack(context.Background(), AttackCommand{
		Attacker:       attacker,
		Target:         target,
		Turn:           turn,
		AttackerSize:   "Medium",
		AttackerVision: VisionCapabilities{DarkvisionFt: 60},
		TargetVision:   VisionCapabilities{DarkvisionFt: 60},
	}, roller)

	require.NoError(t, err)
	// Darkvision negates dim light for both — no attack modifier
	assert.Equal(t, dice.Normal, result.RollMode)
}

// TestServiceAttack_ObscurementFromZones_MagicalDarknessIgnoresDarkvision verifies
// that magical darkness is NOT negated by darkvision.
func TestServiceAttack_ObscurementFromZones_MagicalDarknessIgnoresDarkvision(t *testing.T) {
	encounterID := uuid.New()
	attacker := makeTestCombatant("Thorn", "C", 3)
	attacker.EncounterID = encounterID
	target := makeTestCombatant("Goblin", "C", 4)
	target.EncounterID = encounterID

	charID := uuid.New()
	attacker.CharacterID = uuid.NullUUID{UUID: charID, Valid: true}

	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			AbilityScores:    json.RawMessage(`{"str":16,"dex":12,"con":14,"int":10,"wis":12,"cha":8}`),
			ProficiencyBonus: 2,
			EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
			Classes:          json.RawMessage(`[{"class":"fighter","level":5}]`),
		}, nil
	}
	store.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return refdata.Weapon{
			ID:         "longsword",
			Name:       "Longsword",
			Damage:     "1d8",
			DamageType: "slashing",
			WeaponType: "martial_melee",
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID}, nil
	}
	// Magical darkness covering attacker position
	store.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		return []refdata.EncounterZone{
			{
				ID:          uuid.New(),
				EncounterID: encounterID,
				ZoneType:    "magical_darkness",
				Shape:       "circle",
				OriginCol:   "C",
				OriginRow:   3,
				Dimensions:  json.RawMessage(`{"radius_ft":15}`),
				AnchorMode:  "static",
			},
		}, nil
	}

	svc := NewService(store)
	roller := dice.NewRoller(func(n int) int { return n - 1 })

	turn := refdata.Turn{
		ID:                uuid.New(),
		EncounterID:       encounterID,
		Status:            "active",
		AttacksRemaining:  1,
		BonusActionUsed:   false,
	}

	result, err := svc.Attack(context.Background(), AttackCommand{
		Attacker:       attacker,
		Target:         target,
		Turn:           turn,
		AttackerSize:   "Medium",
		AttackerVision: VisionCapabilities{DarkvisionFt: 60},
		TargetVision:   VisionCapabilities{DarkvisionFt: 60},
	}, roller)

	require.NoError(t, err)
	// Both in magical darkness with darkvision — darkvision doesn't help.
	// Both heavily obscured: attacker gets disadvantage, target grants advantage → cancel
	assert.Equal(t, dice.AdvantageAndDisadvantage, result.RollMode)
	assert.Contains(t, result.AdvantageReasons, "target heavily obscured (blinded)")
	assert.Contains(t, result.DisadvantageReasons, "heavily obscured (blinded)")
}

// TestServiceAttack_ObscurementFromZones_NoZones verifies that with no zones,
// attacks proceed normally.
func TestServiceAttack_ObscurementFromZones_NoZones(t *testing.T) {
	encounterID := uuid.New()
	attacker := makeTestCombatant("Thorn", "C", 3)
	attacker.EncounterID = encounterID
	target := makeTestCombatant("Goblin", "C", 4)
	target.EncounterID = encounterID

	charID := uuid.New()
	attacker.CharacterID = uuid.NullUUID{UUID: charID, Valid: true}

	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			AbilityScores:    json.RawMessage(`{"str":16,"dex":12,"con":14,"int":10,"wis":12,"cha":8}`),
			ProficiencyBonus: 2,
			EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
			Classes:          json.RawMessage(`[{"class":"fighter","level":5}]`),
		}, nil
	}
	store.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return refdata.Weapon{
			ID:         "longsword",
			Name:       "Longsword",
			Damage:     "1d8",
			DamageType: "slashing",
			WeaponType: "martial_melee",
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID}, nil
	}
	// No zones
	store.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		return []refdata.EncounterZone{}, nil
	}

	svc := NewService(store)
	roller := dice.NewRoller(func(n int) int { return n - 1 })

	turn := refdata.Turn{
		ID:                uuid.New(),
		EncounterID:       encounterID,
		Status:            "active",
		AttacksRemaining:  1,
		BonusActionUsed:   false,
	}

	result, err := svc.Attack(context.Background(), AttackCommand{
		Attacker:     attacker,
		Target:       target,
		Turn:         turn,
		AttackerSize: "Medium",
	}, roller)

	require.NoError(t, err)
	assert.Equal(t, dice.Normal, result.RollMode)
}
