package combat

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/refdata"
)

func TestService_ListActionLogWithRounds(t *testing.T) {
	expectedLogs := []refdata.ListActionLogWithRoundsRow{
		{RoundNumber: 1, Description: sql.NullString{String: "test", Valid: true}},
	}
	ms := defaultMockStore()
	ms.listActionLogWithRoundsFn = func(_ context.Context, _ uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
		return expectedLogs, nil
	}
	svc := NewService(ms)

	logs, err := svc.ListActionLogWithRounds(context.Background(), uuid.New())
	assert.NoError(t, err)
	assert.Equal(t, expectedLogs, logs)
}

func TestService_GetMostRecentCompletedEncounter(t *testing.T) {
	encID := uuid.New()
	ms := defaultMockStore()
	ms.getMostRecentCompletedEncounterFn = func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: encID, Status: "completed"}, nil
	}
	svc := NewService(ms)

	enc, err := svc.GetMostRecentCompletedEncounter(context.Background(), uuid.New())
	assert.NoError(t, err)
	assert.Equal(t, encID, enc.ID)
}

func TestService_GetLastCompletedTurnByCombatant(t *testing.T) {
	turnID := uuid.New()
	ms := defaultMockStore()
	ms.getLastCompletedTurnByCombatantFn = func(_ context.Context, _ refdata.GetLastCompletedTurnByCombatantParams) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, RoundNumber: 3}, nil
	}
	svc := NewService(ms)
	turn, err := svc.GetLastCompletedTurnByCombatant(context.Background(), uuid.New(), uuid.New())
	assert.NoError(t, err)
	assert.Equal(t, turnID, turn.ID)
}

func TestService_GetCampaignByEncounterID(t *testing.T) {
	campID := uuid.New()
	ms := defaultMockStore()
	ms.getCampaignByEncounterIDFn = func(_ context.Context, _ uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: campID}, nil
	}
	svc := NewService(ms)
	camp, err := svc.GetCampaignByEncounterID(context.Background(), uuid.New())
	assert.NoError(t, err)
	assert.Equal(t, campID, camp.ID)
}

func TestFormatRecap_EmptyLogs(t *testing.T) {
	result := FormatRecap(nil, "Rounds 1–2")
	assert.Equal(t, "No combat activity to recap.", result)
}

func TestFormatRecap_SkipsNullDescriptions(t *testing.T) {
	logs := []refdata.ListActionLogWithRoundsRow{
		{
			ID:          uuid.New(),
			RoundNumber: 1,
			Description: sql.NullString{Valid: false},
		},
		{
			ID:          uuid.New(),
			RoundNumber: 1,
			Description: sql.NullString{String: "Thorn attacked", Valid: true},
		},
	}

	result := FormatRecap(logs, "Round 1")
	assert.Contains(t, result, "Thorn attacked")
	assert.NotContains(t, result, "\n\n\n") // no blank lines for skipped entries
}

func TestFilterLogsSinceRound(t *testing.T) {
	logs := []refdata.ListActionLogWithRoundsRow{
		{RoundNumber: 1, Description: sql.NullString{String: "r1", Valid: true}},
		{RoundNumber: 2, Description: sql.NullString{String: "r2", Valid: true}},
		{RoundNumber: 3, Description: sql.NullString{String: "r3", Valid: true}},
		{RoundNumber: 4, Description: sql.NullString{String: "r4", Valid: true}},
	}

	filtered := FilterLogsSinceRound(logs, 3)
	assert.Len(t, filtered, 2)
	assert.Equal(t, int32(3), filtered[0].RoundNumber)
	assert.Equal(t, int32(4), filtered[1].RoundNumber)
}

func TestFilterLogsLastNRounds_ZeroOrNegative(t *testing.T) {
	logs := []refdata.ListActionLogWithRoundsRow{
		{RoundNumber: 1},
	}
	assert.Nil(t, FilterLogsLastNRounds(logs, 0))
	assert.Nil(t, FilterLogsLastNRounds(logs, -1))
	assert.Nil(t, FilterLogsLastNRounds(nil, 5))
}

func TestFilterLogsLastNRounds_MoreThanAvailable(t *testing.T) {
	logs := []refdata.ListActionLogWithRoundsRow{
		{RoundNumber: 1, Description: sql.NullString{String: "r1", Valid: true}},
		{RoundNumber: 2, Description: sql.NullString{String: "r2", Valid: true}},
	}
	// Requesting 10 rounds but only 2 exist — return all
	filtered := FilterLogsLastNRounds(logs, 10)
	assert.Len(t, filtered, 2)
}

func TestFilterLogsLastNRounds(t *testing.T) {
	logs := []refdata.ListActionLogWithRoundsRow{
		{RoundNumber: 1, Description: sql.NullString{String: "r1", Valid: true}},
		{RoundNumber: 2, Description: sql.NullString{String: "r2", Valid: true}},
		{RoundNumber: 3, Description: sql.NullString{String: "r3", Valid: true}},
		{RoundNumber: 5, Description: sql.NullString{String: "r5", Valid: true}},
	}

	// Last 2 rounds = rounds 3 and 5
	filtered := FilterLogsLastNRounds(logs, 2)
	assert.Len(t, filtered, 2)
	assert.Equal(t, int32(3), filtered[0].RoundNumber)
	assert.Equal(t, int32(5), filtered[1].RoundNumber)
}

func TestRecapRoundRange(t *testing.T) {
	logs := []refdata.ListActionLogWithRoundsRow{
		{RoundNumber: 3},
		{RoundNumber: 3},
		{RoundNumber: 5},
	}
	assert.Equal(t, "Rounds 3\u20135", RecapRoundRange(logs))

	single := []refdata.ListActionLogWithRoundsRow{{RoundNumber: 2}}
	assert.Equal(t, "Round 2", RecapRoundRange(single))

	assert.Equal(t, "", RecapRoundRange(nil))
}

func TestTruncateRecap(t *testing.T) {
	short := "Hello, world!"
	assert.Equal(t, short, TruncateRecap(short, 2000))

	long := strings.Repeat("A", 2100)
	truncated := TruncateRecap(long, 2000)
	assert.True(t, len(truncated) <= 2000)
	assert.Contains(t, truncated, "... (truncated)")
}

func TestFormatRecap_GroupsByRound(t *testing.T) {
	logs := []refdata.ListActionLogWithRoundsRow{
		{
			ID:          uuid.New(),
			RoundNumber: 1,
			Description: sql.NullString{String: "Goblin #1 attacked Thorn", Valid: true},
		},
		{
			ID:          uuid.New(),
			RoundNumber: 1,
			Description: sql.NullString{String: "Thorn attacked Goblin #1", Valid: true},
		},
		{
			ID:          uuid.New(),
			RoundNumber: 2,
			Description: sql.NullString{String: "Goblin #1 Disengaged", Valid: true},
		},
	}

	result := FormatRecap(logs, "Rounds 1\u20132")
	assert.Contains(t, result, "\u2500\u2500 Round 1 \u2500\u2500")
	assert.Contains(t, result, "\u2500\u2500 Round 2 \u2500\u2500")
	assert.Contains(t, result, "Goblin #1 attacked Thorn")
	assert.Contains(t, result, "Thorn attacked Goblin #1")
	assert.Contains(t, result, "Goblin #1 Disengaged")
	assert.Contains(t, result, "\U0001f4dc Recap \u2014 Rounds 1\u20132")
}
