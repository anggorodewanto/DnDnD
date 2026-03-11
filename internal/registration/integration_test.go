package registration_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/registration"
	"github.com/ab/dndnd/internal/testutil"
	"github.com/google/uuid"
)

// setupTestDB creates a migrated test DB and returns a context, the DB, and
// a refdata.Queries instance.
func setupTestDB(t *testing.T) (context.Context, *sql.DB, *refdata.Queries) {
	t.Helper()
	db := testutil.NewMigratedTestDB(t, dbfs.Migrations)
	return context.Background(), db, refdata.New(db)
}

// createTestCampaign inserts a campaign and returns its UUID.
func createTestCampaign(t *testing.T, db *sql.DB, guildID string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := db.QueryRow(
		`INSERT INTO campaigns (guild_id, dm_user_id, name, status) VALUES ($1, $2, $3, $4) RETURNING id`,
		guildID, "dm-user", "Test Campaign", "active",
	).Scan(&id)
	if err != nil {
		t.Fatalf("createTestCampaign: %v", err)
	}
	return id
}

// createTestCharacter inserts a minimal character and returns its UUID.
func createTestCharacter(t *testing.T, db *sql.DB, campaignID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	classes, _ := json.Marshal([]map[string]interface{}{{"name": "Fighter", "level": 1}})
	abilities, _ := json.Marshal(map[string]int{"str": 10, "dex": 10, "con": 10, "int": 10, "wis": 10, "cha": 10})
	hitDice, _ := json.Marshal(map[string]int{"d10": 1})

	var id uuid.UUID
	err := db.QueryRow(
		`INSERT INTO characters (campaign_id, name, race, classes, level, ability_scores,
			hp_max, hp_current, ac, speed_ft, proficiency_bonus, hit_dice_remaining, languages)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING id`,
		campaignID, name, "Human", classes, 1, abilities,
		10, 10, 10, 30, 2, hitDice, []string{"Common"},
	).Scan(&id)
	if err != nil {
		t.Fatalf("createTestCharacter(%s): %v", name, err)
	}
	return id
}

func TestIntegration_Register_ExactMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx, db, queries := setupTestDB(t)
	svc := registration.NewService(queries)
	campaignID := createTestCampaign(t, db, "guild-reg-exact")
	createTestCharacter(t, db, campaignID, "Thorn")

	result, err := svc.Register(ctx, campaignID, "player-1", "Thorn")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if result.Status != registration.ResultExactMatch {
		t.Fatalf("expected ExactMatch, got %v", result.Status)
	}
	if result.PlayerCharacter == nil {
		t.Fatal("expected PlayerCharacter to be set")
	}
	if result.PlayerCharacter.Status != "pending" {
		t.Errorf("expected status pending, got %s", result.PlayerCharacter.Status)
	}
	if result.PlayerCharacter.CreatedVia != "register" {
		t.Errorf("expected created_via register, got %s", result.PlayerCharacter.CreatedVia)
	}
}

func TestIntegration_Register_CaseInsensitiveMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx, db, queries := setupTestDB(t)
	svc := registration.NewService(queries)
	campaignID := createTestCampaign(t, db, "guild-reg-ci")
	createTestCharacter(t, db, campaignID, "Thorn")

	result, err := svc.Register(ctx, campaignID, "player-1", "thorn")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if result.Status != registration.ResultExactMatch {
		t.Fatalf("expected ExactMatch for case-insensitive, got %v", result.Status)
	}
}

func TestIntegration_Register_FuzzyMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx, db, queries := setupTestDB(t)
	svc := registration.NewService(queries)
	campaignID := createTestCampaign(t, db, "guild-reg-fuzzy")
	createTestCharacter(t, db, campaignID, "Thorn")
	createTestCharacter(t, db, campaignID, "Thorin")
	createTestCharacter(t, db, campaignID, "Thora")

	result, err := svc.Register(ctx, campaignID, "player-1", "Thron")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if result.Status != registration.ResultFuzzyMatch {
		t.Fatalf("expected FuzzyMatch, got %v", result.Status)
	}
	if len(result.Suggestions) == 0 {
		t.Fatal("expected suggestions")
	}
	if len(result.Suggestions) > 3 {
		t.Errorf("expected at most 3 suggestions, got %d", len(result.Suggestions))
	}
}

func TestIntegration_Register_NoMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx, db, queries := setupTestDB(t)
	svc := registration.NewService(queries)
	campaignID := createTestCampaign(t, db, "guild-reg-none")
	createTestCharacter(t, db, campaignID, "Thorn")

	result, err := svc.Register(ctx, campaignID, "player-1", "Xyzzy")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if result.Status != registration.ResultNoMatch {
		t.Fatalf("expected NoMatch, got %v", result.Status)
	}
}

func TestIntegration_StatusTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx, db, queries := setupTestDB(t)
	svc := registration.NewService(queries)
	campaignID := createTestCampaign(t, db, "guild-status")

	t.Run("pending to approved", func(t *testing.T) {
		createTestCharacter(t, db, campaignID, "Fighter1")
		result, err := svc.Register(ctx, campaignID, "player-approve", "Fighter1")
		if err != nil {
			t.Fatalf("Register: %v", err)
		}

		pc, err := svc.Approve(ctx, result.PlayerCharacter.ID)
		if err != nil {
			t.Fatalf("Approve: %v", err)
		}
		if pc.Status != "approved" {
			t.Errorf("expected approved, got %s", pc.Status)
		}
	})

	t.Run("pending to changes_requested", func(t *testing.T) {
		createTestCharacter(t, db, campaignID, "Fighter2")
		result, err := svc.Register(ctx, campaignID, "player-changes", "Fighter2")
		if err != nil {
			t.Fatalf("Register: %v", err)
		}

		pc, err := svc.RequestChanges(ctx, result.PlayerCharacter.ID, "Fix your backstory")
		if err != nil {
			t.Fatalf("RequestChanges: %v", err)
		}
		if pc.Status != "changes_requested" {
			t.Errorf("expected changes_requested, got %s", pc.Status)
		}
		if !pc.DmFeedback.Valid || pc.DmFeedback.String != "Fix your backstory" {
			t.Errorf("expected dm_feedback, got %v", pc.DmFeedback)
		}
	})

	t.Run("pending to rejected", func(t *testing.T) {
		createTestCharacter(t, db, campaignID, "Fighter3")
		result, err := svc.Register(ctx, campaignID, "player-reject", "Fighter3")
		if err != nil {
			t.Fatalf("Register: %v", err)
		}

		pc, err := svc.Reject(ctx, result.PlayerCharacter.ID, "Not suitable")
		if err != nil {
			t.Fatalf("Reject: %v", err)
		}
		if pc.Status != "rejected" {
			t.Errorf("expected rejected, got %s", pc.Status)
		}
		if !pc.DmFeedback.Valid || pc.DmFeedback.String != "Not suitable" {
			t.Errorf("expected dm_feedback, got %v", pc.DmFeedback)
		}
	})

	t.Run("pending to retired", func(t *testing.T) {
		createTestCharacter(t, db, campaignID, "Fighter4")
		result, err := svc.Register(ctx, campaignID, "player-retire", "Fighter4")
		if err != nil {
			t.Fatalf("Register: %v", err)
		}

		pc, err := svc.Retire(ctx, result.PlayerCharacter.ID)
		if err != nil {
			t.Fatalf("Retire: %v", err)
		}
		if pc.Status != "retired" {
			t.Errorf("expected retired, got %s", pc.Status)
		}
	})
}

func TestIntegration_GetStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx, db, queries := setupTestDB(t)
	svc := registration.NewService(queries)
	campaignID := createTestCampaign(t, db, "guild-getstatus")
	createTestCharacter(t, db, campaignID, "Ranger1")

	// Before registration
	pc, err := svc.GetStatus(ctx, campaignID, "player-gs")
	if err == nil {
		t.Fatalf("expected error for unregistered player, got %+v", pc)
	}

	// After registration
	_, err = svc.Register(ctx, campaignID, "player-gs", "Ranger1")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	pc, err = svc.GetStatus(ctx, campaignID, "player-gs")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if pc.Status != "pending" {
		t.Errorf("expected pending, got %s", pc.Status)
	}
}

func TestIntegration_UniqueConstraints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx, db, queries := setupTestDB(t)
	svc := registration.NewService(queries)
	campaignID := createTestCampaign(t, db, "guild-unique")
	createTestCharacter(t, db, campaignID, "Paladin1")
	createTestCharacter(t, db, campaignID, "Paladin2")

	t.Run("one character per player per campaign", func(t *testing.T) {
		_, err := svc.Register(ctx, campaignID, "player-dup", "Paladin1")
		if err != nil {
			t.Fatalf("first Register: %v", err)
		}
		_, err = svc.Register(ctx, campaignID, "player-dup", "Paladin2")
		if err == nil {
			t.Fatal("expected error for duplicate player in campaign")
		}
	})

	t.Run("one player per character per campaign", func(t *testing.T) {
		createTestCharacter(t, db, campaignID, "SharedChar")
		_, err := svc.Register(ctx, campaignID, "player-share1", "SharedChar")
		if err != nil {
			t.Fatalf("first Register: %v", err)
		}
		_, err = svc.Register(ctx, campaignID, "player-share2", "SharedChar")
		if err == nil {
			t.Fatal("expected error for duplicate character in campaign")
		}
	})
}

func TestIntegration_CreatePlaceholder_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx, db, queries := setupTestDB(t)
	svc := registration.NewService(queries)
	campaignID := createTestCampaign(t, db, "guild-placeholder-happy")

	char, err := svc.CreatePlaceholder(ctx, campaignID, "Imported (https://dndbeyond.com/char/1)", "https://dndbeyond.com/characters/12345")
	if err != nil {
		t.Fatalf("CreatePlaceholder: %v", err)
	}
	if char.Name != "Imported (https://dndbeyond.com/char/1)" {
		t.Errorf("expected name, got %s", char.Name)
	}
	if char.CampaignID != campaignID {
		t.Errorf("expected campaign ID %s, got %s", campaignID, char.CampaignID)
	}
	if !char.DdbUrl.Valid || char.DdbUrl.String != "https://dndbeyond.com/characters/12345" {
		t.Errorf("expected ddb_url set, got %v", char.DdbUrl)
	}
	if char.Level != 1 {
		t.Errorf("expected level 1, got %d", char.Level)
	}
}

func TestIntegration_CreatePlaceholder_EmptyDdbURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx, db, queries := setupTestDB(t)
	svc := registration.NewService(queries)
	campaignID := createTestCampaign(t, db, "guild-placeholder-empty-url")

	char, err := svc.CreatePlaceholder(ctx, campaignID, "New Character", "")
	if err != nil {
		t.Fatalf("CreatePlaceholder: %v", err)
	}
	if char.Name != "New Character" {
		t.Errorf("expected name, got %s", char.Name)
	}
	if char.DdbUrl.Valid {
		t.Errorf("expected null ddb_url for empty string, got %v", char.DdbUrl)
	}
}

func TestIntegration_CreatePlaceholder_DBError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx, _, queries := setupTestDB(t)
	svc := registration.NewService(queries)

	// Use a non-existent campaign ID to trigger a foreign key error
	fakeCampaignID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	_, err := svc.CreatePlaceholder(ctx, fakeCampaignID, "Bad Char", "")
	if err == nil {
		t.Fatal("expected error for non-existent campaign, got nil")
	}
}

func TestIntegration_Import(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx, db, queries := setupTestDB(t)
	svc := registration.NewService(queries)
	campaignID := createTestCampaign(t, db, "guild-import")
	charID := createTestCharacter(t, db, campaignID, "ImportChar")

	pc, err := svc.Import(ctx, campaignID, "player-import", charID)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if pc.Status != "pending" {
		t.Errorf("expected pending, got %s", pc.Status)
	}
	if pc.CreatedVia != "import" {
		t.Errorf("expected created_via import, got %s", pc.CreatedVia)
	}
}

func TestIntegration_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx, db, queries := setupTestDB(t)
	svc := registration.NewService(queries)
	campaignID := createTestCampaign(t, db, "guild-create")
	charID := createTestCharacter(t, db, campaignID, "CreateChar")

	pc, err := svc.Create(ctx, campaignID, "player-create", charID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if pc.Status != "pending" {
		t.Errorf("expected pending, got %s", pc.Status)
	}
	if pc.CreatedVia != "create" {
		t.Errorf("expected created_via create, got %s", pc.CreatedVia)
	}
}

func TestIntegration_InvalidStatusTransition(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx, db, queries := setupTestDB(t)
	svc := registration.NewService(queries)
	campaignID := createTestCampaign(t, db, "guild-invalid-trans")
	createTestCharacter(t, db, campaignID, "TransChar")

	result, err := svc.Register(ctx, campaignID, "player-trans", "TransChar")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Approve first
	_, err = svc.Approve(ctx, result.PlayerCharacter.ID)
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}

	// Try to approve again (already approved) - should fail
	_, err = svc.Approve(ctx, result.PlayerCharacter.ID)
	if err == nil {
		t.Fatal("expected error for invalid status transition (approved -> approved)")
	}
}
