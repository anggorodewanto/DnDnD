package registration_test

import (
	"testing"

	"github.com/ab/dndnd/internal/registration"
)

func TestGetStatus_F16_ExcludesRetiredRow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx, db, queries := setupTestDB(t)
	svc := registration.NewService(queries)
	campaignID := createTestCampaign(t, db, "guild-f16")
	charRetired := createTestCharacter(t, db, campaignID, "OldHero")
	charActive := createTestCharacter(t, db, campaignID, "NewHero")

	// Insert a retired row for the player.
	db.ExecContext(ctx,
		`INSERT INTO player_characters (campaign_id, character_id, discord_user_id, status, created_via)
		 VALUES ($1, $2, $3, 'retired', 'register')`,
		campaignID, charRetired, "player-f16",
	)
	// Insert an active (approved) row for the same player.
	db.ExecContext(ctx,
		`INSERT INTO player_characters (campaign_id, character_id, discord_user_id, status, created_via)
		 VALUES ($1, $2, $3, 'approved', 'register')`,
		campaignID, charActive, "player-f16",
	)

	pc, err := svc.GetStatus(ctx, campaignID, "player-f16")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if pc.Status == "retired" {
		t.Fatal("GetStatus returned a retired row; expected the active row")
	}
	if pc.CharacterID != charActive {
		t.Errorf("expected character_id %s, got %s", charActive, pc.CharacterID)
	}
}
