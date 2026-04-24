package combat_test

import (
	"context"
	"testing"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_Surprise_FullFlow walks through Phase 114's "Done when"
// criteria end-to-end against a real Postgres testcontainer:
//
//  1. DM starts combat with a surprised NPC and a normal PC (both alive).
//  2. The surprised combatant's round-1 turn is auto-skipped with a combat
//     log narration and the surprised condition is removed from its row.
//  3. Before the skip the surprised combatant can't declare reactions;
//     after the skip they can.
//  4. Normal (non-surprised) combatants keep their turn.
func TestIntegration_Surprise_FullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	campaignID := createTestCampaign(t, db)
	mapID := createTestMap(t, db, campaignID)

	svc := combat.NewService(combat.NewStoreAdapter(queries))
	enc, err := svc.CreateEncounter(ctx, combat.CreateEncounterInput{
		CampaignID: campaignID,
		MapID:      uuid.NullUUID{UUID: mapID, Valid: true},
		Name:       "surprise-test",
	})
	require.NoError(t, err)

	// Add the hero NPC (non-surprised) — using IsNpc=true keeps things simple
	// because PC combatants require a character row with ability scores.
	hero, err := svc.AddCombatant(ctx, enc.ID, combat.CombatantParams{
		ShortID:     "HE",
		DisplayName: "Hero",
		HPMax:       30, HPCurrent: 30, AC: 16, SpeedFt: 30,
		PositionCol: "A", PositionRow: 1,
		IsNPC: true, IsAlive: true, IsVisible: true,
	})
	require.NoError(t, err)

	// Add the goblin NPC — this is the one that will be surprised.
	goblin, err := svc.AddCombatant(ctx, enc.ID, combat.CombatantParams{
		ShortID:     "GB2",
		DisplayName: "Goblin #2",
		HPMax:       7, HPCurrent: 7, AC: 15, SpeedFt: 30,
		PositionCol: "B", PositionRow: 2,
		IsNPC: true, IsAlive: true, IsVisible: true,
	})
	require.NoError(t, err)

	// Mark the goblin surprised via the public API.
	require.NoError(t, svc.MarkSurprised(ctx, goblin.ID))

	// Sanity: the surprised condition is stored on the combatant row.
	goblinRow, err := svc.GetCombatant(ctx, goblin.ID)
	require.NoError(t, err)
	assert.True(t, combat.IsSurprised(goblinRow.Conditions), "goblin should have surprised condition")

	// While surprised, the goblin cannot declare a reaction (spec: surprised
	// creatures can't take reactions until the end of their first turn).
	ok, err := svc.CanDeclareReaction(ctx, enc.ID, goblin.ID)
	require.NoError(t, err)
	assert.False(t, ok, "surprised goblin must not be able to declare a reaction")

	// DeclareReaction itself returns the sentinel error.
	_, err = svc.DeclareReaction(ctx, enc.ID, goblin.ID, "Shield if hit")
	require.Error(t, err)
	assert.ErrorIs(t, err, combat.ErrReactionSurprised)

	// Pin encounter to round 1 and set initiative order manually so AdvanceTurn
	// doesn't try to roll for combatants without character/creature rows.
	_, err = db.Exec(`UPDATE encounters SET status='active', round_number=1 WHERE id=$1`, enc.ID)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE combatants SET initiative_order=1 WHERE id=$1`, goblin.ID)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE combatants SET initiative_order=2 WHERE id=$1`, hero.ID)
	require.NoError(t, err)

	// Advance turn: expect the goblin's turn to be skipped and the hero's
	// active turn returned. The surprised combatant should surface via
	// SkippedCombatants so the done-handler can post the combat-log line.
	turnInfo, err := svc.AdvanceTurn(ctx, enc.ID)
	require.NoError(t, err)
	assert.Equal(t, hero.ID, turnInfo.CombatantID, "hero's turn should be active")
	require.Len(t, turnInfo.SkippedCombatants, 1, "goblin surprise-skip should be in SkippedCombatants")
	assert.Equal(t, "Goblin #2", turnInfo.SkippedCombatants[0].DisplayName)
	assert.Equal(t, "surprised", turnInfo.SkippedCombatants[0].ConditionName)
	assert.Equal(t, goblin.ID, turnInfo.SkippedCombatants[0].CombatantID)

	// Narration string matches the spec exactly.
	msg := combat.FormatAutoSkipMessage(
		turnInfo.SkippedCombatants[0].DisplayName,
		turnInfo.SkippedCombatants[0].ConditionName,
	)
	assert.Equal(t, "⏭️ Goblin #2 is surprised — turn skipped", msg)

	// After the skip, the surprised condition is gone from the goblin row.
	goblinRow, err = svc.GetCombatant(ctx, goblin.ID)
	require.NoError(t, err)
	assert.False(t, combat.IsSurprised(goblinRow.Conditions),
		"surprised condition should be removed at end of skipped turn")

	// And now the goblin can use reactions (post-skip).
	ok, err = svc.CanDeclareReaction(ctx, enc.ID, goblin.ID)
	require.NoError(t, err)
	assert.True(t, ok, "once surprised is removed, goblin can declare reactions")

	// One recorded turn this round for the goblin, status=skipped, plus the
	// hero's active turn.
	turns, err := queries.ListTurnsByEncounterAndRound(ctx, refdata.ListTurnsByEncounterAndRoundParams{
		EncounterID: enc.ID,
		RoundNumber: 1,
	})
	require.NoError(t, err)
	var goblinTurnStatus, heroTurnStatus string
	for _, tn := range turns {
		switch tn.CombatantID {
		case goblin.ID:
			goblinTurnStatus = tn.Status
		case hero.ID:
			heroTurnStatus = tn.Status
		}
	}
	assert.Equal(t, "skipped", goblinTurnStatus, "goblin's round-1 turn must be recorded as skipped")
	assert.Equal(t, "active", heroTurnStatus, "hero's round-1 turn should be active")
}

// TestIntegration_Surprise_AllSurprisedAdvancesRound covers the edge case
// where every candidate in round 1 is surprised — AdvanceTurn must skip them
// all, roll into round 2, and start the first combatant whose surprise was
// cleared by their round-1 skip.
func TestIntegration_Surprise_AllSurprisedAdvancesRound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	campaignID := createTestCampaign(t, db)
	mapID := createTestMap(t, db, campaignID)

	svc := combat.NewService(combat.NewStoreAdapter(queries))
	enc, err := svc.CreateEncounter(ctx, combat.CreateEncounterInput{
		CampaignID: campaignID,
		MapID:      uuid.NullUUID{UUID: mapID, Valid: true},
		Name:       "all-surprised",
	})
	require.NoError(t, err)

	a, err := svc.AddCombatant(ctx, enc.ID, combat.CombatantParams{
		ShortID: "AA", DisplayName: "A",
		HPMax: 10, HPCurrent: 10, AC: 12, SpeedFt: 30,
		PositionCol: "A", PositionRow: 1,
		IsNPC: true, IsAlive: true, IsVisible: true,
	})
	require.NoError(t, err)
	b, err := svc.AddCombatant(ctx, enc.ID, combat.CombatantParams{
		ShortID: "BB", DisplayName: "B",
		HPMax: 10, HPCurrent: 10, AC: 12, SpeedFt: 30,
		PositionCol: "B", PositionRow: 2,
		IsNPC: true, IsAlive: true, IsVisible: true,
	})
	require.NoError(t, err)
	require.NoError(t, svc.MarkSurprised(ctx, a.ID))
	require.NoError(t, svc.MarkSurprised(ctx, b.ID))

	// Pin the encounter to round 1 and set initiative manually so AdvanceTurn
	// doesn't need to roll for combatants without character/creature rows.
	_, err = db.Exec(`UPDATE encounters SET status='active', round_number=1 WHERE id=$1`, enc.ID)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE combatants SET initiative_order=1 WHERE id=$1`, a.ID)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE combatants SET initiative_order=2 WHERE id=$1`, b.ID)
	require.NoError(t, err)

	turnInfo, err := svc.AdvanceTurn(ctx, enc.ID)
	require.NoError(t, err)

	// Should have advanced to round 2 with an active turn for one of them.
	assert.Equal(t, int32(2), turnInfo.RoundNumber, "all surprised should push into round 2")
	// Both surprise conditions should have been cleared by their round-1 skips.
	updatedA, err := svc.GetCombatant(ctx, a.ID)
	require.NoError(t, err)
	updatedB, err := svc.GetCombatant(ctx, b.ID)
	require.NoError(t, err)
	assert.False(t, combat.IsSurprised(updatedA.Conditions))
	assert.False(t, combat.IsSurprised(updatedB.Conditions))
}
