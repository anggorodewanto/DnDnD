package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// ErrWaitAlreadyUsed is returned when Wait has already been used for this timeout cycle.
var ErrWaitAlreadyUsed = fmt.Errorf("wait has already been used for this timeout")

// FormatDMDecisionPrompt produces the DM decision prompt message when a turn times out.
func FormatDMDecisionPrompt(combatant refdata.Combatant, turn refdata.Turn) string {
	var b strings.Builder

	fmt.Fprintf(&b, "\u26a1 @DM \u2014 %s's turn has timed out!\n", combatant.DisplayName)

	var pending []string
	if turn.MovementRemainingFt > 0 {
		pending = append(pending, fmt.Sprintf("\U0001f3c3 %dft move", turn.MovementRemainingFt))
	}
	if turn.AttacksRemaining > 0 {
		pending = append(pending, fmt.Sprintf("\u2694\ufe0f %d attacks", turn.AttacksRemaining))
	}
	if !turn.BonusActionUsed {
		pending = append(pending, "\U0001f381 Bonus action")
	}
	if len(pending) > 0 {
		fmt.Fprintf(&b, "\U0001f4cb Pending: %s\n", strings.Join(pending, " | "))
	}

	b.WriteString("[\u23f3 Wait] [\U0001f3ae Roll for Player] [\u26a1 Auto-Resolve]")
	return b.String()
}

// FormatAutoResolveResult produces a combat log message for an auto-resolved turn.
func FormatAutoResolveResult(combatant refdata.Combatant, actions []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\u26a1 %s's turn was auto-resolved (player timed out):\n", combatant.DisplayName)
	for _, action := range actions {
		fmt.Fprintf(&b, "  \u2022 %s\n", action)
	}
	b.WriteString("(auto-resolved \u2014 player timed out)")
	return b.String()
}

// processTimeouts checks for timed-out turns and sends DM decision prompts.
func (t *TurnTimer) processTimeouts(ctx context.Context) error {
	turns, err := t.store.ListTurnsTimedOut(ctx)
	if err != nil {
		return err
	}

	for _, turn := range turns {
		if err := t.sendDMDecisionPrompt(ctx, turn); err != nil {
			log.Printf("failed to send DM decision prompt for turn %s: %v", turn.ID, err)
			continue
		}
	}
	return nil
}

func (t *TurnTimer) sendDMDecisionPrompt(ctx context.Context, turn refdata.Turn) error {
	combatant, err := t.store.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		return err
	}

	channelID, err := t.getYourTurnChannel(ctx, turn.EncounterID)
	if err != nil {
		return err
	}

	if channelID != "" {
		msg := FormatDMDecisionPrompt(combatant, turn)
		if err := t.notifier.SendMessage(channelID, msg); err != nil {
			return err
		}
	}

	_, err = t.store.UpdateTurnDMDecisionSent(ctx, turn.ID)
	return err
}

// processDMAutoResolves checks for turns where DM deadline has passed and auto-resolves them.
func (t *TurnTimer) processDMAutoResolves(ctx context.Context) error {
	turns, err := t.store.ListTurnsNeedingDMAutoResolve(ctx)
	if err != nil {
		return err
	}

	roller := dice.NewRoller(nil)
	for _, turn := range turns {
		if _, err := t.AutoResolveTurn(ctx, turn.ID, roller); err != nil {
			log.Printf("failed to auto-resolve turn %s: %v", turn.ID, err)
			continue
		}
	}
	return nil
}

// AutoResolveTurn auto-resolves a turn: Dodge action, no movement, auto-roll saves/death saves,
// forfeit reactions, and track prolonged absence.
func (t *TurnTimer) AutoResolveTurn(ctx context.Context, turnID uuid.UUID, roller *dice.Roller) ([]string, error) {
	turn, err := t.store.GetTurn(ctx, turnID)
	if err != nil {
		return nil, fmt.Errorf("getting turn: %w", err)
	}

	combatant, err := t.store.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		return nil, fmt.Errorf("getting combatant: %w", err)
	}

	var actions []string

	// Check if dying — auto-roll death save
	if IsDying(combatant.IsAlive, int(combatant.HpCurrent), mustParseDeathSaves(combatant.DeathSaves)) {
		rollResult, err := roller.Roll("1d20")
		if err != nil {
			return nil, fmt.Errorf("rolling death save: %w", err)
		}
		roll := rollResult.Total
		ds := mustParseDeathSaves(combatant.DeathSaves)
		outcome := RollDeathSave(combatant.DisplayName, ds, roll)
		actions = append(actions, outcome.Messages...)

		// Update combatant with new death saves state
		if outcome.HPCurrent > 0 {
			// Nat 20 — regain 1 HP
			if _, err := t.store.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
				ID:        combatant.ID,
				HpCurrent: int32(outcome.HPCurrent),
				TempHp:    combatant.TempHp,
				IsAlive:   outcome.IsAlive,
			}); err != nil {
				return nil, fmt.Errorf("updating combatant HP: %w", err)
			}
		} else {
			newDS := MarshalDeathSaves(outcome.DeathSaves)
			if _, err := t.store.UpdateCombatantDeathSaves(ctx, refdata.UpdateCombatantDeathSavesParams{
				ID:         combatant.ID,
				DeathSaves: newDS,
			}); err != nil {
				return nil, fmt.Errorf("updating death saves: %w", err)
			}
			if !outcome.IsAlive {
				// Dead
				if _, err := t.store.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
					ID:        combatant.ID,
					HpCurrent: 0,
					TempHp:    0,
					IsAlive:   false,
				}); err != nil {
					return nil, fmt.Errorf("updating combatant alive status: %w", err)
				}
			}
		}
	} else if combatant.IsAlive && combatant.HpCurrent > 0 {
		// Normal turn: apply Dodge action (no movement)
		if !turn.ActionUsed {
			newConditions, err := AddCondition(combatant.Conditions, CombatCondition{
				Condition:      "dodge",
				DurationRounds: 1,
				StartedRound:   int(turn.RoundNumber),
				ExpiresOn:      "start_of_next_turn",
			})
			if err != nil {
				return nil, fmt.Errorf("adding dodge condition: %w", err)
			}
			if _, err := t.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
				ID:              combatant.ID,
				Conditions:      newConditions,
				ExhaustionLevel: combatant.ExhaustionLevel,
			}); err != nil {
				return nil, fmt.Errorf("updating conditions: %w", err)
			}
			actions = append(actions, fmt.Sprintf("%s takes the Dodge action", combatant.DisplayName))
		}
		actions = append(actions, "No movement")
	}

	// Forfeit all active reaction declarations
	if err := t.store.CancelAllReactionDeclarationsByCombatant(ctx, refdata.CancelAllReactionDeclarationsByCombatantParams{
		CombatantID: combatant.ID,
		EncounterID: turn.EncounterID,
	}); err != nil {
		return nil, fmt.Errorf("cancelling reaction declarations: %w", err)
	}
	actions = append(actions, "Reaction declarations forfeited")

	// Mark turn as auto-resolved
	if _, err := t.store.UpdateTurnAutoResolved(ctx, turnID); err != nil {
		return nil, fmt.Errorf("marking turn auto-resolved: %w", err)
	}

	// Track prolonged absence
	newCount := combatant.ConsecutiveAutoResolves + 1
	isAbsent := newCount >= 3
	if _, err := t.store.UpdateCombatantAutoResolveCount(ctx, refdata.UpdateCombatantAutoResolveCountParams{
		ID:                      combatant.ID,
		ConsecutiveAutoResolves: newCount,
		IsAbsent:                isAbsent,
	}); err != nil {
		return nil, fmt.Errorf("updating auto-resolve count: %w", err)
	}

	if isAbsent {
		actions = append(actions, fmt.Sprintf("%s flagged as absent (3 consecutive auto-resolves)", combatant.DisplayName))
	}

	// Post to combat log
	channelID, err := t.getCombatLogChannel(ctx, turn.EncounterID)
	if err == nil && channelID != "" {
		msg := FormatAutoResolveResult(combatant, actions)
		if err := t.notifier.SendMessage(channelID, msg); err != nil {
			log.Printf("failed to send auto-resolve result to combat log: %v", err)
		}
	}

	return actions, nil
}

// WaitExtendTurn extends the turn timer by 50% (Wait option from DM decision prompt).
func (t *TurnTimer) WaitExtendTurn(ctx context.Context, turnID uuid.UUID) error {
	turn, err := t.store.GetTurn(ctx, turnID)
	if err != nil {
		return fmt.Errorf("getting turn: %w", err)
	}

	if turn.WaitExtended {
		return ErrWaitAlreadyUsed
	}

	if !turn.StartedAt.Valid || !turn.TimeoutAt.Valid {
		return fmt.Errorf("turn has no timeout set")
	}

	// Calculate 50% of original timeout duration
	originalDuration := turn.TimeoutAt.Time.Sub(turn.StartedAt.Time)
	extension := originalDuration / 2
	newTimeout := turn.TimeoutAt.Time.Add(extension)

	// Extend timeout
	if _, err := t.store.UpdateTurnTimeout(ctx, refdata.UpdateTurnTimeoutParams{
		ID:        turnID,
		TimeoutAt: sql.NullTime{Time: newTimeout, Valid: true},
	}); err != nil {
		return fmt.Errorf("extending timeout: %w", err)
	}

	// Mark wait as used
	if _, err := t.store.UpdateTurnWaitExtended(ctx, turnID); err != nil {
		return fmt.Errorf("marking wait extended: %w", err)
	}

	// Reset nudge/warning so escalation restarts
	if _, err := t.store.ResetTurnNudgeAndWarning(ctx, turnID); err != nil {
		return fmt.Errorf("resetting nudge/warning: %w", err)
	}

	return nil
}

// ScanStaleTurns processes turns that should have been escalated but weren't (bot was down).
func (t *TurnTimer) ScanStaleTurns(ctx context.Context) error {
	// Process timed-out turns that never got a DM decision prompt
	timedOut, err := t.store.ListTurnsTimedOut(ctx)
	if err != nil {
		return fmt.Errorf("listing stale timed-out turns: %w", err)
	}
	for _, turn := range timedOut {
		if err := t.sendDMDecisionPrompt(ctx, turn); err != nil {
			log.Printf("stale scan: failed to send DM decision prompt for turn %s: %v", turn.ID, err)
			continue
		}
	}

	// Process turns where DM deadline has passed
	dmExpired, err := t.store.ListTurnsNeedingDMAutoResolve(ctx)
	if err != nil {
		return fmt.Errorf("listing stale DM auto-resolve turns: %w", err)
	}
	roller := dice.NewRoller(nil)
	for _, turn := range dmExpired {
		if _, err := t.AutoResolveTurn(ctx, turn.ID, roller); err != nil {
			log.Printf("stale scan: failed to auto-resolve turn %s: %v", turn.ID, err)
			continue
		}
	}

	return nil
}

// getCombatLogChannel returns the combat-log channel ID for the encounter's campaign.
func (t *TurnTimer) getCombatLogChannel(ctx context.Context, encounterID uuid.UUID) (string, error) {
	camp, err := t.store.GetCampaignByEncounterID(ctx, encounterID)
	if err != nil {
		return "", err
	}

	if !camp.Settings.Valid {
		return "", nil
	}

	var settings struct {
		ChannelIDs map[string]string `json:"channel_ids,omitempty"`
	}
	if err := json.Unmarshal(camp.Settings.RawMessage, &settings); err != nil {
		return "", err
	}

	return settings.ChannelIDs["combat-log"], nil
}

// mustParseDeathSaves parses death saves, returning zero value on error.
func mustParseDeathSaves(raw pqtype.NullRawMessage) DeathSaves {
	if !raw.Valid || len(raw.RawMessage) == 0 {
		return DeathSaves{}
	}
	ds, _ := ParseDeathSaves(raw.RawMessage)
	return ds
}

