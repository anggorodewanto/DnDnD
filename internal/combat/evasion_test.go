package combat_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// newEvasionRogueCombatant creates a Rogue-L7 character carrying the `evasion`
// mechanical_effect and a combatant backed by it, so ResolveAoESaves can look up
// the target's Evasion feature. COV-3.
func newEvasionRogueCombatant(t *testing.T, queries *refdata.Queries, campaignID, encID uuid.UUID, short, name string, hp int32) refdata.Combatant {
	t.Helper()
	char, err := queries.CreateCharacter(context.Background(), refdata.CreateCharacterParams{
		CampaignID:       campaignID,
		Name:             name,
		Race:             "human",
		Classes:          []byte(`[{"class":"rogue","level":7}]`),
		Level:            7,
		AbilityScores:    []byte(`{"str":10,"dex":16,"con":12,"int":10,"wis":10,"cha":8}`),
		HpMax:            hp,
		HpCurrent:        hp,
		Ac:               15,
		SpeedFt:          30,
		ProficiencyBonus: 3,
		HitDiceRemaining: []byte(`{"d8":7}`),
		Features:         pqtype.NullRawMessage{RawMessage: []byte(`[{"name":"Evasion","mechanical_effect":"evasion"}]`), Valid: true},
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
		Ac:              15,
		Conditions:      []byte(`[]`),
		IsVisible:       true,
		IsAlive:         true,
		IsNpc:           false,
	})
	require.NoError(t, err)
	return comb
}

// TestResolveAoESaves_EvasionUpgradesDexSaveForHalf is the COV-3 red/green test.
// A Rogue 7+ with Evasion takes NO damage on a made DEX save-for-half and HALF on
// a failed one (2024 Evasion), while a non-Evasion target keeps the normal
// half-on-success outcome. 8d6 with a fixed roller of 4/die = 32 base damage.
func TestResolveAoESaves_EvasionUpgradesDexSaveForHalf(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)

	svc := combat.NewService(combat.NewStoreAdapter(queries))
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{CampaignID: campaignID, Name: "evasion"})
	require.NoError(t, err)

	rogueMade := newEvasionRogueCombatant(t, queries, campaignID, enc.ID, "R1", "Made Rogue", 60)
	rogueFailed := newEvasionRogueCombatant(t, queries, campaignID, enc.ID, "R2", "Failed Rogue", 60)
	npcMade := addBlastMonster(t, svc, enc.ID, "G1", "Goblin", 100, "A")

	res, err := svc.ResolveAoESaves(context.Background(), combat.AoEDamageInput{
		EncounterID: enc.ID,
		SpellName:   "Fireball",
		DamageDice:  "8d6",
		DamageType:  "fire",
		SaveEffect:  "half_damage",
		SaveAbility: "dex",
		SaveResults: []combat.SaveResult{
			{CombatantID: rogueMade.ID, Total: 20, Success: true},
			{CombatantID: rogueFailed.ID, Total: 4, Success: false},
			{CombatantID: npcMade.ID, Total: 20, Success: true},
		},
	}, fixedDamageRoller(4))
	require.NoError(t, err)
	require.Len(t, res.Targets, 3)

	byID := map[uuid.UUID]combat.AoETargetOutcome{}
	for _, tgt := range res.Targets {
		byID[tgt.CombatantID] = tgt
	}
	assert.Equal(t, 0, byID[rogueMade.ID].DamageDealt, "Evasion: made DEX save-for-half = no damage")
	assert.Equal(t, 16, byID[rogueFailed.ID].DamageDealt, "Evasion: failed DEX save = half of 32")
	assert.Equal(t, 16, byID[npcMade.ID].DamageDealt, "no Evasion: made save = normal half of 32")
}

// TestResolveAoESaves_EvasionOnlyAppliesToDexSaves guards the ability gate: an
// Evasion rogue making a CON save-for-half still takes the normal half — Evasion
// is DEX-only (2024).
func TestResolveAoESaves_EvasionOnlyAppliesToDexSaves(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, queries := setupTestDB(t)
	campaignID := createTestCampaign(t, db)

	svc := combat.NewService(combat.NewStoreAdapter(queries))
	enc, err := svc.CreateEncounter(context.Background(), combat.CreateEncounterInput{CampaignID: campaignID, Name: "evasion-con"})
	require.NoError(t, err)

	rogue := newEvasionRogueCombatant(t, queries, campaignID, enc.ID, "R1", "Con Rogue", 60)

	res, err := svc.ResolveAoESaves(context.Background(), combat.AoEDamageInput{
		EncounterID: enc.ID,
		SpellName:   "Thunderwave",
		DamageDice:  "8d6",
		DamageType:  "thunder",
		SaveEffect:  "half_damage",
		SaveAbility: "con",
		SaveResults: []combat.SaveResult{{CombatantID: rogue.ID, Total: 20, Success: true}},
	}, fixedDamageRoller(4))
	require.NoError(t, err)
	require.Len(t, res.Targets, 1)
	assert.Equal(t, 16, res.Targets[0].DamageDealt, "Evasion does not apply to CON saves")
}
