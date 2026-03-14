package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/refdata"
)

// mockNotifier records messages sent via the Notifier interface.
type mockNotifier struct {
	mu       sync.Mutex
	messages []sentMessage
}

type sentMessage struct {
	channelID string
	content   string
}

func (m *mockNotifier) SendMessage(channelID, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, sentMessage{channelID: channelID, content: content})
	return nil
}

func (m *mockNotifier) getMessages() []sentMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]sentMessage, len(m.messages))
	copy(cp, m.messages)
	return cp
}

func settingsJSON(hours int) pqtype.NullRawMessage {
	s := campaign.Settings{
		TurnTimeoutHours: hours,
		ChannelIDs:       map[string]string{"your-turn": "chan-your-turn"},
	}
	data, _ := json.Marshal(s)
	return pqtype.NullRawMessage{RawMessage: data, Valid: true}
}

func TestPollOnce_SendsNudge(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()
	now := time.Now()
	startedAt := now.Add(-13 * time.Hour) // started 13h ago
	timeoutAt := now.Add(11 * time.Hour)  // 24h timeout, 11h left (past 50%)

	store := defaultMockStore()
	store.listTurnsNeedingNudgeFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{{
			ID:          turnID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			StartedAt:   sql.NullTime{Time: startedAt, Valid: true},
			TimeoutAt:   sql.NullTime{Time: timeoutAt, Valid: true},
		}}, nil
	}
	store.listTurnsNeedingWarningFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Aria", Conditions: json.RawMessage(`[]`)}, nil
	}
	nudgeSentCalled := false
	store.updateTurnNudgeSentFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		nudgeSentCalled = true
		return refdata.Turn{ID: id}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{
			ID:       uuid.New(),
			Settings: settingsJSON(24),
		}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.PollOnce(context.Background())
	require.NoError(t, err)

	assert.True(t, nudgeSentCalled)
	msgs := notifier.getMessages()
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0].content, "@Aria")
	assert.Equal(t, "chan-your-turn", msgs[0].channelID)
}

func TestPollOnce_SendsWarning(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()
	now := time.Now()
	startedAt := now.Add(-19 * time.Hour)
	timeoutAt := now.Add(5 * time.Hour) // 24h timeout, 5h left (past 75%)

	store := defaultMockStore()
	store.listTurnsNeedingNudgeFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.listTurnsNeedingWarningFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{{
			ID:          turnID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			StartedAt:   sql.NullTime{Time: startedAt, Valid: true},
			TimeoutAt:   sql.NullTime{Time: timeoutAt, Valid: true},
			MovementRemainingFt: 30,
			AttacksRemaining:    1,
		}}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          combatantID,
			DisplayName: "Aria",
			HpCurrent:   28,
			HpMax:       45,
			Ac:          16,
			Conditions:  json.RawMessage(`[{"condition":"poisoned","duration_rounds":3,"started_round":1}]`),
			PositionCol: "C",
			PositionRow: 3,
			EncounterID: encounterID,
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: combatantID, DisplayName: "Aria", PositionCol: "C", PositionRow: 3, IsAlive: true, IsNpc: false, Conditions: json.RawMessage(`[]`)},
			{ID: uuid.New(), DisplayName: "Goblin #1", ShortID: "G1", PositionCol: "C", PositionRow: 4, IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`)},
			{ID: uuid.New(), DisplayName: "Orc", ShortID: "OR", PositionCol: "E", PositionRow: 10, IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	warningSentCalled := false
	store.updateTurnWarningSentFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		warningSentCalled = true
		return refdata.Turn{ID: id}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{
			ID:       uuid.New(),
			Settings: settingsJSON(24),
		}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.PollOnce(context.Background())
	require.NoError(t, err)

	assert.True(t, warningSentCalled)
	msgs := notifier.getMessages()
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0].content, "@Aria")
	assert.Contains(t, msgs[0].content, "HP: 28/45")
	assert.Contains(t, msgs[0].content, "AC: 16")
	assert.Contains(t, msgs[0].content, "Poisoned")
	assert.Contains(t, msgs[0].content, "Goblin #1 (G1)")
	// Orc is too far (E10 vs C3), should not be in adjacent
	assert.NotContains(t, msgs[0].content, "Orc (OR)")
}

func TestPollOnce_NothingToDo(t *testing.T) {
	store := defaultMockStore()
	store.listTurnsNeedingNudgeFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.listTurnsNeedingWarningFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.PollOnce(context.Background())
	require.NoError(t, err)
	assert.Empty(t, notifier.getMessages())
}

func TestStartStop(t *testing.T) {
	store := defaultMockStore()
	store.listTurnsNeedingNudgeFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.listTurnsNeedingWarningFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 50*time.Millisecond)

	timer.Start()
	time.Sleep(100 * time.Millisecond)
	timer.Stop()
	// Should not panic or hang
}

func TestFindAdjacentEnemies(t *testing.T) {
	combatantID := uuid.New()
	combatant := refdata.Combatant{
		ID:          combatantID,
		PositionCol: "C",
		PositionRow: 3,
		IsNpc:       false,
		IsAlive:     true,
	}
	allCombatants := []refdata.Combatant{
		combatant,
		{ID: uuid.New(), DisplayName: "Goblin", ShortID: "G1", PositionCol: "C", PositionRow: 4, IsNpc: true, IsAlive: true},
		{ID: uuid.New(), DisplayName: "Goblin2", ShortID: "G2", PositionCol: "D", PositionRow: 4, IsNpc: true, IsAlive: true},
		{ID: uuid.New(), DisplayName: "FarGoblin", ShortID: "G3", PositionCol: "F", PositionRow: 10, IsNpc: true, IsAlive: true},
		{ID: uuid.New(), DisplayName: "DeadGoblin", ShortID: "G4", PositionCol: "C", PositionRow: 2, IsNpc: true, IsAlive: false},
		{ID: uuid.New(), DisplayName: "Ally", ShortID: "AL", PositionCol: "B", PositionRow: 3, IsNpc: false, IsAlive: true},
	}

	adjacent := findAdjacentEnemies(combatant, allCombatants)
	assert.Len(t, adjacent, 2)
	assert.Equal(t, "Goblin", adjacent[0].DisplayName)
	assert.Equal(t, "Goblin2", adjacent[1].DisplayName)
}

func TestCreateActiveTurn_SetsStartedAtAndTimeoutAt(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()

	var capturedParams refdata.CreateTurnParams
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		capturedParams = arg
		return refdata.Turn{
			ID:          uuid.New(),
			EncounterID: arg.EncounterID,
			CombatantID: arg.CombatantID,
			Status:      arg.Status,
			StartedAt:   arg.StartedAt,
			TimeoutAt:   arg.TimeoutAt,
		}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{
			ID:       uuid.New(),
			Settings: settingsJSON(24),
		}, nil
	}

	combatant := refdata.Combatant{
		ID:          combatantID,
		IsNpc:       false,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}

	svc := NewService(store)
	_, err := svc.createActiveTurn(context.Background(), encounterID, 1, combatant)
	require.NoError(t, err)

	assert.True(t, capturedParams.StartedAt.Valid, "started_at should be set")
	assert.True(t, capturedParams.TimeoutAt.Valid, "timeout_at should be set")

	// timeout_at should be ~24 hours from started_at
	diff := capturedParams.TimeoutAt.Time.Sub(capturedParams.StartedAt.Time)
	assert.InDelta(t, 24*time.Hour, diff, float64(time.Minute))
}

func TestCreateActiveTurn_NPC_NoTimeout(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()

	var capturedParams refdata.CreateTurnParams
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		capturedParams = arg
		return refdata.Turn{
			ID:          uuid.New(),
			EncounterID: arg.EncounterID,
			CombatantID: arg.CombatantID,
			Status:      arg.Status,
		}, nil
	}

	combatant := refdata.Combatant{
		ID:          combatantID,
		IsNpc:       true,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}

	svc := NewService(store)
	_, err := svc.createActiveTurn(context.Background(), encounterID, 1, combatant)
	require.NoError(t, err)

	assert.False(t, capturedParams.StartedAt.Valid, "started_at should not be set for NPCs")
	assert.False(t, capturedParams.TimeoutAt.Valid, "timeout_at should not be set for NPCs")
}

func TestPollOnce_NudgeListError(t *testing.T) {
	store := defaultMockStore()
	store.listTurnsNeedingNudgeFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return nil, errors.New("db error")
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.PollOnce(context.Background())
	require.Error(t, err)
}

func TestPollOnce_WarningListError(t *testing.T) {
	store := defaultMockStore()
	store.listTurnsNeedingNudgeFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.listTurnsNeedingWarningFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return nil, errors.New("db error")
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.PollOnce(context.Background())
	require.Error(t, err)
}

func TestPollOnce_NudgeWithNoCampaignChannel(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()
	now := time.Now()

	store := defaultMockStore()
	store.listTurnsNeedingNudgeFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{{
			ID:          turnID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			StartedAt:   sql.NullTime{Time: now.Add(-13 * time.Hour), Valid: true},
			TimeoutAt:   sql.NullTime{Time: now.Add(11 * time.Hour), Valid: true},
		}}, nil
	}
	store.listTurnsNeedingWarningFn = func(ctx context.Context) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Aria", Conditions: json.RawMessage(`[]`)}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		// No channel IDs configured
		s := campaign.Settings{TurnTimeoutHours: 24}
		data, _ := json.Marshal(s)
		return refdata.Campaign{
			ID:       uuid.New(),
			Settings: pqtype.NullRawMessage{RawMessage: data, Valid: true},
		}, nil
	}
	nudgeSentCalled := false
	store.updateTurnNudgeSentFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		nudgeSentCalled = true
		return refdata.Turn{ID: id}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	err := timer.PollOnce(context.Background())
	require.NoError(t, err)
	// Nudge should still be marked as sent even without a channel (to avoid retry loops)
	assert.True(t, nudgeSentCalled)
	assert.Empty(t, notifier.getMessages()) // but no message actually sent
}

func TestFindAdjacentEnemies_EmptyList(t *testing.T) {
	combatant := refdata.Combatant{
		ID:          uuid.New(),
		PositionCol: "C",
		PositionRow: 3,
		IsNpc:       false,
		IsAlive:     true,
	}
	adjacent := findAdjacentEnemies(combatant, []refdata.Combatant{combatant})
	assert.Empty(t, adjacent)
}

func TestFindAdjacentEnemies_DiagonalAdjacent(t *testing.T) {
	combatantID := uuid.New()
	combatant := refdata.Combatant{
		ID:          combatantID,
		PositionCol: "C",
		PositionRow: 3,
		IsNpc:       false,
		IsAlive:     true,
	}
	allCombatants := []refdata.Combatant{
		combatant,
		{ID: uuid.New(), DisplayName: "Diagonal", ShortID: "DG", PositionCol: "D", PositionRow: 4, IsNpc: true, IsAlive: true},
	}

	adjacent := findAdjacentEnemies(combatant, allCombatants)
	assert.Len(t, adjacent, 1)
	assert.Equal(t, "Diagonal", adjacent[0].DisplayName)
}

func TestResolveTimerForTurn_CampaignLookupFails(t *testing.T) {
	store := defaultMockStore()
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{}, errors.New("not found")
	}

	svc := NewService(store)
	startedAt, timeoutAt := svc.resolveTimerForTurn(context.Background(), uuid.New())
	assert.False(t, startedAt.Valid)
	assert.False(t, timeoutAt.Valid)
}

func TestResolveTimerForTurn_DefaultTimeout(t *testing.T) {
	store := defaultMockStore()
	// Settings with 0 timeout hours should default to 24
	s := campaign.Settings{TurnTimeoutHours: 0}
	data, _ := json.Marshal(s)
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{
			ID:       uuid.New(),
			Settings: pqtype.NullRawMessage{RawMessage: data, Valid: true},
		}, nil
	}

	svc := NewService(store)
	startedAt, timeoutAt := svc.resolveTimerForTurn(context.Background(), uuid.New())
	assert.True(t, startedAt.Valid)
	assert.True(t, timeoutAt.Valid)

	diff := timeoutAt.Time.Sub(startedAt.Time)
	assert.InDelta(t, 24*time.Hour, diff, float64(time.Minute))
}

func TestTimerColToIndex(t *testing.T) {
	tests := []struct {
		col  string
		want int
	}{
		{"A", 0},
		{"B", 1},
		{"Z", 25},
		{"a", 0},
	}
	for _, tt := range tests {
		got := colToIndex(tt.col)
		assert.Equal(t, tt.want, got, "colToIndex(%q)", tt.col)
	}
}
