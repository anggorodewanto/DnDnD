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
