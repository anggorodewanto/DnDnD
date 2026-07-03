package combat

import (
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/dice"
)

// maxRoller returns a deterministic roller where every die comes up its maximum
// (a d6 → 6, a d8 → 8), so damage totals are exact and assertable.
func maxRoller() *dice.Roller {
	return dice.NewRoller(func(sides int) int { return sides })
}

func TestBuildFESDamageBreakdown_FlatModAttributed(t *testing.T) {
	effects := []ResolvedEffect{
		{FeatureName: "Great Weapon Master", Effect: Effect{Type: EffectModifyDamageRoll, Trigger: TriggerOnDamageRoll, Modifier: 3}},
	}
	extra, comps := buildFESDamageBreakdown(effects, false, maxRoller())
	if extra != 0 {
		t.Fatalf("flat mod must not add to extraDiceTotal (already folded into weapon roll), got %d", extra)
	}
	if len(comps) != 1 || comps[0].SourceName != "Great Weapon Master" || comps[0].Amount != 3 || comps[0].DamageType != "" {
		t.Fatalf("unexpected components: %+v", comps)
	}
}

func TestBuildFESDamageBreakdown_ExtraDiceTypedAttributed(t *testing.T) {
	effects := []ResolvedEffect{
		{FeatureName: "Hex", Effect: Effect{Type: EffectExtraDamageDice, Trigger: TriggerOnDamageRoll, Dice: "1d6", DamageTypes: []string{"necrotic"}}},
	}
	extra, comps := buildFESDamageBreakdown(effects, false, maxRoller())
	if extra != 6 {
		t.Fatalf("extraDiceTotal = %d, want 6 (1d6 max)", extra)
	}
	if len(comps) != 1 || comps[0].SourceName != "Hex" || comps[0].Amount != 6 || comps[0].DamageType != "necrotic" {
		t.Fatalf("unexpected components: %+v", comps)
	}
}

func TestBuildFESDamageBreakdown_CritDoublesDiceNotFlat(t *testing.T) {
	effects := []ResolvedEffect{
		{FeatureName: "Hex", Effect: Effect{Type: EffectExtraDamageDice, Trigger: TriggerOnDamageRoll, Dice: "1d6", DamageTypes: []string{"necrotic"}}},
		{FeatureName: "Great Weapon Master", Effect: Effect{Type: EffectModifyDamageRoll, Trigger: TriggerOnDamageRoll, Modifier: 3}},
	}
	extra, comps := buildFESDamageBreakdown(effects, true, maxRoller())
	if extra != 12 {
		t.Fatalf("crit dice extraDiceTotal = %d, want 12 (2d6 max)", extra)
	}
	var flat *DamageComponent
	for i := range comps {
		if comps[i].SourceName == "Great Weapon Master" {
			flat = &comps[i]
		}
	}
	if flat == nil || flat.Amount != 3 {
		t.Fatalf("flat GWM component must stay 3 on crit (mods never doubled), got %+v", flat)
	}
}

func TestBuildFESDamageBreakdown_TotalMatchesLegacySum(t *testing.T) {
	effects := []ResolvedEffect{
		{FeatureName: "Sneak Attack", Effect: Effect{Type: EffectExtraDamageDice, Dice: "2d6"}},
		{FeatureName: "Flame Tongue", Effect: Effect{Type: EffectExtraDamageDice, Dice: "1d6", DamageTypes: []string{"fire"}}},
	}
	extra, _ := buildFESDamageBreakdown(effects, false, maxRoller())

	// Legacy behavior: roll each extra-dice expression in order and sum.
	legacy := 0
	lr := maxRoller()
	for _, e := range []string{"2d6", "1d6"} {
		r, _ := lr.RollDamage(e, false)
		legacy += r.Total
	}
	if extra != legacy {
		t.Fatalf("breakdown extra %d != legacy blind sum %d (roll-order regression)", extra, legacy)
	}
}

func TestFormatAttackLog_RendersRiderCallouts(t *testing.T) {
	result := AttackResult{
		AttackerName: "Vale", TargetName: "Grix", WeaponName: "Warhammer",
		Hit: true, IsMelee: true, DamageTotal: 15, DamageType: "bludgeoning", DamageDice: "1d8+3",
		D20Roll: dice.D20Result{Total: 18, Chosen: 15, Modifier: 3},
		DamageBreakdown: []DamageComponent{
			{SourceName: "Hex", Amount: 4, DamageType: "necrotic"},
			{SourceName: "Great Weapon Master", Amount: 3},
		},
	}
	out := FormatAttackLog(result)
	if !strings.Contains(out, "Hex") || !strings.Contains(out, "necrotic") {
		t.Fatalf("expected Hex necrotic callout, got:\n%s", out)
	}
	if !strings.Contains(out, "Great Weapon Master") {
		t.Fatalf("expected Great Weapon Master callout, got:\n%s", out)
	}
}

func TestFormatAttackLog_DoesNotDoublePrintSneakAttack(t *testing.T) {
	result := AttackResult{
		AttackerName: "Rook", TargetName: "Grix", WeaponName: "Dagger",
		Hit: true, IsMelee: true, DamageTotal: 12, DamageType: "piercing", DamageDice: "1d4+3",
		D20Roll:                dice.D20Result{Total: 19, Chosen: 16, Modifier: 3},
		OncePerTurnEffectNames: []string{"Sneak Attack"},
		DamageBreakdown:        []DamageComponent{{SourceName: "Sneak Attack", Amount: 7}},
	}
	out := FormatAttackLog(result)
	if got := strings.Count(out, "Sneak Attack"); got != 1 {
		t.Fatalf("Sneak Attack must appear exactly once (via sneakAttackTag, not the breakdown), got %d:\n%s", got, out)
	}
}

func TestDescribeAttack_AppendsBreakdownSuffix(t *testing.T) {
	r := AttackResult{
		AttackerName: "Vale", TargetName: "Grix", WeaponName: "Warhammer",
		Hit: true, DamageTotal: 15,
		DamageBreakdown: []DamageComponent{{SourceName: "Hex", Amount: 4, DamageType: "necrotic"}},
	}
	got := describeAttack(r)
	if !strings.Contains(got, "incl.") || !strings.Contains(got, "Hex") {
		t.Fatalf("expected breakdown suffix with 'incl.' and 'Hex', got %q", got)
	}
}
