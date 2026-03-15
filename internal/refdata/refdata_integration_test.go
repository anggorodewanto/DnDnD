package refdata_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/ab/dndnd/internal/refdata"
)

func TestIntegration_ReferenceTablesMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := sharedDB.AcquireDB(t)

	// Clean up test rows from reference tables after this test so they
	// don't pollute count-based assertions in other tests.
	type refRow struct{ table, id string }
	testRefs := []refRow{
		{"weapons", "test-sword"},
		{"armor", "test-armor"},
		{"conditions_ref", "test-condition"},
		{"classes", "test-class"},
		{"races", "test-race"},
		{"feats", "test-feat"},
		{"spells", "test-spell"},
		{"creatures", "test-creature"},
		{"magic_items", "test-magic-item"},
	}
	t.Cleanup(func() {
		for _, ref := range testRefs {
			if _, err := db.Exec("DELETE FROM "+ref.table+" WHERE id = $1", ref.id); err != nil {
				t.Logf("cleanup %s/%s: %v", ref.table, ref.id, err)
			}
		}
	})

	// Verify weapons table exists
	_, err := db.Exec(`INSERT INTO weapons (id, name, damage, damage_type, weapon_type) VALUES ($1, $2, $3, $4, $5)`,
		"test-sword", "Test Sword", "1d6", "slashing", "simple_melee")
	if err != nil {
		t.Fatalf("weapons table should exist: %v", err)
	}

	// Verify armor table exists
	_, err = db.Exec(`INSERT INTO armor (id, name, ac_base, armor_type) VALUES ($1, $2, $3, $4)`,
		"test-armor", "Test Armor", 12, "light")
	if err != nil {
		t.Fatalf("armor table should exist: %v", err)
	}

	// Verify conditions_ref table exists
	_, err = db.Exec(`INSERT INTO conditions_ref (id, name, description, mechanical_effects) VALUES ($1, $2, $3, $4)`,
		"test-condition", "Test Condition", "A test", `[{"effect_type": "test"}]`)
	if err != nil {
		t.Fatalf("conditions_ref table should exist: %v", err)
	}

	// Verify classes table exists
	_, err = db.Exec(`INSERT INTO classes (id, name, hit_die, primary_ability, save_proficiencies, features_by_level, attacks_per_action, subclass_level, subclasses) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		"test-class", "Test Class", "d10", "str", `{str,con}`, `{"1":[]}`, `{"1":1}`, 3, `{}`)
	if err != nil {
		t.Fatalf("classes table should exist: %v", err)
	}

	// Verify races table exists
	_, err = db.Exec(`INSERT INTO races (id, name, speed_ft, size, ability_bonuses, traits) VALUES ($1, $2, $3, $4, $5, $6)`,
		"test-race", "Test Race", 30, "Medium", `{"str":1}`, `[]`)
	if err != nil {
		t.Fatalf("races table should exist: %v", err)
	}

	// Verify feats table exists
	_, err = db.Exec(`INSERT INTO feats (id, name, description) VALUES ($1, $2, $3)`,
		"test-feat", "Test Feat", "A test feat")
	if err != nil {
		t.Fatalf("feats table should exist: %v", err)
	}

	// Verify spells table exists
	_, err = db.Exec(`INSERT INTO spells (id, name, level, school, casting_time, range_type, components, duration, description, resolution_mode, classes) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		"test-spell", "Test Spell", 1, "evocation", "1 action", "ranged", `{V,S}`, "Instantaneous", "A test spell", "auto", `{wizard}`)
	if err != nil {
		t.Fatalf("spells table should exist: %v", err)
	}

	// Verify creatures table exists
	_, err = db.Exec(`INSERT INTO creatures (id, name, size, type, ac, hp_formula, hp_average, speed, ability_scores, cr, attacks) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		"test-creature", "Test Creature", "Medium", "beast", 12, "2d8+2", 11, `{"walk":30}`, `{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10}`, "1", `[]`)
	if err != nil {
		t.Fatalf("creatures table should exist: %v", err)
	}

	// Verify magic_items table exists
	_, err = db.Exec(`INSERT INTO magic_items (id, name, rarity, description) VALUES ($1, $2, $3, $4)`,
		"test-magic-item", "Test Magic Item", "common", "A test item")
	if err != nil {
		t.Fatalf("magic_items table should exist: %v", err)
	}
}

func TestIntegration_SeedCreaturesAndMagicItems(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := sharedDB.AcquireDB(t)
	ctx := context.Background()

	if err := refdata.SeedAll(ctx, db); err != nil {
		t.Fatalf("SeedAll failed: %v", err)
	}

	q := refdata.New(db)

	// Verify creature count
	creatureCount, err := q.CountCreatures(ctx)
	if err != nil {
		t.Fatalf("CountCreatures failed: %v", err)
	}
	if creatureCount != int64(refdata.CreatureCount) {
		t.Fatalf("expected %d creatures, got %d", refdata.CreatureCount, creatureCount)
	}

	// Spot-check: Goblin
	goblin, err := q.GetCreature(ctx, "goblin")
	if err != nil {
		t.Fatalf("GetCreature(goblin) failed: %v", err)
	}
	if goblin.Ac != 15 {
		t.Errorf("Goblin AC: expected 15, got %d", goblin.Ac)
	}
	if goblin.HpAverage != 7 {
		t.Errorf("Goblin HP average: expected 7, got %d", goblin.HpAverage)
	}
	if goblin.Cr != "1/4" {
		t.Errorf("Goblin CR: expected 1/4, got %s", goblin.Cr)
	}

	// Spot-check: Adult Red Dragon
	dragon, err := q.GetCreature(ctx, "adult-red-dragon")
	if err != nil {
		t.Fatalf("GetCreature(adult-red-dragon) failed: %v", err)
	}
	if dragon.Ac != 19 {
		t.Errorf("Adult Red Dragon AC: expected 19, got %d", dragon.Ac)
	}
	if dragon.Cr != "17" {
		t.Errorf("Adult Red Dragon CR: expected 17, got %s", dragon.Cr)
	}

	// Verify magic item count
	magicItemCount, err := q.CountMagicItems(ctx)
	if err != nil {
		t.Fatalf("CountMagicItems failed: %v", err)
	}
	if magicItemCount != int64(refdata.MagicItemCount) {
		t.Fatalf("expected %d magic items, got %d", refdata.MagicItemCount, magicItemCount)
	}

	// Spot-check: Weapon +1
	weaponPlus1, err := q.GetMagicItem(ctx, "weapon-plus-1")
	if err != nil {
		t.Fatalf("GetMagicItem(weapon-plus-1) failed: %v", err)
	}
	if !weaponPlus1.MagicBonus.Valid || weaponPlus1.MagicBonus.Int32 != 1 {
		t.Errorf("Weapon +1 magic_bonus: expected 1, got %v", weaponPlus1.MagicBonus)
	}
	if weaponPlus1.Rarity != "uncommon" {
		t.Errorf("Weapon +1 rarity: expected uncommon, got %s", weaponPlus1.Rarity)
	}

	// Spot-check: Bag of Holding
	bag, err := q.GetMagicItem(ctx, "bag-of-holding")
	if err != nil {
		t.Fatalf("GetMagicItem(bag-of-holding) failed: %v", err)
	}
	if bag.Rarity != "uncommon" {
		t.Errorf("Bag of Holding rarity: expected uncommon, got %s", bag.Rarity)
	}

	// Verify ListCreatures returns all creatures
	allCreatures, err := q.ListCreatures(ctx)
	if err != nil {
		t.Fatalf("ListCreatures failed: %v", err)
	}
	if len(allCreatures) != refdata.CreatureCount {
		t.Errorf("ListCreatures: expected %d, got %d", refdata.CreatureCount, len(allCreatures))
	}

	// Verify list by type/CR queries work
	beasts, err := q.ListCreaturesByType(ctx, "beast")
	if err != nil {
		t.Fatalf("ListCreaturesByType(beast) failed: %v", err)
	}
	if len(beasts) == 0 {
		t.Error("expected at least one beast creature")
	}

	cr1Creatures, err := q.ListCreaturesByCR(ctx, "1")
	if err != nil {
		t.Fatalf("ListCreaturesByCR(1) failed: %v", err)
	}
	if len(cr1Creatures) == 0 {
		t.Error("expected at least one CR 1 creature")
	}

	// Verify ListMagicItems returns all items
	allItems, err := q.ListMagicItems(ctx)
	if err != nil {
		t.Fatalf("ListMagicItems failed: %v", err)
	}
	if len(allItems) != refdata.MagicItemCount {
		t.Errorf("ListMagicItems: expected %d, got %d", refdata.MagicItemCount, len(allItems))
	}

	// Verify list by rarity query works
	rareItems, err := q.ListMagicItemsByRarity(ctx, "rare")
	if err != nil {
		t.Fatalf("ListMagicItemsByRarity(rare) failed: %v", err)
	}
	if len(rareItems) == 0 {
		t.Error("expected at least one rare magic item")
	}

	// Verify ListMagicItemsByType works
	weaponItems, err := q.ListMagicItemsByType(ctx, sql.NullString{String: "weapon", Valid: true})
	if err != nil {
		t.Fatalf("ListMagicItemsByType(weapon) failed: %v", err)
	}
	if len(weaponItems) == 0 {
		t.Error("expected at least one weapon magic item")
	}
}
