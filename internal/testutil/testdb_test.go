package testutil

import (
	"sort"
	"testing"

	dbfs "github.com/ab/dndnd/db"
)

func TestNewTestDB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := NewTestDB(t)

	var result int
	if err := db.QueryRow("SELECT 1").Scan(&result); err != nil {
		t.Fatalf("failed to query test database: %v", err)
	}
	if result != 1 {
		t.Fatalf("expected 1, got %d", result)
	}
}

func TestNewTestDBConnString(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	connStr := NewTestDBConnString(t)
	if connStr == "" {
		t.Fatal("expected non-empty connection string")
	}
}

func TestSharedTestDB_AcquireDB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	shared := NewSharedTestDB(dbfs.Migrations)
	defer shared.Teardown()

	// First call should lazy-start the container
	db1 := shared.AcquireDB(t)

	// Insert a row
	_, err := db1.Exec(`INSERT INTO campaigns (guild_id, dm_user_id, name) VALUES ($1, $2, $3)`,
		"guild-1", "dm-1", "Test Campaign")
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	var count int
	if err := db1.QueryRow("SELECT count(*) FROM campaigns").Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 campaign, got %d", count)
	}

	// Second call should return the same DB but with tables truncated
	db2 := shared.AcquireDB(t)
	if err := db2.QueryRow("SELECT count(*) FROM campaigns").Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 campaigns after truncate, got %d", count)
	}
}

func TestSharedTestDB_PreservesReferenceTables(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	shared := NewSharedTestDB(dbfs.Migrations)
	defer shared.Teardown()

	db := shared.AcquireDB(t)

	// Insert into a reference table
	_, err := db.Exec(`INSERT INTO weapons (id, name, damage, damage_type, weapon_type) VALUES ($1, $2, $3, $4, $5)`,
		"test-sword", "Test Sword", "1d6", "slashing", "simple_melee")
	if err != nil {
		t.Fatalf("insert weapon failed: %v", err)
	}

	// AcquireDB again — reference table data should survive truncation
	db2 := shared.AcquireDB(t)
	var count int
	if err := db2.QueryRow("SELECT count(*) FROM weapons WHERE id = 'test-sword'").Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected reference data to survive truncate, got count %d", count)
	}
}

func TestTruncateUserTables_PreservesSrdRefdataAndDeletesHomebrew(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	shared := NewSharedTestDB(dbfs.Migrations)
	defer shared.Teardown()

	db := shared.AcquireDB(t)

	// Create a campaign so homebrew rows have a valid FK target.
	var campaignID string
	if err := db.QueryRow(
		`INSERT INTO campaigns (guild_id, dm_user_id, name) VALUES ($1, $2, $3) RETURNING id`,
		"guild-hb", "dm-hb", "HB Campaign",
	).Scan(&campaignID); err != nil {
		t.Fatalf("insert campaign failed: %v", err)
	}

	// Baseline SRD counts (seeded by migrations).
	srdCount := func(table string) int {
		var n int
		q := "SELECT count(*) FROM " + table + " WHERE campaign_id IS NULL"
		if err := db.QueryRow(q).Scan(&n); err != nil {
			t.Fatalf("count %s failed: %v", table, err)
		}
		return n
	}
	spellSRDBefore := srdCount("spells")
	featSRDBefore := srdCount("feats")
	weaponSRDBefore := srdCount("weapons")

	// Insert homebrew rows scoped to the campaign.
	if _, err := db.Exec(
		`INSERT INTO spells (id, name, level, school, casting_time, range_type, components, duration, description, classes, campaign_id, homebrew, source)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, true, 'homebrew')`,
		"hb-spell-1", "HB Spell", 1, "evocation", "1 action", "ranged", []string{"V", "S"}, "Instantaneous", "desc", []string{"wizard"}, campaignID,
	); err != nil {
		t.Fatalf("insert homebrew spell failed: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO feats (id, name, description, campaign_id, homebrew, source)
		 VALUES ($1, $2, $3, $4, true, 'homebrew')`,
		"hb-feat-1", "HB Feat", "desc", campaignID,
	); err != nil {
		t.Fatalf("insert homebrew feat failed: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO weapons (id, name, damage, damage_type, weapon_type, campaign_id, homebrew, source)
		 VALUES ($1, $2, $3, $4, $5, $6, true, 'homebrew')`,
		"hb-weapon-1", "HB Sword", "1d8", "slashing", "simple_melee", campaignID,
	); err != nil {
		t.Fatalf("insert homebrew weapon failed: %v", err)
	}

	if err := TruncateUserTables(db); err != nil {
		t.Fatalf("TruncateUserTables failed: %v", err)
	}

	// Campaign row should be gone.
	var campaignCount int
	if err := db.QueryRow(`SELECT count(*) FROM campaigns WHERE id = $1`, campaignID).Scan(&campaignCount); err != nil {
		t.Fatalf("count campaigns failed: %v", err)
	}
	if campaignCount != 0 {
		t.Fatalf("expected campaign deleted, got count %d", campaignCount)
	}

	// Homebrew rows should be gone.
	homebrewCount := func(table, id string) int {
		var n int
		q := "SELECT count(*) FROM " + table + " WHERE id = $1"
		if err := db.QueryRow(q, id).Scan(&n); err != nil {
			t.Fatalf("count homebrew %s failed: %v", table, err)
		}
		return n
	}
	if got := homebrewCount("spells", "hb-spell-1"); got != 0 {
		t.Errorf("expected homebrew spell deleted, got %d", got)
	}
	if got := homebrewCount("feats", "hb-feat-1"); got != 0 {
		t.Errorf("expected homebrew feat deleted, got %d", got)
	}
	if got := homebrewCount("weapons", "hb-weapon-1"); got != 0 {
		t.Errorf("expected homebrew weapon deleted, got %d", got)
	}

	// SRD rows should be preserved.
	if got := srdCount("spells"); got != spellSRDBefore {
		t.Errorf("expected SRD spells preserved (%d), got %d", spellSRDBefore, got)
	}
	if got := srdCount("feats"); got != featSRDBefore {
		t.Errorf("expected SRD feats preserved (%d), got %d", featSRDBefore, got)
	}
	if got := srdCount("weapons"); got != weaponSRDBefore {
		t.Errorf("expected SRD weapons preserved (%d), got %d", weaponSRDBefore, got)
	}
}

func TestTruncateUserTables(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := NewMigratedTestDB(t, dbfs.Migrations)

	// Insert data into a mutable table
	_, err := db.Exec(`INSERT INTO campaigns (guild_id, dm_user_id, name) VALUES ($1, $2, $3)`,
		"guild-1", "dm-1", "Test Campaign")
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if err := TruncateUserTables(db); err != nil {
		t.Fatalf("TruncateUserTables failed: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT count(*) FROM campaigns").Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 after truncate, got %d", count)
	}
}

func TestTruncationListMatchesSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := NewMigratedTestDB(t, dbfs.Migrations)

	// Query all user-created tables from information_schema
	rows, err := db.Query(`
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_type = 'BASE TABLE'
		AND table_name != 'goose_db_version'
		ORDER BY table_name`)
	if err != nil {
		t.Fatalf("query information_schema failed: %v", err)
	}
	defer rows.Close()

	var allTables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		allTables = append(allTables, name)
	}

	// Combine mutable + reference tables should equal all tables
	combined := make(map[string]bool)
	for _, tbl := range MutableTables {
		combined[tbl] = true
	}
	for _, tbl := range ReferenceTables {
		combined[tbl] = true
	}

	sort.Strings(allTables)
	for _, tbl := range allTables {
		if !combined[tbl] {
			t.Errorf("table %q exists in schema but is not in MutableTables or ReferenceTables — add it to the appropriate list in testdb.go", tbl)
		}
	}

	// Also check that no listed table is missing from the schema
	schemaSet := make(map[string]bool)
	for _, tbl := range allTables {
		schemaSet[tbl] = true
	}
	for _, tbl := range MutableTables {
		if !schemaSet[tbl] {
			t.Errorf("MutableTables lists %q but it does not exist in schema", tbl)
		}
	}
	for _, tbl := range ReferenceTables {
		if !schemaSet[tbl] {
			t.Errorf("ReferenceTables lists %q but it does not exist in schema", tbl)
		}
	}
}
