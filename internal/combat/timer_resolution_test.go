package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// --- Format Tests ---

func TestFormatDMDecisionPrompt(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    2,
		ActionUsed:          false,
		BonusActionUsed:     false,
	}
	combatant := refdata.Combatant{
		DisplayName: "Aria",
	}

	msg := FormatDMDecisionPrompt(combatant, turn)
	assert.Contains(t, msg, "Aria")
	assert.Contains(t, msg, "timed out")
	assert.Contains(t, msg, "30ft move")
	assert.Contains(t, msg, "2 attacks")
	assert.Contains(t, msg, "Bonus action")
	assert.Contains(t, msg, "Wait]")
	assert.Contains(t, msg, "Roll for Player]")
	assert.Contains(t, msg, "Auto-Resolve]")
}

func TestFormatDMDecisionPrompt_AllUsed(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 0,
		AttacksRemaining:    0,
		ActionUsed:          true,
		BonusActionUsed:     true,
	}
	combatant := refdata.Combatant{DisplayName: "Borr"}

	msg := FormatDMDecisionPrompt(combatant, turn)
	assert.Contains(t, msg, "Borr")
	assert.NotContains(t, msg, "Pending:")
}

func TestFormatAutoResolveResult(t *testing.T) {
	combatant := refdata.Combatant{DisplayName: "Aria"}
	actions := []string{"Aria takes the Dodge action", "No movement", "Reaction declarations forfeited"}

	msg := FormatAutoResolveResult(combatant, actions)
	assert.Contains(t, msg, "Aria")
	assert.Contains(t, msg, "auto-resolved")
	assert.Contains(t, msg, "Dodge action")
	assert.Contains(t, msg, "No movement")
	assert.Contains(t, msg, "player timed out")
}

// --- processTimeouts Tests ---

func TestProcessTimeouts_SendsDMDecisionPrompt(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()
	now := time.Now()

	store := defaultMockStore()
	store.listTurnsTimedOutFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{{
			ID:          turnID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			StartedAt:   sql.NullTime{Time: now.Add(-25 * time.Hour), Valid: true},
			TimeoutAt:   sql.NullTime{Time: now.Add(-1 * time.Hour), Valid: true},
			MovementRemainingFt: 30,
			AttacksRemaining:    2,
		}}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Aria", Conditions: json.RawMessage(`[]`)}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}
	dmDecisionSentCalled := false
	store.updateTurnDMDecisionSentFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		dmDecisionSentCalled = true
		assert.Equal(t, turnID, id)
		return refdata.Turn{ID: id}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.processTimeouts(context.Background())
	require.NoError(t, err)

	assert.True(t, dmDecisionSentCalled)
	msgs := notifier.getMessages()
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0].content, "Aria")
	assert.Contains(t, msgs[0].content, "timed out")
}

func TestProcessTimeouts_NothingTimedOut(t *testing.T) {
	store := defaultMockStore()
	store.listTurnsTimedOutFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.processTimeouts(context.Background())
	require.NoError(t, err)
	assert.Empty(t, notifier.getMessages())
}

// --- AutoResolveTurn Tests ---

func TestAutoResolveTurn_NormalTurn(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID:                  turnID,
			EncounterID:         encounterID,
			CombatantID:         combatantID,
			RoundNumber:         1,
			Status:              "active",
			MovementRemainingFt: 30,
			ActionUsed:          false,
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          combatantID,
			EncounterID: encounterID,
			DisplayName: "Aria",
			HpCurrent:   30,
			HpMax:       30,
			IsAlive:     true,
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	dodgeApplied := false
	store.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		dodgeApplied = true
		assert.Contains(t, string(arg.Conditions), "dodge")
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	autoResolvedCalled := false
	store.updateTurnAutoResolvedFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		autoResolvedCalled = true
		return refdata.Turn{ID: id, AutoResolved: true, Status: "completed"}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	roller := dice.NewRoller(func(max int) int { return 10 })

	actions, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)

	assert.True(t, dodgeApplied)
	assert.True(t, autoResolvedCalled)
	assert.Contains(t, actions, "Aria takes the Dodge action")
	assert.Contains(t, actions, "No movement")
	assert.Contains(t, actions, "Reaction declarations forfeited")
}

func TestAutoResolveTurn_DyingCombatant(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()

	ds := DeathSaves{Successes: 1, Failures: 1}
	dsJSON, _ := json.Marshal(ds)

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID:          turnID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			RoundNumber: 2,
			Status:      "active",
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          combatantID,
			EncounterID: encounterID,
			DisplayName: "Borr",
			HpCurrent:   0,
			HpMax:       40,
			IsAlive:     true,
			Conditions:  json.RawMessage(`[]`),
			DeathSaves:  pqtype.NullRawMessage{RawMessage: dsJSON, Valid: true},
		}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	deathSavesUpdated := false
	store.updateCombatantDeathSavesFn = func(ctx context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error) {
		deathSavesUpdated = true
		return refdata.Combatant{ID: arg.ID, DeathSaves: arg.DeathSaves}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	// Roll a 15 = success
	roller := dice.NewRoller(func(max int) int { return 15 })

	actions, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)

	assert.True(t, deathSavesUpdated)
	// Should have a death save message
	found := false
	for _, a := range actions {
		if assert.ObjectsAreEqual(true, len(a) > 0) {
			if containsStr(a, "death save") {
				found = true
			}
		}
	}
	assert.True(t, found, "should contain death save message")
}

func TestAutoResolveTurn_DyingCombatant_Nat20(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()

	ds := DeathSaves{Successes: 0, Failures: 1}
	dsJSON, _ := json.Marshal(ds)

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID: turnID, EncounterID: encounterID, CombatantID: combatantID,
			RoundNumber: 2, Status: "active",
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: combatantID, EncounterID: encounterID, DisplayName: "Borr",
			HpCurrent: 0, HpMax: 40, IsAlive: true, Conditions: json.RawMessage(`[]`),
			DeathSaves: pqtype.NullRawMessage{RawMessage: dsJSON, Valid: true},
		}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	hpUpdated := false
	store.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpUpdated = true
		assert.Equal(t, int32(1), arg.HpCurrent)
		assert.True(t, arg.IsAlive)
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	// Nat 20
	roller := dice.NewRoller(func(max int) int { return 20 })

	actions, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)
	assert.True(t, hpUpdated)
	found := false
	for _, a := range actions {
		if containsStr(a, "NAT 20") {
			found = true
		}
	}
	assert.True(t, found, "should contain NAT 20 message")
}

func TestAutoResolveTurn_ForfeitsReactionDeclarations(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID: turnID, EncounterID: encounterID, CombatantID: combatantID,
			Status: "active", ActionUsed: true, // action already used
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: combatantID, EncounterID: encounterID, DisplayName: "Aria",
			HpCurrent: 20, HpMax: 30, IsAlive: true, Conditions: json.RawMessage(`[]`),
		}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	reactionsCancelled := false
	store.cancelAllReactionDeclarationsByCombatantFn = func(ctx context.Context, arg refdata.CancelAllReactionDeclarationsByCombatantParams) error {
		reactionsCancelled = true
		assert.Equal(t, combatantID, arg.CombatantID)
		assert.Equal(t, encounterID, arg.EncounterID)
		return nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	roller := dice.NewRoller(func(max int) int { return 10 })

	actions, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)
	assert.True(t, reactionsCancelled)
	assert.Contains(t, actions, "Reaction declarations forfeited")
}

func TestAutoResolveTurn_IncrementsProlongedAbsence(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID: turnID, EncounterID: encounterID, CombatantID: combatantID,
			Status: "active", ActionUsed: true,
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: combatantID, EncounterID: encounterID, DisplayName: "Aria",
			HpCurrent: 20, HpMax: 30, IsAlive: true, Conditions: json.RawMessage(`[]`),
			ConsecutiveAutoResolves: 0,
		}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	var capturedCount int32
	var capturedAbsent bool
	store.updateCombatantAutoResolveCountFn = func(ctx context.Context, arg refdata.UpdateCombatantAutoResolveCountParams) (refdata.Combatant, error) {
		capturedCount = arg.ConsecutiveAutoResolves
		capturedAbsent = arg.IsAbsent
		return refdata.Combatant{ID: arg.ID, ConsecutiveAutoResolves: arg.ConsecutiveAutoResolves, IsAbsent: arg.IsAbsent}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)
	assert.Equal(t, int32(1), capturedCount)
	assert.False(t, capturedAbsent)
}

func TestAutoResolveTurn_FlagsAbsentAfterThreeConsecutive(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID: turnID, EncounterID: encounterID, CombatantID: combatantID,
			Status: "active", ActionUsed: true,
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: combatantID, EncounterID: encounterID, DisplayName: "Aria",
			HpCurrent: 20, HpMax: 30, IsAlive: true, Conditions: json.RawMessage(`[]`),
			ConsecutiveAutoResolves: 2, // will become 3
		}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	var capturedAbsent bool
	store.updateCombatantAutoResolveCountFn = func(ctx context.Context, arg refdata.UpdateCombatantAutoResolveCountParams) (refdata.Combatant, error) {
		capturedAbsent = arg.IsAbsent
		return refdata.Combatant{ID: arg.ID, ConsecutiveAutoResolves: arg.ConsecutiveAutoResolves, IsAbsent: arg.IsAbsent}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	roller := dice.NewRoller(func(max int) int { return 10 })

	actions, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)
	assert.True(t, capturedAbsent)
	// Should mention absent in actions
	found := false
	for _, a := range actions {
		if containsStr(a, "absent") {
			found = true
		}
	}
	assert.True(t, found, "should flag absent")
}

func TestAutoResolveTurn_PostsToCombatLog(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID: turnID, EncounterID: encounterID, CombatantID: combatantID,
			Status: "active", ActionUsed: true,
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: combatantID, EncounterID: encounterID, DisplayName: "Aria",
			HpCurrent: 20, HpMax: 30, IsAlive: true, Conditions: json.RawMessage(`[]`),
		}, nil
	}
	// Campaign with combat-log channel
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		settings := `{"turn_timeout_hours":24,"channel_ids":{"your-turn":"chan-your-turn","combat-log":"chan-combat-log"}}`
		return refdata.Campaign{
			ID:       uuid.New(),
			Settings: pqtype.NullRawMessage{RawMessage: json.RawMessage(settings), Valid: true},
		}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)

	msgs := notifier.getMessages()
	require.NotEmpty(t, msgs)
	// Find the combat log message
	found := false
	for _, m := range msgs {
		if m.channelID == "chan-combat-log" {
			found = true
			assert.Contains(t, m.content, "auto-resolved")
			assert.Contains(t, m.content, "player timed out")
		}
	}
	assert.True(t, found, "should post to combat-log channel")
}

// --- WaitExtendTurn Tests ---

func TestWaitExtendTurn_ExtendsBy50Percent(t *testing.T) {
	turnID := uuid.New()
	now := time.Now()
	startedAt := now.Add(-24 * time.Hour)
	timeoutAt := now.Add(-1 * time.Hour) // just past timeout

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID:           turnID,
			WaitExtended: false,
			StartedAt:    sql.NullTime{Time: startedAt, Valid: true},
			TimeoutAt:    sql.NullTime{Time: timeoutAt, Valid: true},
		}, nil
	}

	var capturedTimeout sql.NullTime
	store.updateTurnTimeoutFn = func(ctx context.Context, arg refdata.UpdateTurnTimeoutParams) (refdata.Turn, error) {
		capturedTimeout = arg.TimeoutAt
		return refdata.Turn{ID: arg.ID, TimeoutAt: arg.TimeoutAt}, nil
	}

	waitExtendedCalled := false
	store.updateTurnWaitExtendedFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		waitExtendedCalled = true
		return refdata.Turn{ID: id, WaitExtended: true}, nil
	}

	resetCalled := false
	store.resetTurnNudgeAndWarningFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		resetCalled = true
		return refdata.Turn{ID: id}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.WaitExtendTurn(context.Background(), turnID)
	require.NoError(t, err)

	assert.True(t, waitExtendedCalled)
	assert.True(t, resetCalled)

	// Extension should be 50% of original duration (24h - 1h = ~23h original, 50% = ~11.5h)
	originalDuration := timeoutAt.Sub(startedAt)
	expectedExtension := originalDuration / 2
	expectedTimeout := timeoutAt.Add(expectedExtension)
	assert.InDelta(t, expectedTimeout.Unix(), capturedTimeout.Time.Unix(), 1)
}

func TestWaitExtendTurn_CannotExtendTwice(t *testing.T) {
	turnID := uuid.New()

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID:           turnID,
			WaitExtended: true, // already extended
			StartedAt:    sql.NullTime{Time: time.Now().Add(-24 * time.Hour), Valid: true},
			TimeoutAt:    sql.NullTime{Time: time.Now(), Valid: true},
		}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.WaitExtendTurn(context.Background(), turnID)
	assert.ErrorIs(t, err, ErrWaitAlreadyUsed)
}

// --- processDMAutoResolves Tests ---

func TestProcessDMAutoResolves_AutoResolvesAfterOneHour(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()

	store := defaultMockStore()
	store.listTurnsNeedingDMAutoResolveFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{{
			ID:          turnID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			Status:      "active",
		}}, nil
	}
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID: turnID, EncounterID: encounterID, CombatantID: combatantID,
			Status: "active", ActionUsed: true,
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: combatantID, EncounterID: encounterID, DisplayName: "Aria",
			HpCurrent: 20, HpMax: 30, IsAlive: true, Conditions: json.RawMessage(`[]`),
		}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	autoResolvedCalled := false
	store.updateTurnAutoResolvedFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		autoResolvedCalled = true
		return refdata.Turn{ID: id, AutoResolved: true}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.processDMAutoResolves(context.Background())
	require.NoError(t, err)
	assert.True(t, autoResolvedCalled)
}

// --- ScanStaleTurns Tests ---

func TestScanStaleTurns_ProcessesInDeadlineOrder(t *testing.T) {
	turnID1 := uuid.New()
	turnID2 := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()
	now := time.Now()

	store := defaultMockStore()

	// Two stale timed-out turns (no DM decision sent)
	store.listTurnsTimedOutFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID1, EncounterID: encounterID, CombatantID: combatantID,
				TimeoutAt: sql.NullTime{Time: now.Add(-2 * time.Hour), Valid: true}},
			{ID: turnID2, EncounterID: encounterID, CombatantID: combatantID,
				TimeoutAt: sql.NullTime{Time: now.Add(-1 * time.Hour), Valid: true}},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Aria", Conditions: json.RawMessage(`[]`)}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	// No DM auto-resolve needed
	store.listTurnsNeedingDMAutoResolveFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}

	var sentOrder []uuid.UUID
	store.updateTurnDMDecisionSentFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		sentOrder = append(sentOrder, id)
		return refdata.Turn{ID: id}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.ScanStaleTurns(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{turnID1, turnID2}, sentOrder)
}

// --- PollOnce integration ---

func TestPollOnce_IncludesTimeoutsAndDMAutoResolves(t *testing.T) {
	store := defaultMockStore()
	store.listTurnsNeedingNudgeFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.listTurnsNeedingWarningFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.listTurnsTimedOutFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.listTurnsNeedingDMAutoResolveFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.PollOnce(context.Background())
	require.NoError(t, err)
}

// --- Edge Case Tests ---

func TestProcessTimeouts_ListError(t *testing.T) {
	store := defaultMockStore()
	store.listTurnsTimedOutFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return nil, assert.AnError
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.processTimeouts(context.Background())
	assert.Error(t, err)
}

func TestProcessDMAutoResolves_ListError(t *testing.T) {
	store := defaultMockStore()
	store.listTurnsNeedingDMAutoResolveFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return nil, assert.AnError
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.processDMAutoResolves(context.Background())
	assert.Error(t, err)
}

func TestWaitExtendTurn_NoTimeoutSet(t *testing.T) {
	turnID := uuid.New()

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, WaitExtended: false}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.WaitExtendTurn(context.Background(), turnID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no timeout set")
}

func TestScanStaleTurns_TimedOutListError(t *testing.T) {
	store := defaultMockStore()
	store.listTurnsTimedOutFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return nil, assert.AnError
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.ScanStaleTurns(context.Background())
	assert.Error(t, err)
}

func TestScanStaleTurns_DMAutoResolveListError(t *testing.T) {
	store := defaultMockStore()
	store.listTurnsTimedOutFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.listTurnsNeedingDMAutoResolveFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return nil, assert.AnError
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.ScanStaleTurns(context.Background())
	assert.Error(t, err)
}

func TestAutoResolveTurn_GetTurnError(t *testing.T) {
	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, assert.AnError
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := timer.AutoResolveTurn(context.Background(), uuid.New(), roller)
	assert.Error(t, err)
}

func TestAutoResolveTurn_GetCombatantError(t *testing.T) {
	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: id, CombatantID: uuid.New()}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, assert.AnError
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := timer.AutoResolveTurn(context.Background(), uuid.New(), roller)
	assert.Error(t, err)
}

func TestProcessTimeouts_SendPromptFailsContinues(t *testing.T) {
	// Verifies that if one turn fails to send prompt, others are still processed
	turnID1 := uuid.New()
	turnID2 := uuid.New()
	encounterID := uuid.New()

	store := defaultMockStore()
	callCount := 0
	store.listTurnsTimedOutFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID1, EncounterID: encounterID, CombatantID: uuid.New()},
			{ID: turnID2, EncounterID: encounterID, CombatantID: uuid.New()},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		callCount++
		if callCount == 1 {
			return refdata.Combatant{}, assert.AnError
		}
		return refdata.Combatant{ID: id, DisplayName: "Test", Conditions: json.RawMessage(`[]`)}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}
	var sentIDs []uuid.UUID
	store.updateTurnDMDecisionSentFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		sentIDs = append(sentIDs, id)
		return refdata.Turn{ID: id}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.processTimeouts(context.Background())
	require.NoError(t, err) // Errors are logged, not returned
	assert.Len(t, sentIDs, 1)
	assert.Equal(t, turnID2, sentIDs[0])
}

func TestAutoResolveTurn_DyingCombatant_Dead(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()

	ds := DeathSaves{Successes: 0, Failures: 2}
	dsJSON, _ := json.Marshal(ds)

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID: turnID, EncounterID: encounterID, CombatantID: combatantID,
			RoundNumber: 2, Status: "active",
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: combatantID, EncounterID: encounterID, DisplayName: "Borr",
			HpCurrent: 0, HpMax: 40, IsAlive: true, Conditions: json.RawMessage(`[]`),
			DeathSaves: pqtype.NullRawMessage{RawMessage: dsJSON, Valid: true},
		}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	hpUpdatedForDeath := false
	deathSavesUpdated := false
	store.updateCombatantDeathSavesFn = func(ctx context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error) {
		deathSavesUpdated = true
		return refdata.Combatant{ID: arg.ID, DeathSaves: arg.DeathSaves}, nil
	}
	store.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpUpdatedForDeath = true
		assert.False(t, arg.IsAlive)
		return refdata.Combatant{ID: arg.ID}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	// Roll a 5 = failure -> 3 failures = dead
	roller := dice.NewRoller(func(max int) int { return 5 })

	_, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)
	assert.True(t, deathSavesUpdated)
	assert.True(t, hpUpdatedForDeath)
}

func TestScanStaleTurns_ProcessesBothTimedOutAndDMExpired(t *testing.T) {
	turnID1 := uuid.New()
	turnID2 := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()

	store := defaultMockStore()
	store.listTurnsTimedOutFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID1, EncounterID: encounterID, CombatantID: combatantID},
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Aria", Conditions: json.RawMessage(`[]`),
			HpCurrent: 20, HpMax: 30, IsAlive: true, EncounterID: encounterID}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	store.listTurnsNeedingDMAutoResolveFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID2, EncounterID: encounterID, CombatantID: combatantID, Status: "active"},
		}, nil
	}
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID: id, EncounterID: encounterID, CombatantID: combatantID,
			Status: "active", ActionUsed: true,
		}, nil
	}

	dmPromptSent := false
	store.updateTurnDMDecisionSentFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		dmPromptSent = true
		return refdata.Turn{ID: id}, nil
	}
	autoResolved := false
	store.updateTurnAutoResolvedFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		autoResolved = true
		return refdata.Turn{ID: id, AutoResolved: true}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.ScanStaleTurns(context.Background())
	require.NoError(t, err)
	assert.True(t, dmPromptSent)
	assert.True(t, autoResolved)
}

func TestAutoResolveTurn_NoCombatLogChannel(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID: turnID, EncounterID: encounterID, CombatantID: combatantID,
			Status: "active", ActionUsed: true,
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: combatantID, EncounterID: encounterID, DisplayName: "Aria",
			HpCurrent: 20, HpMax: 30, IsAlive: true, Conditions: json.RawMessage(`[]`),
		}, nil
	}
	// No combat-log channel configured
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	roller := dice.NewRoller(func(max int) int { return 10 })

	actions, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)
	assert.NotEmpty(t, actions)
	// No messages sent to notifier (no combat-log channel)
	assert.Empty(t, notifier.getMessages())
}

// --- Helper ---

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
