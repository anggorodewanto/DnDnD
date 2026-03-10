package refdata_test

import (
	"context"
	"encoding/json"
	"testing"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/database"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/testutil"
)

func TestIntegration_SeedAll_WeaponCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)
	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	ctx := context.Background()
	if err := refdata.SeedAll(ctx, db); err != nil {
		t.Fatalf("SeedAll failed: %v", err)
	}

	q := refdata.New(db)
	count, err := q.CountWeapons(ctx)
	if err != nil {
		t.Fatalf("CountWeapons failed: %v", err)
	}
	if count != 37 {
		t.Fatalf("expected 37 weapons, got %d", count)
	}
}

func TestIntegration_SeedAll_ArmorCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)
	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	ctx := context.Background()
	if err := refdata.SeedAll(ctx, db); err != nil {
		t.Fatalf("SeedAll failed: %v", err)
	}

	q := refdata.New(db)
	count, err := q.CountArmor(ctx)
	if err != nil {
		t.Fatalf("CountArmor failed: %v", err)
	}
	if count != 13 {
		t.Fatalf("expected 13 armor pieces, got %d", count)
	}
}

func TestIntegration_SeedAll_LongswordHasVersatile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)
	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	ctx := context.Background()
	if err := refdata.SeedAll(ctx, db); err != nil {
		t.Fatalf("SeedAll failed: %v", err)
	}

	q := refdata.New(db)
	weapon, err := q.GetWeapon(ctx, "longsword")
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

	db := testutil.NewTestDB(t)
	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	ctx := context.Background()
	if err := refdata.SeedAll(ctx, db); err != nil {
		t.Fatalf("SeedAll failed: %v", err)
	}

	q := refdata.New(db)
	armor, err := q.GetArmor(ctx, "plate")
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

	db := testutil.NewTestDB(t)
	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	ctx := context.Background()
	if err := refdata.SeedAll(ctx, db); err != nil {
		t.Fatalf("SeedAll failed: %v", err)
	}

	q := refdata.New(db)
	cond, err := q.GetCondition(ctx, "stunned")
	if err != nil {
		t.Fatalf("GetCondition failed: %v", err)
	}

	if cond.Name != "Stunned" {
		t.Fatalf("expected name Stunned, got %q", cond.Name)
	}

	var effects []map[string]interface{}
	if err := json.Unmarshal(cond.MechanicalEffects, &effects); err != nil {
		t.Fatalf("failed to unmarshal mechanical_effects: %v", err)
	}
	if len(effects) == 0 {
		t.Fatal("expected non-empty mechanical_effects")
	}

	// Check that incapacitated effect is present
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

	db := testutil.NewTestDB(t)
	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	ctx := context.Background()
	if err := refdata.SeedAll(ctx, db); err != nil {
		t.Fatalf("SeedAll failed: %v", err)
	}

	q := refdata.New(db)
	count, err := q.CountConditions(ctx)
	if err != nil {
		t.Fatalf("CountConditions failed: %v", err)
	}
	if count != 15 {
		t.Fatalf("expected 15 conditions, got %d", count)
	}
}

func TestIntegration_SeedAll_Idempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)
	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

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
	if weaponCount != 37 {
		t.Fatalf("expected 37 weapons after double seed, got %d", weaponCount)
	}

	armorCount, err := q.CountArmor(ctx)
	if err != nil {
		t.Fatalf("CountArmor failed: %v", err)
	}
	if armorCount != 13 {
		t.Fatalf("expected 13 armor after double seed, got %d", armorCount)
	}

	condCount, err := q.CountConditions(ctx)
	if err != nil {
		t.Fatalf("CountConditions failed: %v", err)
	}
	if condCount != 15 {
		t.Fatalf("expected 15 conditions after double seed, got %d", condCount)
	}
}

func TestIntegration_SeedAll_LongbowRange(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)
	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	ctx := context.Background()
	if err := refdata.SeedAll(ctx, db); err != nil {
		t.Fatalf("SeedAll failed: %v", err)
	}

	q := refdata.New(db)
	weapon, err := q.GetWeapon(ctx, "longbow")
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

	db := testutil.NewTestDB(t)
	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	ctx := context.Background()
	if err := refdata.SeedAll(ctx, db); err != nil {
		t.Fatalf("SeedAll failed: %v", err)
	}

	q := refdata.New(db)
	armor, err := q.GetArmor(ctx, "shield")
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

	db := testutil.NewTestDB(t)
	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	ctx := context.Background()
	if err := refdata.SeedAll(ctx, db); err != nil {
		t.Fatalf("SeedAll failed: %v", err)
	}

	q := refdata.New(db)
	weapons, err := q.ListWeapons(ctx)
	if err != nil {
		t.Fatalf("ListWeapons failed: %v", err)
	}

	if len(weapons) != 37 {
		t.Fatalf("expected 37 weapons from list, got %d", len(weapons))
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

	db := testutil.NewTestDB(t)
	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	ctx := context.Background()
	if err := refdata.SeedAll(ctx, db); err != nil {
		t.Fatalf("SeedAll failed: %v", err)
	}

	q := refdata.New(db)
	conditions, err := q.ListConditions(ctx)
	if err != nil {
		t.Fatalf("ListConditions failed: %v", err)
	}

	if len(conditions) != 15 {
		t.Fatalf("expected 15 conditions from list, got %d", len(conditions))
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

	db := testutil.NewTestDB(t)
	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	ctx := context.Background()
	if err := refdata.SeedAll(ctx, db); err != nil {
		t.Fatalf("SeedAll failed: %v", err)
	}

	q := refdata.New(db)
	armorList, err := q.ListArmor(ctx)
	if err != nil {
		t.Fatalf("ListArmor failed: %v", err)
	}

	if len(armorList) != 13 {
		t.Fatalf("expected 13 armor from list, got %d", len(armorList))
	}

	// Verify sorted by name (first should be Breastplate)
	if armorList[0].Name != "Breastplate" {
		t.Fatalf("expected first armor Breastplate, got %q", armorList[0].Name)
	}
}

func TestIntegration_SeedAll_NilDB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	err := refdata.SeedAll(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil db, got nil")
	}
}

func TestIntegration_WithTx(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)
	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	ctx := context.Background()
	if err := refdata.SeedAll(ctx, db); err != nil {
		t.Fatalf("SeedAll failed: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}
	defer tx.Rollback()

	q := refdata.New(db).WithTx(tx)
	count, err := q.CountWeapons(ctx)
	if err != nil {
		t.Fatalf("CountWeapons in tx failed: %v", err)
	}
	if count != 37 {
		t.Fatalf("expected 37 weapons in tx, got %d", count)
	}
}
