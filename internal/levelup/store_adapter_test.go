package levelup_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/levelup"
	"github.com/ab/dndnd/internal/refdata"
)

// These integration tests exercise the DB-backed adapters that bridge
// *refdata.Queries onto the levelup.CharacterStore / ClassStore / Notifier
// contracts. They back Phase 104c so levelup.Handler can finally be mounted
// in cmd/dndnd/main.go with real persistence.

func TestCharacterStoreAdapter_GetCharacterForLevelUp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	_, q := setupLevelUpTestDB(t)
	camp := createLevelUpCampaign(t, q, "guild-levelup-adapter")
	char := createLevelUpCharacter(t, q, camp.ID, "Aria")
	createLevelUpPlayerCharacter(t, q, camp.ID, char.ID, "discord-42")

	adapter := levelup.NewCharacterStoreAdapter(q)

	got, err := adapter.GetCharacterForLevelUp(ctx, char.ID)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, char.ID, got.ID)
	assert.Equal(t, "Aria", got.Name)
	assert.Equal(t, "discord-42", got.DiscordUserID)
	assert.Equal(t, int32(5), got.Level)
	assert.Equal(t, int32(44), got.HPMax)
	assert.Equal(t, int32(44), got.HPCurrent)
	assert.Equal(t, int32(3), got.ProficiencyBonus)
	assert.NotEmpty(t, got.Classes)
	assert.NotEmpty(t, got.AbilityScores)
}

func TestCharacterStoreAdapter_GetCharacterForLevelUp_NoPlayerCharacter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	_, q := setupLevelUpTestDB(t)
	camp := createLevelUpCampaign(t, q, "guild-levelup-adapter-npc")
	char := createLevelUpCharacter(t, q, camp.ID, "Nameless")

	adapter := levelup.NewCharacterStoreAdapter(q)

	got, err := adapter.GetCharacterForLevelUp(ctx, char.ID)
	require.NoError(t, err)
	assert.Equal(t, "", got.DiscordUserID)
}

func TestCharacterStoreAdapter_UpdateCharacterStats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	_, q := setupLevelUpTestDB(t)
	camp := createLevelUpCampaign(t, q, "guild-levelup-update")
	char := createLevelUpCharacter(t, q, camp.ID, "Bree")

	adapter := levelup.NewCharacterStoreAdapter(q)

	newClasses := json.RawMessage(`[{"class":"fighter","level":6}]`)
	newSlots := json.RawMessage(`{"1":4,"2":2}`)

	err := adapter.UpdateCharacterStats(ctx, char.ID, levelup.StatsUpdate{
		Level:            6,
		HPMax:            55,
		HPCurrent:        55,
		ProficiencyBonus: 3,
		Classes:          newClasses,
		SpellSlots:       newSlots,
		Features:         json.RawMessage(`[{"name":"Extra Attack","source":"class","description":"x"}]`),
	})
	require.NoError(t, err)

	row, err := q.GetCharacterForLevelUp(ctx, char.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(6), row.Level)
	assert.Equal(t, int32(55), row.HpMax)
	assert.Equal(t, int32(55), row.HpCurrent)
	assert.Equal(t, int32(3), row.ProficiencyBonus)
	assert.JSONEq(t, string(newClasses), string(row.Classes))
	assert.True(t, row.SpellSlots.Valid)
	assert.JSONEq(t, string(newSlots), string(row.SpellSlots.RawMessage))
}

func TestCharacterStoreAdapter_UpdateCharacterStats_NilOptionalSlots(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	_, q := setupLevelUpTestDB(t)
	camp := createLevelUpCampaign(t, q, "guild-levelup-update-nil")
	char := createLevelUpCharacter(t, q, camp.ID, "Caro")

	adapter := levelup.NewCharacterStoreAdapter(q)

	// Pre-set spell slots so we can verify nil input does NOT clobber them.
	_, err := q.UpdateCharacterSpellSlots(ctx, refdata.UpdateCharacterSpellSlotsParams{
		ID:         char.ID,
		SpellSlots: pqtype.NullRawMessage{RawMessage: []byte(`{"1":2}`), Valid: true},
	})
	require.NoError(t, err)

	err = adapter.UpdateCharacterStats(ctx, char.ID, levelup.StatsUpdate{
		Level:            6,
		HPMax:            50,
		HPCurrent:        50,
		ProficiencyBonus: 3,
		Classes:          json.RawMessage(`[{"class":"fighter","level":6}]`),
		// SpellSlots, PactMagicSlots, Features all nil — must preserve existing
	})
	require.NoError(t, err)

	row, err := q.GetCharacterForLevelUp(ctx, char.ID)
	require.NoError(t, err)
	require.True(t, row.SpellSlots.Valid, "spell slots should be preserved when adapter receives nil")
	assert.JSONEq(t, `{"1":2}`, string(row.SpellSlots.RawMessage))
}

func TestCharacterStoreAdapter_UpdateAbilityScores(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	_, q := setupLevelUpTestDB(t)
	camp := createLevelUpCampaign(t, q, "guild-levelup-scores")
	char := createLevelUpCharacter(t, q, camp.ID, "Dima")

	adapter := levelup.NewCharacterStoreAdapter(q)

	newScores := json.RawMessage(`{"str":18,"dex":14,"con":14,"int":10,"wis":12,"cha":8}`)
	err := adapter.UpdateAbilityScores(ctx, char.ID, newScores)
	require.NoError(t, err)

	row, err := q.GetCharacterForLevelUp(ctx, char.ID)
	require.NoError(t, err)
	assert.JSONEq(t, string(newScores), string(row.AbilityScores))
}

func TestCharacterStoreAdapter_UpdateFeatures(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	_, q := setupLevelUpTestDB(t)
	camp := createLevelUpCampaign(t, q, "guild-levelup-feats")
	char := createLevelUpCharacter(t, q, camp.ID, "Erin")

	adapter := levelup.NewCharacterStoreAdapter(q)

	feats := json.RawMessage(`[{"name":"Tough","source":"feat","description":"+2 HP per level"}]`)
	err := adapter.UpdateFeatures(ctx, char.ID, feats)
	require.NoError(t, err)

	row, err := q.GetCharacterForLevelUp(ctx, char.ID)
	require.NoError(t, err)
	require.True(t, row.Features.Valid)
	assert.JSONEq(t, string(feats), string(row.Features.RawMessage))
}

func TestCharacterStoreAdapter_GetCharacterForLevelUp_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	_, q := setupLevelUpTestDB(t)
	adapter := levelup.NewCharacterStoreAdapter(q)

	_, err := adapter.GetCharacterForLevelUp(ctx, uuid.New())
	require.Error(t, err)
}

func TestClassStoreAdapter_GetClassRefData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	_, q := setupLevelUpTestDB(t)
	seedLevelUpClasses(t, q)

	adapter := levelup.NewClassStoreAdapter(q)

	ref, err := adapter.GetClassRefData(ctx, "fighter")
	require.NoError(t, err)
	require.NotNil(t, ref)

	assert.Equal(t, "d10", ref.HitDie)
	assert.NotEmpty(t, ref.AttacksPerAction)
	assert.Equal(t, 1, ref.AttacksPerAction[1])
	assert.Equal(t, 2, ref.AttacksPerAction[5])
	assert.Equal(t, 3, ref.SubclassLevel)
}

func TestClassStoreAdapter_GetClassRefData_Spellcasting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	_, q := setupLevelUpTestDB(t)
	seedLevelUpClasses(t, q)

	adapter := levelup.NewClassStoreAdapter(q)

	ref, err := adapter.GetClassRefData(ctx, "wizard")
	require.NoError(t, err)
	require.NotNil(t, ref.Spellcasting)
	assert.Equal(t, "full", ref.Spellcasting.SlotProgression)
	assert.Equal(t, "int", ref.Spellcasting.SpellAbility)
}

func TestClassStoreAdapter_GetClassRefData_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	_, q := setupLevelUpTestDB(t)
	adapter := levelup.NewClassStoreAdapter(q)

	_, err := adapter.GetClassRefData(ctx, "nosuchclass")
	require.Error(t, err)
}
