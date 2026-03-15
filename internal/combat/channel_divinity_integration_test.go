package combat_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type channelDivinityFixture struct {
	db        *sql.DB
	queries   *refdata.Queries
	svc       *combat.Service
	campaign  uuid.UUID
	encounter refdata.Encounter
	cleric    refdata.Combatant
	turn      refdata.Turn
	charID    uuid.UUID
}

func setupChannelDivinityFixture(t *testing.T, clericLevel int, featureUses int) channelDivinityFixture {
	t.Helper()
	db := sharedDB.AcquireDB(t)
	queries := refdata.New(db)
	svc := combat.NewService(&testStoreAdapter{queries})
	ctx := context.Background()

	// Create campaign and map
	campaignID := createTestCampaign(t, db)
	mapID := createTestMap(t, db, campaignID)

	// Create cleric character
	charID := uuid.New()
	classesJSON, _ := json.Marshal([]map[string]any{{"class": "Cleric", "level": clericLevel}})
	featureUsesJSON, _ := json.Marshal(map[string]int{"channel-divinity": featureUses})
	_, err := db.Exec(`INSERT INTO characters (id, campaign_id, name, race, classes, level, ability_scores, hp_max, hp_current, ac, speed_ft, proficiency_bonus, hit_dice_remaining, languages, feature_uses) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		charID, campaignID, "Thorn", "human", classesJSON, clericLevel,
		`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`,
		35, 35, 18, 30, profBonusForLevel(clericLevel), `[{"die":"d8","remaining":5}]`, `{Common}`,
		featureUsesJSON)
	require.NoError(t, err)

	// Create encounter
	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID:  campaignID,
		MapID:       uuid.NullUUID{UUID: mapID, Valid: true},
		Name:        "Channel Divinity Test",
		Status:      "active",
		RoundNumber: 1,
	})
	require.NoError(t, err)

	// Create cleric combatant
	cleric, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		ShortID:     "TH",
		DisplayName: "Thorn",
		PositionCol: "A",
		PositionRow: 1,
		HpMax:       35,
		HpCurrent:   35,
		Ac:          18,
		Conditions:  json.RawMessage(`[]`),
		IsVisible:   true,
		IsAlive:     true,
		IsNpc:       false,
	})
	require.NoError(t, err)

	// Create turn
	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:        enc.ID,
		CombatantID:        cleric.ID,
		RoundNumber:        1,
		Status:             "active",
		MovementRemainingFt: 30,
	})
	require.NoError(t, err)

	return channelDivinityFixture{
		db:        db,
		queries:   queries,
		svc:       svc,
		campaign:  campaignID,
		encounter: enc,
		cleric:    cleric,
		turn:      turn,
		charID:    charID,
	}
}

func profBonusForLevel(level int) int32 {
	if level >= 17 {
		return 6
	}
	if level >= 13 {
		return 5
	}
	if level >= 9 {
		return 4
	}
	if level >= 5 {
		return 3
	}
	return 2
}

func createUndeadCombatant(t *testing.T, db *sql.DB, queries *refdata.Queries, encounterID uuid.UUID, creatureID, displayName, posCol string, posRow int32, cr string) refdata.Combatant {
	t.Helper()

	// Insert creature if not exists
	_, err := db.Exec(`INSERT INTO creatures (id, name, size, type, ac, hp_formula, hp_average, speed, ability_scores, cr, attacks)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO NOTHING`,
		creatureID, displayName, "Medium", "undead", 13, "2d8+4", 13,
		`{"walk":30}`, `{"str":10,"dex":14,"con":15,"int":6,"wis":8,"cha":5}`, cr, `[]`)
	require.NoError(t, err)

	ctx := context.Background()
	combatant, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID:   encounterID,
		CreatureRefID: sql.NullString{String: creatureID, Valid: true},
		ShortID:       displayName[:2],
		DisplayName:   displayName,
		PositionCol:   posCol,
		PositionRow:   posRow,
		HpMax:         13,
		HpCurrent:     13,
		Ac:            13,
		Conditions:    json.RawMessage(`[]`),
		IsVisible:     true,
		IsAlive:       true,
		IsNpc:         true,
	})
	require.NoError(t, err)
	return combatant
}

// Integration Test: Turn Undead with WIS saves

func TestIntegration_TurnUndead_SavesAndTurnedCondition(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	f := setupChannelDivinityFixture(t, 3, 1)
	ctx := context.Background()

	// Create skeleton (undead, CR 1/4) within 30ft
	skeleton := createUndeadCombatant(t, f.db, f.queries, f.encounter.ID,
		"skeleton", "Skeleton #1", "A", 3, "1/4")

	// Roll 5 for WIS save. Skeleton WIS 8 = -1 mod. Total = 4.
	// DC = 8 + 2 (prof) + 3 (WIS 16 mod) = 13. Fail.
	roller := dice.NewRoller(func(max int) int { return 5 })

	result, err := f.svc.TurnUndead(ctx, combat.TurnUndeadCommand{
		Cleric:       f.cleric,
		Turn:         f.turn,
		CurrentRound: 1,
	}, roller)
	require.NoError(t, err)

	require.Equal(t, 1, len(result.Targets))
	target := result.Targets[0]
	assert.Equal(t, skeleton.DisplayName, target.DisplayName)
	assert.False(t, target.SaveSucceeded)
	assert.True(t, target.Turned)
	assert.False(t, target.Destroyed)
	assert.Equal(t, 13, result.DC)
	assert.Contains(t, result.CombatLog, "Turned")

	// Verify Turned condition is in DB
	updated, err := f.queries.GetCombatant(ctx, skeleton.ID)
	require.NoError(t, err)
	assert.True(t, combat.HasCondition(updated.Conditions, "turned"))

	// Verify channel divinity use was deducted
	char, err := f.queries.GetCharacter(ctx, f.charID)
	require.NoError(t, err)
	var fu map[string]int
	require.NoError(t, json.Unmarshal(char.FeatureUses.RawMessage, &fu))
	assert.Equal(t, 0, fu["channel-divinity"])

	// Verify action was used
	updatedTurn, err := f.queries.GetTurn(ctx, f.turn.ID)
	require.NoError(t, err)
	assert.True(t, updatedTurn.ActionUsed)
}

func TestIntegration_TurnUndead_SaveSucceeds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	f := setupChannelDivinityFixture(t, 3, 1)
	ctx := context.Background()

	createUndeadCombatant(t, f.db, f.queries, f.encounter.ID,
		"skeleton2", "Skeleton #2", "A", 3, "1/4")

	// Roll 18 for WIS save. Total = 18 + (-1) = 17 >= DC 13. Pass.
	roller := dice.NewRoller(func(max int) int { return 18 })

	result, err := f.svc.TurnUndead(ctx, combat.TurnUndeadCommand{
		Cleric:       f.cleric,
		Turn:         f.turn,
		CurrentRound: 1,
	}, roller)
	require.NoError(t, err)

	require.Equal(t, 1, len(result.Targets))
	assert.True(t, result.Targets[0].SaveSucceeded)
	assert.False(t, result.Targets[0].Turned)
	assert.Contains(t, result.CombatLog, "Resists")
}

// Integration Test: Destroy Undead CR threshold

func TestIntegration_DestroyUndead_CRThreshold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	f := setupChannelDivinityFixture(t, 5, 1)
	ctx := context.Background()

	// Skeleton CR 1/4, Cleric level 5 destroys CR 1/2 or lower
	skeleton := createUndeadCombatant(t, f.db, f.queries, f.encounter.ID,
		"skeleton3", "Skeleton #3", "A", 3, "1/4")

	// Roll 3 for WIS save. Total = 3 + (-1) = 2 < DC 14 (8+3+3). Fail → Destroyed.
	roller := dice.NewRoller(func(max int) int { return 3 })

	result, err := f.svc.TurnUndead(ctx, combat.TurnUndeadCommand{
		Cleric:       f.cleric,
		Turn:         f.turn,
		CurrentRound: 1,
	}, roller)
	require.NoError(t, err)

	require.Equal(t, 1, len(result.Targets))
	assert.True(t, result.Targets[0].Destroyed)
	assert.False(t, result.Targets[0].Turned)
	assert.Contains(t, result.CombatLog, "destroyed")

	// Verify HP set to 0 in DB
	updated, err := f.queries.GetCombatant(ctx, skeleton.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), updated.HpCurrent)
	assert.False(t, updated.IsAlive)
}

// Integration Test: Preserve Life HP distribution

func TestIntegration_PreserveLife_DistributesHP(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	f := setupChannelDivinityFixture(t, 5, 1)
	ctx := context.Background()

	// Create an ally combatant (character) at 10/40 HP
	allyCharID := uuid.New()
	_, err := f.db.Exec(`INSERT INTO characters (id, campaign_id, name, race, classes, level, ability_scores, hp_max, hp_current, ac, speed_ft, proficiency_bonus, hit_dice_remaining, languages) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		allyCharID, f.campaign, "Ally", "elf", `[{"class":"Fighter","level":5}]`, 5,
		`{"str":16,"dex":14,"con":14,"int":10,"wis":12,"cha":10}`,
		40, 10, 18, 30, 3, `[{"die":"d10","remaining":5}]`, `{Common}`)
	require.NoError(t, err)

	ally, err := f.queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: f.encounter.ID,
		CharacterID: uuid.NullUUID{UUID: allyCharID, Valid: true},
		ShortID:     "AL",
		DisplayName: "Ally",
		PositionCol: "A",
		PositionRow: 3, // within 30ft
		HpMax:       40,
		HpCurrent:   10,
		Ac:          18,
		Conditions:  json.RawMessage(`[]`),
		IsVisible:   true,
		IsAlive:     true,
		IsNpc:       false,
	})
	require.NoError(t, err)

	// Budget = 5 * 5 = 25. Ally at 10/40 HP. Half max = 20. Can heal up to 10 (20-10).
	result, err := f.svc.PreserveLife(ctx, combat.PreserveLifeCommand{
		Cleric: f.cleric,
		Turn:   f.turn,
		TargetHealing: map[string]int32{
			ally.ID.String(): 10,
		},
	})
	require.NoError(t, err)

	require.Equal(t, 1, len(result.HealedTargets))
	assert.Equal(t, int32(10), result.HealedTargets[0].HPRestored)
	assert.Equal(t, int32(20), result.HealedTargets[0].HPAfter)
	assert.Contains(t, result.CombatLog, "Preserve Life")

	// Verify HP in DB
	updated, err := f.queries.GetCombatant(ctx, ally.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(20), updated.HpCurrent)

	// Verify action used and feature deducted
	updatedTurn, err := f.queries.GetTurn(ctx, f.turn.ID)
	require.NoError(t, err)
	assert.True(t, updatedTurn.ActionUsed)
}

// Integration Test: DM-queue routing for narrative options

func TestIntegration_ChannelDivinityDMQueue(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	f := setupChannelDivinityFixture(t, 3, 1)
	ctx := context.Background()

	result, err := f.svc.ChannelDivinityDMQueue(ctx, combat.ChannelDivinityDMQueueCommand{
		Caster:     f.cleric,
		Turn:       f.turn,
		OptionName: "Knowledge of the Ages",
		ClassName:  "Cleric",
	})
	require.NoError(t, err)

	assert.Contains(t, result.CombatLog, "Knowledge of the Ages")
	assert.Contains(t, result.CombatLog, "#dm-queue")
	assert.Equal(t, "Knowledge of the Ages", result.OptionName)
	assert.Equal(t, 0, result.UsesLeft)

	// Verify action used
	updatedTurn, err := f.queries.GetTurn(ctx, f.turn.ID)
	require.NoError(t, err)
	assert.True(t, updatedTurn.ActionUsed)

	// Verify feature uses deducted in DB
	char, err := f.queries.GetCharacter(ctx, f.charID)
	require.NoError(t, err)
	var fu map[string]int
	require.NoError(t, json.Unmarshal(char.FeatureUses.RawMessage, &fu))
	assert.Equal(t, 0, fu["channel-divinity"])
}

// Integration Test: Paladin Sacred Weapon

func TestIntegration_SacredWeapon(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := sharedDB.AcquireDB(t)
	queries := refdata.New(db)
	svc := combat.NewService(&testStoreAdapter{queries})
	ctx := context.Background()

	campaignID := createTestCampaign(t, db)
	mapID := createTestMap(t, db, campaignID)

	// Create paladin character
	charID := uuid.New()
	classesJSON, _ := json.Marshal([]map[string]any{{"class": "Paladin", "level": 3}})
	featureUsesJSON, _ := json.Marshal(map[string]int{"channel-divinity": 1})
	_, err := db.Exec(`INSERT INTO characters (id, campaign_id, name, race, classes, level, ability_scores, hp_max, hp_current, ac, speed_ft, proficiency_bonus, hit_dice_remaining, languages, feature_uses) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		charID, campaignID, "Oath", "human", classesJSON, 3,
		`{"str":16,"dex":10,"con":14,"int":10,"wis":12,"cha":16}`,
		30, 30, 18, 30, 2, `[{"die":"d10","remaining":3}]`, `{Common}`,
		featureUsesJSON)
	require.NoError(t, err)

	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID:  campaignID,
		MapID:       uuid.NullUUID{UUID: mapID, Valid: true},
		Name:        "Sacred Weapon Test",
		Status:      "active",
		RoundNumber: 1,
	})
	require.NoError(t, err)

	paladin, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		ShortID:     "OA",
		DisplayName: "Oath",
		PositionCol: "A",
		PositionRow: 1,
		HpMax:       30,
		HpCurrent:   30,
		Ac:          18,
		Conditions:  json.RawMessage(`[]`),
		IsVisible:   true,
		IsAlive:     true,
		IsNpc:       false,
	})
	require.NoError(t, err)

	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:        enc.ID,
		CombatantID:        paladin.ID,
		RoundNumber:        1,
		Status:             "active",
		MovementRemainingFt: 30,
	})
	require.NoError(t, err)

	result, err := svc.SacredWeapon(ctx, combat.SacredWeaponCommand{
		Paladin:      paladin,
		Turn:         turn,
		CurrentRound: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, result.CHAModifier) // CHA 16 → +3
	assert.Contains(t, result.CombatLog, "Sacred Weapon")
	assert.Contains(t, result.CombatLog, "+3")

	// Verify sacred_weapon condition in DB
	updated, err := queries.GetCombatant(ctx, paladin.ID)
	require.NoError(t, err)
	assert.True(t, combat.HasCondition(updated.Conditions, "sacred_weapon"))
}

// Integration Test: Usage tracking validation — no uses remaining

func TestIntegration_ChannelDivinity_NoUsesRemaining(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	f := setupChannelDivinityFixture(t, 3, 0) // 0 uses remaining
	ctx := context.Background()

	createUndeadCombatant(t, f.db, f.queries, f.encounter.ID,
		"skeleton4", "Skeleton #4", "A", 3, "1/4")

	roller := dice.NewRoller(func(max int) int { return 10 })
	_, err := f.svc.TurnUndead(ctx, combat.TurnUndeadCommand{
		Cleric:       f.cleric,
		Turn:         f.turn,
		CurrentRound: 1,
	}, roller)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no Channel Divinity uses remaining")
}

// Integration Test: Vow of Enmity (Vengeance Paladin)

func TestIntegration_VowOfEnmity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := sharedDB.AcquireDB(t)
	queries := refdata.New(db)
	svc := combat.NewService(&testStoreAdapter{queries})
	ctx := context.Background()

	campaignID := createTestCampaign(t, db)
	mapID := createTestMap(t, db, campaignID)

	// Create paladin character
	charID := uuid.New()
	classesJSON, _ := json.Marshal([]map[string]any{{"class": "Paladin", "level": 3}})
	featureUsesJSON, _ := json.Marshal(map[string]int{"channel-divinity": 1})
	_, err := db.Exec(`INSERT INTO characters (id, campaign_id, name, race, classes, level, ability_scores, hp_max, hp_current, ac, speed_ft, proficiency_bonus, hit_dice_remaining, languages, feature_uses) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		charID, campaignID, "Oath", "human", classesJSON, 3,
		`{"str":16,"dex":10,"con":14,"int":10,"wis":12,"cha":16}`,
		30, 30, 18, 30, 2, `[{"die":"d10","remaining":3}]`, `{Common}`,
		featureUsesJSON)
	require.NoError(t, err)

	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID:  campaignID,
		MapID:       uuid.NullUUID{UUID: mapID, Valid: true},
		Name:        "Vow of Enmity Test",
		Status:      "active",
		RoundNumber: 1,
	})
	require.NoError(t, err)

	paladin, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		ShortID:     "OA",
		DisplayName: "Oath",
		PositionCol: "A",
		PositionRow: 1,
		HpMax:       30,
		HpCurrent:   30,
		Ac:          18,
		Conditions:  json.RawMessage(`[]`),
		IsVisible:   true,
		IsAlive:     true,
		IsNpc:       false,
	})
	require.NoError(t, err)

	// Create target NPC within 10ft
	target, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		ShortID:     "FI",
		DisplayName: "Fiend",
		PositionCol: "B",
		PositionRow: 1,
		HpMax:       50,
		HpCurrent:   50,
		Ac:          15,
		Conditions:  json.RawMessage(`[]`),
		IsVisible:   true,
		IsAlive:     true,
		IsNpc:       true,
	})
	require.NoError(t, err)

	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:        enc.ID,
		CombatantID:        paladin.ID,
		RoundNumber:        1,
		Status:             "active",
		MovementRemainingFt: 30,
	})
	require.NoError(t, err)

	result, err := svc.VowOfEnmity(ctx, combat.VowOfEnmityCommand{
		Paladin:      paladin,
		Target:       target,
		Turn:         turn,
		CurrentRound: 1,
	})
	require.NoError(t, err)
	assert.Contains(t, result.CombatLog, "Vow of Enmity")
	assert.Contains(t, result.CombatLog, "Fiend")
	assert.Equal(t, 0, result.UsesLeft)

	// Verify vow_of_enmity condition is in DB on the target
	updated, err := queries.GetCombatant(ctx, target.ID)
	require.NoError(t, err)
	assert.True(t, combat.HasCondition(updated.Conditions, "vow_of_enmity"))

	// Verify channel divinity use was deducted
	char, err := queries.GetCharacter(ctx, charID)
	require.NoError(t, err)
	var fu map[string]int
	require.NoError(t, json.Unmarshal(char.FeatureUses.RawMessage, &fu))
	assert.Equal(t, 0, fu["channel-divinity"])

	// Verify action was used
	updatedTurn, err := queries.GetTurn(ctx, turn.ID)
	require.NoError(t, err)
	assert.True(t, updatedTurn.ActionUsed)
}

// Suppress unused import warnings
var _ = pqtype.NullRawMessage{}
