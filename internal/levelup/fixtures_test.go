package levelup_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/testutil"
)

// setupLevelUpTestDB acquires the shared testcontainer Postgres DB and
// returns both the raw *sql.DB and a *refdata.Queries instance. Callers
// should not close the DB — the shared container is torn down via TestMain.
func setupLevelUpTestDB(t *testing.T) (*sql.DB, *refdata.Queries) {
	t.Helper()
	db := sharedDB.AcquireDB(t)
	return db, refdata.New(db)
}

// createLevelUpCampaign delegates to the shared fixture helper.
func createLevelUpCampaign(t *testing.T, q *refdata.Queries, guildID string) refdata.Campaign {
	t.Helper()
	return testutil.NewTestCampaign(t, q, guildID)
}

// createLevelUpCharacter creates a level-5 fighter. Defaults match the
// Phase 104c adapter tests — AC 18 and HP 44 — so the existing assertions
// stay green. We can't use NewTestCharacter directly because its defaults
// (AC 16, HP scaled by level*8 = 50) differ.
func createLevelUpCharacter(t *testing.T, q *refdata.Queries, campID uuid.UUID, name string) refdata.Character {
	t.Helper()
	classes := json.RawMessage(`[{"class":"fighter","level":5}]`)
	scores := json.RawMessage(`{"str":16,"dex":14,"con":14,"int":10,"wis":12,"cha":8}`)
	char, err := q.CreateCharacter(context.Background(), refdata.CreateCharacterParams{
		CampaignID:       campID,
		Name:             name,
		Race:             "human",
		Classes:          classes,
		Level:            5,
		AbilityScores:    scores,
		HpMax:            44,
		HpCurrent:        44,
		TempHp:           0,
		Ac:               18,
		SpeedFt:          30,
		ProficiencyBonus: 3,
		HitDiceRemaining: json.RawMessage(`{"d10":5}`),
		Languages:        []string{"common"},
	})
	require.NoError(t, err)
	return char
}

// createLevelUpPlayerCharacter delegates to the shared fixture helper.
func createLevelUpPlayerCharacter(t *testing.T, q *refdata.Queries, campID, charID uuid.UUID, discordUserID string) {
	t.Helper()
	testutil.NewTestPlayerCharacter(t, q, campID, charID, discordUserID)
}

// seedLevelUpClasses upserts the minimum class rows the class-store adapter
// tests need (fighter + wizard). We hand-craft the JSON payloads so tests
// aren't coupled to the full SRD seed (which also seeds ~700 spells and
// creatures — too slow for an adapter unit test).
func seedLevelUpClasses(t *testing.T, q *refdata.Queries) {
	t.Helper()
	ctx := context.Background()

	fighterArgs := refdata.UpsertClassParams{
		ID:                  "fighter",
		Name:                "Fighter",
		HitDie:              "d10",
		PrimaryAbility:      "str",
		SaveProficiencies:   []string{"str", "con"},
		ArmorProficiencies:  []string{"light", "medium", "heavy", "shields"},
		WeaponProficiencies: []string{"simple", "martial"},
		Spellcasting:        pqtype.NullRawMessage{Valid: false},
		FeaturesByLevel:     json.RawMessage(`{"1":[{"name":"Fighting Style","description":"x","mechanical_effect":"fighting_style_choice"}],"5":[{"name":"Extra Attack","description":"y","mechanical_effect":"attacks_per_action_2"}]}`),
		AttacksPerAction:    json.RawMessage(`{"1":1,"5":2,"11":3,"20":4}`),
		SubclassLevel:       3,
		Subclasses:          json.RawMessage(`{"champion":{"name":"Champion","features_by_level":{}}}`),
	}
	require.NoError(t, q.UpsertClass(ctx, fighterArgs))

	wizardArgs := refdata.UpsertClassParams{
		ID:                  "wizard",
		Name:                "Wizard",
		HitDie:              "d6",
		PrimaryAbility:      "int",
		SaveProficiencies:   []string{"int", "wis"},
		ArmorProficiencies:  []string{},
		WeaponProficiencies: []string{"dagger", "quarterstaff"},
		Spellcasting:        pqtype.NullRawMessage{RawMessage: []byte(`{"ability":"int","slot_progression":"full"}`), Valid: true},
		FeaturesByLevel:     json.RawMessage(`{"1":[{"name":"Spellcasting","description":"x","mechanical_effect":"spellcasting_int"}]}`),
		AttacksPerAction:    json.RawMessage(`{"1":1}`),
		SubclassLevel:       2,
		Subclasses:          json.RawMessage(`{"evocation":{"name":"School of Evocation","features_by_level":{}}}`),
	}
	require.NoError(t, q.UpsertClass(ctx, wizardArgs))
}
