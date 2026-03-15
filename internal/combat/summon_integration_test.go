package combat_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestCreatureForSummon(t *testing.T, db *sql.DB, id, name string, ac, hp int32) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO creatures (id, name, size, type, ac, hp_formula, hp_average, speed, ability_scores, cr, attacks)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO NOTHING`,
		id, name, "Small", "beast", ac, "1d4", hp,
		`{"walk":5,"fly":60}`,
		`{"str":3,"dex":13,"con":8,"int":2,"wis":12,"cha":7}`,
		"0",
		`[{"name":"Talons","to_hit":3,"damage":"1","damage_type":"slashing"}]`)
	require.NoError(t, err)
}

func TestIntegration_SummonCreature(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	mapID := createTestMap(t, db, campaignID)
	charID := createTestCharacter(t, db, campaignID)
	createTestCreatureForSummon(t, db, "owl", "Owl", 11, 1)

	// Create encounter and add PC combatant as summoner
	svc := combat.NewService(&testStoreAdapter{queries})
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{
		CampaignID: campaignID,
		MapID:      uuid.NullUUID{UUID: mapID, Valid: true},
		Name:       "summon-test",
	})
	require.NoError(t, err)

	summoner, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		CharacterID: charID.String(),
		ShortID:     "AR",
		DisplayName: "Aragorn",
		HPMax:       45,
		HPCurrent:   45,
		AC:          18,
		SpeedFt:     30,
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
		IsVisible:   true,
	})
	require.NoError(t, err)

	// Summon an owl
	familiar, err := svc.SummonCreature(context.Background(), combat.SummonCreatureInput{
		EncounterID:   enc.ID,
		SummonerID:    summoner.ID,
		CreatureRefID: "owl",
		ShortID:       "FAM",
		DisplayName:   "Aragorn's Owl",
		PositionCol:   "C",
		PositionRow:   5,
	})
	require.NoError(t, err)
	assert.Equal(t, "FAM", familiar.ShortID)
	assert.Equal(t, "Aragorn's Owl", familiar.DisplayName)
	assert.True(t, familiar.SummonerID.Valid)
	assert.Equal(t, summoner.ID, familiar.SummonerID.UUID)
	assert.Equal(t, int32(1), familiar.HpMax)
	assert.Equal(t, int32(11), familiar.Ac)
	assert.True(t, familiar.IsAlive)
	assert.True(t, familiar.IsNpc)

	// Verify it appears in combatant list
	combatants, err := svc.ListCombatantsByEncounterID(context.Background(), enc.ID)
	require.NoError(t, err)
	assert.Len(t, combatants, 2)

	// Verify summoned creatures listing
	summons := combat.ListSummonedCreatures(combatants, summoner.ID)
	assert.Len(t, summons, 1)
	assert.Equal(t, "FAM", summons[0].ShortID)
}

func TestIntegration_CommandDismiss(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	mapID := createTestMap(t, db, campaignID)
	charID := createTestCharacter(t, db, campaignID)
	createTestCreatureForSummon(t, db, "owl", "Owl", 11, 1)

	svc := combat.NewService(&testStoreAdapter{queries})
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{
		CampaignID: campaignID,
		MapID:      uuid.NullUUID{UUID: mapID, Valid: true},
		Name:       "dismiss-test",
	})
	require.NoError(t, err)

	summoner, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		CharacterID: charID.String(), ShortID: "AR", DisplayName: "Aragorn",
		HPMax: 45, HPCurrent: 45, AC: 18, SpeedFt: 30,
		PositionCol: "A", PositionRow: 1, IsAlive: true, IsVisible: true,
	})
	require.NoError(t, err)

	_, err = svc.SummonCreature(context.Background(), combat.SummonCreatureInput{
		EncounterID: enc.ID, SummonerID: summoner.ID, CreatureRefID: "owl",
		ShortID: "FAM", DisplayName: "Aragorn's Owl", PositionCol: "C", PositionRow: 5,
	})
	require.NoError(t, err)

	// Dismiss via /command
	result, err := svc.CommandCreature(context.Background(), combat.CommandCreatureInput{
		EncounterID: enc.ID, SummonerID: summoner.ID, SummonerName: "Aragorn",
		CreatureShortID: "FAM", Action: "dismiss",
	})
	require.NoError(t, err)
	assert.Equal(t, "dismiss", result.Action)
	assert.Contains(t, result.CombatLog, "dismisses")

	// Verify removed from combatants
	combatants, err := svc.ListCombatantsByEncounterID(context.Background(), enc.ID)
	require.NoError(t, err)
	assert.Len(t, combatants, 1) // only the PC remains
}

func TestIntegration_SummonDeath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	mapID := createTestMap(t, db, campaignID)
	charID := createTestCharacter(t, db, campaignID)
	createTestCreatureForSummon(t, db, "owl", "Owl", 11, 1)

	svc := combat.NewService(&testStoreAdapter{queries})
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{
		CampaignID: campaignID,
		MapID:      uuid.NullUUID{UUID: mapID, Valid: true},
		Name:       "death-test",
	})
	require.NoError(t, err)

	summoner, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		CharacterID: charID.String(), ShortID: "AR", DisplayName: "Aragorn",
		HPMax: 45, HPCurrent: 45, AC: 18, SpeedFt: 30,
		PositionCol: "A", PositionRow: 1, IsAlive: true, IsVisible: true,
	})
	require.NoError(t, err)

	familiar, err := svc.SummonCreature(context.Background(), combat.SummonCreatureInput{
		EncounterID: enc.ID, SummonerID: summoner.ID, CreatureRefID: "owl",
		ShortID: "FAM", DisplayName: "Aragorn's Owl", PositionCol: "C", PositionRow: 5,
	})
	require.NoError(t, err)

	// Reduce HP to 0 and mark as dead
	_, err = queries.UpdateCombatantHP(context.Background(), refdata.UpdateCombatantHPParams{
		ID: familiar.ID, HpCurrent: 0, TempHp: 0, IsAlive: false,
	})
	require.NoError(t, err)

	// Re-fetch the creature with updated state
	deadFamiliar, err := queries.GetCombatant(context.Background(), familiar.ID)
	require.NoError(t, err)

	// HandleSummonDeath should remove the creature
	removed, err := svc.HandleSummonDeath(context.Background(), deadFamiliar)
	require.NoError(t, err)
	assert.True(t, removed)

	// Verify removed
	combatants, err := svc.ListCombatantsByEncounterID(context.Background(), enc.ID)
	require.NoError(t, err)
	assert.Len(t, combatants, 1)
}

func TestIntegration_ConcentrationDismissal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	mapID := createTestMap(t, db, campaignID)
	charID := createTestCharacter(t, db, campaignID)

	// Create wolf creature
	_, err := db.Exec(`INSERT INTO creatures (id, name, size, type, ac, hp_formula, hp_average, speed, ability_scores, cr, attacks)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO NOTHING`,
		"wolf", "Wolf", "Medium", "beast", 13, "2d8+2", 11,
		`{"walk":40}`,
		`{"str":12,"dex":15,"con":12,"int":3,"wis":12,"cha":6}`,
		"1/4",
		`[{"name":"Bite","to_hit":4,"damage":"2d4+2","damage_type":"piercing"}]`)
	require.NoError(t, err)

	svc := combat.NewService(&testStoreAdapter{queries})
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{
		CampaignID: campaignID,
		MapID:      uuid.NullUUID{UUID: mapID, Valid: true},
		Name:       "conc-dismiss-test",
	})
	require.NoError(t, err)

	summoner, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		CharacterID: charID.String(), ShortID: "AR", DisplayName: "Aragorn",
		HPMax: 45, HPCurrent: 45, AC: 18, SpeedFt: 30,
		PositionCol: "A", PositionRow: 1, IsAlive: true, IsVisible: true,
	})
	require.NoError(t, err)

	// Summon 3 wolves
	_, err = svc.SummonMultipleCreatures(context.Background(), combat.SummonMultipleInput{
		EncounterID: enc.ID, SummonerID: summoner.ID, CreatureRefID: "wolf",
		BaseShortID: "WF", BaseDisplayName: "Wolf", Quantity: 3,
		PositionCol: "D", PositionRow: 5,
	})
	require.NoError(t, err)

	// Verify 4 combatants (1 PC + 3 wolves)
	combatants, err := svc.ListCombatantsByEncounterID(context.Background(), enc.ID)
	require.NoError(t, err)
	assert.Len(t, combatants, 4)

	// Break concentration -> dismiss all summoned creatures
	removed, err := svc.DismissSummonsByConcentration(context.Background(), enc.ID, summoner.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, removed)

	// Verify only PC remains
	combatants, err = svc.ListCombatantsByEncounterID(context.Background(), enc.ID)
	require.NoError(t, err)
	assert.Len(t, combatants, 1)
	assert.Equal(t, "AR", combatants[0].ShortID)
}

func TestIntegration_CommandOwnershipValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	mapID := createTestMap(t, db, campaignID)
	charID := createTestCharacter(t, db, campaignID)
	createTestCreatureForSummon(t, db, "owl", "Owl", 11, 1)

	svc := combat.NewService(&testStoreAdapter{queries})
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{
		CampaignID: campaignID,
		MapID:      uuid.NullUUID{UUID: mapID, Valid: true},
		Name:       "ownership-test",
	})
	require.NoError(t, err)

	summoner, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		CharacterID: charID.String(), ShortID: "AR", DisplayName: "Aragorn",
		HPMax: 45, HPCurrent: 45, AC: 18, SpeedFt: 30,
		PositionCol: "A", PositionRow: 1, IsAlive: true, IsVisible: true,
	})
	require.NoError(t, err)

	// Create a second character as another player
	charID2 := uuid.New()
	_, err = db.Exec(`INSERT INTO characters (id, campaign_id, name, race, classes, level, ability_scores, hp_max, hp_current, ac, speed_ft, proficiency_bonus, hit_dice_remaining, languages) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		charID2, campaignID, "Legolas", "elf", `[{"class":"ranger","level":5}]`, 5,
		`{"str":12,"dex":18,"con":12,"int":10,"wis":14,"cha":10}`,
		38, 38, 15, 35, 3, `[{"die":"d10","remaining":5}]`, `{Common,Elvish}`)
	require.NoError(t, err)

	otherPlayer, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		CharacterID: charID2.String(), ShortID: "LE", DisplayName: "Legolas",
		HPMax: 38, HPCurrent: 38, AC: 15, SpeedFt: 35,
		PositionCol: "B", PositionRow: 1, IsAlive: true, IsVisible: true,
	})
	require.NoError(t, err)

	// Aragorn summons an owl
	_, err = svc.SummonCreature(context.Background(), combat.SummonCreatureInput{
		EncounterID: enc.ID, SummonerID: summoner.ID, CreatureRefID: "owl",
		ShortID: "FAM", DisplayName: "Aragorn's Owl", PositionCol: "C", PositionRow: 5,
	})
	require.NoError(t, err)

	// Legolas tries to command Aragorn's owl — should fail
	_, err = svc.CommandCreature(context.Background(), combat.CommandCreatureInput{
		EncounterID: enc.ID, SummonerID: otherPlayer.ID,
		CreatureShortID: "FAM", Action: "dismiss",
	})
	assert.ErrorIs(t, err, combat.ErrNotSummoner)

	// Aragorn commands his own owl — should succeed
	result, err := svc.CommandCreature(context.Background(), combat.CommandCreatureInput{
		EncounterID: enc.ID, SummonerID: summoner.ID, SummonerName: "Aragorn",
		CreatureShortID: "FAM", Action: "done",
	})
	require.NoError(t, err)
	assert.Equal(t, "done", result.Action)
}

func TestIntegration_CommandNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	mapID := createTestMap(t, db, campaignID)

	svc := combat.NewService(&testStoreAdapter{queries})
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{
		CampaignID: campaignID,
		MapID:      uuid.NullUUID{UUID: mapID, Valid: true},
		Name:       "notfound-test",
	})
	require.NoError(t, err)

	_, err = svc.CommandCreature(context.Background(), combat.CommandCreatureInput{
		EncounterID: enc.ID, SummonerID: uuid.New(),
		CreatureShortID: "NOPE", Action: "done",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NOPE")
}

func TestIntegration_SummonMultiple(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	mapID := createTestMap(t, db, campaignID)
	charID := createTestCharacter(t, db, campaignID)

	// Create wolf creature
	_, err := db.Exec(`INSERT INTO creatures (id, name, size, type, ac, hp_formula, hp_average, speed, ability_scores, cr, attacks)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO NOTHING`,
		"wolf", "Wolf", "Medium", "beast", 13, "2d8+2", 11,
		`{"walk":40}`,
		`{"str":12,"dex":15,"con":12,"int":3,"wis":12,"cha":6}`,
		"1/4",
		`[{"name":"Bite","to_hit":4,"damage":"2d4+2","damage_type":"piercing"}]`)
	require.NoError(t, err)

	svc := combat.NewService(&testStoreAdapter{queries})
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{
		CampaignID: campaignID,
		MapID:      uuid.NullUUID{UUID: mapID, Valid: true},
		Name:       "multi-summon-test",
	})
	require.NoError(t, err)

	summoner, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		CharacterID: charID.String(), ShortID: "AR", DisplayName: "Aragorn",
		HPMax: 45, HPCurrent: 45, AC: 18, SpeedFt: 30,
		PositionCol: "A", PositionRow: 1, IsAlive: true, IsVisible: true,
	})
	require.NoError(t, err)

	results, err := svc.SummonMultipleCreatures(context.Background(), combat.SummonMultipleInput{
		EncounterID: enc.ID, SummonerID: summoner.ID, CreatureRefID: "wolf",
		BaseShortID: "WF", BaseDisplayName: "Wolf", Quantity: 4,
		PositionCol: "D", PositionRow: 5,
	})
	require.NoError(t, err)
	assert.Len(t, results, 4)

	// Verify summoner links
	for _, r := range results {
		assert.True(t, r.SummonerID.Valid)
		assert.Equal(t, summoner.ID, r.SummonerID.UUID)
	}

	// Verify all in encounter
	combatants, err := svc.ListCombatantsByEncounterID(context.Background(), enc.ID)
	require.NoError(t, err)
	assert.Len(t, combatants, 5) // 1 PC + 4 wolves
}
