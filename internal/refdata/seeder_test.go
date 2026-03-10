package refdata_test

import (
	"context"
	"encoding/json"
	"testing"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/testutil"
)

func setupSeededDB(t *testing.T) *refdata.Queries {
	t.Helper()
	db := testutil.NewMigratedTestDB(t, dbfs.Migrations)
	ctx := context.Background()
	if err := refdata.SeedAll(ctx, db); err != nil {
		t.Fatalf("SeedAll failed: %v", err)
	}
	return refdata.New(db)
}

func TestIntegration_SeedAll_WeaponCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	count, err := q.CountWeapons(context.Background())
	if err != nil {
		t.Fatalf("CountWeapons failed: %v", err)
	}
	if count != refdata.WeaponCount {
		t.Fatalf("expected %d weapons, got %d", refdata.WeaponCount, count)
	}
}

func TestIntegration_SeedAll_ArmorCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	count, err := q.CountArmor(context.Background())
	if err != nil {
		t.Fatalf("CountArmor failed: %v", err)
	}
	if count != refdata.ArmorCount {
		t.Fatalf("expected %d armor pieces, got %d", refdata.ArmorCount, count)
	}
}

func TestIntegration_SeedAll_LongswordHasVersatile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	weapon, err := q.GetWeapon(context.Background(), "longsword")
	if err != nil {
		t.Fatalf("GetWeapon failed: %v", err)
	}

	if weapon.Name != "Longsword" {
		t.Fatalf("expected name Longsword, got %q", weapon.Name)
	}
	if weapon.Damage != "1d8" {
		t.Fatalf("expected damage 1d8, got %q", weapon.Damage)
	}
	if weapon.DamageType != "slashing" {
		t.Fatalf("expected damage_type slashing, got %q", weapon.DamageType)
	}
	if weapon.WeaponType != "martial_melee" {
		t.Fatalf("expected weapon_type martial_melee, got %q", weapon.WeaponType)
	}

	hasVersatile := false
	for _, p := range weapon.Properties {
		if p == "versatile" {
			hasVersatile = true
			break
		}
	}
	if !hasVersatile {
		t.Fatalf("expected longsword to have versatile property, got %v", weapon.Properties)
	}
	if !weapon.VersatileDamage.Valid || weapon.VersatileDamage.String != "1d10" {
		t.Fatalf("expected versatile_damage 1d10, got %v", weapon.VersatileDamage)
	}
}

func TestIntegration_SeedAll_PlateArmor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	armor, err := q.GetArmor(context.Background(), "plate")
	if err != nil {
		t.Fatalf("GetArmor failed: %v", err)
	}

	if armor.Name != "Plate" {
		t.Fatalf("expected name Plate, got %q", armor.Name)
	}
	if armor.AcBase != 18 {
		t.Fatalf("expected ac_base 18, got %d", armor.AcBase)
	}
	if armor.ArmorType != "heavy" {
		t.Fatalf("expected armor_type heavy, got %q", armor.ArmorType)
	}
	if !armor.AcDexBonus.Valid || armor.AcDexBonus.Bool != false {
		t.Fatalf("expected ac_dex_bonus false, got %v", armor.AcDexBonus)
	}
	if !armor.StrengthReq.Valid || armor.StrengthReq.Int32 != 15 {
		t.Fatalf("expected strength_req 15, got %v", armor.StrengthReq)
	}
	if !armor.StealthDisadv.Valid || armor.StealthDisadv.Bool != true {
		t.Fatalf("expected stealth_disadv true, got %v", armor.StealthDisadv)
	}
}

func TestIntegration_SeedAll_StunnedCondition(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	cond, err := q.GetCondition(context.Background(), "stunned")
	if err != nil {
		t.Fatalf("GetCondition failed: %v", err)
	}

	if cond.Name != "Stunned" {
		t.Fatalf("expected name Stunned, got %q", cond.Name)
	}

	var effects []map[string]any
	if err := json.Unmarshal(cond.MechanicalEffects, &effects); err != nil {
		t.Fatalf("failed to unmarshal mechanical_effects: %v", err)
	}
	if len(effects) == 0 {
		t.Fatal("expected non-empty mechanical_effects")
	}

	hasIncapacitated := false
	for _, e := range effects {
		if e["effect_type"] == "incapacitated" {
			hasIncapacitated = true
			break
		}
	}
	if !hasIncapacitated {
		t.Fatal("expected stunned to have incapacitated effect")
	}
}

func TestIntegration_SeedAll_ConditionCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	count, err := q.CountConditions(context.Background())
	if err != nil {
		t.Fatalf("CountConditions failed: %v", err)
	}
	if count != refdata.ConditionCount {
		t.Fatalf("expected %d conditions, got %d", refdata.ConditionCount, count)
	}
}

func TestIntegration_SeedAll_ClassCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	count, err := q.CountClasses(context.Background())
	if err != nil {
		t.Fatalf("CountClasses failed: %v", err)
	}
	if count != refdata.ClassCount {
		t.Fatalf("expected %d classes, got %d", refdata.ClassCount, count)
	}
}

func TestIntegration_SeedAll_RaceCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	count, err := q.CountRaces(context.Background())
	if err != nil {
		t.Fatalf("CountRaces failed: %v", err)
	}
	if count != refdata.RaceCount {
		t.Fatalf("expected %d races, got %d", refdata.RaceCount, count)
	}
}

func TestIntegration_SeedAll_FeatCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	count, err := q.CountFeats(context.Background())
	if err != nil {
		t.Fatalf("CountFeats failed: %v", err)
	}
	if count != refdata.FeatCount {
		t.Fatalf("expected %d feats, got %d", refdata.FeatCount, count)
	}
}

func TestIntegration_SeedAll_FighterClass(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	class, err := q.GetClass(context.Background(), "fighter")
	if err != nil {
		t.Fatalf("GetClass failed: %v", err)
	}

	if class.Name != "Fighter" {
		t.Fatalf("expected name Fighter, got %q", class.Name)
	}
	if class.HitDie != "d10" {
		t.Fatalf("expected hit_die d10, got %q", class.HitDie)
	}
	if class.PrimaryAbility != "str" {
		t.Fatalf("expected primary_ability str, got %q", class.PrimaryAbility)
	}

	// Check attacks_per_action has multiple attack tiers
	var attacks map[string]int
	if err := json.Unmarshal(class.AttacksPerAction, &attacks); err != nil {
		t.Fatalf("failed to unmarshal attacks_per_action: %v", err)
	}
	if attacks["5"] != 2 {
		t.Fatalf("expected fighter to get 2 attacks at level 5, got %d", attacks["5"])
	}
	if attacks["11"] != 3 {
		t.Fatalf("expected fighter to get 3 attacks at level 11, got %d", attacks["11"])
	}

	// Check subclass_level
	if class.SubclassLevel != 3 {
		t.Fatalf("expected subclass_level 3, got %d", class.SubclassLevel)
	}

	// Check Champion subclass exists
	var subclasses map[string]any
	if err := json.Unmarshal(class.Subclasses, &subclasses); err != nil {
		t.Fatalf("failed to unmarshal subclasses: %v", err)
	}
	if _, ok := subclasses["champion"]; !ok {
		t.Fatal("expected Champion subclass to exist")
	}
}

func TestIntegration_SeedAll_WizardSpellcasting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	class, err := q.GetClass(context.Background(), "wizard")
	if err != nil {
		t.Fatalf("GetClass failed: %v", err)
	}

	if !class.Spellcasting.Valid {
		t.Fatal("expected wizard to have spellcasting")
	}

	var sc map[string]any
	if err := json.Unmarshal(class.Spellcasting.RawMessage, &sc); err != nil {
		t.Fatalf("failed to unmarshal spellcasting: %v", err)
	}
	if sc["ability"] != "int" {
		t.Fatalf("expected spellcasting ability int, got %v", sc["ability"])
	}
	if sc["slot_progression"] != "full" {
		t.Fatalf("expected slot_progression full, got %v", sc["slot_progression"])
	}
}

func TestIntegration_SeedAll_ElfRace(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	race, err := q.GetRace(context.Background(), "elf")
	if err != nil {
		t.Fatalf("GetRace failed: %v", err)
	}

	if race.Name != "Elf" {
		t.Fatalf("expected name Elf, got %q", race.Name)
	}
	if race.SpeedFt != 30 {
		t.Fatalf("expected speed_ft 30, got %d", race.SpeedFt)
	}
	if race.DarkvisionFt != 60 {
		t.Fatalf("expected darkvision_ft 60, got %d", race.DarkvisionFt)
	}

	// Check ability_bonuses
	var bonuses map[string]any
	if err := json.Unmarshal(race.AbilityBonuses, &bonuses); err != nil {
		t.Fatalf("failed to unmarshal ability_bonuses: %v", err)
	}
	if bonuses["dex"] != float64(2) {
		t.Fatalf("expected dex bonus 2, got %v", bonuses["dex"])
	}

	// Check subraces
	if !race.Subraces.Valid {
		t.Fatal("expected elf to have subraces")
	}
	var subraces []map[string]any
	if err := json.Unmarshal(race.Subraces.RawMessage, &subraces); err != nil {
		t.Fatalf("failed to unmarshal subraces: %v", err)
	}
	if len(subraces) == 0 {
		t.Fatal("expected at least one subrace")
	}
	if subraces[0]["id"] != "high-elf" {
		t.Fatalf("expected first subrace id high-elf, got %v", subraces[0]["id"])
	}
}

func TestIntegration_SeedAll_GreatWeaponMasterFeat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	feat, err := q.GetFeat(context.Background(), "great-weapon-master")
	if err != nil {
		t.Fatalf("GetFeat failed: %v", err)
	}

	if feat.Name != "Great Weapon Master" {
		t.Fatalf("expected name Great Weapon Master, got %q", feat.Name)
	}
	if feat.Description == "" {
		t.Fatal("expected non-empty description")
	}
}

func TestIntegration_SeedAll_ListClasses(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	classes, err := q.ListClasses(context.Background())
	if err != nil {
		t.Fatalf("ListClasses failed: %v", err)
	}
	if len(classes) != refdata.ClassCount {
		t.Fatalf("expected %d classes from list, got %d", refdata.ClassCount, len(classes))
	}
	// Verify sorted by name (first should be Barbarian)
	if classes[0].Name != "Barbarian" {
		t.Fatalf("expected first class Barbarian, got %q", classes[0].Name)
	}
}

func TestIntegration_SeedAll_ListRaces(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	races, err := q.ListRaces(context.Background())
	if err != nil {
		t.Fatalf("ListRaces failed: %v", err)
	}
	if len(races) != refdata.RaceCount {
		t.Fatalf("expected %d races from list, got %d", refdata.RaceCount, len(races))
	}
	// Verify sorted by name (first should be Dragonborn)
	if races[0].Name != "Dragonborn" {
		t.Fatalf("expected first race Dragonborn, got %q", races[0].Name)
	}
}

func TestIntegration_SeedAll_ListFeats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	feats, err := q.ListFeats(context.Background())
	if err != nil {
		t.Fatalf("ListFeats failed: %v", err)
	}
	if len(feats) != refdata.FeatCount {
		t.Fatalf("expected %d feats from list, got %d", refdata.FeatCount, len(feats))
	}
	// Verify sorted by name (first should be Alert)
	if feats[0].Name != "Alert" {
		t.Fatalf("expected first feat Alert, got %q", feats[0].Name)
	}
}

func TestIntegration_SeedAll_SpellCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	count, err := q.CountSpells(context.Background())
	if err != nil {
		t.Fatalf("CountSpells failed: %v", err)
	}
	if count != int64(refdata.SpellCount) {
		t.Fatalf("expected %d spells, got %d", refdata.SpellCount, count)
	}
}

func TestIntegration_SeedAll_FireballSpell(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	spell, err := q.GetSpell(context.Background(), "fireball")
	if err != nil {
		t.Fatalf("GetSpell failed: %v", err)
	}

	if spell.Name != "Fireball" {
		t.Fatalf("expected name Fireball, got %q", spell.Name)
	}
	if spell.Level != 3 {
		t.Fatalf("expected level 3, got %d", spell.Level)
	}
	if spell.School != "evocation" {
		t.Fatalf("expected school evocation, got %q", spell.School)
	}
	if spell.ResolutionMode != "auto" {
		t.Fatalf("expected resolution_mode auto, got %q", spell.ResolutionMode)
	}
	if spell.CastingTime != "1 action" {
		t.Fatalf("expected casting_time '1 action', got %q", spell.CastingTime)
	}
	if !spell.RangeFt.Valid || spell.RangeFt.Int32 != 150 {
		t.Fatalf("expected range_ft 150, got %v", spell.RangeFt)
	}

	// Check damage JSON
	var damage map[string]any
	if err := json.Unmarshal(spell.Damage.RawMessage, &damage); err != nil {
		t.Fatalf("failed to unmarshal damage: %v", err)
	}
	if damage["dice"] != "8d6" {
		t.Fatalf("expected damage dice 8d6, got %v", damage["dice"])
	}
	if damage["type"] != "fire" {
		t.Fatalf("expected damage type fire, got %v", damage["type"])
	}

	// Check area_of_effect
	var aoe map[string]any
	if err := json.Unmarshal(spell.AreaOfEffect.RawMessage, &aoe); err != nil {
		t.Fatalf("failed to unmarshal area_of_effect: %v", err)
	}
	if aoe["shape"] != "sphere" {
		t.Fatalf("expected aoe shape sphere, got %v", aoe["shape"])
	}

	// Check save
	if !spell.SaveAbility.Valid || spell.SaveAbility.String != "dex" {
		t.Fatalf("expected save_ability dex, got %v", spell.SaveAbility)
	}
	if !spell.SaveEffect.Valid || spell.SaveEffect.String != "half_damage" {
		t.Fatalf("expected save_effect half_damage, got %v", spell.SaveEffect)
	}

	// Check components
	hasV, hasS, hasM := false, false, false
	for _, c := range spell.Components {
		switch c {
		case "V":
			hasV = true
		case "S":
			hasS = true
		case "M":
			hasM = true
		}
	}
	if !hasV || !hasS || !hasM {
		t.Fatalf("expected V,S,M components, got %v", spell.Components)
	}

	// Check classes
	hasWizard, hasSorcerer := false, false
	for _, c := range spell.Classes {
		switch c {
		case "wizard":
			hasWizard = true
		case "sorcerer":
			hasSorcerer = true
		}
	}
	if !hasWizard || !hasSorcerer {
		t.Fatalf("expected wizard and sorcerer in classes, got %v", spell.Classes)
	}
}

func TestIntegration_SeedAll_CureWoundsHealing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	spell, err := q.GetSpell(context.Background(), "cure-wounds")
	if err != nil {
		t.Fatalf("GetSpell failed: %v", err)
	}

	if spell.ResolutionMode != "auto" {
		t.Fatalf("expected resolution_mode auto, got %q", spell.ResolutionMode)
	}

	if !spell.Healing.Valid {
		t.Fatal("expected healing to be set")
	}
	var healing map[string]any
	if err := json.Unmarshal(spell.Healing.RawMessage, &healing); err != nil {
		t.Fatalf("failed to unmarshal healing: %v", err)
	}
	if healing["dice"] != "1d8+mod" {
		t.Fatalf("expected healing dice 1d8+mod, got %v", healing["dice"])
	}
}

func TestIntegration_SeedAll_MistyStepTeleport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	spell, err := q.GetSpell(context.Background(), "misty-step")
	if err != nil {
		t.Fatalf("GetSpell failed: %v", err)
	}

	if spell.ResolutionMode != "auto" {
		t.Fatalf("expected resolution_mode auto, got %q", spell.ResolutionMode)
	}

	if !spell.Teleport.Valid {
		t.Fatal("expected teleport to be set")
	}
	var teleport map[string]any
	if err := json.Unmarshal(spell.Teleport.RawMessage, &teleport); err != nil {
		t.Fatalf("failed to unmarshal teleport: %v", err)
	}
	if teleport["target"] != "self" {
		t.Fatalf("expected teleport target self, got %v", teleport["target"])
	}
}

func TestIntegration_SeedAll_WishIsDmRequired(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	spell, err := q.GetSpell(context.Background(), "wish")
	if err != nil {
		t.Fatalf("GetSpell failed: %v", err)
	}

	if spell.ResolutionMode != "dm_required" {
		t.Fatalf("expected wish to be dm_required, got %q", spell.ResolutionMode)
	}
}

func TestIntegration_SeedAll_PolymorphIsDmRequired(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	spell, err := q.GetSpell(context.Background(), "polymorph")
	if err != nil {
		t.Fatalf("GetSpell failed: %v", err)
	}

	if spell.ResolutionMode != "dm_required" {
		t.Fatalf("expected polymorph to be dm_required, got %q", spell.ResolutionMode)
	}
}

func TestIntegration_SeedAll_ListSpells(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	spells, err := q.ListSpells(context.Background())
	if err != nil {
		t.Fatalf("ListSpells failed: %v", err)
	}
	if len(spells) != refdata.SpellCount {
		t.Fatalf("expected %d spells from list, got %d", refdata.SpellCount, len(spells))
	}
}

func TestIntegration_SeedAll_ListSpellsByClass(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	spells, err := q.ListSpellsByClass(context.Background(), "wizard")
	if err != nil {
		t.Fatalf("ListSpellsByClass failed: %v", err)
	}
	if len(spells) == 0 {
		t.Fatal("expected at least one wizard spell")
	}
	// All returned spells should have wizard in their classes
	for _, s := range spells {
		hasWizard := false
		for _, c := range s.Classes {
			if c == "wizard" {
				hasWizard = true
				break
			}
		}
		if !hasWizard {
			t.Fatalf("spell %q does not have wizard in classes: %v", s.ID, s.Classes)
		}
	}
}

func TestIntegration_SeedAll_ListSpellsByLevel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	cantrips, err := q.ListSpellsByLevel(context.Background(), 0)
	if err != nil {
		t.Fatalf("ListSpellsByLevel failed: %v", err)
	}
	if len(cantrips) == 0 {
		t.Fatal("expected at least one cantrip")
	}
	for _, s := range cantrips {
		if s.Level != 0 {
			t.Fatalf("expected level 0, got %d for %q", s.Level, s.ID)
		}
	}
}

func TestIntegration_SeedAll_ListSpellsByResolutionMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	autoSpells, err := q.ListSpellsByResolutionMode(context.Background(), "auto")
	if err != nil {
		t.Fatalf("ListSpellsByResolutionMode failed: %v", err)
	}
	if len(autoSpells) == 0 {
		t.Fatal("expected at least one auto spell")
	}

	dmSpells, err := q.ListSpellsByResolutionMode(context.Background(), "dm_required")
	if err != nil {
		t.Fatalf("ListSpellsByResolutionMode failed: %v", err)
	}
	if len(dmSpells) == 0 {
		t.Fatal("expected at least one dm_required spell")
	}

	// Total should match
	total := int64(len(autoSpells) + len(dmSpells))
	if total != int64(refdata.SpellCount) {
		t.Fatalf("auto(%d) + dm_required(%d) = %d, expected %d", len(autoSpells), len(dmSpells), total, refdata.SpellCount)
	}
}

func TestIntegration_SeedAll_ListSpellsBySchool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	spells, err := q.ListSpellsBySchool(context.Background(), "evocation")
	if err != nil {
		t.Fatalf("ListSpellsBySchool failed: %v", err)
	}
	if len(spells) == 0 {
		t.Fatal("expected at least one evocation spell")
	}
	for _, s := range spells {
		if s.School != "evocation" {
			t.Fatalf("expected school evocation, got %q for %q", s.School, s.ID)
		}
	}
}

func TestIntegration_SeedAll_HoldPersonConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	spell, err := q.GetSpell(context.Background(), "hold-person")
	if err != nil {
		t.Fatalf("GetSpell failed: %v", err)
	}

	hasParalyzed := false
	for _, c := range spell.ConditionsApplied {
		if c == "paralyzed" {
			hasParalyzed = true
			break
		}
	}
	if !hasParalyzed {
		t.Fatalf("expected hold-person to apply paralyzed condition, got %v", spell.ConditionsApplied)
	}
}

func TestIntegration_SeedAll_FindFamiliarMaterialCost(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	spell, err := q.GetSpell(context.Background(), "find-familiar")
	if err != nil {
		t.Fatalf("GetSpell failed: %v", err)
	}

	if !spell.MaterialCostGp.Valid || spell.MaterialCostGp.Float64 != 10 {
		t.Fatalf("expected material_cost_gp 10, got %v", spell.MaterialCostGp)
	}
	if !spell.MaterialConsumed.Valid || !spell.MaterialConsumed.Bool {
		t.Fatalf("expected material_consumed true, got %v", spell.MaterialConsumed)
	}
	if !spell.Ritual.Valid || !spell.Ritual.Bool {
		t.Fatal("expected find-familiar to be ritual")
	}
}

func TestIntegration_SeedAll_Idempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewMigratedTestDB(t, dbfs.Migrations)
	ctx := context.Background()

	// Seed twice
	if err := refdata.SeedAll(ctx, db); err != nil {
		t.Fatalf("first SeedAll failed: %v", err)
	}
	if err := refdata.SeedAll(ctx, db); err != nil {
		t.Fatalf("second SeedAll failed: %v", err)
	}

	q := refdata.New(db)

	weaponCount, err := q.CountWeapons(ctx)
	if err != nil {
		t.Fatalf("CountWeapons failed: %v", err)
	}
	if weaponCount != refdata.WeaponCount {
		t.Fatalf("expected %d weapons after double seed, got %d", refdata.WeaponCount, weaponCount)
	}

	armorCount, err := q.CountArmor(ctx)
	if err != nil {
		t.Fatalf("CountArmor failed: %v", err)
	}
	if armorCount != refdata.ArmorCount {
		t.Fatalf("expected %d armor after double seed, got %d", refdata.ArmorCount, armorCount)
	}

	condCount, err := q.CountConditions(ctx)
	if err != nil {
		t.Fatalf("CountConditions failed: %v", err)
	}
	if condCount != refdata.ConditionCount {
		t.Fatalf("expected %d conditions after double seed, got %d", refdata.ConditionCount, condCount)
	}

	classCount, err := q.CountClasses(ctx)
	if err != nil {
		t.Fatalf("CountClasses failed: %v", err)
	}
	if classCount != refdata.ClassCount {
		t.Fatalf("expected %d classes after double seed, got %d", refdata.ClassCount, classCount)
	}

	raceCount, err := q.CountRaces(ctx)
	if err != nil {
		t.Fatalf("CountRaces failed: %v", err)
	}
	if raceCount != refdata.RaceCount {
		t.Fatalf("expected %d races after double seed, got %d", refdata.RaceCount, raceCount)
	}

	featCount, err := q.CountFeats(ctx)
	if err != nil {
		t.Fatalf("CountFeats failed: %v", err)
	}
	if featCount != refdata.FeatCount {
		t.Fatalf("expected %d feats after double seed, got %d", refdata.FeatCount, featCount)
	}

	spellCount, err := q.CountSpells(ctx)
	if err != nil {
		t.Fatalf("CountSpells failed: %v", err)
	}
	if spellCount != int64(refdata.SpellCount) {
		t.Fatalf("expected %d spells after double seed, got %d", refdata.SpellCount, spellCount)
	}
}

func TestIntegration_SeedAll_LongbowRange(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	weapon, err := q.GetWeapon(context.Background(), "longbow")
	if err != nil {
		t.Fatalf("GetWeapon failed: %v", err)
	}

	if !weapon.RangeNormalFt.Valid || weapon.RangeNormalFt.Int32 != 150 {
		t.Fatalf("expected longbow range_normal_ft 150, got %v", weapon.RangeNormalFt)
	}
	if !weapon.RangeLongFt.Valid || weapon.RangeLongFt.Int32 != 600 {
		t.Fatalf("expected longbow range_long_ft 600, got %v", weapon.RangeLongFt)
	}
	if weapon.WeaponType != "martial_ranged" {
		t.Fatalf("expected weapon_type martial_ranged, got %q", weapon.WeaponType)
	}
}

func TestIntegration_SeedAll_ShieldArmor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	armor, err := q.GetArmor(context.Background(), "shield")
	if err != nil {
		t.Fatalf("GetArmor failed: %v", err)
	}

	if armor.AcBase != 2 {
		t.Fatalf("expected shield ac_base 2, got %d", armor.AcBase)
	}
	if armor.ArmorType != "shield" {
		t.Fatalf("expected armor_type shield, got %q", armor.ArmorType)
	}
}

func TestIntegration_SeedAll_ListWeapons(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	weapons, err := q.ListWeapons(context.Background())
	if err != nil {
		t.Fatalf("ListWeapons failed: %v", err)
	}

	if len(weapons) != refdata.WeaponCount {
		t.Fatalf("expected %d weapons from list, got %d", refdata.WeaponCount, len(weapons))
	}

	// Verify sorted by name (first should be Battleaxe)
	if weapons[0].Name != "Battleaxe" {
		t.Fatalf("expected first weapon Battleaxe, got %q", weapons[0].Name)
	}
}

func TestIntegration_SeedAll_ListConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	conditions, err := q.ListConditions(context.Background())
	if err != nil {
		t.Fatalf("ListConditions failed: %v", err)
	}

	if len(conditions) != refdata.ConditionCount {
		t.Fatalf("expected %d conditions from list, got %d", refdata.ConditionCount, len(conditions))
	}

	// Verify sorted by name (first should be Blinded)
	if conditions[0].Name != "Blinded" {
		t.Fatalf("expected first condition Blinded, got %q", conditions[0].Name)
	}
}

func TestIntegration_SeedAll_ListArmor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q := setupSeededDB(t)
	armorList, err := q.ListArmor(context.Background())
	if err != nil {
		t.Fatalf("ListArmor failed: %v", err)
	}

	if len(armorList) != refdata.ArmorCount {
		t.Fatalf("expected %d armor from list, got %d", refdata.ArmorCount, len(armorList))
	}

	// Verify sorted by name (first should be Breastplate)
	if armorList[0].Name != "Breastplate" {
		t.Fatalf("expected first armor Breastplate, got %q", armorList[0].Name)
	}
}
