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

// TestEncounterLookupAdapter_FindsActiveEncounter seeds a campaign, character,
// encounter, and combatant row, then verifies the adapter resolves the active
// encounter ID for the character. Also covers the "not in combat" and
// "no-rows" branches.
func TestEncounterLookupAdapter_FindsActiveEncounter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)
	require.NoError(t, database.MigrateUp(db, dbfs.Migrations))

	queries := refdata.New(db)
	adapter := encounterLookupAdapter{queries: queries}
	ctx := context.Background()

	// Seed baseline rows.
	campaignID := uuid.New()
	_, err := db.Exec(`INSERT INTO campaigns (id, guild_id, dm_user_id, name, status) VALUES ($1, $2, $3, $4, $5)`,
		campaignID, "g-lookup", "dm", "Lookup Campaign", "active")
	require.NoError(t, err)

	charInCombat := uuid.New()
	_, err = db.Exec(`INSERT INTO characters (id, campaign_id, name, race, classes, level, ability_scores, hp_max, hp_current, ac, speed_ft, proficiency_bonus, hit_dice_remaining, languages) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		charInCombat, campaignID, "Hero", "human", `[{"class":"fighter","level":1}]`, 1,
		`{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10}`,
		10, 10, 10, 30, 2, `[{"die":"d10","remaining":1}]`, `{Common}`)
	require.NoError(t, err)

	charOutOfCombat := uuid.New()
	_, err = db.Exec(`INSERT INTO characters (id, campaign_id, name, race, classes, level, ability_scores, hp_max, hp_current, ac, speed_ft, proficiency_bonus, hit_dice_remaining, languages) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		charOutOfCombat, campaignID, "Bystander", "human", `[{"class":"fighter","level":1}]`, 1,
		`{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10}`,
		10, 10, 10, 30, 2, `[{"die":"d10","remaining":1}]`, `{Common}`)
	require.NoError(t, err)

	activeEncID := uuid.New()
	_, err = db.Exec(`INSERT INTO encounters (id, campaign_id, name, display_name, status, round_number) VALUES ($1, $2, $3, $4, $5, $6)`,
		activeEncID, campaignID, "Live Fight", "Live Fight", "active", 1)
	require.NoError(t, err)

	prepEncID := uuid.New()
	_, err = db.Exec(`INSERT INTO encounters (id, campaign_id, name, display_name, status, round_number) VALUES ($1, $2, $3, $4, $5, $6)`,
		prepEncID, campaignID, "Draft Fight", "Draft Fight", "preparing", 1)
	require.NoError(t, err)

	// charInCombat is a combatant in the active encounter.
	_, err = db.Exec(`INSERT INTO combatants (id, encounter_id, character_id, short_id, display_name, hp_max, hp_current, ac, position_col, position_row, initiative_roll) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		uuid.New(), activeEncID, charInCombat, "H1", "Hero", 10, 10, 10, "A", 1, 15)
	require.NoError(t, err)

	// Case 1: character in active encounter.
	encID, inCombat, err := adapter.ActiveEncounterIDForCharacter(ctx, charInCombat)
	require.NoError(t, err)
	assert.True(t, inCombat)
	assert.Equal(t, activeEncID, encID)

	// Case 2: character not in any encounter -> no rows.
	encID, inCombat, err = adapter.ActiveEncounterIDForCharacter(ctx, charOutOfCombat)
	require.NoError(t, err)
	assert.False(t, inCombat)
	assert.Equal(t, uuid.Nil, encID)

	// Case 3: unknown character -> still no rows, no error.
	encID, inCombat, err = adapter.ActiveEncounterIDForCharacter(ctx, uuid.New())
	require.NoError(t, err)
	assert.False(t, inCombat)
	assert.Equal(t, uuid.Nil, encID)
}
