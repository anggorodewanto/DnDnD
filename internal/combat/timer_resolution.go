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

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// ErrWaitAlreadyUsed is returned when Wait has already been used for this timeout cycle.
var ErrWaitAlreadyUsed = fmt.Errorf("wait has already been used for this timeout")

// FormatDMDecisionPrompt produces the DM decision prompt message when a turn times out.
func FormatDMDecisionPrompt(combatant refdata.Combatant, turn refdata.Turn) string {
	return FormatDMDecisionPromptWithSaves(combatant, turn, nil)
}

// FormatDMDecisionPromptWithSaves produces the DM decision prompt including pending saves.
func FormatDMDecisionPromptWithSaves(combatant refdata.Combatant, turn refdata.Turn, pendingSaves []refdata.PendingSafe) string {
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

	if len(pendingSaves) > 0 {
		var saveDescs []string
		for _, s := range pendingSaves {
			desc := fmt.Sprintf("%s save DC %d", s.Ability, s.Dc)
			if s.Source != "" {
				desc += fmt.Sprintf(" (%s)", s.Source)
			}
			saveDescs = append(saveDescs, desc)
		}
		fmt.Fprintf(&b, "\U0001f3b2 Pending saves: %s\n", strings.Join(saveDescs, ", "))
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
		pendingSaves, _ := t.store.ListPendingSavesByCombatant(ctx, combatant.ID)
		msg := FormatDMDecisionPromptWithSaves(combatant, turn, pendingSaves)
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
	ds := mustParseDeathSaves(combatant.DeathSaves)
	if IsDying(combatant.IsAlive, int(combatant.HpCurrent), ds) {
		rollResult, err := roller.Roll("1d20")
		if err != nil {
			return nil, fmt.Errorf("rolling death save: %w", err)
		}
		roll := rollResult.Total
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
			// C-43-followup: mirror MaybeResetDeathSavesOnHeal — clear death
			// save tallies and remove the dying-condition bundle
			// (unconscious + prone) so the PC fully wakes up.
			if err := t.resetDyingStateAfterNat20(ctx, combatant); err != nil {
				return nil, fmt.Errorf("resetting dying state after Nat-20: %w", err)
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
		// Normal turn: apply Dodge action (no movement).
		// SR-020: ExpiresOn must be "start_of_turn" and SourceCombatantID must
		// be the dodging creature so isExpired matches at the start of their
		// next turn. Mirrors hand-rolled /action dodge (standard_actions.go).
		if !turn.ActionUsed {
			newConditions, err := AddCondition(combatant.Conditions, CombatCondition{
				Condition:         "dodge",
				DurationRounds:    1,
				StartedRound:      int(turn.RoundNumber),
				SourceCombatantID: combatant.ID.String(),
				ExpiresOn:         "start_of_turn",
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

	// Auto-roll pending saves
	pendingSaves, err := t.store.ListPendingSavesByCombatant(ctx, combatant.ID)
	if err != nil {
		return nil, fmt.Errorf("listing pending saves: %w", err)
	}
	// F-21: load character data once so timeout saves include ability mod + proficiency.
	var charScores character.AbilityScores
	var charProfSaves []string
	var charProfBonus int
	var hasCharData bool
	if combatant.CharacterID.Valid {
		if char, err := t.store.GetCharacter(ctx, combatant.CharacterID.UUID); err == nil {
			if json.Unmarshal(char.AbilityScores, &charScores) == nil {
				hasCharData = true
				var profData struct {
					Saves []string `json:"saves"`
				}
				if char.Proficiencies.Valid {
					_ = json.Unmarshal(char.Proficiencies.RawMessage, &profData)
				}
				charProfSaves = profData.Saves
				charProfBonus = int(char.ProficiencyBonus)
			}
		}
	}
	for _, ps := range pendingSaves {
		rollResult, err := roller.Roll("1d20")
		if err != nil {
			return nil, fmt.Errorf("rolling pending save: %w", err)
		}
		roll := rollResult.Total
		// F-21: apply save modifier (ability mod + proficiency if proficient)
		total := roll
		if hasCharData {
			total += character.SavingThrowModifier(charScores, strings.ToLower(ps.Ability), charProfSaves, charProfBonus)
		}
		success := total >= int(ps.Dc)
		resolved, err := t.store.UpdatePendingSaveResult(ctx, refdata.UpdatePendingSaveResultParams{
			ID:         ps.ID,
			RollResult: sql.NullInt32{Int32: int32(total), Valid: true},
			Success:    sql.NullBool{Bool: success, Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("updating pending save result: %w", err)
		}
		// Phase 118: hand resolved concentration save rows to the registered
		// resolver so the cleanup pipeline runs on failures.
		if t.concentrationResolver != nil && resolved.Source == ConcentrationSaveSource {
			if rerr := t.concentrationResolver(ctx, resolved); rerr != nil {
				return nil, fmt.Errorf("resolving concentration save: %w", rerr)
			}
		}
		outcome := "FAIL"
		if success {
			outcome = "PASS"
		}
		source := ps.Source
		if source == "" {
			source = "unknown"
		}
		actions = append(actions, fmt.Sprintf("%s save vs DC %d (%s): rolled %d — %s",
			ps.Ability, ps.Dc, source, total, outcome))
	}

	// Explicitly decline on-hit decisions and Bardic Inspiration (best-effort).
	if combatant.IsAlive && combatant.HpCurrent > 0 {
		// Divine Smite — paladin with feature + available slots.
		if combatant.CharacterID.Valid {
			if char, err := t.store.GetCharacter(ctx, combatant.CharacterID.UUID); err == nil {
				if HasFeatureByName(char.Features.RawMessage, "Divine Smite") {
					if slots, err := ParseSpellSlots(char.SpellSlots.RawMessage); err == nil {
						if len(AvailableSmiteSlots(slots)) > 0 {
							actions = append(actions, "⚡ Divine Smite declined (auto-resolved)")
						}
					}
				}
			}
		}
		// Bardic Inspiration — combatant holds an active die.
		if CombatantHasBardicInspiration(combatant) {
			actions = append(actions, "🎵 Bardic Inspiration declined (auto-resolved)")
		}
	}

	// Forfeit pending actions
	if err := t.store.CancelAllPendingActionsByCombatant(ctx, refdata.CancelAllPendingActionsByCombatantParams{
		CombatantID: combatant.ID,
		EncounterID: turn.EncounterID,
	}); err != nil {
		return nil, fmt.Errorf("cancelling pending actions: %w", err)
	}
	actions = append(actions, "Pending actions forfeited")

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
// It reuses the same processTimeouts and processDMAutoResolves logic used in normal polling.
func (t *TurnTimer) ScanStaleTurns(ctx context.Context) error {
	if err := t.processTimeouts(ctx); err != nil {
		return fmt.Errorf("scanning stale timed-out turns: %w", err)
	}
	return t.processDMAutoResolves(ctx)
}

// getCombatLogChannel returns the combat-log channel ID for the encounter's campaign.
func (t *TurnTimer) getCombatLogChannel(ctx context.Context, encounterID uuid.UUID) (string, error) {
	return t.getChannel(ctx, encounterID, "combat-log")
}

// mustParseDeathSaves parses death saves, returning zero value on error.
func mustParseDeathSaves(raw pqtype.NullRawMessage) DeathSaves {
	if !raw.Valid || len(raw.RawMessage) == 0 {
		return DeathSaves{}
	}
	ds, _ := ParseDeathSaves(raw.RawMessage)
	return ds
}

// resetDyingStateAfterNat20 clears death-save tallies and removes the
// dying-condition bundle (unconscious + prone) after the timer auto-resolves a
// natural-20 death save back to 1 HP. Mirrors Service.resetDyingState which is
// invoked from MaybeResetDeathSavesOnHeal for /heal and Lay on Hands so the
// timer path no longer leaves stale failure tallies or sleeping conditions on
// a revived PC. (C-43-followup)
func (t *TurnTimer) resetDyingStateAfterNat20(ctx context.Context, combatant refdata.Combatant) error {
	if _, err := t.store.UpdateCombatantDeathSaves(ctx, refdata.UpdateCombatantDeathSavesParams{
		ID:         combatant.ID,
		DeathSaves: MarshalDeathSaves(DeathSaves{}),
	}); err != nil {
		return fmt.Errorf("resetting death saves: %w", err)
	}
	conds := combatant.Conditions
	for _, cond := range ConditionsForDying() {
		// Spec: "Nat 20 → regain 1 HP, conscious, still prone."
		if cond.Condition == "prone" {
			continue
		}
		if !HasCondition(conds, cond.Condition) {
			continue
		}
		next, err := RemoveCondition(conds, cond.Condition)
		if err != nil {
			return fmt.Errorf("removing %s on Nat-20 heal: %w", cond.Condition, err)
		}
		conds = next
	}
	if _, err := t.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
		ID:              combatant.ID,
		Conditions:      conds,
		ExhaustionLevel: combatant.ExhaustionLevel,
	}); err != nil {
		return fmt.Errorf("updating conditions after Nat-20: %w", err)
	}
	return nil
}

