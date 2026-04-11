package combat_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Phase 105 — integration: two encounters can be active at the same time
// on the same campaign with independent round counters and independent
// combatant lists, and the workspace response surfaces both for the DM's
// tabbed Combat Workspace.
func TestPhase105Integration_TwoSimultaneousActiveEncounters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	campaignID := createTestCampaign(t, db)

	// Create two encounters via direct DB insert so we don't rely on the
	// template path. Distinct display_name values so the spec-example
	// labels come out cleanly in the shared combat channels.
	encA := uuid.New()
	encB := uuid.New()
	_, err := db.Exec(`INSERT INTO encounters (id, campaign_id, name, display_name, status, round_number) VALUES ($1, $2, $3, $4, $5, $6)`,
		encA, campaignID, "boss-fight-a", "Rooftop Ambush", "active", 3)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO encounters (id, campaign_id, name, display_name, status, round_number) VALUES ($1, $2, $3, $4, $5, $6)`,
		encB, campaignID, "room-escape-b", "Basement Escape", "active", 1)
	require.NoError(t, err)

	encounters, err := queries.ListEncountersByCampaignID(ctx, campaignID)
	require.NoError(t, err)
	require.Len(t, encounters, 2)

	byID := map[uuid.UUID]refdata.Encounter{}
	for _, e := range encounters {
		byID[e.ID] = e
	}
	assert.Equal(t, int32(3), byID[encA].RoundNumber)
	assert.Equal(t, int32(1), byID[encB].RoundNumber)
	assert.Equal(t, "Rooftop Ambush", byID[encA].DisplayName.String)
	assert.Equal(t, "Basement Escape", byID[encB].DisplayName.String)

	// Spec example labels — "⚔️ Rooftop Ambush — Round 3" etc. come out
	// verbatim from FormatEncounterLabel so players sharing the combat
	// channels can distinguish interleaved messages.
	labelA := combat.FormatEncounterLabel(combat.EncounterDisplayName(byID[encA]), byID[encA].RoundNumber)
	labelB := combat.FormatEncounterLabel(combat.EncounterDisplayName(byID[encB]), byID[encB].RoundNumber)
	assert.Equal(t, "\u2694\ufe0f Rooftop Ambush \u2014 Round 3", labelA)
	assert.Equal(t, "\u2694\ufe0f Basement Escape \u2014 Round 1", labelB)
}

// Phase 105 — integration: a character that is already a live combatant in
// one active encounter cannot be added to a second active encounter. Service
// layer enforces this rule via the GetActiveEncounterIDByCharacterID check.
func TestPhase105Integration_CharacterOneActiveEncounterConstraint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	campaignID := createTestCampaign(t, db)
	charID := createTestCharacter(t, db, campaignID)

	// Encounter A is active and has the character as a combatant.
	encA := uuid.New()
	_, err := db.Exec(`INSERT INTO encounters (id, campaign_id, name, status, round_number) VALUES ($1, $2, $3, $4, $5)`,
		encA, campaignID, "enc-a", "active", 1)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO combatants (encounter_id, character_id, short_id, display_name, hp_max, hp_current, ac, position_col, position_row) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		encA, charID, "AR", "Aragorn", 45, 45, 18, "A", 1)
	require.NoError(t, err)

	// Encounter B is created as a new (preparing) encounter.
	encB := uuid.New()
	_, err = db.Exec(`INSERT INTO encounters (id, campaign_id, name, status, round_number) VALUES ($1, $2, $3, $4, $5)`,
		encB, campaignID, "enc-b", "preparing", 0)
	require.NoError(t, err)

	// Adding the same character to encounter B must fail with the
	// Phase 105 constraint error.
	svc := combat.NewService(combat.NewStoreAdapter(queries))
	_, err = svc.AddCombatant(ctx, encB, combat.CombatantParams{
		CharacterID: charID.String(),
		ShortID:     "AR",
		DisplayName: "Aragorn",
		HPMax:       45,
		HPCurrent:   45,
		AC:          18,
		IsAlive:     true,
		IsVisible:   true,
		PositionCol: "B",
		PositionRow: 2,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, combat.ErrCharacterAlreadyInActiveEncounter))

	// After encounter A is completed, the same character can join
	// encounter B.
	_, err = queries.UpdateEncounterStatus(ctx, refdata.UpdateEncounterStatusParams{ID: encA, Status: "completed"})
	require.NoError(t, err)

	_, err = svc.AddCombatant(ctx, encB, combat.CombatantParams{
		CharacterID: charID.String(),
		ShortID:     "AR",
		DisplayName: "Aragorn",
		HPMax:       45,
		HPCurrent:   45,
		AC:          18,
		IsAlive:     true,
		IsVisible:   true,
		PositionCol: "B",
		PositionRow: 2,
	})
	require.NoError(t, err)
}

// Phase 105 — integration: UpdateEncounterDisplayName round-trips through
// the service and is reflected in FormatEncounterLabel so players see the
// new vague name in the next bot message.
func TestPhase105Integration_UpdateEncounterDisplayNameRoundTrips(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	campaignID := createTestCampaign(t, db)

	encID := uuid.New()
	_, err := db.Exec(`INSERT INTO encounters (id, campaign_id, name, status, round_number) VALUES ($1, $2, $3, $4, $5)`,
		encID, campaignID, "boss-fight", "active", 2)
	require.NoError(t, err)

	svc := combat.NewService(combat.NewStoreAdapter(queries))

	// Set to a vague name.
	enc, err := svc.UpdateEncounterDisplayName(ctx, encID, "The Shadows Stir")
	require.NoError(t, err)
	assert.Equal(t, "The Shadows Stir", combat.EncounterDisplayName(enc))
	assert.Equal(t, "\u2694\ufe0f The Shadows Stir \u2014 Round 2",
		combat.FormatEncounterLabel(combat.EncounterDisplayName(enc), enc.RoundNumber))

	// Reset to the internal name by passing empty string.
	enc, err = svc.UpdateEncounterDisplayName(ctx, encID, "")
	require.NoError(t, err)
	assert.Equal(t, "boss-fight", combat.EncounterDisplayName(enc))
}
