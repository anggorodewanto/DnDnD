package refdata_test

import (
	"testing"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/database"
	"github.com/ab/dndnd/internal/testutil"
)

func TestIntegration_ReferenceTablesMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)

	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

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
}
