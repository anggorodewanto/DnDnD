package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

func TestRageEffects_DamageOnlyMeleeSTR(t *testing.T) {
	features := []FeatureDefinition{RageFeature(5)}

	// Raging + melee + STR: should get +2
	ctx := EffectContext{IsRaging: true, AttackType: "melee", AbilityUsed: "str"}
	result := ProcessEffects(features, TriggerOnDamageRoll, ctx)
	if result.FlatModifier != 2 {
		t.Errorf("melee STR raging: FlatModifier = %d, want 2", result.FlatModifier)
	}

	// Raging + ranged: no bonus
	ctx = EffectContext{IsRaging: true, AttackType: "ranged", AbilityUsed: "str"}
	result = ProcessEffects(features, TriggerOnDamageRoll, ctx)
	if result.FlatModifier != 0 {
		t.Errorf("ranged raging: FlatModifier = %d, want 0", result.FlatModifier)
	}

	// Raging + melee + DEX: no bonus
	ctx = EffectContext{IsRaging: true, AttackType: "melee", AbilityUsed: "dex"}
	result = ProcessEffects(features, TriggerOnDamageRoll, ctx)
	if result.FlatModifier != 0 {
		t.Errorf("melee DEX raging: FlatModifier = %d, want 0", result.FlatModifier)
	}

	// Not raging: no bonus
	ctx = EffectContext{IsRaging: false, AttackType: "melee", AbilityUsed: "str"}
	result = ProcessEffects(features, TriggerOnDamageRoll, ctx)
	if result.FlatModifier != 0 {
		t.Errorf("not raging: FlatModifier = %d, want 0", result.FlatModifier)
	}
}

func TestRageEffects_ResistanceBPS(t *testing.T) {
	features := []FeatureDefinition{RageFeature(5)}

	ctx := EffectContext{IsRaging: true}
	result := ProcessEffects(features, TriggerOnTakeDamage, ctx)
	if len(result.Resistances) != 3 {
		t.Fatalf("expected 3 resistances, got %d: %v", len(result.Resistances), result.Resistances)
	}

	// Not raging: no resistances
	ctx = EffectContext{IsRaging: false}
	result = ProcessEffects(features, TriggerOnTakeDamage, ctx)
	if len(result.Resistances) != 0 {
		t.Errorf("not raging: expected 0 resistances, got %d", len(result.Resistances))
	}
}

func TestRageEffects_AdvantageSTRChecks(t *testing.T) {
	features := []FeatureDefinition{RageFeature(5)}

	// Raging + STR check: advantage
	ctx := EffectContext{IsRaging: true, AbilityUsed: "str"}
	result := ProcessEffects(features, TriggerOnCheck, ctx)
	if result.RollMode != 1 { // dice.Advantage
		t.Errorf("STR check raging: RollMode = %v, want Advantage", result.RollMode)
	}

	// Raging + DEX check: no advantage
	ctx = EffectContext{IsRaging: true, AbilityUsed: "dex"}
	result = ProcessEffects(features, TriggerOnCheck, ctx)
	if result.RollMode != 0 { // dice.Normal
		t.Errorf("DEX check raging: RollMode = %v, want Normal", result.RollMode)
	}
}

func TestApplyRageToCombatant(t *testing.T) {
	c := refdata.Combatant{DisplayName: "Kael"}
	c = ApplyRageToCombatant(c)

	if !c.IsRaging {
		t.Error("expected IsRaging = true")
	}
	if !c.RageRoundsRemaining.Valid || c.RageRoundsRemaining.Int32 != 10 {
		t.Errorf("RageRoundsRemaining = %v, want 10", c.RageRoundsRemaining)
	}
	if c.RageAttackedThisRound {
		t.Error("expected RageAttackedThisRound = false")
	}
	if c.RageTookDamageThisRound {
		t.Error("expected RageTookDamageThisRound = false")
	}
}

func TestClearRageFromCombatant(t *testing.T) {
	c := ApplyRageToCombatant(refdata.Combatant{DisplayName: "Kael"})
	c.RageAttackedThisRound = true
	c = ClearRageFromCombatant(c)

	if c.IsRaging {
		t.Error("expected IsRaging = false")
	}
	if c.RageRoundsRemaining.Valid {
		t.Error("expected RageRoundsRemaining to be null")
	}
	if c.RageAttackedThisRound {
		t.Error("expected RageAttackedThisRound = false")
	}
}

func TestShouldRageEndOnTurnEnd(t *testing.T) {
	tests := []struct {
		name     string
		raging   bool
		attacked bool
		tookDmg  bool
		want     bool
	}{
		{"not raging", false, false, false, false},
		{"attacked", true, true, false, false},
		{"took damage", true, false, true, false},
		{"both", true, true, true, false},
		{"neither", true, false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := refdata.Combatant{
				IsRaging:                tt.raging,
				RageAttackedThisRound:   tt.attacked,
				RageTookDamageThisRound: tt.tookDmg,
			}
			if got := ShouldRageEndOnTurnEnd(c); got != tt.want {
				t.Errorf("ShouldRageEndOnTurnEnd() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldRageEndOnTurnStart(t *testing.T) {
	tests := []struct {
		name   string
		c      refdata.Combatant
		want   bool
	}{
		{"not raging", refdata.Combatant{IsRaging: false}, false},
		{"rounds remaining", refdata.Combatant{
			IsRaging:            true,
			RageRoundsRemaining: sql.NullInt32{Int32: 5, Valid: true},
		}, false},
		{"rounds expired", refdata.Combatant{
			IsRaging:            true,
			RageRoundsRemaining: sql.NullInt32{Int32: 0, Valid: true},
		}, true},
		{"negative rounds", refdata.Combatant{
			IsRaging:            true,
			RageRoundsRemaining: sql.NullInt32{Int32: -1, Valid: true},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldRageEndOnTurnStart(tt.c); got != tt.want {
				t.Errorf("ShouldRageEndOnTurnStart() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecrementRageRound(t *testing.T) {
	c := ApplyRageToCombatant(refdata.Combatant{})
	c.RageAttackedThisRound = true
	c.RageTookDamageThisRound = true

	c = DecrementRageRound(c)

	if c.RageRoundsRemaining.Int32 != 9 {
		t.Errorf("RageRoundsRemaining = %d, want 9", c.RageRoundsRemaining.Int32)
	}
	if c.RageAttackedThisRound {
		t.Error("expected RageAttackedThisRound reset to false")
	}
	if c.RageTookDamageThisRound {
		t.Error("expected RageTookDamageThisRound reset to false")
	}
}

func TestValidateRageActivation(t *testing.T) {
	baseInput := RageActivateInput{
		Combatant: refdata.Combatant{IsRaging: false},
		Character: refdata.Character{},
	}

	// Normal: no error
	if err := ValidateRageActivation(baseInput, "medium"); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Already raging
	ragingInput := baseInput
	ragingInput.Combatant.IsRaging = true
	if err := ValidateRageActivation(ragingInput, ""); err == nil {
		t.Error("expected error for already raging")
	}

	// Heavy armor
	if err := ValidateRageActivation(baseInput, "heavy"); err == nil {
		t.Error("expected error for heavy armor")
	}
}

func TestFormatRageActivation(t *testing.T) {
	got := FormatRageActivation("Kael", 3)
	want := "\U0001f525  Kael enters a Rage! (3 rages remaining today)"
	if got != want {
		t.Errorf("FormatRageActivation = %q, want %q", got, want)
	}
}

func TestFormatRageEnd(t *testing.T) {
	got := FormatRageEnd("Kael", "didn't attack or take damage this round")
	want := "\U0001f525  Kael's Rage ends \u2014 didn't attack or take damage this round"
	if got != want {
		t.Errorf("FormatRageEnd = %q, want %q", got, want)
	}
}

func TestFormatRageEndVoluntary(t *testing.T) {
	got := FormatRageEndVoluntary("Kael")
	want := "\U0001f525  Kael ends their Rage"
	if got != want {
		t.Errorf("FormatRageEndVoluntary = %q, want %q", got, want)
	}
}

func TestBuildFeatureDefinitions_Rage(t *testing.T) {
	classes := []CharacterClass{{Class: "Barbarian", Level: 5}}
	features := []CharacterFeature{
		{Name: "Rage", MechanicalEffect: "rage"},
	}

	defs := BuildFeatureDefinitions(classes, features)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if defs[0].Name != "Rage" {
		t.Errorf("Name = %q, want Rage", defs[0].Name)
	}
	// Level 5 barbarian should have +2 damage
	if defs[0].Effects[0].Modifier != 2 {
		t.Errorf("Modifier = %d, want 2", defs[0].Effects[0].Modifier)
	}
}

func TestBuildFeatureDefinitions_RageLevel9(t *testing.T) {
	classes := []CharacterClass{{Class: "Barbarian", Level: 9}}
	features := []CharacterFeature{
		{Name: "Rage", MechanicalEffect: "rage"},
	}

	defs := BuildFeatureDefinitions(classes, features)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	// Level 9 barbarian should have +3 damage
	if defs[0].Effects[0].Modifier != 3 {
		t.Errorf("Modifier = %d, want 3", defs[0].Effects[0].Modifier)
	}
}

func TestService_ActivateRage_Success(t *testing.T) {
	charID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:      charID,
			Classes: json.RawMessage(`[{"class":"Barbarian","level":5}]`),
			FeatureUses: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`{"rage":3}`),
				Valid:      true,
			},
			EquippedArmor: sql.NullString{String: "chain-mail", Valid: true},
		}, nil
	}
	store.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		return refdata.Armor{ID: "chain-mail", ArmorType: "medium"}, nil
	}
	store.updateCombatantRageFn = func(ctx context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:                  arg.ID,
			DisplayName:         "Kael",
			IsRaging:            arg.IsRaging,
			RageRoundsRemaining: arg.RageRoundsRemaining,
			Conditions:          json.RawMessage(`[]`),
		}, nil
	}
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: true}, nil
	}

	svc := NewService(store)
	result, err := svc.ActivateRage(context.Background(), RageCommand{
		Combatant: refdata.Combatant{
			ID:          combatantID,
			DisplayName: "Kael",
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		},
		Turn: refdata.Turn{ID: turnID},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Combatant.IsRaging {
		t.Error("expected combatant to be raging")
	}
	if result.RagesLeft != 2 {
		t.Errorf("RagesLeft = %d, want 2", result.RagesLeft)
	}
}

func TestService_ActivateRage_BonusActionSpent(t *testing.T) {
	store := defaultMockStore()
	svc := NewService(store)

	_, err := svc.ActivateRage(context.Background(), RageCommand{
		Combatant: refdata.Combatant{
			CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		},
		Turn: refdata.Turn{BonusActionUsed: true},
	})
	if err == nil {
		t.Error("expected error for spent bonus action")
	}
}

func TestService_ActivateRage_AlreadyRaging(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:      charID,
			Classes: json.RawMessage(`[{"class":"Barbarian","level":5}]`),
			FeatureUses: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`{"rage":3}`),
				Valid:      true,
			},
		}, nil
	}
	svc := NewService(store)

	_, err := svc.ActivateRage(context.Background(), RageCommand{
		Combatant: refdata.Combatant{
			IsRaging:    true,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		},
		Turn: refdata.Turn{},
	})
	if err == nil {
		t.Error("expected error for already raging")
	}
}

func TestService_ActivateRage_HeavyArmor(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:            charID,
			Classes:       json.RawMessage(`[{"class":"Barbarian","level":5}]`),
			EquippedArmor: sql.NullString{String: "plate", Valid: true},
			FeatureUses: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`{"rage":3}`),
				Valid:      true,
			},
		}, nil
	}
	store.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		return refdata.Armor{ID: "plate", ArmorType: "heavy"}, nil
	}
	svc := NewService(store)

	_, err := svc.ActivateRage(context.Background(), RageCommand{
		Combatant: refdata.Combatant{
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		},
		Turn: refdata.Turn{},
	})
	if err == nil {
		t.Error("expected error for heavy armor")
	}
}

func TestService_ActivateRage_NoRagesLeft(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:      charID,
			Classes: json.RawMessage(`[{"class":"Barbarian","level":5}]`),
			FeatureUses: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`{"rage":0}`),
				Valid:      true,
			},
		}, nil
	}
	svc := NewService(store)

	_, err := svc.ActivateRage(context.Background(), RageCommand{
		Combatant: refdata.Combatant{
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		},
		Turn: refdata.Turn{},
	})
	if err == nil {
		t.Error("expected error for no rages left")
	}
}

func TestService_ActivateRage_NotBarbarian(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:      charID,
			Classes: json.RawMessage(`[{"class":"Fighter","level":5}]`),
			FeatureUses: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`{}`),
				Valid:      true,
			},
		}, nil
	}
	svc := NewService(store)

	_, err := svc.ActivateRage(context.Background(), RageCommand{
		Combatant: refdata.Combatant{
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		},
		Turn: refdata.Turn{},
	})
	if err == nil {
		t.Error("expected error for non-barbarian")
	}
}

func TestService_EndRage_Success(t *testing.T) {
	store := defaultMockStore()
	store.updateCombatantRageFn = func(ctx context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:       arg.ID,
			IsRaging: arg.IsRaging,
		}, nil
	}
	svc := NewService(store)

	result, err := svc.EndRage(context.Background(), RageCommand{
		Combatant: refdata.Combatant{
			ID:          uuid.New(),
			DisplayName: "Kael",
			IsRaging:    true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Combatant.IsRaging {
		t.Error("expected IsRaging = false after EndRage")
	}
	if result.CombatLog != "\U0001f525  Kael ends their Rage" {
		t.Errorf("CombatLog = %q", result.CombatLog)
	}
}

func TestService_EndRage_NotRaging(t *testing.T) {
	store := defaultMockStore()
	svc := NewService(store)

	_, err := svc.EndRage(context.Background(), RageCommand{
		Combatant: refdata.Combatant{IsRaging: false},
	})
	if err == nil {
		t.Error("expected error for not raging")
	}
}

func TestShouldRageEndOnUnconscious(t *testing.T) {
	// Raging + hp = 0 → should end
	c := refdata.Combatant{IsRaging: true, HpCurrent: 0}
	if !ShouldRageEndOnUnconscious(c) {
		t.Error("expected rage to end on hp=0")
	}

	// Raging + hp > 0 → should not end
	c = refdata.Combatant{IsRaging: true, HpCurrent: 10}
	if ShouldRageEndOnUnconscious(c) {
		t.Error("expected rage to continue with hp > 0")
	}

	// Not raging → false
	c = refdata.Combatant{IsRaging: false, HpCurrent: 0}
	if ShouldRageEndOnUnconscious(c) {
		t.Error("expected false when not raging")
	}
}

func TestService_ActivateRage_UnlimitedLevel20(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:      charID,
			Classes: json.RawMessage(`[{"class":"Barbarian","level":20}]`),
			FeatureUses: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`{"rage":0}`),
				Valid:      true,
			},
		}, nil
	}
	store.updateCombatantRageFn = func(ctx context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:                  arg.ID,
			DisplayName:         "Kael",
			IsRaging:            arg.IsRaging,
			RageRoundsRemaining: arg.RageRoundsRemaining,
		}, nil
	}
	updateFeatureUsesCalled := false
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		updateFeatureUsesCalled = true
		return refdata.Character{ID: arg.ID}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: true}, nil
	}

	svc := NewService(store)
	result, err := svc.ActivateRage(context.Background(), RageCommand{
		Combatant: refdata.Combatant{
			ID:          uuid.New(),
			DisplayName: "Kael",
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		},
		Turn: refdata.Turn{ID: uuid.New()},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Combatant.IsRaging != true {
		t.Error("expected raging at level 20 despite 0 uses")
	}
	if updateFeatureUsesCalled {
		t.Error("should not deduct feature_uses for unlimited rages")
	}
}

func TestBuildAttackEffectContext_AbilityUsed(t *testing.T) {
	// STR-based melee weapon
	ctx := BuildAttackEffectContext(AttackEffectInput{
		Weapon: refdata.Weapon{
			ID:         "greataxe",
			Name:       "Greataxe",
			Properties: []string{"heavy", "two-handed"},
			WeaponType: "martial_melee",
		},
		IsRaging:    true,
		AbilityUsed: "str",
	})
	if ctx.AbilityUsed != "str" {
		t.Errorf("AbilityUsed = %q, want str", ctx.AbilityUsed)
	}
	if !ctx.IsRaging {
		t.Error("expected IsRaging = true")
	}
}

func TestIntegration_RageDamageBonusViaEffectSystem(t *testing.T) {
	classes := []CharacterClass{{Class: "Barbarian", Level: 5}}
	charFeatures := []CharacterFeature{
		{Name: "Rage", MechanicalEffect: "rage"},
	}
	defs := BuildFeatureDefinitions(classes, charFeatures)

	// Raging + STR melee attack with greataxe → +2 damage
	ctx := BuildAttackEffectContext(AttackEffectInput{
		Weapon: refdata.Weapon{
			ID:         "greataxe",
			Name:       "Greataxe",
			Properties: []string{"heavy", "two-handed"},
			WeaponType: "martial_melee",
		},
		IsRaging:    true,
		AbilityUsed: "str",
	})
	result := ProcessEffects(defs, TriggerOnDamageRoll, ctx)
	if result.FlatModifier != 2 {
		t.Errorf("expected +2 rage damage, got %d", result.FlatModifier)
	}

	// Not raging → no bonus
	ctx.IsRaging = false
	result = ProcessEffects(defs, TriggerOnDamageRoll, ctx)
	if result.FlatModifier != 0 {
		t.Errorf("expected 0 when not raging, got %d", result.FlatModifier)
	}
}

func TestIntegration_RageResistanceViaEffectSystem(t *testing.T) {
	classes := []CharacterClass{{Class: "Barbarian", Level: 5}}
	charFeatures := []CharacterFeature{
		{Name: "Rage", MechanicalEffect: "rage"},
	}
	defs := BuildFeatureDefinitions(classes, charFeatures)

	// Raging → B/P/S resistance
	ctx := EffectContext{IsRaging: true}
	result := ProcessEffects(defs, TriggerOnTakeDamage, ctx)
	if len(result.Resistances) != 3 {
		t.Fatalf("expected 3 resistances, got %d", len(result.Resistances))
	}
	expected := map[string]bool{"bludgeoning": true, "piercing": true, "slashing": true}
	for _, r := range result.Resistances {
		if !expected[r] {
			t.Errorf("unexpected resistance: %q", r)
		}
	}
}

func TestBarbarianLevel(t *testing.T) {
	tests := []struct {
		name    string
		classes json.RawMessage
		want    int
	}{
		{"barbarian 5", json.RawMessage(`[{"class":"Barbarian","level":5}]`), 5},
		{"no barbarian", json.RawMessage(`[{"class":"Fighter","level":10}]`), 0},
		{"multiclass", json.RawMessage(`[{"class":"Barbarian","level":3},{"class":"Fighter","level":2}]`), 3},
		{"empty", json.RawMessage(`[]`), 0},
		{"nil", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := barbarianLevel(tt.classes)
			if got != tt.want {
				t.Errorf("barbarianLevel() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestIsHeavyArmor(t *testing.T) {
	if !IsHeavyArmor(refdata.Armor{ArmorType: "heavy"}) {
		t.Error("expected heavy armor")
	}
	if IsHeavyArmor(refdata.Armor{ArmorType: "medium"}) {
		t.Error("expected not heavy armor")
	}
	if IsHeavyArmor(refdata.Armor{ArmorType: "light"}) {
		t.Error("expected not heavy armor")
	}
}

func TestService_ActivateRage_NPC(t *testing.T) {
	store := defaultMockStore()
	svc := NewService(store)

	_, err := svc.ActivateRage(context.Background(), RageCommand{
		Combatant: refdata.Combatant{},
		Turn:      refdata.Turn{},
	})
	if err == nil {
		t.Error("expected error for NPC")
	}
}

func TestService_ActivateRage_NoArmorEquipped(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:      charID,
			Classes: json.RawMessage(`[{"class":"Barbarian","level":5}]`),
			FeatureUses: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`{"rage":3}`),
				Valid:      true,
			},
			// No armor equipped
		}, nil
	}
	store.updateCombatantRageFn = func(ctx context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, DisplayName: "Kael", IsRaging: arg.IsRaging, RageRoundsRemaining: arg.RageRoundsRemaining}, nil
	}
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: true}, nil
	}

	svc := NewService(store)
	result, err := svc.ActivateRage(context.Background(), RageCommand{
		Combatant: refdata.Combatant{
			ID:          uuid.New(),
			DisplayName: "Kael",
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		},
		Turn: refdata.Turn{ID: uuid.New()},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Combatant.IsRaging {
		t.Error("expected raging with no armor")
	}
}

func TestParseRageUses_NoFeatureUses(t *testing.T) {
	char := refdata.Character{}
	uses, remaining, err := parseRageUses(char)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remaining != 0 {
		t.Errorf("remaining = %d, want 0", remaining)
	}
	if len(uses) != 0 {
		t.Errorf("uses len = %d, want 0", len(uses))
	}
}

func TestService_ActivateRage_GetCharacterError(t *testing.T) {
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}
	svc := NewService(store)
	_, err := svc.ActivateRage(context.Background(), RageCommand{
		Combatant: refdata.Combatant{CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true}},
		Turn:      refdata.Turn{},
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestService_ActivateRage_UpdateCombatantRageError(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:      charID,
			Classes: json.RawMessage(`[{"class":"Barbarian","level":5}]`),
			FeatureUses: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`{"rage":3}`),
				Valid:      true,
			},
		}, nil
	}
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID}, nil
	}
	store.updateCombatantRageFn = func(ctx context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}
	svc := NewService(store)
	_, err := svc.ActivateRage(context.Background(), RageCommand{
		Combatant: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		},
		Turn: refdata.Turn{ID: uuid.New()},
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestService_ActivateRage_UpdateTurnError(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:      charID,
			Classes: json.RawMessage(`[{"class":"Barbarian","level":5}]`),
			FeatureUses: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`{"rage":3}`),
				Valid:      true,
			},
		}, nil
	}
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}
	svc := NewService(store)
	_, err := svc.ActivateRage(context.Background(), RageCommand{
		Combatant: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		},
		Turn: refdata.Turn{ID: uuid.New()},
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestService_ActivateRage_UpdateFeatureUsesError(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:      charID,
			Classes: json.RawMessage(`[{"class":"Barbarian","level":5}]`),
			FeatureUses: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`{"rage":3}`),
				Valid:      true,
			},
		}, nil
	}
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}
	svc := NewService(store)
	_, err := svc.ActivateRage(context.Background(), RageCommand{
		Combatant: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		},
		Turn: refdata.Turn{ID: uuid.New()},
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestService_EndRage_UpdateError(t *testing.T) {
	store := defaultMockStore()
	store.updateCombatantRageFn = func(ctx context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}
	svc := NewService(store)
	_, err := svc.EndRage(context.Background(), RageCommand{
		Combatant: refdata.Combatant{ID: uuid.New(), IsRaging: true},
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestParseRageUses_InvalidJSON(t *testing.T) {
	char := refdata.Character{
		FeatureUses: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`invalid`),
			Valid:      true,
		},
	}
	_, _, err := parseRageUses(char)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestRageDamageBonus(t *testing.T) {
	tests := []struct {
		level int
		want  int
	}{
		{1, 2}, {2, 2}, {3, 2}, {8, 2},
		{9, 3}, {10, 3}, {15, 3},
		{16, 4}, {17, 4}, {20, 4},
	}
	for _, tt := range tests {
		got := RageDamageBonus(tt.level)
		if got != tt.want {
			t.Errorf("RageDamageBonus(%d) = %d, want %d", tt.level, got, tt.want)
		}
	}
}

func TestRageUsesPerDay(t *testing.T) {
	tests := []struct {
		level int
		want  int
	}{
		{1, 2}, {2, 2},
		{3, 3}, {5, 3},
		{6, 4}, {8, 4},
		{9, 4}, {11, 4},
		{12, 5}, {15, 5},
		{16, 5},
		{17, 6}, {19, 6},
		{20, -1}, // unlimited
	}
	for _, tt := range tests {
		got := RageUsesPerDay(tt.level)
		if got != tt.want {
			t.Errorf("RageUsesPerDay(%d) = %d, want %d", tt.level, got, tt.want)
		}
	}
}

func TestRageFeature_DamageScaling(t *testing.T) {
	tests := []struct {
		level    int
		wantMod  int
	}{
		{1, 2}, {8, 2}, {9, 3}, {15, 3}, {16, 4}, {20, 4},
	}
	for _, tt := range tests {
		fd := RageFeature(tt.level)
		got := fd.Effects[0].Modifier
		if got != tt.wantMod {
			t.Errorf("RageFeature(%d) damage modifier = %d, want %d", tt.level, got, tt.wantMod)
		}
	}
}

func TestRageFeature_Level1(t *testing.T) {
	fd := RageFeature(1)

	if fd.Name != "Rage" {
		t.Errorf("Name = %q, want %q", fd.Name, "Rage")
	}
	if fd.Source != "barbarian" {
		t.Errorf("Source = %q, want %q", fd.Source, "barbarian")
	}
	if len(fd.Effects) != 3 {
		t.Fatalf("expected 3 effects, got %d", len(fd.Effects))
	}

	// Effect 0: damage bonus
	e0 := fd.Effects[0]
	if e0.Type != EffectModifyDamageRoll {
		t.Errorf("Effect[0].Type = %q, want %q", e0.Type, EffectModifyDamageRoll)
	}
	if e0.Trigger != TriggerOnDamageRoll {
		t.Errorf("Effect[0].Trigger = %q, want %q", e0.Trigger, TriggerOnDamageRoll)
	}
	if e0.Modifier != 2 {
		t.Errorf("Effect[0].Modifier = %d, want 2", e0.Modifier)
	}
	if !e0.Conditions.WhenRaging {
		t.Error("Effect[0] expected WhenRaging = true")
	}
	if e0.Conditions.AttackType != "melee" {
		t.Errorf("Effect[0].AttackType = %q, want melee", e0.Conditions.AttackType)
	}
	if e0.Conditions.AbilityUsed != "str" {
		t.Errorf("Effect[0].AbilityUsed = %q, want str", e0.Conditions.AbilityUsed)
	}

	// Effect 1: resistance to B/P/S
	e1 := fd.Effects[1]
	if e1.Type != EffectGrantResistance {
		t.Errorf("Effect[1].Type = %q, want %q", e1.Type, EffectGrantResistance)
	}
	if e1.Trigger != TriggerOnTakeDamage {
		t.Errorf("Effect[1].Trigger = %q, want %q", e1.Trigger, TriggerOnTakeDamage)
	}
	if len(e1.DamageTypes) != 3 {
		t.Fatalf("Effect[1] expected 3 damage types, got %d", len(e1.DamageTypes))
	}
	if !e1.Conditions.WhenRaging {
		t.Error("Effect[1] expected WhenRaging = true")
	}

	// Effect 2: advantage on STR checks
	e2 := fd.Effects[2]
	if e2.Type != EffectConditionalAdvantage {
		t.Errorf("Effect[2].Type = %q, want %q", e2.Type, EffectConditionalAdvantage)
	}
	if e2.Trigger != TriggerOnCheck {
		t.Errorf("Effect[2].Trigger = %q, want %q", e2.Trigger, TriggerOnCheck)
	}
	if !e2.Conditions.WhenRaging {
		t.Error("Effect[2] expected WhenRaging = true")
	}
	if e2.Conditions.AbilityUsed != "str" {
		t.Errorf("Effect[2].AbilityUsed = %q, want str", e2.Conditions.AbilityUsed)
	}
}
