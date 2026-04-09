package combat_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// recordingNotifier captures SendMessage calls in order so we can assert that
// PollOnce processes stale rows in deadline order.
type recordingNotifier struct {
	mu       sync.Mutex
	messages []recordedMessage
}

type recordedMessage struct {
	channelID string
	content   string
}

func (r *recordingNotifier) SendMessage(channelID, content string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = append(r.messages, recordedMessage{channelID: channelID, content: content})
	return nil
}

func (r *recordingNotifier) snapshot() []recordedMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedMessage, len(r.messages))
	copy(out, r.messages)
	return out
}

// TestIntegration_TurnTimer_PollOnce_StaleTurnsProcessedInDeadlineOrder seeds
// two overdue turns whose nudge_sent_at is NULL and whose started_at is in
// the past, then calls TurnTimer.PollOnce and verifies:
//  1. Nudges are sent for BOTH turns (stale state recovered),
//  2. They are processed in deadline order (earliest started_at first),
//  3. nudge_sent_at is marked so a second poll is a no-op.
//
// This is the "stale-turn scan" Phase 104 requires at startup.
func TestIntegration_TurnTimer_PollOnce_StaleTurnsProcessedInDeadlineOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := sharedDB.AcquireDB(t)
	queries := refdata.New(db)
	ctx := context.Background()

	// Campaign with a your-turn channel wired into settings.
	campaignID := uuid.New()
	settingsJSON := json.RawMessage(`{"channel_ids":{"your-turn":"chan-your-turn"},"turn_timeout_hours":24}`)
	_, err := db.Exec(`INSERT INTO campaigns (id, guild_id, dm_user_id, name, status, settings) VALUES ($1, $2, $3, $4, $5, $6)`,
		campaignID, "guild-stale", "dm-stale", "Stale Campaign", "active", []byte(settingsJSON))
	require.NoError(t, err)

	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID: campaignID,
		Name:       "Stale Encounter",
		Status:     "active",
	})
	require.NoError(t, err)

	// Two combatants.
	c1, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		ShortID:     "C1",
		DisplayName: "Alpha",
		HpMax:       10, HpCurrent: 10, Ac: 12,
		Conditions: json.RawMessage(`[]`),
		IsAlive:    true,
		IsNpc:      false,
	})
	require.NoError(t, err)

	c2, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		ShortID:     "C2",
		DisplayName: "Bravo",
		HpMax:       10, HpCurrent: 10, Ac: 12,
		Conditions: json.RawMessage(`[]`),
		IsAlive:    true,
		IsNpc:      false,
	})
	require.NoError(t, err)

	// Seed two stale turns. The nudge threshold is started_at + 50% of
	// (timeout_at - started_at); with a 24h window, that's 12h after
	// started_at. We use started_at = now()-20h (well past the 12h mark).
	// Turn A is the earlier one (started 22h ago), turn B is newer
	// (started 20h ago). Deadline order = A then B.
	turnA, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID: enc.ID,
		CombatantID: c1.ID,
		RoundNumber: 1,
		Status:      "active",
	})
	require.NoError(t, err)
	turnB, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID: enc.ID,
		CombatantID: c2.ID,
		RoundNumber: 1,
		Status:      "active",
	})
	require.NoError(t, err)

	now := time.Now()
	_, err = db.ExecContext(ctx,
		`UPDATE turns SET started_at = $1, timeout_at = $2 WHERE id = $3`,
		now.Add(-22*time.Hour), now.Add(2*time.Hour), turnA.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`UPDATE turns SET started_at = $1, timeout_at = $2 WHERE id = $3`,
		now.Add(-20*time.Hour), now.Add(4*time.Hour), turnB.ID)
	require.NoError(t, err)

	store := &testStoreAdapter{queries}
	notifier := &recordingNotifier{}
	timer := combat.NewTurnTimer(store, notifier, 30*time.Second)

	// Run the stale scan (first PollOnce at startup).
	err = timer.PollOnce(ctx)
	require.NoError(t, err)

	msgs := notifier.snapshot()
	// Both turns are past BOTH the nudge (50%) and warning (75%) thresholds
	// because started_at is 20-22h ago with a 24h window. That means each
	// PollOnce yields a nudge AND a warning per stale turn — 4 messages.
	require.Len(t, msgs, 4, "each stale turn should produce a nudge and a warning")

	// Nudges come first (processNudges before processWarnings), in deadline
	// order: Alpha (earliest started_at), then Bravo.
	assert.Contains(t, msgs[0].content, "Alpha", "earliest stale nudge first")
	assert.Contains(t, msgs[1].content, "Bravo", "later stale nudge second")
	// Warnings come next, same deadline order.
	assert.Contains(t, msgs[2].content, "Alpha", "earliest stale warning first")
	assert.Contains(t, msgs[3].content, "Bravo", "later stale warning second")

	// Verify nudge_sent_at was set so a second poll is idempotent.
	var count int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM turns WHERE nudge_sent_at IS NOT NULL AND id IN ($1, $2)`,
		turnA.ID, turnB.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "both turns should be marked nudge_sent_at")

	// Verify warning_sent_at is also set (warnings were processed).
	var warnCount int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM turns WHERE warning_sent_at IS NOT NULL AND id IN ($1, $2)`,
		turnA.ID, turnB.ID).Scan(&warnCount)
	require.NoError(t, err)
	assert.Equal(t, 2, warnCount, "both turns should be marked warning_sent_at")

	err = timer.PollOnce(ctx)
	require.NoError(t, err)
	assert.Len(t, notifier.snapshot(), 4,
		"second poll must be a no-op — nudge and warning already sent")

	// Cleanup
	_, _ = db.Exec(`DELETE FROM turns WHERE encounter_id = $1`, enc.ID)
	_, _ = db.Exec(`DELETE FROM combatants WHERE encounter_id = $1`, enc.ID)
	_, _ = db.Exec(`DELETE FROM encounters WHERE id = $1`, enc.ID)
	_, _ = db.Exec(`DELETE FROM campaigns WHERE id = $1`, campaignID)
}
