package testutil_test

import (
	"os"
	"testing"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/testutil"
)

// sharedFixturesDB is a package-scoped shared container for the fixtures
// tests. It is started lazily by AcquireDB and torn down once via TestMain.
var sharedFixturesDB = testutil.NewSharedTestDB(dbfs.Migrations)

func TestMain(m *testing.M) {
	code := m.Run()
	sharedFixturesDB.Teardown()
	os.Exit(code)
}

func TestFixtures_NewTestCampaign_CreatesUniqueRowsPerCall(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := sharedFixturesDB.AcquireDB(t)
	q := refdata.New(db)

	camp1 := testutil.NewTestCampaign(t, q, "guild-fixtures-a")
	camp2 := testutil.NewTestCampaign(t, q, "guild-fixtures-a")

	if camp1.ID == camp2.ID {
		t.Fatalf("expected unique campaign IDs, got %s twice", camp1.ID)
	}
	if camp1.GuildID == camp2.GuildID {
		t.Fatalf("expected unique guild IDs (uniqueness suffix), got %s twice", camp1.GuildID)
	}
	if camp1.Name == "" {
		t.Fatalf("expected non-empty default name")
	}
}

func TestFixtures_NewTestCharacter_LinksToCampaign(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := sharedFixturesDB.AcquireDB(t)
	q := refdata.New(db)

	camp := testutil.NewTestCampaign(t, q, "guild-fixtures-char")
	char := testutil.NewTestCharacter(t, q, camp.ID, "Aria", 5)

	if char.CampaignID != camp.ID {
		t.Fatalf("expected campaign id %s, got %s", camp.ID, char.CampaignID)
	}
	if char.Name != "Aria" {
		t.Fatalf("expected name 'Aria', got %q", char.Name)
	}
	if char.Level != 5 {
		t.Fatalf("expected level 5, got %d", char.Level)
	}
}

func TestFixtures_NewTestPlayerCharacter_LinksDiscordUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := sharedFixturesDB.AcquireDB(t)
	q := refdata.New(db)

	camp := testutil.NewTestCampaign(t, q, "guild-fixtures-pc")
	char := testutil.NewTestCharacter(t, q, camp.ID, "Bree", 3)
	pc := testutil.NewTestPlayerCharacter(t, q, camp.ID, char.ID, "discord-pc-42")

	if pc.DiscordUserID != "discord-pc-42" {
		t.Fatalf("expected discord-pc-42, got %q", pc.DiscordUserID)
	}
	if pc.Status != "approved" {
		t.Fatalf("expected approved status, got %q", pc.Status)
	}
}

func TestFixtures_NewTestEncounter_LinksToCampaign(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := sharedFixturesDB.AcquireDB(t)
	q := refdata.New(db)

	camp := testutil.NewTestCampaign(t, q, "guild-fixtures-enc")
	enc := testutil.NewTestEncounter(t, q, camp.ID)

	if enc.CampaignID != camp.ID {
		t.Fatalf("expected campaign id %s, got %s", camp.ID, enc.CampaignID)
	}
	if enc.Status == "" {
		t.Fatalf("expected non-empty status")
	}
}

func TestFixtures_NewTestCombatant_LinksEncounterAndCharacter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := sharedFixturesDB.AcquireDB(t)
	q := refdata.New(db)

	camp := testutil.NewTestCampaign(t, q, "guild-fixtures-comb")
	char := testutil.NewTestCharacter(t, q, camp.ID, "Cara", 4)
	enc := testutil.NewTestEncounter(t, q, camp.ID)
	comb := testutil.NewTestCombatant(t, q, enc.ID, char.ID)

	if comb.EncounterID != enc.ID {
		t.Fatalf("expected encounter id %s, got %s", enc.ID, comb.EncounterID)
	}
	if !comb.CharacterID.Valid || comb.CharacterID.UUID != char.ID {
		t.Fatalf("expected character id %s, got %+v", char.ID, comb.CharacterID)
	}
	if comb.DisplayName == "" {
		t.Fatalf("expected non-empty display name")
	}
}

func TestFixtures_NewTestMap_LinksToCampaign(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := sharedFixturesDB.AcquireDB(t)
	q := refdata.New(db)

	camp := testutil.NewTestCampaign(t, q, "guild-fixtures-map")
	m := testutil.NewTestMap(t, q, camp.ID)

	if m.CampaignID != camp.ID {
		t.Fatalf("expected campaign id %s, got %s", camp.ID, m.CampaignID)
	}
	if m.WidthSquares <= 0 || m.HeightSquares <= 0 {
		t.Fatalf("expected positive map dimensions, got %dx%d", m.WidthSquares, m.HeightSquares)
	}
}
