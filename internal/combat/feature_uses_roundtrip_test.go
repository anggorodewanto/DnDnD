package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/rest"
)

// SR-009 round-trip test: combat writes a Rage row (deducting one use), the
// rest service runs a long rest, and the combat read path sees the row at
// full uses again. This is mutually unparseable under the divergent shapes
// — the test fails fast under either fork.
func TestFeatureUses_RoundTrip_RageRechargesOnLongRest(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()

	// Seed: rage 3/3, long recharge — canonical character.FeatureUse shape.
	initial := map[string]character.FeatureUse{
		FeatureKeyRage: {Current: 3, Max: 3, Recharge: "long"},
	}
	initialJSON, err := json.Marshal(initial)
	if err != nil {
		t.Fatalf("marshal initial: %v", err)
	}

	var persistedFeatureUses []byte
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		raw := initialJSON
		if persistedFeatureUses != nil {
			raw = persistedFeatureUses
		}
		return refdata.Character{
			ID:          charID,
			Classes:     json.RawMessage(`[{"class":"Barbarian","level":5}]`),
			FeatureUses: pqtype.NullRawMessage{RawMessage: raw, Valid: true},
		}, nil
	}
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		persistedFeatureUses = append([]byte(nil), arg.FeatureUses.RawMessage...)
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: true}, nil
	}
	store.updateCombatantRageFn = func(ctx context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, IsRaging: arg.IsRaging, Conditions: json.RawMessage(`[]`)}, nil
	}

	svc := NewService(store)

	// 1. Combat writes: ActivateRage deducts one use → 2/3 remaining.
	_, err = svc.ActivateRage(ctx, RageCommand{
		Combatant: refdata.Combatant{
			ID:          uuid.New(),
			DisplayName: "Kael",
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Turn: refdata.Turn{ID: uuid.New()},
	})
	if err != nil {
		t.Fatalf("ActivateRage failed: %v", err)
	}

	// Verify the persisted shape is the canonical struct — Max + Recharge
	// must round-trip (this is the regression: the old flat-int writer
	// stripped Max/Recharge so the rest service had nothing to recharge).
	var afterDeduct map[string]character.FeatureUse
	if err := json.Unmarshal(persistedFeatureUses, &afterDeduct); err != nil {
		t.Fatalf("combat-persisted feature_uses unparseable as canonical shape: %v\nraw=%s", err, string(persistedFeatureUses))
	}
	rage := afterDeduct[FeatureKeyRage]
	if rage.Current != 2 {
		t.Fatalf("after rage deduct: Current=%d, want 2", rage.Current)
	}
	if rage.Max != 3 {
		t.Fatalf("after rage deduct: Max=%d, want 3 (combat write must preserve Max)", rage.Max)
	}
	if rage.Recharge != "long" {
		t.Fatalf("after rage deduct: Recharge=%q, want \"long\" (combat write must preserve Recharge)", rage.Recharge)
	}

	// 2. Rest service performs a long rest using the persisted shape.
	restSvc := rest.NewService(dice.NewRoller(func(int) int { return 6 }))
	restResult := restSvc.LongRest(rest.LongRestInput{
		HPCurrent:        10,
		HPMax:            10,
		HitDiceRemaining: map[string]int{"d12": 5},
		Classes:          []character.ClassEntry{{Class: "barbarian", Level: 5}},
		FeatureUses:      afterDeduct,
		SpellSlots:       map[string]character.SlotInfo{},
	})

	rechargedRage := restResult.FeaturesRecharged
	foundRageRecharge := false
	for _, name := range rechargedRage {
		if name == FeatureKeyRage {
			foundRageRecharge = true
			break
		}
	}
	if !foundRageRecharge {
		t.Fatalf("long rest did not recharge rage; recharged=%v", rechargedRage)
	}
	if got := afterDeduct[FeatureKeyRage].Current; got != 3 {
		t.Fatalf("after long rest: rage Current=%d, want 3", got)
	}

	// 3. Persist the rest result and read it back via combat's ParseFeatureUses.
	rechargedJSON, err := json.Marshal(afterDeduct)
	if err != nil {
		t.Fatalf("marshal recharged: %v", err)
	}
	persistedFeatureUses = rechargedJSON

	char, _ := store.getCharacterFn(ctx, charID)
	_, remaining, err := ParseFeatureUses(char, FeatureKeyRage)
	if err != nil {
		t.Fatalf("ParseFeatureUses after rest failed: %v", err)
	}
	if remaining != 3 {
		t.Fatalf("combat read after rest: remaining=%d, want 3", remaining)
	}
}

// Ki round-trip: combat decrements a structured row → long rest re-recharges
// → combat reads decremented-and-recharged value.
func TestFeatureUses_RoundTrip_KiDecrementsAndRecharges(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()

	scores := AbilityScores{Str: 10, Dex: 14, Con: 12, Int: 10, Wis: 16, Cha: 10}
	scoresJSON, _ := json.Marshal(scores)
	classesJSON := json.RawMessage(`[{"class":"Monk","level":5}]`)
	initial := map[string]character.FeatureUse{
		FeatureKeyKi: {Current: 5, Max: 5, Recharge: "short"},
	}
	initialJSON, _ := json.Marshal(initial)

	var persistedFeatureUses []byte
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		raw := initialJSON
		if persistedFeatureUses != nil {
			raw = persistedFeatureUses
		}
		return refdata.Character{
			ID:               charID,
			AbilityScores:    scoresJSON,
			ProficiencyBonus: 3,
			Classes:          classesJSON,
			FeatureUses:      pqtype.NullRawMessage{RawMessage: raw, Valid: true},
		}, nil
	}
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		persistedFeatureUses = append([]byte(nil), arg.FeatureUses.RawMessage...)
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: true}, nil
	}

	svc := NewService(store)

	// Combat: spend 1 ki via PatientDefense.
	_, err := svc.PatientDefense(ctx, KiAbilityCommand{
		Combatant: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Turn: refdata.Turn{ID: uuid.New()},
	})
	if err != nil {
		t.Fatalf("PatientDefense failed: %v", err)
	}

	var afterSpend map[string]character.FeatureUse
	if err := json.Unmarshal(persistedFeatureUses, &afterSpend); err != nil {
		t.Fatalf("ki persisted shape unparseable: %v\nraw=%s", err, string(persistedFeatureUses))
	}
	ki := afterSpend[FeatureKeyKi]
	if ki.Current != 4 {
		t.Fatalf("after ki spend: Current=%d, want 4", ki.Current)
	}
	if ki.Max != 5 || ki.Recharge != "short" {
		t.Fatalf("after ki spend: lost Max/Recharge; got %+v", ki)
	}

	// Rest: short rest should bring ki back to Max.
	restSvc := rest.NewService(dice.NewRoller(func(int) int { return 4 }))
	res, err := restSvc.ShortRest(rest.ShortRestInput{
		HPCurrent:        10,
		HPMax:            10,
		HitDiceRemaining: map[string]int{"d8": 5},
		HitDiceSpend:     map[string]int{},
		FeatureUses:      afterSpend,
		Classes:          []character.ClassEntry{{Class: "monk", Level: 5}},
	})
	if err != nil {
		t.Fatalf("ShortRest failed: %v", err)
	}
	if len(res.FeaturesRecharged) == 0 {
		t.Fatalf("expected ki to recharge on short rest")
	}
	if afterSpend[FeatureKeyKi].Current != 5 {
		t.Fatalf("after short rest: ki Current=%d, want 5", afterSpend[FeatureKeyKi].Current)
	}

	// Persist the recharged state and read back through ParseFeatureUses.
	rechargedJSON, _ := json.Marshal(afterSpend)
	persistedFeatureUses = rechargedJSON

	char, _ := store.getCharacterFn(ctx, charID)
	_, remaining, err := ParseFeatureUses(char, FeatureKeyKi)
	if err != nil {
		t.Fatalf("ParseFeatureUses after rest: %v", err)
	}
	if remaining != 5 {
		t.Fatalf("combat read ki after rest: remaining=%d, want 5", remaining)
	}
}
