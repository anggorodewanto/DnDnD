package refdata_test

import (
	"testing"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/testutil"
)

func TestIntegration_ReferenceTablesMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewMigratedTestDB(t, dbfs.Migrations)

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
}
