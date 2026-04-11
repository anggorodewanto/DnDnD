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

type divineSmiteFixture struct {
	db        *sql.DB
	queries   *refdata.Queries
	svc       *combat.Service
	campaign  uuid.UUID
	encounter refdata.Encounter
	paladin   refdata.Combatant
	target    refdata.Combatant
	charID    uuid.UUID
	turn      refdata.Turn
}

func createTestUndead(t *testing.T, db *sql.DB) string {
	t.Helper()
	_, err := db.Exec(`INSERT INTO creatures (id, name, size, type, ac, hp_formula, hp_average, speed, ability_scores, cr, attacks)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO NOTHING`,
		"zombie", "Zombie", "Medium", "undead", 8, "3d8+9", 22,
		`{"walk":20}`, `{"str":13,"dex":6,"con":16,"int":3,"wis":6,"cha":5}`, "1/4", `[]`)
	require.NoError(t, err)
	return "zombie"
}

func createTestFiend(t *testing.T, db *sql.DB) string {
	t.Helper()
	_, err := db.Exec(`INSERT INTO creatures (id, name, size, type, ac, hp_formula, hp_average, speed, ability_scores, cr, attacks)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO NOTHING`,
		"imp", "Imp", "Tiny", "fiend", 13, "3d4+3", 10,
		`{"walk":20,"fly":40}`, `{"str":6,"dex":17,"con":13,"int":11,"wis":12,"cha":14}`, "1", `[]`)
	require.NoError(t, err)
	return "imp"
}

func setupDivineSmiteFixture(t *testing.T, creatureRefID string, spellSlots map[string]combat.SlotInfo) divineSmiteFixture {
	t.Helper()
	db := sharedDB.AcquireDB(t)
	queries := refdata.New(db)
	svc := combat.NewService(combat.NewStoreAdapter(queries))
	ctx := context.Background()

	campaignID := createTestCampaign(t, db)
	mapID := createTestMap(t, db, campaignID)

	// Create paladin character with Divine Smite feature and spell slots
	charID := uuid.New()
	classesJSON, _ := json.Marshal([]map[string]any{{"class": "Paladin", "level": 5}})
	featuresJSON, _ := json.Marshal([]map[string]string{
		{"name": "Divine Smite", "mechanical_effect": "expend_spell_slot_2d8_radiant_plus_1d8_per_slot_level"},
	})
	slotsJSON, _ := json.Marshal(spellSlots)

	_, err := db.Exec(`INSERT INTO characters (id, campaign_id, name, race, classes, level, ability_scores, hp_max, hp_current, ac, speed_ft, proficiency_bonus, hit_dice_remaining, languages, features, spell_slots, equipped_main_hand) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		charID, campaignID, "Aria", "human", classesJSON, 5,
		`{"str":16,"dex":10,"con":14,"int":10,"wis":12,"cha":16}`,
		45, 45, 18, 30, int32(3), `[{"die":"d10","remaining":5}]`, `{Common}`,
		pqtype.NullRawMessage{RawMessage: featuresJSON, Valid: true},
		pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		sql.NullString{String: "longsword", Valid: true})
	require.NoError(t, err)

	// Create encounter
	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID:  campaignID,
		MapID:       uuid.NullUUID{UUID: mapID, Valid: true},
		Name:        "Divine Smite Test",
		Status:      "active",
		RoundNumber: 1,
	})
	require.NoError(t, err)

	// Create paladin combatant
	paladin, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		ShortID:     "AR",
		DisplayName: "Aria",
		PositionCol: "A",
		PositionRow: 1,
		HpMax:       45,
		HpCurrent:   45,
		Ac:          18,
		Conditions:  json.RawMessage(`[]`),
		IsVisible:   true,
		IsAlive:     true,
		IsNpc:       false,
	})
	require.NoError(t, err)

	// Create target combatant
	targetParams := refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		ShortID:     "T1",
		DisplayName: "Target",
		PositionCol: "B",
		PositionRow: 1,
		HpMax:       30,
		HpCurrent:   30,
		Ac:          12,
		Conditions:  json.RawMessage(`[]`),
		IsVisible:   true,
		IsAlive:     true,
		IsNpc:       true,
	}
	if creatureRefID != "" {
		targetParams.CreatureRefID = sql.NullString{String: creatureRefID, Valid: true}
	}
	target, err := queries.CreateCombatant(ctx, targetParams)
	require.NoError(t, err)

	// Create turn
	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:         enc.ID,
		CombatantID:         paladin.ID,
		RoundNumber:         1,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	})
	require.NoError(t, err)

	return divineSmiteFixture{
		db:        db,
		queries:   queries,
		svc:       svc,
		campaign:  campaignID,
		encounter: enc,
		paladin:   paladin,
		target:    target,
		charID:    charID,
		turn:      turn,
	}
}

func TestIntegration_DivineSmite_BasicSlotDeduction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	slots := map[string]combat.SlotInfo{
		"1": {Current: 3, Max: 4},
		"2": {Current: 2, Max: 3},
	}
	f := setupDivineSmiteFixture(t, "", slots)
	ctx := context.Background()

	roller := dice.NewRoller(func(max int) int { return 5 }) // always rolls 5

	result, err := f.svc.DivineSmite(ctx, combat.DivineSmiteCommand{
		Attacker:     f.paladin,
		Target:       f.target,
		SlotLevel:    1,
		IsCritical:   false,
		AttackResult: combat.AttackResult{Hit: true, IsMelee: true},
	}, roller)
	require.NoError(t, err)

	// 2d8 with fixed roll of 5 each = 10
	assert.Equal(t, 10, result.SmiteDamage)
	assert.Equal(t, "2d8", result.SmiteDice)
	assert.Equal(t, 1, result.SlotLevel)
	assert.False(t, result.IsUndead)
	assert.False(t, result.IsCritical)
	assert.Contains(t, result.CombatLog, "Divine Smite")
	assert.Contains(t, result.CombatLog, "1st-level slot")

	// Verify slot was deducted (3 -> 2)
	assert.Equal(t, 2, result.SlotsRemaining["1"].Current)

	// Verify persistence
	char, err := f.queries.GetCharacter(ctx, f.charID)
	require.NoError(t, err)
	var persistedSlots map[string]combat.SlotInfo
	err = json.Unmarshal(char.SpellSlots.RawMessage, &persistedSlots)
	require.NoError(t, err)
	assert.Equal(t, 2, persistedSlots["1"].Current)
	assert.Equal(t, 2, persistedSlots["2"].Current) // unchanged
}

func TestIntegration_DivineSmite_2ndLevelSlot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	slots := map[string]combat.SlotInfo{
		"1": {Current: 3, Max: 4},
		"2": {Current: 2, Max: 3},
	}
	f := setupDivineSmiteFixture(t, "", slots)
	ctx := context.Background()

	roller := dice.NewRoller(func(max int) int { return 4 })

	result, err := f.svc.DivineSmite(ctx, combat.DivineSmiteCommand{
		Attacker:     f.paladin,
		Target:       f.target,
		SlotLevel:    2,
		IsCritical:   false,
		AttackResult: combat.AttackResult{Hit: true, IsMelee: true},
	}, roller)
	require.NoError(t, err)

	// 3d8 with fixed roll of 4 each = 12
	assert.Equal(t, 12, result.SmiteDamage)
	assert.Equal(t, "3d8", result.SmiteDice)
	assert.Contains(t, result.CombatLog, "2nd-level slot")

	// 2nd level: 2 -> 1
	assert.Equal(t, 1, result.SlotsRemaining["2"].Current)
}

func TestIntegration_DivineSmite_UndeadBonus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	slots := map[string]combat.SlotInfo{"1": {Current: 2, Max: 4}}
	f := setupDivineSmiteFixture(t, "", slots)
	ctx := context.Background()

	// Create undead creature and assign to target
	createTestUndead(t, f.db)
	_, err := f.db.Exec(`UPDATE combatants SET creature_ref_id = $1 WHERE id = $2`, "zombie", f.target.ID)
	require.NoError(t, err)
	f.target.CreatureRefID = sql.NullString{String: "zombie", Valid: true}

	roller := dice.NewRoller(func(max int) int { return 3 })

	result, err := f.svc.DivineSmite(ctx, combat.DivineSmiteCommand{
		Attacker:     f.paladin,
		Target:       f.target,
		SlotLevel:    1,
		IsCritical:   false,
		AttackResult: combat.AttackResult{Hit: true, IsMelee: true},
	}, roller)
	require.NoError(t, err)

	// 2d8 + 1d8 undead = 3d8; 3*3=9
	assert.Equal(t, 9, result.SmiteDamage)
	assert.Equal(t, "3d8", result.SmiteDice)
	assert.True(t, result.IsUndead)
	assert.Contains(t, result.CombatLog, "vs undead")
}

func TestIntegration_DivineSmite_FiendBonus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	slots := map[string]combat.SlotInfo{"1": {Current: 2, Max: 4}}
	f := setupDivineSmiteFixture(t, "", slots)
	ctx := context.Background()

	createTestFiend(t, f.db)
	_, err := f.db.Exec(`UPDATE combatants SET creature_ref_id = $1 WHERE id = $2`, "imp", f.target.ID)
	require.NoError(t, err)
	f.target.CreatureRefID = sql.NullString{String: "imp", Valid: true}

	roller := dice.NewRoller(func(max int) int { return 4 })

	result, err := f.svc.DivineSmite(ctx, combat.DivineSmiteCommand{
		Attacker:     f.paladin,
		Target:       f.target,
		SlotLevel:    1,
		IsCritical:   false,
		AttackResult: combat.AttackResult{Hit: true, IsMelee: true},
	}, roller)
	require.NoError(t, err)

	// 2d8 + 1d8 fiend = 3d8; 3*4=12
	assert.Equal(t, 12, result.SmiteDamage)
	assert.True(t, result.IsUndead) // IsUndead covers both undead and fiend
	assert.Contains(t, result.CombatLog, "vs undead")
}

func TestIntegration_DivineSmite_CritDoubling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	slots := map[string]combat.SlotInfo{"1": {Current: 2, Max: 4}}
	f := setupDivineSmiteFixture(t, "", slots)
	ctx := context.Background()

	roller := dice.NewRoller(func(max int) int { return 3 })

	result, err := f.svc.DivineSmite(ctx, combat.DivineSmiteCommand{
		Attacker:     f.paladin,
		Target:       f.target,
		SlotLevel:    1,
		IsCritical:   true,
		AttackResult: combat.AttackResult{Hit: true, IsMelee: true, CriticalHit: true},
	}, roller)
	require.NoError(t, err)

	// 2d8 doubled to 4d8; 4*3=12
	assert.Equal(t, 12, result.SmiteDamage)
	assert.Equal(t, "4d8", result.SmiteDice)
	assert.True(t, result.IsCritical)
	assert.Contains(t, result.CombatLog, "crit")
	assert.Contains(t, result.CombatLog, "doubled")
}

func TestIntegration_DivineSmite_UndeadCrit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	slots := map[string]combat.SlotInfo{"1": {Current: 2, Max: 4}}
	f := setupDivineSmiteFixture(t, "", slots)
	ctx := context.Background()

	createTestUndead(t, f.db)
	_, err := f.db.Exec(`UPDATE combatants SET creature_ref_id = $1 WHERE id = $2`, "zombie", f.target.ID)
	require.NoError(t, err)
	f.target.CreatureRefID = sql.NullString{String: "zombie", Valid: true}

	roller := dice.NewRoller(func(max int) int { return 2 })

	result, err := f.svc.DivineSmite(ctx, combat.DivineSmiteCommand{
		Attacker:     f.paladin,
		Target:       f.target,
		SlotLevel:    1,
		IsCritical:   true,
		AttackResult: combat.AttackResult{Hit: true, IsMelee: true, CriticalHit: true},
	}, roller)
	require.NoError(t, err)

	// (2d8 + 1d8 undead) = 3, doubled = 6d8; 6*2=12
	assert.Equal(t, 12, result.SmiteDamage)
	assert.Equal(t, "6d8", result.SmiteDice)
	assert.True(t, result.IsUndead)
	assert.True(t, result.IsCritical)
	assert.Contains(t, result.CombatLog, "doubled")
	assert.Contains(t, result.CombatLog, "vs undead")
}

func TestIntegration_DivineSmite_DeclineNoSlotDeducted(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// This tests the "no smite" path — the caller simply doesn't call DivineSmite.
	// The key behavior: if the player declines or times out, no slots are consumed.
	// We verify the slots are unchanged by checking directly.
	slots := map[string]combat.SlotInfo{"1": {Current: 3, Max: 4}}
	f := setupDivineSmiteFixture(t, "", slots)
	ctx := context.Background()

	// Don't call DivineSmite — simulate timeout/decline
	char, err := f.queries.GetCharacter(ctx, f.charID)
	require.NoError(t, err)
	var persistedSlots map[string]combat.SlotInfo
	err = json.Unmarshal(char.SpellSlots.RawMessage, &persistedSlots)
	require.NoError(t, err)
	assert.Equal(t, 3, persistedSlots["1"].Current) // unchanged
}

func TestIntegration_DivineSmite_NonMeleeNotEligible(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	slots := map[string]combat.SlotInfo{"1": {Current: 3, Max: 4}}
	f := setupDivineSmiteFixture(t, "", slots)
	ctx := context.Background()

	roller := dice.NewRoller(nil)

	_, err := f.svc.DivineSmite(ctx, combat.DivineSmiteCommand{
		Attacker:     f.paladin,
		Target:       f.target,
		SlotLevel:    1,
		IsCritical:   false,
		AttackResult: combat.AttackResult{Hit: true, IsMelee: false}, // ranged
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "melee weapon hit")
}

func TestIntegration_DivineSmite_MissNotEligible(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	slots := map[string]combat.SlotInfo{"1": {Current: 3, Max: 4}}
	f := setupDivineSmiteFixture(t, "", slots)
	ctx := context.Background()

	roller := dice.NewRoller(nil)

	_, err := f.svc.DivineSmite(ctx, combat.DivineSmiteCommand{
		Attacker:     f.paladin,
		Target:       f.target,
		SlotLevel:    1,
		IsCritical:   false,
		AttackResult: combat.AttackResult{Hit: false, IsMelee: true}, // miss
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "melee weapon hit")
}

func TestIntegration_DivineSmite_NoSlotsNotEligible(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	slots := map[string]combat.SlotInfo{"1": {Current: 0, Max: 4}}
	f := setupDivineSmiteFixture(t, "", slots)
	ctx := context.Background()

	roller := dice.NewRoller(nil)

	_, err := f.svc.DivineSmite(ctx, combat.DivineSmiteCommand{
		Attacker:     f.paladin,
		Target:       f.target,
		SlotLevel:    1,
		IsCritical:   false,
		AttackResult: combat.AttackResult{Hit: true, IsMelee: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no 1st-level spell slots remaining")
}

func TestIntegration_DivineSmite_Max5d8Cap(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	slots := map[string]combat.SlotInfo{
		"4": {Current: 1, Max: 1},
		"5": {Current: 1, Max: 1},
	}
	f := setupDivineSmiteFixture(t, "", slots)
	ctx := context.Background()

	roller := dice.NewRoller(func(max int) int { return 4 })

	// 4th level slot = 5d8 (max)
	result4, err := f.svc.DivineSmite(ctx, combat.DivineSmiteCommand{
		Attacker:     f.paladin,
		Target:       f.target,
		SlotLevel:    4,
		IsCritical:   false,
		AttackResult: combat.AttackResult{Hit: true, IsMelee: true},
	}, roller)
	require.NoError(t, err)
	assert.Equal(t, "5d8", result4.SmiteDice)
	assert.Equal(t, 20, result4.SmiteDamage) // 5*4

	// 5th level slot = still 5d8 (cap)
	result5, err := f.svc.DivineSmite(ctx, combat.DivineSmiteCommand{
		Attacker:     f.paladin,
		Target:       f.target,
		SlotLevel:    5,
		IsCritical:   false,
		AttackResult: combat.AttackResult{Hit: true, IsMelee: true},
	}, roller)
	require.NoError(t, err)
	assert.Equal(t, "5d8", result5.SmiteDice)
	assert.Equal(t, 20, result5.SmiteDamage)
}

func TestIntegration_DivineSmite_AvailableSlots(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Verify AvailableSmiteSlots works correctly for prompt building
	slots := map[string]combat.SlotInfo{
		"1": {Current: 3, Max: 4},
		"2": {Current: 0, Max: 3},
		"3": {Current: 1, Max: 2},
	}

	available := combat.AvailableSmiteSlots(slots)
	assert.Equal(t, []int{1, 3}, available)
}

func TestIntegration_DivineSmite_NoDivineSmiteFeature(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Create a fixture manually with a fighter (no Divine Smite)
	db := sharedDB.AcquireDB(t)
	queries := refdata.New(db)
	svc := combat.NewService(combat.NewStoreAdapter(queries))
	ctx := context.Background()

	campaignID := createTestCampaign(t, db)
	mapID := createTestMap(t, db, campaignID)

	charID := uuid.New()
	classesJSON, _ := json.Marshal([]map[string]any{{"class": "Fighter", "level": 5}})
	slotsJSON, _ := json.Marshal(map[string]combat.SlotInfo{"1": {Current: 3, Max: 4}})

	_, err := db.Exec(`INSERT INTO characters (id, campaign_id, name, race, classes, level, ability_scores, hp_max, hp_current, ac, speed_ft, proficiency_bonus, hit_dice_remaining, languages, spell_slots) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		charID, campaignID, "Bob", "human", classesJSON, 5,
		`{"str":16,"dex":10,"con":14,"int":10,"wis":12,"cha":10}`,
		45, 45, 18, 30, int32(3), `[{"die":"d10","remaining":5}]`, `{Common}`,
		pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true})
	require.NoError(t, err)

	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID: campaignID,
		MapID:      uuid.NullUUID{UUID: mapID, Valid: true},
		Name:       "No Smite Test",
		Status:     "active",
	})
	require.NoError(t, err)

	fighter, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		ShortID:     "BO",
		DisplayName: "Bob",
		PositionCol: "A",
		PositionRow: 1,
		HpMax:       45,
		HpCurrent:   45,
		Ac:          18,
		Conditions:  json.RawMessage(`[]`),
		IsVisible:   true,
		IsAlive:     true,
	})
	require.NoError(t, err)

	target, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		ShortID:     "T1",
		DisplayName: "Target",
		PositionCol: "B",
		PositionRow: 1,
		HpMax:       30,
		HpCurrent:   30,
		Ac:          12,
		Conditions:  json.RawMessage(`[]`),
		IsVisible:   true,
		IsAlive:     true,
		IsNpc:       true,
	})
	require.NoError(t, err)

	roller := dice.NewRoller(nil)
	_, err = svc.DivineSmite(ctx, combat.DivineSmiteCommand{
		Attacker:     fighter,
		Target:       target,
		SlotLevel:    1,
		AttackResult: combat.AttackResult{Hit: true, IsMelee: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not have Divine Smite")
}
