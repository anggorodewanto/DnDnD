package combat

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/refdata"
)

func TestBuildImpactSummary_NoLogs(t *testing.T) {
	result := BuildImpactSummary(nil)
	assert.Equal(t, "", result)
}

func TestBuildImpactSummary_DamageTaken(t *testing.T) {
	logs := []refdata.ActionLog{
		{
			ActionType:  "damage",
			Description: sql.NullString{String: "Goblin #2 hits Aria for 8 slashing damage", Valid: true},
			CreatedAt:   time.Now(),
		},
	}
	result := BuildImpactSummary(logs)
	assert.Contains(t, result, "Since your last turn")
	assert.Contains(t, result, "Goblin #2 hits Aria for 8 slashing damage")
}

func TestBuildImpactSummary_MultipleLogs(t *testing.T) {
	actorID := uuid.New()
	logs := []refdata.ActionLog{
		{
			ActionType:  "damage",
			ActorID:     uuid.NullUUID{UUID: actorID, Valid: true},
			Description: sql.NullString{String: "You took 8 dmg from Goblin #2", Valid: true},
			CreatedAt:   time.Now(),
		},
		{
			ActionType:  "condition_applied",
			ActorID:     uuid.NullUUID{UUID: actorID, Valid: true},
			Description: sql.NullString{String: "Orc Shaman cast Hold Person on you", Valid: true},
			CreatedAt:   time.Now().Add(time.Second),
		},
	}
	result := BuildImpactSummary(logs)
	assert.Contains(t, result, "Since your last turn")
	assert.Contains(t, result, "You took 8 dmg from Goblin #2")
	assert.Contains(t, result, "Orc Shaman cast Hold Person on you")
}

func TestBuildImpactSummary_EmptyDescription(t *testing.T) {
	logs := []refdata.ActionLog{
		{
			ActionType:  "damage",
			Description: sql.NullString{Valid: false},
			CreatedAt:   time.Now(),
		},
	}
	result := BuildImpactSummary(logs)
	assert.Equal(t, "", result)
}

func TestBuildImpactSummary_MixedValidInvalid(t *testing.T) {
	logs := []refdata.ActionLog{
		{
			ActionType:  "damage",
			Description: sql.NullString{Valid: false},
		},
		{
			ActionType:  "healing",
			Description: sql.NullString{String: "Cleric healed you for 10 HP", Valid: true},
		},
	}
	result := BuildImpactSummary(logs)
	assert.Contains(t, result, "Since your last turn")
	assert.Contains(t, result, "Cleric healed you for 10 HP")
}

func TestGetImpactSummary_NoLastTurn(t *testing.T) {
	ms := &mockStore{
		getLastCompletedTurnByCombatantFn: func(_ context.Context, _ refdata.GetLastCompletedTurnByCombatantParams) (refdata.Turn, error) {
			return refdata.Turn{}, fmt.Errorf("not found")
		},
	}
	svc := NewService(ms)
	result := svc.GetImpactSummary(context.Background(), uuid.New(), uuid.New())
	assert.Equal(t, "", result)
}

func TestGetImpactSummary_NoCompletedAt(t *testing.T) {
	ms := &mockStore{
		getLastCompletedTurnByCombatantFn: func(_ context.Context, _ refdata.GetLastCompletedTurnByCombatantParams) (refdata.Turn, error) {
			return refdata.Turn{CompletedAt: sql.NullTime{Valid: false}}, nil
		},
	}
	svc := NewService(ms)
	result := svc.GetImpactSummary(context.Background(), uuid.New(), uuid.New())
	assert.Equal(t, "", result)
}

func TestGetImpactSummary_WithLogs(t *testing.T) {
	completedAt := time.Now().Add(-time.Minute)
	ms := &mockStore{
		getLastCompletedTurnByCombatantFn: func(_ context.Context, _ refdata.GetLastCompletedTurnByCombatantParams) (refdata.Turn, error) {
			return refdata.Turn{CompletedAt: sql.NullTime{Time: completedAt, Valid: true}}, nil
		},
		listActionLogSinceTurnFn: func(_ context.Context, _ refdata.ListActionLogSinceTurnParams) ([]refdata.ActionLog, error) {
			return []refdata.ActionLog{
				{Description: sql.NullString{String: "Took 8 damage", Valid: true}},
			}, nil
		},
	}
	svc := NewService(ms)
	result := svc.GetImpactSummary(context.Background(), uuid.New(), uuid.New())
	assert.Contains(t, result, "Since your last turn")
	assert.Contains(t, result, "Took 8 damage")
}

func TestGetImpactSummary_LogQueryError(t *testing.T) {
	completedAt := time.Now().Add(-time.Minute)
	ms := &mockStore{
		getLastCompletedTurnByCombatantFn: func(_ context.Context, _ refdata.GetLastCompletedTurnByCombatantParams) (refdata.Turn, error) {
			return refdata.Turn{CompletedAt: sql.NullTime{Time: completedAt, Valid: true}}, nil
		},
		listActionLogSinceTurnFn: func(_ context.Context, _ refdata.ListActionLogSinceTurnParams) ([]refdata.ActionLog, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	svc := NewService(ms)
	result := svc.GetImpactSummary(context.Background(), uuid.New(), uuid.New())
	assert.Equal(t, "", result)
}

func TestFormatTurnStartPromptWithImpact_NoImpact(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    2,
	}
	result := FormatTurnStartPromptWithImpact("Rooftop Ambush", 3, "Aria", turn, nil, "")
	assert.Contains(t, result, "Rooftop Ambush")
	assert.Contains(t, result, "Round 3")
	assert.Contains(t, result, "@Aria")
	assert.NotContains(t, result, "Since your last turn")
}

func TestFormatTurnStartPromptWithImpact_AllSpent(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 0,
		AttacksRemaining:    0,
		ActionUsed:          true,
		BonusActionUsed:     true,
		ReactionUsed:        true,
		FreeInteractUsed:    true,
	}
	result := FormatTurnStartPromptWithImpact("Arena", 1, "Bob", turn, nil, "")
	assert.Contains(t, result, "All actions spent")
}

func TestFormatTurnStartPromptWithImpact_WithCombatant(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	}
	combatant := &refdata.Combatant{DisplayName: "Aria"}
	result := FormatTurnStartPromptWithImpact("Arena", 1, "Aria", turn, combatant, "")
	assert.Contains(t, result, "Available:")
}

func TestFormatTurnStartPromptWithImpact_WithImpact(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    2,
	}
	impact := "\u26a0\ufe0f Since your last turn: Orc Shaman cast Hold Person on you. You took 8 dmg from Goblin #2."
	result := FormatTurnStartPromptWithImpact("Rooftop Ambush", 3, "Aria", turn, nil, impact)
	assert.Contains(t, result, "Since your last turn")
	assert.Contains(t, result, "@Aria")
	assert.Contains(t, result, "Available:")
}
