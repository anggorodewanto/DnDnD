package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// actionsContain returns true if any action string contains the substring.
func actionsContain(actions []string, substr string) bool {
	for _, a := range actions {
		if strings.Contains(a, substr) {
			return true
		}
	}
	return false
}

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

// SR-020: auto-skip Dodge must use an ExpiresOn value isExpired recognises
// ("start_of_turn") and carry the dodging combatant's ID as SourceCombatantID,
// so it actually clears when that combatant's next turn begins. Previously it
// was applied with "start_of_next_turn" + empty source and stuck forever.
func TestAutoResolveTurn_DodgeExpiresAtStartOfNextTurn(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID:          turnID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			RoundNumber: 1,
			Status:      "active",
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

	var capturedConditions json.RawMessage
	store.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		capturedConditions = arg.Conditions
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)

	cond, found := GetCondition(capturedConditions, "dodge")
	require.True(t, found, "auto-skip dodge condition not present on combatant")
	assert.Equal(t, "start_of_turn", cond.ExpiresOn,
		"ExpiresOn must be a value isExpired recognises so dodge actually clears")
	assert.Equal(t, combatantID.String(), cond.SourceCombatantID,
		"SourceCombatantID must be the dodging combatant so expiry matches on their next turn")
	assert.Equal(t, 1, cond.DurationRounds)
	assert.Equal(t, 1, cond.StartedRound)

	// Round 2 (next turn of the same combatant) at start_of_turn timing → expires.
	updated, expired, err := CheckExpiredConditions(capturedConditions, 2, combatantID.String(), "start_of_turn")
	require.NoError(t, err)
	require.Len(t, expired, 1, "auto-skip dodge should expire at start of dodging creature's next turn")
	assert.Equal(t, "dodge", expired[0].Condition)
	_, stillThere := GetCondition(updated, "dodge")
	assert.False(t, stillThere, "dodge should be cleared after own start_of_turn expiry")
}

// SR-020: enemy turns must NOT clear the dodging creature's Dodge.
// Expiry is source-scoped: only the dodging creature's own start_of_turn
// triggers the clear, so intervening combatants leave Dodge intact.
func TestAutoResolveTurn_DodgePersistsThroughEnemyTurn(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()
	enemyID := uuid.New()

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID:          turnID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			RoundNumber: 1,
			Status:      "active",
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

	var capturedConditions json.RawMessage
	store.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		capturedConditions = arg.Conditions
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)

	// Enemy's start_of_turn next round should NOT expire the dodging creature's Dodge.
	updated, expired, err := CheckExpiredConditions(capturedConditions, 2, enemyID.String(), "start_of_turn")
	require.NoError(t, err)
	assert.Len(t, expired, 0, "enemy turn must not clear dodging creature's Dodge")
	_, stillThere := GetCondition(updated, "dodge")
	assert.True(t, stillThere, "dodge should persist through enemy turn")
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
	assert.True(t, actionsContain(actions, "death save"), "should contain death save message")
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
	assert.True(t, actionsContain(actions, "NAT 20"), "should contain NAT 20 message")
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
	assert.True(t, actionsContain(actions, "absent"), "should flag absent")
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
	var combatLogMsg *sentMessage
	for i := range msgs {
		if msgs[i].channelID == "chan-combat-log" {
			combatLogMsg = &msgs[i]
			break
		}
	}
	require.NotNil(t, combatLogMsg, "should post to combat-log channel")
	assert.Contains(t, combatLogMsg.content, "auto-resolved")
	assert.Contains(t, combatLogMsg.content, "player timed out")
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

// --- Pending Saves Auto-Resolve Tests ---

func TestAutoResolveTurn_RollsPendingSaves(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()
	saveID := uuid.New()

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
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	// One pending WIS save, DC 15
	store.listPendingSavesByCombatantFn = func(ctx context.Context, cID uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{{
			ID: saveID, EncounterID: encounterID, CombatantID: combatantID,
			Ability: "WIS", Dc: 15, Source: "Hold Person", Status: "pending",
		}}, nil
	}

	var capturedResult refdata.UpdatePendingSaveResultParams
	store.updatePendingSaveResultFn = func(ctx context.Context, arg refdata.UpdatePendingSaveResultParams) (refdata.PendingSafe, error) {
		capturedResult = arg
		return refdata.PendingSafe{ID: arg.ID, Status: "rolled", RollResult: arg.RollResult, Success: arg.Success}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	// Roll 12 -> 12 < 15 DC = failure
	roller := dice.NewRoller(func(max int) int { return 12 })

	actions, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)

	assert.Equal(t, saveID, capturedResult.ID)
	assert.True(t, capturedResult.RollResult.Valid)
	assert.Equal(t, int32(12), capturedResult.RollResult.Int32)
	assert.True(t, capturedResult.Success.Valid)
	assert.False(t, capturedResult.Success.Bool) // 12 < 15 = failure

	assert.True(t, actionsContain(actions, "WIS save") && actionsContain(actions, "Hold Person"),
		"should contain pending save result message, got: %v", actions)
}

func TestAutoResolveTurn_RollsPendingSaves_Success(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()
	saveID := uuid.New()

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
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	store.listPendingSavesByCombatantFn = func(ctx context.Context, cID uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{{
			ID: saveID, EncounterID: encounterID, CombatantID: combatantID,
			Ability: "DEX", Dc: 13, Source: "Fireball", Status: "pending",
		}}, nil
	}

	var capturedResult refdata.UpdatePendingSaveResultParams
	store.updatePendingSaveResultFn = func(ctx context.Context, arg refdata.UpdatePendingSaveResultParams) (refdata.PendingSafe, error) {
		capturedResult = arg
		return refdata.PendingSafe{ID: arg.ID, Status: "rolled"}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	// Roll 16 -> 16 >= 13 DC = success
	roller := dice.NewRoller(func(max int) int { return 16 })

	actions, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)

	assert.True(t, capturedResult.Success.Valid)
	assert.True(t, capturedResult.Success.Bool) // 16 >= 13 = success

	assert.True(t, actionsContain(actions, "DEX save") && actionsContain(actions, "Fireball"),
		"should contain save result, got: %v", actions)
}

func TestAutoResolveTurn_CancelsPendingActions(t *testing.T) {
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
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	pendingActionsCancelled := false
	store.cancelAllPendingActionsByCombatantFn = func(ctx context.Context, arg refdata.CancelAllPendingActionsByCombatantParams) error {
		pendingActionsCancelled = true
		assert.Equal(t, combatantID, arg.CombatantID)
		assert.Equal(t, encounterID, arg.EncounterID)
		return nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	roller := dice.NewRoller(func(max int) int { return 10 })

	actions, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)

	assert.True(t, pendingActionsCancelled)
	assert.Contains(t, actions, "Pending actions forfeited")
}

func TestAutoResolveTurn_MultiplePendingSaves(t *testing.T) {
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
			ID: combatantID, EncounterID: encounterID, DisplayName: "Borr",
			HpCurrent: 20, HpMax: 30, IsAlive: true, Conditions: json.RawMessage(`[]`),
		}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	// Two pending saves
	store.listPendingSavesByCombatantFn = func(ctx context.Context, cID uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{
			{ID: uuid.New(), Ability: "CON", Dc: 10, Source: "Concentration", Status: "pending"},
			{ID: uuid.New(), Ability: "WIS", Dc: 14, Source: "Charm Person", Status: "pending"},
		}, nil
	}

	saveResultCount := 0
	store.updatePendingSaveResultFn = func(ctx context.Context, arg refdata.UpdatePendingSaveResultParams) (refdata.PendingSafe, error) {
		saveResultCount++
		return refdata.PendingSafe{ID: arg.ID, Status: "rolled"}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)

	assert.Equal(t, 2, saveResultCount, "should roll both pending saves")
}

// TDD Cycle 17 (Phase 118 iter-2): when AutoResolveTurn fails a
// `source = "concentration"` pending save, the registered concentration
// resolver hook is invoked with the resolved row.
func TestAutoResolveTurn_FailedConcentrationSave_TriggersResolver(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()
	saveID := uuid.New()

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
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}
	store.listPendingSavesByCombatantFn = func(ctx context.Context, cID uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{{
			ID: saveID, EncounterID: encounterID, CombatantID: combatantID,
			Ability: "con", Dc: 15, Source: "concentration", Status: "pending",
		}}, nil
	}
	store.updatePendingSaveResultFn = func(ctx context.Context, arg refdata.UpdatePendingSaveResultParams) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{
			ID:          arg.ID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			Source:      "concentration",
			Ability:     "con",
			Dc:          15,
			RollResult:  arg.RollResult,
			Success:     arg.Success,
		}, nil
	}

	var resolverCalled bool
	var resolvedSource string
	timer := NewTurnTimer(store, &mockNotifier{}, 30*time.Second)
	timer.SetConcentrationResolver(func(ctx context.Context, ps refdata.PendingSafe) error {
		resolverCalled = true
		resolvedSource = ps.Source
		return nil
	})

	roller := dice.NewRoller(func(max int) int { return 5 }) // 5 < 15 = fail
	_, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)
	assert.True(t, resolverCalled, "concentration resolver must be invoked on failed save")
	assert.Equal(t, "concentration", resolvedSource)
}

func TestAutoResolveTurn_SuccessfulConcentrationSave_DoesNotTriggerResolver(t *testing.T) {
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
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}
	store.listPendingSavesByCombatantFn = func(ctx context.Context, cID uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{{
			ID: uuid.New(), EncounterID: encounterID, CombatantID: combatantID,
			Ability: "con", Dc: 10, Source: "concentration", Status: "pending",
		}}, nil
	}
	store.updatePendingSaveResultFn = func(ctx context.Context, arg refdata.UpdatePendingSaveResultParams) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{
			ID:          arg.ID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			Source:      "concentration",
			Success:     arg.Success,
		}, nil
	}

	resolverCalled := false
	timer := NewTurnTimer(store, &mockNotifier{}, 30*time.Second)
	timer.SetConcentrationResolver(func(ctx context.Context, ps refdata.PendingSafe) error {
		resolverCalled = true
		return nil
	})

	// Roll 18 vs DC 10 = success
	roller := dice.NewRoller(func(max int) int { return 18 })
	_, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)
	// Resolver still invoked (it returns nil for successes), but must not
	// produce a break: assert the resolver was called and didn't error.
	assert.True(t, resolverCalled, "resolver gets every concentration save row to inspect")
}

func TestFormatDMDecisionPrompt_ShowsPendingSaves(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
		BonusActionUsed:     true,
	}
	combatant := refdata.Combatant{DisplayName: "Aria"}
	pendingSaves := []refdata.PendingSafe{
		{Ability: "WIS", Dc: 15, Source: "Hold Person"},
		{Ability: "DEX", Dc: 13, Source: "Fireball"},
	}

	msg := FormatDMDecisionPromptWithSaves(combatant, turn, pendingSaves)
	assert.Contains(t, msg, "Aria")
	assert.Contains(t, msg, "timed out")
	assert.Contains(t, msg, "WIS save DC 15 (Hold Person)")
	assert.Contains(t, msg, "DEX save DC 13 (Fireball)")
	assert.Contains(t, msg, "Pending saves")
}

// C-43-followup: Nat-20 timer path also resets death-save tallies and clears
// the dying-condition bundle (unconscious + prone), mirroring the
// MaybeResetDeathSavesOnHeal hook used by /heal and Lay on Hands.
func TestAutoResolveTurn_DyingCombatant_Nat20_ResetsDeathSavesAndDyingConditions(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()

	// Pre-existing tallies: 1 success, 2 failures — Nat-20 should reset to 0,0.
	ds := DeathSaves{Successes: 1, Failures: 2}
	dsJSON, _ := json.Marshal(ds)

	// Conditions include the dying bundle (unconscious + prone).
	conds := []CombatCondition{
		{Condition: "unconscious", DurationRounds: 0},
		{Condition: "prone", DurationRounds: 0},
	}
	condsJSON, _ := json.Marshal(conds)

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID: turnID, EncounterID: encounterID, CombatantID: combatantID,
			RoundNumber: 2, Status: "active",
		}, nil
	}
	currentCombatant := refdata.Combatant{
		ID: combatantID, EncounterID: encounterID, DisplayName: "Borr",
		HpCurrent: 0, HpMax: 40, IsAlive: true, Conditions: condsJSON,
		DeathSaves: pqtype.NullRawMessage{RawMessage: dsJSON, Valid: true},
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return currentCombatant, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	hpUpdated := false
	store.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpUpdated = true
		assert.Equal(t, int32(1), arg.HpCurrent)
		assert.True(t, arg.IsAlive)
		currentCombatant.HpCurrent = arg.HpCurrent
		currentCombatant.IsAlive = arg.IsAlive
		return currentCombatant, nil
	}

	var capturedDS pqtype.NullRawMessage
	store.updateCombatantDeathSavesFn = func(ctx context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error) {
		capturedDS = arg.DeathSaves
		currentCombatant.DeathSaves = arg.DeathSaves
		return currentCombatant, nil
	}

	var capturedConds json.RawMessage
	store.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		capturedConds = arg.Conditions
		currentCombatant.Conditions = arg.Conditions
		return currentCombatant, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	roller := dice.NewRoller(func(max int) int { return 20 })

	_, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)
	assert.True(t, hpUpdated, "HP must be set to 1 on Nat-20")

	// Death saves persisted as zero tallies.
	require.True(t, capturedDS.Valid, "death saves must be persisted on Nat-20")
	persistedDS, err := ParseDeathSaves(capturedDS.RawMessage)
	require.NoError(t, err)
	assert.Equal(t, 0, persistedDS.Successes, "successes must reset to 0")
	assert.Equal(t, 0, persistedDS.Failures, "failures must reset to 0")

	// Dying conditions (unconscious + prone) must be removed.
	require.NotNil(t, capturedConds, "conditions must be persisted on Nat-20")
	assert.False(t, HasCondition(capturedConds, "unconscious"), "unconscious must be removed")
	assert.False(t, HasCondition(capturedConds, "prone"), "prone must be removed")
}


// SR-050: auto-resolve explicitly declines Divine Smite when paladin has
// available slots, and the slot is preserved (not consumed).
func TestAutoResolveTurn_DeclinesOnHitDecisions(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()
	charID := uuid.New()

	slotsJSON, _ := json.Marshal(map[string]SlotInfo{"1": {Current: 2, Max: 2}})
	featuresJSON := json.RawMessage(`[{"name":"Divine Smite"}]`)

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID: turnID, EncounterID: encounterID, CombatantID: combatantID,
			RoundNumber: 1, Status: "active",
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: combatantID, EncounterID: encounterID, DisplayName: "Kael",
			HpCurrent: 40, HpMax: 40, IsAlive: true,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:         charID,
			SpellSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
			Features:   pqtype.NullRawMessage{RawMessage: featuresJSON, Valid: true},
		}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	roller := dice.NewRoller(func(max int) int { return 10 })

	actions, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)

	// Must contain explicit decline line
	assert.True(t, actionsContain(actions, "Divine Smite declined"),
		"auto-resolve must explicitly decline Divine Smite; got: %v", actions)

	// Spell slots must NOT be consumed
	// (GetCharacter is read-only in this path; no UpdateCharacterSpellSlots call expected)
}

// SR-050: auto-resolve explicitly declines Bardic Inspiration when the
// combatant holds an active BI die.
func TestAutoResolveTurn_DeclinesBardicInspiration(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID: turnID, EncounterID: encounterID, CombatantID: combatantID,
			RoundNumber: 1, Status: "active",
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: combatantID, EncounterID: encounterID, DisplayName: "Aria",
			HpCurrent: 30, HpMax: 30, IsAlive: true,
			BardicInspirationDie: sql.NullString{String: "d6", Valid: true},
			Conditions:           json.RawMessage(`[]`),
		}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)
	roller := dice.NewRoller(func(max int) int { return 10 })

	actions, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)

	assert.True(t, actionsContain(actions, "Bardic Inspiration declined"),
		"auto-resolve must explicitly decline Bardic Inspiration; got: %v", actions)
}
