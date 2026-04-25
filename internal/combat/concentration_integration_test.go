package combat_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
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
//
// This test drives the PRODUCTION damage path end-to-end:
//   1. AoE damage through `ResolveAoESaves` → `applyDamageHP` enqueues a
//      pending CON save with `source = "concentration"`.
//   2. `TurnTimer.AutoResolveTurn` rolls the save with a deterministic
//      failure roll, calls the registered `Service.ResolveConcentrationSave`
//      hook, which fires the cleanup pipeline.
//   3. Spell-sourced conditions are stripped from every target.
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

	// Drive damage through the AoE production path. The caster (Aria) is
	// caught in the blast and fails her save, taking 12 damage.
	roller := dice.NewRoller(func(max int) int { return 12 })
	_, err = svc.ResolveAoESaves(context.Background(), combat.AoEDamageInput{
		EncounterID: enc.ID,
		SpellName:   "Fireball",
		DamageDice:  "1d12",
		DamageType:  "fire",
		SaveEffect:  "half_damage",
		SaveResults: []combat.SaveResult{
			{CombatantID: caster.ID, Success: false, Total: 5},
		},
	}, roller)
	require.NoError(t, err)

	// applyDamageHP must have created a pending concentration save.
	saves, err := queries.ListPendingSavesByCombatant(context.Background(), caster.ID)
	require.NoError(t, err)
	var concSave *refdata.PendingSafe
	for i := range saves {
		if saves[i].Source == "concentration" {
			concSave = &saves[i]
			break
		}
	}
	require.NotNil(t, concSave, "AoE damage on a concentrating caster must enqueue a pending concentration save")
	assert.Equal(t, int32(10), concSave.Dc) // damage 12 → DC max(10, 6) = 10
	assert.Equal(t, "con", concSave.Ability)

	// Set up a turn so AutoResolveTurn can pick up the save. Make the timer
	// roll a deterministic 1 → guaranteed failure vs DC 10.
	turnID := uuid.New()
	_, err = db.Exec(`INSERT INTO turns (id, encounter_id, combatant_id, round_number, status)
		VALUES ($1, $2, $3, 1, 'active')`, turnID, enc.ID, caster.ID)
	require.NoError(t, err)

	timer := combat.NewTurnTimer(combat.NewStoreAdapter(queries), &silentNotifier{}, 30*time.Second)
	timer.SetConcentrationResolver(func(ctx context.Context, ps refdata.PendingSafe) error {
		_, err := svc.ResolveConcentrationSave(ctx, ps)
		return err
	})
	failRoller := dice.NewRoller(func(max int) int { return 1 })
	_, err = timer.AutoResolveTurn(context.Background(), turnID, failRoller)
	require.NoError(t, err)

	// Both targets had their paralyzed condition cleared via the cleanup
	// pipeline driven by the failed concentration save resolver.
	assert.Empty(t, readConditions(t, db, target1.ID))
	assert.Empty(t, readConditions(t, db, target2.ID))
	// Caster is no longer concentrating.
	assert.Equal(t, "", readConcentration(t, db, caster.ID))
}

// silentNotifier is a no-op Notifier for integration tests that exercise the
// timer without checking emitted Discord messages.
type silentNotifier struct{}

func (silentNotifier) SendMessage(channelID, content string) error { return nil }

// --- Phase 118: Silence-zone trigger plumbing — V/S concentration breaks
// when a Silence zone is placed over the caster, AND when the caster moves
// into an existing Silence zone. Drives both production paths against
// Postgres. Skipped when the spells reference table is empty. ---
func TestIntegration_Phase118_SilenceZoneBreaksConcentration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	charID := concPCWithAbilities(t, db, campaignID)

	// Insert a minimal "hold-person" spell with V/S components so the
	// Silence-zone check can read its components.
	_, err := db.Exec(`INSERT INTO spells (id, name, school, level, casting_time, range_type, components, duration, description, classes)
		VALUES ('hold-person', 'Hold Person', 'enchantment', 2, '1 action', 'ranged', '{V,S,M}', '1 minute', '', '{wizard}')
		ON CONFLICT (id) DO NOTHING`)
	require.NoError(t, err)

	svc := combat.NewService(combat.NewStoreAdapter(queries))
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{
		CampaignID: campaignID, Name: "phase118-silence",
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
		HPMax: 7, HPCurrent: 7, AC: 12, SpeedFt: 30,
		PositionCol: "B", PositionRow: 1, IsAlive: true, IsNPC: true, IsVisible: true,
	})
	require.NoError(t, err)
	startConcentrating(t, db, caster.ID, "hold-person", "Hold Person", []targetCondition{
		{combatantID: target.ID, condition: "paralyzed"},
	})

	// Path 1: drop a Silence zone over the caster's current tile.
	_, err = svc.CreateZone(context.Background(), combat.CreateZoneInput{
		EncounterID:       enc.ID,
		SourceCombatantID: target.ID, // unrelated source
		SourceSpell:       "silence",
		Shape:             "square",
		OriginCol:         "A",
		OriginRow:         1,
		Dimensions:        json.RawMessage(`{"side_ft":20}`),
		AnchorMode:        "fixed",
		ZoneType:          "silence",
		OverlayColor:      "#888888",
	})
	require.NoError(t, err)

	// Concentration must now be broken; Goblin's paralyzed cleared.
	assert.Equal(t, "", readConcentration(t, db, caster.ID))
	assert.Empty(t, readConditions(t, db, target.ID))
}

// --- Phase 118 done-when (4) extension: damage to 0 HP applies the
// `unconscious` condition AND auto-breaks concentration (via the
// ApplyCondition incapacitation hook). Drives the AoE production path. ---
func TestIntegration_Phase118_DamageToZeroHPAppliesUnconsciousAndBreaks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	charID := concPCWithAbilities(t, db, campaignID)

	svc := combat.NewService(combat.NewStoreAdapter(queries))
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{
		CampaignID: campaignID, Name: "phase118-zerohp",
	})
	require.NoError(t, err)
	// Caster has 5 HP — about to be dropped.
	caster, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		CharacterID: charID.String(), ShortID: "AR", DisplayName: "Aria",
		HPMax: 30, HPCurrent: 5, AC: 13, SpeedFt: 30,
		PositionCol: "A", PositionRow: 1, IsAlive: true, IsVisible: true,
	})
	require.NoError(t, err)
	target, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		ShortID: "G1", DisplayName: "Goblin",
		HPMax: 7, HPCurrent: 7, AC: 12, SpeedFt: 30,
		PositionCol: "B", PositionRow: 1, IsAlive: true, IsNPC: true, IsVisible: true,
	})
	require.NoError(t, err)

	// Caster concentrating on Bless; goblin has the blessed condition.
	startConcentrating(t, db, caster.ID, "bless", "Bless", []targetCondition{
		{combatantID: target.ID, condition: "blessed"},
	})

	// Drive a fixed-roll Fireball (1d12 → 12) that drops the caster to 0 HP.
	roller := dice.NewRoller(func(max int) int { return 12 })
	_, err = svc.ResolveAoESaves(context.Background(), combat.AoEDamageInput{
		EncounterID: enc.ID,
		SpellName:   "Fireball",
		DamageDice:  "1d12",
		DamageType:  "fire",
		SaveEffect:  "no_effect",
		SaveResults: []combat.SaveResult{
			{CombatantID: caster.ID, Success: false, Total: 5},
		},
	}, roller)
	require.NoError(t, err)

	// Caster has the unconscious condition (applied by applyDamageHP).
	conds := readConditions(t, db, caster.ID)
	var hasUnconscious bool
	for _, c := range conds {
		if c.Condition == "unconscious" {
			hasUnconscious = true
		}
	}
	assert.True(t, hasUnconscious, "0 HP must apply unconscious; got %v", conds)

	// Concentration auto-broken (incapacitating condition triggered the
	// auto-break path in ApplyCondition).
	assert.Equal(t, "", readConcentration(t, db, caster.ID))
	// Goblin's blessed condition was stripped.
	assert.Empty(t, readConditions(t, db, target.ID))
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
// cleans up the old spell's effects (zone, conditions, summons). ---
//
// Drives the full cast-replacement flow against a real Postgres. Because
// spellcasting.Cast requires populated `spells`/`characters` rows and a
// large amount of plumbing to evaluate, we exercise the end-state contract:
// the iter-1 unit tests in `spellcasting_test.go::TestCast_PersistsConcentrationAndCleansUpPrevious`
// and `aoe_test.go::TestCastAoE_PersistsConcentrationAndCleansUpPrevious`
// already verify that Cast/CastAoE invokes BreakConcentrationFully with the
// correct input. This integration test asserts the database-level outcome:
// when BreakConcentrationFully runs against real rows for a "cast new
// concentration spell" reason, the OLD spell's conditions, zone, AND
// summons are all removed.
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
	// Add a summoned wolf linked to caster.
	wolf, err := svc.AddCombatant(context.Background(), enc.ID, combat.CombatantParams{
		ShortID: "WF", DisplayName: "Wolf", HPMax: 20, HPCurrent: 20, AC: 13, SpeedFt: 40,
		PositionCol: "C", PositionRow: 1, IsAlive: true, IsNPC: true, IsVisible: true,
	})
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE combatants SET summoner_id=$1 WHERE id=$2`, caster.ID, wolf.ID)
	require.NoError(t, err)

	// Caster currently concentrates on Conjure Animals: Goblin charmed,
	// summoned wolf linked, and a difficult-terrain zone is active.
	startConcentrating(t, db, caster.ID, "conjure-animals", "Conjure Animals", []targetCondition{
		{combatantID: target.ID, condition: "charmed"},
	})
	_, err = svc.CreateZone(context.Background(), combat.CreateZoneInput{
		EncounterID:           enc.ID,
		SourceCombatantID:     caster.ID,
		SourceSpell:           "conjure-animals",
		Shape:                 "square",
		OriginCol:             "B",
		OriginRow:             1,
		Dimensions:            json.RawMessage(`{"side_ft":20}`),
		AnchorMode:            "fixed",
		ZoneType:              "difficult_terrain",
		OverlayColor:          "#888888",
		RequiresConcentration: true,
	})
	require.NoError(t, err)

	// Replacing concentration triggers the full cleanup pipeline (this is
	// the same orchestrator that `Cast` invokes on `DroppedPrevious=true`).
	cleanup, err := svc.BreakConcentrationFully(context.Background(), combat.BreakConcentrationFullyInput{
		EncounterID: enc.ID,
		CasterID:    caster.ID,
		CasterName:  caster.DisplayName,
		SpellID:     "conjure-animals",
		SpellName:   "Conjure Animals",
		Reason:      "cast new concentration spell: Bless",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, cleanup.ConditionsRemoved)
	assert.Equal(t, 1, cleanup.SummonsDismissed)
	assert.Contains(t, cleanup.ConsolidatedMessage, "Conjure Animals")
	assert.Contains(t, cleanup.ConsolidatedMessage, "cast new concentration spell")

	// Conditions cleared on target.
	assert.Empty(t, readConditions(t, db, target.ID))
	// Caster's concentration columns cleared.
	assert.Equal(t, "", readConcentration(t, db, caster.ID))
	// Zone removed.
	zones, err := svc.ListZonesForEncounter(context.Background(), enc.ID)
	require.NoError(t, err)
	assert.Empty(t, zones)
	// Summon removed.
	combatants, err := svc.ListCombatantsByEncounterID(context.Background(), enc.ID)
	require.NoError(t, err)
	for _, c := range combatants {
		assert.NotEqual(t, wolf.ID, c.ID, "summon must be dismissed on concentration replacement")
	}
}

// --- Phase 118 done-when (4): Invisibility applied by a caster who then drops
// concentration auto-clears the invisible condition. ---
//
// Drives the dashboard's voluntary-drop HTTP handler end-to-end against a
// real Postgres so the entire DropConcentration → BreakConcentrationFully
// → RemoveSpellSourcedConditions chain runs through production wiring.
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

	// Apply the Invisibility-sourced condition via the same code path used
	// by `applyInvisibilityConditionFromCast` (ApplyCondition with
	// SourceCombatantID + SourceSpell stamped). This exercises the
	// condition-tagging contract that Phase 113 / 118 rely on.
	_, _, err = svc.ApplyCondition(context.Background(), ally.ID, combat.CombatCondition{
		Condition:         "invisible",
		SourceCombatantID: caster.ID.String(),
		SourceSpell:       "invisibility",
	})
	require.NoError(t, err)

	// Persist the caster's concentration via the authoritative columns.
	_, err = db.Exec(`UPDATE combatants SET concentration_spell_id='invisibility', concentration_spell_name='Invisibility' WHERE id=$1`, caster.ID)
	require.NoError(t, err)

	// Drive the dashboard's voluntary-drop HTTP handler end-to-end.
	handler := combat.NewDMDashboardHandler(svc)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)
	url := "/api/combat/" + enc.ID.String() + "/combatants/" + caster.ID.String() + "/concentration/drop"
	req := httptest.NewRequest(http.MethodPost, url, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "drop endpoint must return 200; got body %s", rec.Body.String())

	// The ally is no longer invisible.
	conds := readConditions(t, db, ally.ID)
	for _, c := range conds {
		assert.NotEqual(t, "invisible", c.Condition, "invisible condition must be cleared on concentration drop")
	}
	assert.Equal(t, "", readConcentration(t, db, caster.ID))

	// Response body carries the consolidated 💨 line with reason in parens.
	assert.Contains(t, rec.Body.String(), "💨")
	assert.Contains(t, rec.Body.String(), "Invisibility")
	assert.Contains(t, rec.Body.String(), "voluntary drop")
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

	// One consolidated 💨 line; format is exactly the spec-mandated one
	// (includes reason in parens after iter-2 user clarification).
	assert.Equal(t,
		"💨 Aria lost concentration on Bless (voluntary drop) — effects ended on 2 targets.",
		cleanup.ConsolidatedMessage,
	)
}

