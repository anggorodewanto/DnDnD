package combat

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// GetImpactSummary retrieves action log entries that affected this combatant
// since their last completed turn and formats them as a summary line.
// Returns empty string if no events affected them or if they have no prior turn.
func (s *Service) GetImpactSummary(ctx context.Context, encounterID uuid.UUID, combatantID uuid.UUID) string {
	lastTurn, err := s.store.GetLastCompletedTurnByCombatant(ctx, refdata.GetLastCompletedTurnByCombatantParams{
		EncounterID: encounterID,
		CombatantID: combatantID,
	})
	if err != nil {
		// No prior turn — first turn of combat, no impact to show
		return ""
	}

	if !lastTurn.CompletedAt.Valid {
		return ""
	}

	logs, err := s.store.ListActionLogSinceTurn(ctx, refdata.ListActionLogSinceTurnParams{
		EncounterID: uuid.NullUUID{UUID: encounterID, Valid: true},
		TargetID:    uuid.NullUUID{UUID: combatantID, Valid: true},
		CreatedAt:   lastTurn.CompletedAt.Time,
	})
	if err != nil {
		return ""
	}

	return BuildImpactSummary(logs)
}

// FormatTurnStartPromptWithImpact produces the turn start notification
// with an optional impact summary line inserted between the ping and resources.
func FormatTurnStartPromptWithImpact(encounterName string, roundNumber int32, combatantName string, turn refdata.Turn, combatant *refdata.Combatant, impactSummary string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\u2694\ufe0f %s \u2014 Round %d\n", encounterName, roundNumber)
	fmt.Fprintf(&b, "\U0001f514 @%s \u2014 it's your turn!\n", combatantName)

	if impactSummary != "" {
		fmt.Fprintf(&b, "%s\n", impactSummary)
	}

	var parts []string
	if combatant != nil {
		parts = BuildResourceListWithInspiration(turn, *combatant)
	} else {
		parts = buildResourceList(turn)
	}
	if len(parts) > 0 {
		fmt.Fprintf(&b, "\U0001f4cb Available: %s", strings.Join(parts, " | "))
	} else {
		b.WriteString("\U0001f4cb All actions spent \u2014 type /done to end your turn.")
	}
	return b.String()
}

// BuildImpactSummary formats action log entries that affected a combatant
// since their last turn into a single summary line.
// Returns empty string if no relevant events occurred.
func BuildImpactSummary(logs []refdata.ActionLog) string {
	if len(logs) == 0 {
		return ""
	}

	var descriptions []string
	for _, log := range logs {
		if !log.Description.Valid || log.Description.String == "" {
			continue
		}
		descriptions = append(descriptions, log.Description.String)
	}

	if len(descriptions) == 0 {
		return ""
	}

	return fmt.Sprintf("\u26a0\ufe0f Since your last turn: %s", strings.Join(descriptions, ". ")+".")
}
