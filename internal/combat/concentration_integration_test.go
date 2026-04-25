package combat_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// Phase 118 — concentration cleanup integration tests. Each test exercises the
// real production wiring (service ↔ store ↔ Postgres) for one of the
// "Done when" scenarios listed in docs/phases.md.

// concPCWithAbilities inserts a level-5 wizard character with full ability
// scores so spell casts and CON saves can resolve. Returns the character ID.
func concPCWithAbilities(t *testing.T, db *sql.DB, campaignID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	classes := json.RawMessage(`[{"class":"wizard","level":5}]`)
	scores := json.RawMessage(`{"str":10,"dex":12,"con":14,"int":18,"wis":12,"cha":8}`)
	slots := json.RawMessage(`{"1":{"current":4,"max":4},"2":{"current":3,"max":3},"3":{"current":2,"max":2}}`)
	_, err := db.Exec(`INSERT INTO characters
		(id, campaign_id, name, race, classes, level, ability_scores, hp_max, hp_current, ac, speed_ft, proficiency_bonus, hit_dice_remaining, languages, spell_slots)
		VALUES ($1,$2,'Aria','elf',$3,5,$4,30,30,13,30,3,'[{"die":"d6","remaining":5}]','{Common}',$5)`,
		id, campaignID, classes, scores, slots)
	require.NoError(t, err)
	return id
}

// startConcentrating writes an authoritative concentration row for the caster
// AND adds a matching condition to a target combatant. This simulates the
// state right after a successful concentration spell that applied conditions
// to others — exactly what Phase 118's cleanup must dismantle.
func startConcentrating(t *testing.T, db *sql.DB, casterID uuid.UUID, spellID, spellName string, targets []targetCondition) {
	t.Helper()
	_, err := db.Exec(`UPDATE combatants SET concentration_spell_id=$1, concentration_spell_name=$2 WHERE id=$3`,
		spellID, spellName, casterID)
	require.NoError(t, err)
	for _, tc := range targets {
		conds := []combat.CombatCondition{{
			Condition:         tc.condition,
			SourceCombatantID: casterID.String(),
			SourceSpell:       spellID,
		}}
		raw, err := json.Marshal(conds)
		require.NoError(t, err)
		_, err = db.Exec(`UPDATE combatants SET conditions=$1 WHERE id=$2`, raw, tc.combatantID)
		require.NoError(t, err)
	}
}

type targetCondition struct {
	combatantID uuid.UUID
	condition   string
}

// readConditions returns the combatant's parsed conditions array.
func readConditions(t *testing.T, db *sql.DB, id uuid.UUID) []combat.CombatCondition {
	t.Helper()
	var raw []byte
	require.NoError(t, db.QueryRow(`SELECT conditions FROM combatants WHERE id=$1`, id).Scan(&raw))
	var conds []combat.CombatCondition
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &conds))
	}
	return conds
}

// readConcentration returns the caster's concentration_spell_name (or "").
func readConcentration(t *testing.T, db *sql.DB, id uuid.UUID) string {
	t.Helper()
	var name sql.NullString
	require.NoError(t, db.QueryRow(`SELECT concentration_spell_name FROM combatants WHERE id=$1`, id).Scan(&name))
	if !name.Valid {
		return ""
	}
	return name.String
}

// --- Phase 118 done-when (1): Damage CON-save failure removes spell-sourced
// conditions across ALL affected targets. ---

func TestIntegration_Phase118_DamageSaveFailureCleansEffects(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	charID := concPCWithAbilities(t, db, campaignID)

	svc := combat.NewService(combat.NewStoreAdapter(queries))
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{
		CampaignID: campaignID, Name: "phase118-damage",
	})
	require.NoError(t, err)

	caster, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		CharacterID: charID.String(),
		ShortID:     "AR", DisplayName: "Aria",
		HPMax: 30, HPCurrent: 30, AC: 13, SpeedFt: 30,
		PositionCol: "A", PositionRow: 1, IsAlive: true, IsVisible: true,
	})
	require.NoError(t, err)
	target1, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		ShortID: "G1", DisplayName: "Goblin 1",
		HPMax: 7, HPCurrent: 7, AC: 12, SpeedFt: 30,
		PositionCol: "B", PositionRow: 1, IsAlive: true, IsNPC: true, IsVisible: true,
	})
	require.NoError(t, err)
	target2, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		ShortID: "G2", DisplayName: "Goblin 2",
		HPMax: 7, HPCurrent: 7, AC: 12, SpeedFt: 30,
		PositionCol: "C", PositionRow: 1, IsAlive: true, IsNPC: true, IsVisible: true,
	})
	require.NoError(t, err)

	// Caster is concentrating on Hold Person; both goblins are paralyzed.
	startConcentrating(t, db, caster.ID, "hold-person", "Hold Person", []targetCondition{
		{combatantID: target1.ID, condition: "paralyzed"},
		{combatantID: target2.ID, condition: "paralyzed"},
	})

	// Damage triggers a pending CON save. Caster takes 30 damage → DC 15.
	ps, err := svc.MaybeCreateConcentrationSaveOnDamage(context.Background(), enc.ID, caster.ID, 30)
	require.NoError(t, err)
	require.NotNil(t, ps)
	assert.Equal(t, int32(15), ps.Dc)
	assert.Equal(t, "concentration", ps.Source)
	assert.Equal(t, "con", ps.Ability)

	// Resolve the save as a failure.
	resolved, err := queries.UpdatePendingSaveResult(context.Background(), refdata.UpdatePendingSaveResultParams{
		ID:         ps.ID,
		RollResult: sql.NullInt32{Int32: 5, Valid: true},
		Success:    sql.NullBool{Bool: false, Valid: true},
	})
	require.NoError(t, err)

	cleanup, err := svc.ResolveConcentrationSave(context.Background(), resolved)
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	assert.True(t, cleanup.Broken)
	assert.Equal(t, 2, cleanup.ConditionsRemoved)
	assert.Contains(t, cleanup.ConsolidatedMessage, "💨")
	assert.Contains(t, cleanup.ConsolidatedMessage, "Hold Person")
	assert.Contains(t, cleanup.ConsolidatedMessage, "2 targets")

	// Both targets had their paralyzed condition cleared.
	assert.Empty(t, readConditions(t, db, target1.ID))
	assert.Empty(t, readConditions(t, db, target2.ID))
	// Caster is no longer concentrating.
	assert.Equal(t, "", readConcentration(t, db, caster.ID))
}

// --- Phase 118 done-when (2): incapacitation auto-break removes
// spell-sourced conditions and zones. ---

func TestIntegration_Phase118_IncapacitationAutoBreakCleansAll(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	charID := concPCWithAbilities(t, db, campaignID)

	svc := combat.NewService(combat.NewStoreAdapter(queries))
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{
		CampaignID: campaignID, Name: "phase118-incap",
	})
	require.NoError(t, err)
	caster, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		CharacterID: charID.String(), ShortID: "AR", DisplayName: "Aria",
		HPMax: 30, HPCurrent: 30, AC: 13, SpeedFt: 30,
		PositionCol: "A", PositionRow: 1, IsAlive: true, IsVisible: true,
	})
	require.NoError(t, err)
	target, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		ShortID: "G1", DisplayName: "Goblin", HPMax: 7, HPCurrent: 7, AC: 12, SpeedFt: 30,
		PositionCol: "B", PositionRow: 1, IsAlive: true, IsNPC: true, IsVisible: true,
	})
	require.NoError(t, err)

	startConcentrating(t, db, caster.ID, "web", "Web", []targetCondition{
		{combatantID: target.ID, condition: "restrained"},
	})

	// Also create a concentration-tagged zone owned by the caster.
	_, err = svc.CreateZone(context.Background(), combat.CreateZoneInput{
		EncounterID:           enc.ID,
		SourceCombatantID:     caster.ID,
		SourceSpell:           "web",
		Shape:                 "square",
		OriginCol:             "A",
		OriginRow:             1,
		Dimensions:            json.RawMessage(`{"side_ft":20}`),
		AnchorMode:            "fixed",
		ZoneType:              "difficult_terrain",
		OverlayColor:          "#888888",
		RequiresConcentration: true,
	})
	require.NoError(t, err)

	// Apply stunned to the caster — this is an incapacitating condition that
	// auto-breaks concentration via the ApplyCondition hook.
	_, msgs, err := svc.ApplyCondition(context.Background(), caster.ID, combat.CombatCondition{Condition: "stunned"})
	require.NoError(t, err)

	// The cleanup line was emitted alongside the condition-applied line.
	var foundCleanup bool
	for _, m := range msgs {
		if strings.Contains(m, "💨") && strings.Contains(m, "Web") {
			foundCleanup = true
		}
	}
	assert.True(t, foundCleanup, "expected 💨 cleanup line in apply-condition messages, got %v", msgs)

	// The target's restrained condition (sourced by the caster + Web) is gone.
	assert.Empty(t, readConditions(t, db, target.ID))
	// Concentration columns cleared.
	assert.Equal(t, "", readConcentration(t, db, caster.ID))
	// Concentration zone removed.
	zones, err := svc.ListZonesForEncounter(context.Background(), enc.ID)
	require.NoError(t, err)
	assert.Empty(t, zones)
}

// --- Phase 118 done-when (3): replacing concentration with a new spell
// cleans up the old spell's effects. ---

func TestIntegration_Phase118_ReplacingConcentrationCleansOld(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	charID := concPCWithAbilities(t, db, campaignID)

	svc := combat.NewService(combat.NewStoreAdapter(queries))
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{
		CampaignID: campaignID, Name: "phase118-replace",
	})
	require.NoError(t, err)
	caster, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		CharacterID: charID.String(), ShortID: "AR", DisplayName: "Aria",
		HPMax: 30, HPCurrent: 30, AC: 13, SpeedFt: 30,
		PositionCol: "A", PositionRow: 1, IsAlive: true, IsVisible: true,
	})
	require.NoError(t, err)
	target, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		ShortID: "G1", DisplayName: "Goblin",
		HPMax: 100, HPCurrent: 100, AC: 12, SpeedFt: 30,
		PositionCol: "B", PositionRow: 1, IsAlive: true, IsNPC: true, IsVisible: true,
	})
	require.NoError(t, err)

	// Caster currently concentrates on Hold Person; Goblin is paralyzed.
	startConcentrating(t, db, caster.ID, "hold-person", "Hold Person", []targetCondition{
		{combatantID: target.ID, condition: "paralyzed"},
	})

	// Drive the BreakConcentrationFully path manually (acts as the spellcasting
	// flow's 10a step) to verify the outcome end-to-end.
	cleanup, err := svc.BreakConcentrationFully(context.Background(), combat.BreakConcentrationFullyInput{
		EncounterID: enc.ID,
		CasterID:    caster.ID,
		CasterName:  caster.DisplayName,
		SpellID:     "hold-person",
		SpellName:   "Hold Person",
		Reason:      "cast new concentration spell: Bless",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, cleanup.ConditionsRemoved)
	assert.Contains(t, cleanup.ConsolidatedMessage, "Hold Person")

	// The paralyzed condition on the target is gone.
	assert.Empty(t, readConditions(t, db, target.ID))
	// Caster's concentration columns cleared.
	assert.Equal(t, "", readConcentration(t, db, caster.ID))
}

// --- Phase 118 done-when (4): Invisibility applied by a caster who then drops
// concentration auto-clears the invisible condition. ---

func TestIntegration_Phase118_InvisibilityClearsOnDrop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	charID := concPCWithAbilities(t, db, campaignID)

	svc := combat.NewService(combat.NewStoreAdapter(queries))
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{
		CampaignID: campaignID, Name: "phase118-invis",
	})
	require.NoError(t, err)
	caster, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		CharacterID: charID.String(), ShortID: "AR", DisplayName: "Aria",
		HPMax: 30, HPCurrent: 30, AC: 13, SpeedFt: 30,
		PositionCol: "A", PositionRow: 1, IsAlive: true, IsVisible: true,
	})
	require.NoError(t, err)
	ally, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		ShortID: "BO", DisplayName: "Boromir", HPMax: 30, HPCurrent: 30, AC: 16, SpeedFt: 30,
		PositionCol: "B", PositionRow: 1, IsAlive: true, IsVisible: true,
	})
	require.NoError(t, err)

	// Caster cast Invisibility on the ally and is concentrating on it.
	startConcentrating(t, db, caster.ID, "invisibility", "Invisibility", []targetCondition{
		{combatantID: ally.ID, condition: "invisible"},
	})

	// Voluntary drop via the dashboard endpoint flow.
	cleanup, err := svc.BreakConcentrationFully(context.Background(), combat.BreakConcentrationFullyInput{
		EncounterID: enc.ID,
		CasterID:    caster.ID,
		CasterName:  caster.DisplayName,
		SpellID:     "invisibility",
		SpellName:   "Invisibility",
		Reason:      "voluntary drop",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, cleanup.ConditionsRemoved)

	// The ally is no longer invisible.
	conds := readConditions(t, db, ally.ID)
	for _, c := range conds {
		assert.NotEqual(t, "invisible", c.Condition, "invisible condition must be cleared on concentration drop")
	}
	assert.Equal(t, "", readConcentration(t, db, caster.ID))
}

// --- Phase 118 done-when (5): Combat log posts a single consolidated cleanup
// line. ---

func TestIntegration_Phase118_ConsolidatedLogLine(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	charID := concPCWithAbilities(t, db, campaignID)

	svc := combat.NewService(combat.NewStoreAdapter(queries))
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{
		CampaignID: campaignID, Name: "phase118-log",
	})
	require.NoError(t, err)
	caster, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		CharacterID: charID.String(), ShortID: "AR", DisplayName: "Aria",
		HPMax: 30, HPCurrent: 30, AC: 13, SpeedFt: 30,
		PositionCol: "A", PositionRow: 1, IsAlive: true, IsVisible: true,
	})
	require.NoError(t, err)
	target1, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		ShortID: "G1", DisplayName: "Goblin 1", HPMax: 7, HPCurrent: 7, AC: 12, SpeedFt: 30,
		PositionCol: "B", PositionRow: 1, IsAlive: true, IsNPC: true, IsVisible: true,
	})
	require.NoError(t, err)
	target2, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		ShortID: "G2", DisplayName: "Goblin 2", HPMax: 7, HPCurrent: 7, AC: 12, SpeedFt: 30,
		PositionCol: "C", PositionRow: 1, IsAlive: true, IsNPC: true, IsVisible: true,
	})
	require.NoError(t, err)

	startConcentrating(t, db, caster.ID, "bless", "Bless", []targetCondition{
		{combatantID: target1.ID, condition: "blessed"},
		{combatantID: target2.ID, condition: "blessed"},
	})

	cleanup, err := svc.BreakConcentrationFully(context.Background(), combat.BreakConcentrationFullyInput{
		EncounterID: enc.ID,
		CasterID:    caster.ID,
		CasterName:  caster.DisplayName,
		SpellID:     "bless",
		SpellName:   "Bless",
		Reason:      "voluntary drop",
	})
	require.NoError(t, err)

	// One consolidated 💨 line; format is exactly the spec-mandated one.
	assert.Equal(t,
		"💨 Aria lost concentration on Bless — effects ended on 2 targets.",
		cleanup.ConsolidatedMessage,
	)
}

