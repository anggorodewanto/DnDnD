package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/database"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/testutil"
)

// TestDiscordUserEncounterResolver_Integration exercises the full
// guild->campaign->player_character->character->encounter chain against a
// real Postgres database so the production wiring in main.go is covered end
// to end.
func TestDiscordUserEncounterResolver_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)
	require.NoError(t, database.MigrateUp(db, dbfs.Migrations))

	queries := refdata.New(db)
	resolver := newDiscordUserEncounterResolver(queries)
	ctx := context.Background()

	campaignID := uuid.New()
	_, err := db.Exec(`INSERT INTO campaigns (id, guild_id, dm_user_id, name, status) VALUES ($1, $2, $3, $4, $5)`,
		campaignID, "g-resolver", "dm", "Resolver Campaign", "active")
	require.NoError(t, err)

	charID := uuid.New()
	_, err = db.Exec(`INSERT INTO characters (id, campaign_id, name, race, classes, level, ability_scores, hp_max, hp_current, ac, speed_ft, proficiency_bonus, hit_dice_remaining, languages) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		charID, campaignID, "Hero", "human", `[{"class":"fighter","level":1}]`, 1,
		`{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10}`,
		10, 10, 10, 30, 2, `[{"die":"d10","remaining":1}]`, `{Common}`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO player_characters (id, campaign_id, character_id, discord_user_id, status, created_via) VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New(), campaignID, charID, "discord-user-77", "approved", "register")
	require.NoError(t, err)

	encID := uuid.New()
	_, err = db.Exec(`INSERT INTO encounters (id, campaign_id, name, display_name, status, round_number) VALUES ($1, $2, $3, $4, $5, $6)`,
		encID, campaignID, "Skirmish", "Skirmish", "active", 1)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO combatants (id, encounter_id, character_id, short_id, display_name, hp_max, hp_current, ac, position_col, position_row, initiative_roll) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		uuid.New(), encID, charID, "H1", "Hero", 10, 10, 10, "A", 1, 15)
	require.NoError(t, err)

	// Happy path.
	got, err := resolver.ActiveEncounterForUser(ctx, "g-resolver", "discord-user-77")
	require.NoError(t, err)
	assert.Equal(t, encID, got)

	// Missing guild -> error.
	_, err = resolver.ActiveEncounterForUser(ctx, "g-ghost", "discord-user-77")
	require.Error(t, err)

	// Missing player_character -> error.
	_, err = resolver.ActiveEncounterForUser(ctx, "g-resolver", "unknown-discord")
	require.Error(t, err)
}
