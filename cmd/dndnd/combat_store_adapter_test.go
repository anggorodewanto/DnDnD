package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/database"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/testutil"
)

// TestCombatStoreAdapter_CharacterFieldBridges exercises the positional-arg
// bridge by round-tripping inventory and gold updates through a real
// testcontainers-backed database. This doubles as a smoke test that the
// adapter correctly satisfies combat.Store.
func TestCombatStoreAdapter_CharacterFieldBridges(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)
	queries := refdata.New(db)
	ctx := context.Background()

	require.NoError(t, database.MigrateUp(db, dbfs.Migrations))

	campaignID := uuid.New()
	_, err := db.Exec(`INSERT INTO campaigns (id, guild_id, dm_user_id, name, status) VALUES ($1, $2, $3, $4, $5)`,
		campaignID, "g-adap", "dm", "Adapter Campaign", "active")
	require.NoError(t, err)

	charID := uuid.New()
	_, err = db.Exec(`INSERT INTO characters (id, campaign_id, name, race, classes, level, ability_scores, hp_max, hp_current, ac, speed_ft, proficiency_bonus, hit_dice_remaining, languages, gold) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		charID, campaignID, "Adapter Hero", "human", `[{"class":"fighter","level":1}]`, 1,
		`{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10}`,
		10, 10, 10, 30, 2, `[{"die":"d10","remaining":1}]`, `{Common}`, 100)
	require.NoError(t, err)

	adapter := &combatStoreAdapter{queries}

	inv := pqtype.NullRawMessage{RawMessage: json.RawMessage(`[{"id":"sword"}]`), Valid: true}
	require.NoError(t, adapter.UpdateCharacterInventory(ctx, charID, inv))
	require.NoError(t, adapter.UpdateCharacterGold(ctx, charID, 250))

	var gold int32
	var invBytes []byte
	err = db.QueryRow(`SELECT gold, inventory FROM characters WHERE id = $1`, charID).Scan(&gold, &invBytes)
	require.NoError(t, err)
	assert.Equal(t, int32(250), gold)
	assert.JSONEq(t, `[{"id":"sword"}]`, string(invBytes))
}
