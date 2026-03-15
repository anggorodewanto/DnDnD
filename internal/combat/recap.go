package combat

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// ListActionLogWithRounds retrieves all action log entries for an encounter with round info.
func (s *Service) ListActionLogWithRounds(ctx context.Context, encounterID uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
	return s.store.ListActionLogWithRounds(ctx, encounterID)
}

// GetMostRecentCompletedEncounter finds the most recently completed encounter for a campaign.
func (s *Service) GetMostRecentCompletedEncounter(ctx context.Context, campaignID uuid.UUID) (refdata.Encounter, error) {
	return s.store.GetMostRecentCompletedEncounter(ctx, campaignID)
}

// GetLastCompletedTurnByCombatant finds the last completed/skipped turn for a combatant.
func (s *Service) GetLastCompletedTurnByCombatant(ctx context.Context, encounterID, combatantID uuid.UUID) (refdata.Turn, error) {
	return s.store.GetLastCompletedTurnByCombatant(ctx, refdata.GetLastCompletedTurnByCombatantParams{
		EncounterID: encounterID,
		CombatantID: combatantID,
	})
}

// GetCampaignByEncounterID looks up the campaign associated with an encounter.
func (s *Service) GetCampaignByEncounterID(ctx context.Context, encounterID uuid.UUID) (refdata.Campaign, error) {
	return s.store.GetCampaignByEncounterID(ctx, encounterID)
}

// FilterLogsSinceRound returns only logs from the given round number onwards.
func FilterLogsSinceRound(logs []refdata.ListActionLogWithRoundsRow, sinceRound int32) []refdata.ListActionLogWithRoundsRow {
	var result []refdata.ListActionLogWithRoundsRow
	for _, l := range logs {
		if l.RoundNumber >= sinceRound {
			result = append(result, l)
		}
	}
	return result
}

// FilterLogsLastNRounds returns logs from the last N distinct rounds.
func FilterLogsLastNRounds(logs []refdata.ListActionLogWithRoundsRow, n int) []refdata.ListActionLogWithRoundsRow {
	if len(logs) == 0 || n <= 0 {
		return nil
	}

	// Collect distinct rounds in order
	var rounds []int32
	seen := make(map[int32]bool)
	for _, l := range logs {
		if !seen[l.RoundNumber] {
			seen[l.RoundNumber] = true
			rounds = append(rounds, l.RoundNumber)
		}
	}

	if n >= len(rounds) {
		return logs
	}

	cutoff := rounds[len(rounds)-n]
	return FilterLogsSinceRound(logs, cutoff)
}

// TruncateRecap truncates a recap message to fit within the given maxLen (e.g., Discord's 2000 char limit).
func TruncateRecap(msg string, maxLen int) string {
	if len(msg) <= maxLen {
		return msg
	}
	suffix := "\n... (truncated)"
	return msg[:maxLen-len(suffix)] + suffix
}

// RecapRoundRange returns a human-readable round range string like "Rounds 3–5" or "Round 2".
func RecapRoundRange(logs []refdata.ListActionLogWithRoundsRow) string {
	if len(logs) == 0 {
		return ""
	}
	first := logs[0].RoundNumber
	last := logs[len(logs)-1].RoundNumber
	if first == last {
		return fmt.Sprintf("Round %d", first)
	}
	return fmt.Sprintf("Rounds %d\u2013%d", first, last)
}

// FormatRecap formats action log entries grouped by round into a readable recap string.
// The subtitle describes the scope (e.g., "Rounds 1–3" or "since your last turn").
func FormatRecap(logs []refdata.ListActionLogWithRoundsRow, subtitle string) string {
	if len(logs) == 0 {
		return "No combat activity to recap."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\U0001f4dc Recap \u2014 %s\n", subtitle))

	currentRound := int32(-1)
	for _, log := range logs {
		if log.RoundNumber != currentRound {
			currentRound = log.RoundNumber
			sb.WriteString(fmt.Sprintf("\n\u2500\u2500 Round %d \u2500\u2500\n", currentRound))
		}
		if !log.Description.Valid || log.Description.String == "" {
			continue
		}
		sb.WriteString(log.Description.String + "\n")
	}

	return sb.String()
}
