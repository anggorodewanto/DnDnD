package combat_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// seedShieldArmor inserts the "shield" armor row (ArmorType "shield") so
// hasEquippedShield resolves it. Idempotent — the shared test DB preserves
// reference rows across tests. COV-9.
func seedShieldArmor(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO armor (id, name, ac_base, ac_dex_bonus, armor_type, weight_lb)
		VALUES ('shield', 'Shield', 2, false, 'shield', 6) ON CONFLICT (id) DO NOTHING`)
	require.NoError(t, err)
}

// newShieldMasterCombatant creates a Fighter carrying the Shield Master feat
// (name-detected) and, when shield is true, a shield in the off-hand — the two
// prerequisites for Interpose Shield. When feat/shield are false it yields a
// plain fighter, so a single ResolveAoESaves call can assert the gates. COV-9.
func newShieldMasterCombatant(t *testing.T, queries *refdata.Queries, campaignID, encID uuid.UUID, short, name string, hp int32, feat, shield bool) refdata.Combatant {
	t.Helper()
	features := `[]`
	if feat {
		features = `[{"name":"Shield Master"}]`
	}
	offHand := sql.NullString{}
	if shield {
		offHand = sql.NullString{String: "shield", Valid: true}
	}
	char, err := queries.CreateCharacter(context.Background(), refdata.CreateCharacterParams{
		CampaignID:       campaignID,
		Name:             name,
		Race:             "human",
		Classes:          []byte(`[{"class":"fighter","level":5}]`),
		Level:            5,
		AbilityScores:    []byte(`{"str":16,"dex":12,"con":14,"int":10,"wis":10,"cha":8}`),
		HpMax:            hp,
		HpCurrent:        hp,
		Ac:               18,
		SpeedFt:          30,
		ProficiencyBonus: 3,
		HitDiceRemaining: []byte(`{"d10":5}`),
		Features:         pqtype.NullRawMessage{RawMessage: []byte(features), Valid: true},
		EquippedOffHand:  offHand,
		Languages:        []string{"common"},
	})
	require.NoError(t, err)

	comb, err := queries.CreateCombatant(context.Background(), refdata.CreateCombatantParams{
		EncounterID:     encID,
		CharacterID:     uuid.NullUUID{UUID: char.ID, Valid: true},
		ShortID:         short,
		DisplayName:     name,
		InitiativeRoll:  10,
		InitiativeOrder: 1,
		PositionCol:     "B",
		PositionRow:     1,
		HpMax:           hp,
		HpCurrent:       hp,
		Ac:              18,
		Conditions:      []byte(`[]`),
		IsVisible:       true,
		IsAlive:         true,
		IsNpc:           false,
	})
	require.NoError(t, err)
	return comb
}

// TestResolveAoESaves_ShieldMasterInterpose is the COV-9 red/green test. A Shield
// Master holding a shield takes NO damage on a made DEX save-for-half (Interpose
// Shield) but FULL damage on a failed one — Interpose, unlike Evasion, never helps
// a failed save. The feat is inert without a shield and for non-feat targets.
// 8d6 with a fixed roller of 4/die = 32 base damage.
func TestResolveAoESaves_ShieldMasterInterpose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, queries := setupTestDB(t)
	seedShieldArmor(t, db)
	campaignID := createTestCampaign(t, db)

	svc := combat.NewService(combat.NewStoreAdapter(queries))
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{CampaignID: campaignID, Name: "interpose"})
	require.NoError(t, err)

	made := newShieldMasterCombatant(t, queries, campaignID, enc.ID, "S1", "Made Shield", 60, true, true)
	failed := newShieldMasterCombatant(t, queries, campaignID, enc.ID, "S2", "Failed Shield", 60, true, true)
	noShield := newShieldMasterCombatant(t, queries, campaignID, enc.ID, "S3", "No Shield", 60, true, false)
	noFeat := newShieldMasterCombatant(t, queries, campaignID, enc.ID, "S4", "Plain Fighter", 60, false, true)

	res, err := svc.ResolveAoESaves(context.Background(), combat.AoEDamageInput{
		EncounterID: enc.ID,
		SpellName:   "Fireball",
		DamageDice:  "8d6",
		DamageType:  "fire",
		SaveEffect:  "half_damage",
		SaveAbility: "dex",
		SaveResults: []combat.SaveResult{
			{CombatantID: made.ID, Total: 20, Success: true},
			{CombatantID: failed.ID, Total: 4, Success: false},
			{CombatantID: noShield.ID, Total: 20, Success: true},
			{CombatantID: noFeat.ID, Total: 20, Success: true},
		},
	}, fixedDamageRoller(4))
	require.NoError(t, err)
	require.Len(t, res.Targets, 4)

	byID := map[uuid.UUID]combat.AoETargetOutcome{}
	for _, tgt := range res.Targets {
		byID[tgt.CombatantID] = tgt
	}
	assert.Equal(t, 0, byID[made.ID].DamageDealt, "Interpose Shield: made DEX save-for-half + shield = no damage")
	assert.Equal(t, 32, byID[failed.ID].DamageDealt, "Interpose Shield: failed save = full damage (never helps a fail)")
	assert.Equal(t, 16, byID[noShield.ID].DamageDealt, "Shield Master without a shield = normal half")
	assert.Equal(t, 16, byID[noFeat.ID].DamageDealt, "no feat = normal half")
}

// TestResolveAoESaves_EvasionBeatsShieldMasterInterpose locks the precedence when a
// target has BOTH Evasion and Shield Master + shield: Evasion is chosen because it
// strictly dominates (both give 0 on a made save, but Evasion halves a failed save
// where Interpose gives full). Asserted on a FAILED save, the only case they differ.
func TestResolveAoESaves_EvasionBeatsShieldMasterInterpose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, queries := setupTestDB(t)
	seedShieldArmor(t, db)
	campaignID := createTestCampaign(t, db)

	svc := combat.NewService(combat.NewStoreAdapter(queries))
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{CampaignID: campaignID, Name: "interpose-vs-evasion"})
	require.NoError(t, err)

	char, err := queries.CreateCharacter(context.Background(), refdata.CreateCharacterParams{
		CampaignID:       campaignID,
		Name:             "Rogue-Fighter",
		Race:             "human",
		Classes:          []byte(`[{"class":"rogue","level":7}]`),
		Level:            7,
		AbilityScores:    []byte(`{"str":10,"dex":16,"con":12,"int":10,"wis":10,"cha":8}`),
		HpMax:            60,
		HpCurrent:        60,
		Ac:               17,
		SpeedFt:          30,
		ProficiencyBonus: 3,
		HitDiceRemaining: []byte(`{"d8":7}`),
		Features:         pqtype.NullRawMessage{RawMessage: []byte(`[{"name":"Evasion","mechanical_effect":"evasion"},{"name":"Shield Master"}]`), Valid: true},
		EquippedOffHand:  sql.NullString{String: "shield", Valid: true},
		Languages:        []string{"common"},
	})
	require.NoError(t, err)

	comb, err := queries.CreateCombatant(context.Background(), refdata.CreateCombatantParams{
		EncounterID:     enc.ID,
		CharacterID:     uuid.NullUUID{UUID: char.ID, Valid: true},
		ShortID:         "RF",
		DisplayName:     "Rogue-Fighter",
		InitiativeRoll:  10,
		InitiativeOrder: 1,
		PositionCol:     "B",
		PositionRow:     1,
		HpMax:           60,
		HpCurrent:       60,
		Ac:              17,
		Conditions:      []byte(`[]`),
		IsVisible:       true,
		IsAlive:         true,
		IsNpc:           false,
	})
	require.NoError(t, err)

	res, err := svc.ResolveAoESaves(context.Background(), combat.AoEDamageInput{
		EncounterID: enc.ID,
		SpellName:   "Fireball",
		DamageDice:  "8d6",
		DamageType:  "fire",
		SaveEffect:  "half_damage",
		SaveAbility: "dex",
		SaveResults: []combat.SaveResult{{CombatantID: comb.ID, Total: 4, Success: false}},
	}, fixedDamageRoller(4))
	require.NoError(t, err)
	require.Len(t, res.Targets, 1)
	assert.Equal(t, 16, res.Targets[0].DamageDealt, "Evasion dominates: failed save = half (32/2), not Interpose's full")
}
