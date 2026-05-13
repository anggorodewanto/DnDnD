package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// fixedRoller returns a roller whose d20 always rolls the given value.
func fixedRoller(v int) *dice.Roller {
	return dice.NewRoller(func(_ int) int { return v })
}

// ---------------------------------------------------------------------------
// D-46-rage-spellcasting-block
// A raging caster cannot cast a spell. The cast service must reject before
// burning resources.
// ---------------------------------------------------------------------------

func TestService_Cast_BlockedWhileRaging(t *testing.T) {
	charID := uuid.New()
	combatantID := uuid.New()
	encID := uuid.New()

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, id string) (refdata.Spell, error) {
		return refdata.Spell{
			ID:          "fire-bolt",
			Name:        "Fire Bolt",
			Level:       0,
			CastingTime: "1 action",
		}, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          combatantID,
			DisplayName: "Kael",
			IsRaging:    true,
			EncounterID: encID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}

	svc := NewService(store)
	_, err := svc.Cast(context.Background(), CastCommand{
		SpellID:  "fire-bolt",
		CasterID: combatantID,
		Turn:     refdata.Turn{ID: uuid.New()},
	}, nil)

	if err == nil {
		t.Fatal("expected Cast to error while raging")
	}
	if want := "rag"; !containsLower(err.Error(), want) {
		t.Errorf("error %q should mention rage", err.Error())
	}
}

// ---------------------------------------------------------------------------
// D-46-rage-spellcasting-block — concentration drop on rage activation
// Starting rage on a concentrating combatant drops the concentration spell
// via BreakConcentrationFully (clear columns, dismiss summons, etc.).
// ---------------------------------------------------------------------------

func TestService_ActivateRage_DropsConcentration(t *testing.T) {
	charID := uuid.New()
	combatantID := uuid.New()
	encID := uuid.New()

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:      charID,
			Classes: json.RawMessage(`[{"class":"Barbarian","level":5}]`),
			FeatureUses: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`{"rage":{"current":3,"max":3,"recharge":"long"}}`),
				Valid:      true,
			},
		}, nil
	}
	store.updateCombatantRageFn = func(_ context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:                  arg.ID,
			DisplayName:         "Kael",
			IsRaging:            arg.IsRaging,
			RageRoundsRemaining: arg.RageRoundsRemaining,
			EncounterID:         encID,
			Conditions:          json.RawMessage(`[]`),
		}, nil
	}
	store.updateCharacterFeatureUsesFn = func(_ context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: true}, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, DisplayName: "Kael", EncounterID: encID, Conditions: json.RawMessage(`[]`)}, nil
	}
	// Combatant is concentrating on Bless.
	store.getCombatantConcentrationFn = func(_ context.Context, _ uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
		return refdata.GetCombatantConcentrationRow{
			ConcentrationSpellID:   sql.NullString{String: "bless", Valid: true},
			ConcentrationSpellName: sql.NullString{String: "Bless", Valid: true},
		}, nil
	}
	cleared := false
	store.clearCombatantConcentrationFn = func(_ context.Context, _ uuid.UUID) error {
		cleared = true
		return nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return nil, nil
	}
	store.deleteConcentrationZonesByCombatantFn = func(_ context.Context, _ uuid.UUID) (int64, error) {
		return 0, nil
	}

	svc := NewService(store)
	_, err := svc.ActivateRage(context.Background(), RageCommand{
		Combatant: refdata.Combatant{
			ID:          combatantID,
			DisplayName: "Kael",
			EncounterID: encID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		},
		Turn: refdata.Turn{ID: uuid.New()},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cleared {
		t.Error("expected concentration to be cleared on rage activation")
	}
}

// ---------------------------------------------------------------------------
// D-46-rage-auto-end-quiet — Attack() must mark RageAttackedThisRound true
// on the attacker when they are raging.
// ---------------------------------------------------------------------------

func TestService_Attack_SetsRageAttackedThisRound(t *testing.T) {
	charID := uuid.New()
	combatantID := uuid.New()
	encID := uuid.New()
	targetID := uuid.New()

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			Classes:          json.RawMessage(`[{"class":"Barbarian","level":5}]`),
			AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":8,"wis":10,"cha":8}`),
			EquippedMainHand: sql.NullString{String: "greataxe", Valid: true},
			ProficiencyBonus: 3,
		}, nil
	}
	store.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) {
		return refdata.Weapon{
			ID:         "greataxe",
			Name:       "Greataxe",
			Damage:     "1d12",
			DamageType: "slashing",
			Properties: []string{"heavy", "two-handed"},
			WeaponType: "martial_melee",
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return nil, nil
	}
	store.listEncounterZonesByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.EncounterZone, error) {
		return nil, nil
	}
	var rageUpdates []refdata.UpdateCombatantRageParams
	store.updateCombatantRageFn = func(_ context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
		rageUpdates = append(rageUpdates, arg)
		return refdata.Combatant{ID: arg.ID, IsRaging: arg.IsRaging, RageAttackedThisRound: arg.RageAttackedThisRound, Conditions: json.RawMessage(`[]`)}, nil
	}

	svc := NewService(store)
	attacker := refdata.Combatant{
		ID:          combatantID,
		DisplayName: "Kael",
		EncounterID: encID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		PositionCol: "A",
		PositionRow: 1,
		IsRaging:    true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin",
		EncounterID: encID,
		PositionCol: "A",
		PositionRow: 2,
		Ac:          10,
		HpCurrent:   10,
		HpMax:       10,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	_, err := svc.Attack(context.Background(), AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     refdata.Turn{ID: uuid.New(), AttacksRemaining: 1},
	}, fixedRoller(15))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rageUpdates) == 0 {
		t.Fatal("expected at least one UpdateCombatantRage call after raging attacker attacks")
	}
	hit := false
	for _, u := range rageUpdates {
		if u.RageAttackedThisRound {
			hit = true
		}
	}
	if !hit {
		t.Errorf("expected RageAttackedThisRound = true to be persisted, got %+v", rageUpdates)
	}
}

// Non-raging attacker must NOT trigger the rage flag persistence.
func TestService_Attack_NonRagingDoesNotMarkRage(t *testing.T) {
	charID := uuid.New()
	combatantID := uuid.New()
	encID := uuid.New()
	targetID := uuid.New()

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			Classes:          json.RawMessage(`[{"class":"Fighter","level":5}]`),
			AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":8,"wis":10,"cha":8}`),
			EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
			ProficiencyBonus: 3,
		}, nil
	}
	store.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) {
		return refdata.Weapon{
			ID:         "longsword",
			Name:       "Longsword",
			Damage:     "1d8",
			DamageType: "slashing",
			Properties: []string{},
			WeaponType: "martial_melee",
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return nil, nil
	}
	store.listEncounterZonesByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.EncounterZone, error) {
		return nil, nil
	}
	rageUpdates := 0
	store.updateCombatantRageFn = func(_ context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
		rageUpdates++
		return refdata.Combatant{ID: arg.ID}, nil
	}

	svc := NewService(store)
	attacker := refdata.Combatant{
		ID:          combatantID,
		DisplayName: "NotRaging",
		EncounterID: encID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		PositionCol: "A",
		PositionRow: 1,
		IsRaging:    false,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin",
		EncounterID: encID,
		PositionCol: "A",
		PositionRow: 2,
		Ac:          10,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	_, err := svc.Attack(context.Background(), AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     refdata.Turn{ID: uuid.New(), AttacksRemaining: 1},
	}, fixedRoller(15))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rageUpdates != 0 {
		t.Errorf("non-raging attacker must not touch rage state, got %d updates", rageUpdates)
	}
}

// ---------------------------------------------------------------------------
// D-46-rage-auto-end-quiet — ApplyDamage must mark RageTookDamageThisRound
// on a raging target that takes positive damage.
// ---------------------------------------------------------------------------

func TestService_ApplyDamage_SetsRageTookDamageThisRound(t *testing.T) {
	targetID := uuid.New()
	encID := uuid.New()

	store, _ := applyDamageMockStore()
	rageMarked := false
	store.updateCombatantRageFn = func(_ context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
		if arg.RageTookDamageThisRound {
			rageMarked = true
		}
		return refdata.Combatant{ID: arg.ID, IsRaging: arg.IsRaging, RageTookDamageThisRound: arg.RageTookDamageThisRound, Conditions: json.RawMessage(`[]`)}, nil
	}
	svc := NewService(store)
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Kael",
		EncounterID: encID,
		HpCurrent:   25,
		HpMax:       30,
		IsRaging:    true,
		IsNpc:       false,
		Conditions:  json.RawMessage(`[]`),
	}
	_, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encID,
		Target:      target,
		RawDamage:   5,
		DamageType:  "slashing",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rageMarked {
		t.Error("expected RageTookDamageThisRound = true to be persisted")
	}
}

// ---------------------------------------------------------------------------
// D-46-rage-end-on-unconscious — dropping a raging combatant to 0 HP must
// clear rage state via ClearRageFromCombatant.
// ---------------------------------------------------------------------------

func TestService_ApplyDamage_RageEndsOnUnconscious(t *testing.T) {
	targetID := uuid.New()
	encID := uuid.New()

	store, _ := applyDamageMockStore()
	rageClearedToFalse := false
	var rageUpdates []refdata.UpdateCombatantRageParams
	store.updateCombatantRageFn = func(_ context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
		rageUpdates = append(rageUpdates, arg)
		if !arg.IsRaging {
			rageClearedToFalse = true
		}
		return refdata.Combatant{
			ID: arg.ID, IsRaging: arg.IsRaging,
			RageRoundsRemaining:     arg.RageRoundsRemaining,
			RageAttackedThisRound:   arg.RageAttackedThisRound,
			RageTookDamageThisRound: arg.RageTookDamageThisRound,
			Conditions:              json.RawMessage(`[]`),
		}, nil
	}
	svc := NewService(store)
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Kael",
		EncounterID: encID,
		HpCurrent:   5,
		HpMax:       30,
		IsRaging:    true,
		IsNpc:       false,
		Conditions:  json.RawMessage(`[]`),
	}
	_, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encID,
		Target:      target,
		RawDamage:   8, // drops below 0
		DamageType:  "slashing",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rageClearedToFalse {
		t.Errorf("expected rage to be cleared on unconscious, updates=%+v", rageUpdates)
	}
}

// ---------------------------------------------------------------------------
// D-48a unarmored-movement-consumer — createActiveTurn should re-evaluate
// turn-start effects so an unarmored monk's speed bonus appears in the new
// turn's movement_remaining.
// ---------------------------------------------------------------------------

func TestService_ResolveTurnResources_UnarmoredMonkGetsSpeedBonus(t *testing.T) {
	charID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:      charID,
			Classes: json.RawMessage(`[{"class":"Monk","level":6}]`),
			SpeedFt: 30,
			Features: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`[{"name":"Unarmored Movement","mechanical_effect":"speed_plus_10"}]`),
				Valid:      true,
			},
			// No armor equipped.
		}, nil
	}
	store.getClassFn = func(_ context.Context, _ string) (refdata.Class, error) {
		return refdata.Class{AttacksPerAction: json.RawMessage(`{"1":1}`)}, nil
	}

	svc := NewService(store)
	combatant := refdata.Combatant{
		ID:          combatantID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		Conditions:  json.RawMessage(`[]`),
	}
	speed, _, err := svc.ResolveTurnResources(context.Background(), combatant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Level 6 monk: +15 speed when unarmored. 30 + 15 = 45.
	if speed != 45 {
		t.Errorf("expected speed 45 (30 base + 15 UM at L6), got %d", speed)
	}
}

func TestService_ResolveTurnResources_ArmoredMonkNoSpeedBonus(t *testing.T) {
	charID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:            charID,
			Classes:       json.RawMessage(`[{"class":"Monk","level":6}]`),
			SpeedFt:       30,
			EquippedArmor: sql.NullString{String: "chain-mail", Valid: true},
			Features: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`[{"name":"Unarmored Movement","mechanical_effect":"speed_plus_10"}]`),
				Valid:      true,
			},
		}, nil
	}
	store.getClassFn = func(_ context.Context, _ string) (refdata.Class, error) {
		return refdata.Class{AttacksPerAction: json.RawMessage(`{"1":1}`)}, nil
	}

	svc := NewService(store)
	combatant := refdata.Combatant{
		ID:          combatantID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		Conditions:  json.RawMessage(`[]`),
	}
	speed, _, err := svc.ResolveTurnResources(context.Background(), combatant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if speed != 30 {
		t.Errorf("expected speed 30 (armor blocks UM), got %d", speed)
	}
}

// ---------------------------------------------------------------------------
// D-48b Stunning Strike — Attack() on a monk hit must surface a prompt
// eligibility hint in AttackResult.
// ---------------------------------------------------------------------------

func TestService_Attack_MonkHit_SurfacesStunningStrikePrompt(t *testing.T) {
	charID := uuid.New()
	combatantID := uuid.New()
	encID := uuid.New()
	targetID := uuid.New()

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			Classes:          json.RawMessage(`[{"class":"Monk","level":5}]`),
			AbilityScores:    json.RawMessage(`{"str":12,"dex":16,"con":14,"int":10,"wis":14,"cha":8}`),
			EquippedMainHand: sql.NullString{String: "shortsword", Valid: true},
			ProficiencyBonus: 3,
			FeatureUses: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`{"ki":{"current":5,"max":5,"recharge":"long"}}`),
				Valid:      true,
			},
		}, nil
	}
	store.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) {
		return refdata.Weapon{
			ID:         "shortsword",
			Name:       "Shortsword",
			Damage:     "1d6",
			DamageType: "piercing",
			Properties: []string{"finesse", "light"},
			WeaponType: "martial_melee",
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return nil, nil
	}
	store.listEncounterZonesByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.EncounterZone, error) {
		return nil, nil
	}

	svc := NewService(store)
	attacker := refdata.Combatant{
		ID:          combatantID,
		DisplayName: "Kira",
		EncounterID: encID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		PositionCol: "A",
		PositionRow: 1,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin",
		EncounterID: encID,
		PositionCol: "A",
		PositionRow: 2,
		Ac:          10,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	result, err := svc.Attack(context.Background(), AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     refdata.Turn{ID: uuid.New(), AttacksRemaining: 1},
	}, fixedRoller(15))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Hit {
		t.Fatal("expected hit (rolled 15 vs AC 10)")
	}
	if !result.PromptStunningStrikeEligible {
		t.Error("expected PromptStunningStrikeEligible = true (monk, melee hit, ki available)")
	}
	if result.PromptStunningStrikeKiAvailable != 5 {
		t.Errorf("expected ki available = 5, got %d", result.PromptStunningStrikeKiAvailable)
	}
}

func TestService_Attack_MonkOutOfKi_NoStunningStrikePrompt(t *testing.T) {
	charID := uuid.New()
	combatantID := uuid.New()
	encID := uuid.New()
	targetID := uuid.New()

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			Classes:          json.RawMessage(`[{"class":"Monk","level":5}]`),
			AbilityScores:    json.RawMessage(`{"str":12,"dex":16,"con":14,"int":10,"wis":14,"cha":8}`),
			EquippedMainHand: sql.NullString{String: "shortsword", Valid: true},
			ProficiencyBonus: 3,
			FeatureUses: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`{"ki":{"current":0,"max":0,"recharge":"long"}}`),
				Valid:      true,
			},
		}, nil
	}
	store.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) {
		return refdata.Weapon{
			ID:         "shortsword",
			Name:       "Shortsword",
			Damage:     "1d6",
			DamageType: "piercing",
			Properties: []string{"finesse", "light"},
			WeaponType: "martial_melee",
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return nil, nil
	}
	store.listEncounterZonesByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.EncounterZone, error) {
		return nil, nil
	}

	svc := NewService(store)
	attacker := refdata.Combatant{
		ID:          combatantID,
		DisplayName: "Kira",
		EncounterID: encID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		PositionCol: "A",
		PositionRow: 1,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin",
		EncounterID: encID,
		PositionCol: "A",
		PositionRow: 2,
		Ac:          10,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	result, err := svc.Attack(context.Background(), AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     refdata.Turn{ID: uuid.New(), AttacksRemaining: 1},
	}, fixedRoller(15))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PromptStunningStrikeEligible {
		t.Error("expected PromptStunningStrikeEligible = false (no ki remaining)")
	}
}

// ---------------------------------------------------------------------------
// D-49 Bardic Inspiration — Attack() by a combatant holding an inspiration
// die must surface a prompt-eligibility hint in AttackResult.
// ---------------------------------------------------------------------------

func TestService_Attack_HolderHasBardicInspiration_SurfacesPrompt(t *testing.T) {
	charID := uuid.New()
	combatantID := uuid.New()
	encID := uuid.New()
	targetID := uuid.New()

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			Classes:          json.RawMessage(`[{"class":"Fighter","level":3}]`),
			AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":10,"wis":10,"cha":8}`),
			EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
			ProficiencyBonus: 2,
		}, nil
	}
	store.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) {
		return refdata.Weapon{
			ID: "longsword", Name: "Longsword",
			Damage: "1d8", DamageType: "slashing",
			Properties: []string{}, WeaponType: "martial_melee",
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return nil, nil
	}
	store.listEncounterZonesByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.EncounterZone, error) {
		return nil, nil
	}

	svc := NewService(store)
	attacker := refdata.Combatant{
		ID:                   combatantID,
		DisplayName:          "Borin",
		EncounterID:          encID,
		CharacterID:          uuid.NullUUID{UUID: charID, Valid: true},
		PositionCol:          "A",
		PositionRow:          1,
		IsVisible:            true,
		Conditions:           json.RawMessage(`[]`),
		BardicInspirationDie: sql.NullString{String: "d8", Valid: true},
	}
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin",
		EncounterID: encID,
		PositionCol: "A",
		PositionRow: 2,
		Ac:          10,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	result, err := svc.Attack(context.Background(), AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     refdata.Turn{ID: uuid.New(), AttacksRemaining: 1},
	}, fixedRoller(15))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.PromptBardicInspirationEligible {
		t.Error("expected PromptBardicInspirationEligible = true (holder has BI die)")
	}
	if result.PromptBardicInspirationDie != "d8" {
		t.Errorf("expected die d8, got %q", result.PromptBardicInspirationDie)
	}
}

// ---------------------------------------------------------------------------
// D-51 Divine Smite — Attack() on a paladin melee hit must surface prompt
// eligibility + the list of available slot levels.
// ---------------------------------------------------------------------------

func TestService_Attack_PaladinHit_SurfacesDivineSmitePrompt(t *testing.T) {
	charID := uuid.New()
	combatantID := uuid.New()
	encID := uuid.New()
	targetID := uuid.New()

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			Classes:          json.RawMessage(`[{"class":"Paladin","level":5}]`),
			AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":10,"wis":10,"cha":16}`),
			EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
			ProficiencyBonus: 3,
			Features: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`[{"name":"Divine Smite","mechanical_effect":"divine_smite"}]`),
				Valid:      true,
			},
			SpellSlots: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`{"1":{"current":3,"max":4},"2":{"current":2,"max":3}}`),
				Valid:      true,
			},
		}, nil
	}
	store.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) {
		return refdata.Weapon{
			ID: "longsword", Name: "Longsword",
			Damage: "1d8", DamageType: "slashing",
			Properties: []string{}, WeaponType: "martial_melee",
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return nil, nil
	}
	store.listEncounterZonesByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.EncounterZone, error) {
		return nil, nil
	}

	svc := NewService(store)
	attacker := refdata.Combatant{
		ID:          combatantID,
		DisplayName: "Aria",
		EncounterID: encID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		PositionCol: "A",
		PositionRow: 1,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin",
		EncounterID: encID,
		PositionCol: "A",
		PositionRow: 2,
		Ac:          10,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	result, err := svc.Attack(context.Background(), AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     refdata.Turn{ID: uuid.New(), AttacksRemaining: 1},
	}, fixedRoller(15))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Hit {
		t.Fatal("expected hit")
	}
	if !result.PromptDivineSmiteEligible {
		t.Error("expected PromptDivineSmiteEligible = true (paladin melee hit, slots available)")
	}
	if len(result.PromptDivineSmiteSlots) == 0 {
		t.Error("expected at least one available slot level surfaced")
	}
}

func TestService_Attack_PaladinMiss_NoDivineSmitePrompt(t *testing.T) {
	charID := uuid.New()
	combatantID := uuid.New()
	encID := uuid.New()
	targetID := uuid.New()

	store := defaultMockStore()
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			Classes:          json.RawMessage(`[{"class":"Paladin","level":5}]`),
			AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":10,"wis":10,"cha":16}`),
			EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
			ProficiencyBonus: 3,
			Features: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`[{"name":"Divine Smite","mechanical_effect":"divine_smite"}]`),
				Valid:      true,
			},
			SpellSlots: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`{"1":{"current":3,"max":4}}`),
				Valid:      true,
			},
		}, nil
	}
	store.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) {
		return refdata.Weapon{
			ID: "longsword", Name: "Longsword",
			Damage: "1d8", DamageType: "slashing",
			Properties: []string{}, WeaponType: "martial_melee",
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return nil, nil
	}
	store.listEncounterZonesByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.EncounterZone, error) {
		return nil, nil
	}

	svc := NewService(store)
	attacker := refdata.Combatant{
		ID:          combatantID,
		DisplayName: "Aria",
		EncounterID: encID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		PositionCol: "A",
		PositionRow: 1,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin",
		EncounterID: encID,
		PositionCol: "A",
		PositionRow: 2,
		Ac:          25, // unreachable
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	result, err := svc.Attack(context.Background(), AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     refdata.Turn{ID: uuid.New(), AttacksRemaining: 1},
	}, fixedRoller(2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Hit {
		t.Fatal("expected miss")
	}
	if result.PromptDivineSmiteEligible {
		t.Error("expected PromptDivineSmiteEligible = false on miss")
	}
}

// containsLower is a case-insensitive substring helper.
func containsLower(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
