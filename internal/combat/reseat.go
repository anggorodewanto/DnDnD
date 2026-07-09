package combat

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// APP-2 re-seat sentinels. Handlers map these to 409 (state conflict); a target
// lookup failure maps to 404 separately.
// ErrNoActiveTurn (encounter has no active turn to move) is declared in
// turnvalidation.go and reused here.
var (
	// ErrAlreadyActiveTurn is returned when the target is already the active combatant.
	ErrAlreadyActiveTurn = errors.New("target is already the active combatant")
	// ErrTurnAlreadyActed is returned when the current turn has spent an action,
	// so re-seating away from it would discard real progress.
	ErrTurnAlreadyActed = errors.New("current turn has already acted")
	// ErrCombatantAlreadyActed is returned when the target already took a turn
	// this round; re-seating it would let it act twice.
	ErrCombatantAlreadyActed = errors.New("target has already taken a turn this round")
	// ErrCombatantNotAlive is returned when the target is not alive.
	ErrCombatantNotAlive = errors.New("target combatant is not alive")
	// ErrCombatantNotInEncounter is returned when the target belongs to a
	// different encounter.
	ErrCombatantNotInEncounter = errors.New("target combatant is not in this encounter")
	// ErrCombatantSharesCasterTurn is returned when the target is a summoned
	// creature that acts on its summoner's turn and so has no seat of its own.
	ErrCombatantSharesCasterTurn = errors.New("target combatant shares its summoner's turn")
)

// turnHasCommittedAction reports whether a turn has spent any committed action
// (as opposed to only movement, which is safely discarded on a re-seat because
// the displaced combatant takes a fresh turn later).
func turnHasCommittedAction(t refdata.Turn) bool {
	return t.ActionUsed || t.BonusActionUsed || t.ActionSpellCast || t.BonusActionSpellCast ||
		t.ReactionUsed || t.HasDisengaged || t.ActionSurged || t.HasStoodThisTurn
}

// ReseatActiveTurn moves the active turn to targetCombatantID (APP-2) — the
// in-app replacement for the manual DB seat-repair. It reassigns the current,
// un-acted active turn row to the target and fully resets that turn's resources
// (movement, attacks, action/bonus/reaction flags, timer), then runs the
// target's start-of-turn processing and turn-start notifier. Because the
// existing row is reassigned in place, the encounter's current-turn pointer
// already references it — no pointer write, no double active row. The
// previously-seated combatant loses its turn row and is picked again at its true
// initiative order later this round.
//
// It is intended for correcting a wrongly-seated first actor BEFORE anyone has
// acted; the guards below reject any state where a re-seat would lose progress
// or let a combatant act twice.
func (s *Service) ReseatActiveTurn(ctx context.Context, encounterID, targetCombatantID uuid.UUID) (TurnInfo, error) {
	enc, err := s.store.GetEncounter(ctx, encounterID)
	if err != nil {
		return TurnInfo{}, fmt.Errorf("getting encounter: %w", err)
	}
	if !enc.CurrentTurnID.Valid {
		return TurnInfo{}, ErrNoActiveTurn
	}

	current, err := s.store.GetTurn(ctx, enc.CurrentTurnID.UUID)
	if err != nil {
		return TurnInfo{}, fmt.Errorf("getting active turn: %w", err)
	}
	if current.CombatantID == targetCombatantID {
		return TurnInfo{}, ErrAlreadyActiveTurn
	}
	if turnHasCommittedAction(current) {
		return TurnInfo{}, ErrTurnAlreadyActed
	}

	target, err := s.store.GetCombatant(ctx, targetCombatantID)
	if err != nil {
		return TurnInfo{}, fmt.Errorf("getting target combatant: %w", err)
	}
	if target.EncounterID != encounterID {
		return TurnInfo{}, ErrCombatantNotInEncounter
	}
	if !target.IsAlive {
		return TurnInfo{}, ErrCombatantNotAlive
	}
	// Summoned creatures act on their summoner's turn (excluded from the
	// initiative order everywhere else) and so cannot be seated on their own.
	if SharesCasterTurn(target) {
		return TurnInfo{}, ErrCombatantSharesCasterTurn
	}

	// The target's only permitted turn row this round is the active one we are
	// about to reassign; any other row means it has already acted.
	turns, err := s.store.ListTurnsByEncounterAndRound(ctx, refdata.ListTurnsByEncounterAndRoundParams{
		EncounterID: encounterID,
		RoundNumber: current.RoundNumber,
	})
	if err != nil {
		return TurnInfo{}, fmt.Errorf("listing turns: %w", err)
	}
	for _, t := range turns {
		if t.CombatantID == targetCombatantID && t.ID != current.ID {
			return TurnInfo{}, ErrCombatantAlreadyActed
		}
	}

	speedFt, attacksRemaining, err := s.ResolveTurnResources(ctx, target)
	if err != nil {
		return TurnInfo{}, fmt.Errorf("resolving turn resources: %w", err)
	}
	params := refdata.ReseatTurnParams{
		ID:                  current.ID,
		CombatantID:         target.ID,
		MovementRemainingFt: speedFt,
		AttacksRemaining:    attacksRemaining,
	}
	// Timer applies to PC turns only, matching createActiveTurn.
	if !target.IsNpc {
		params.StartedAt, params.TimeoutAt = s.resolveTimerForTurn(ctx, encounterID)
	}

	// Cancel any enemy_turn_ready #dm-queue stub left by the displaced NPC
	// (ISSUE-057), mirroring createActiveTurn's opening.
	s.resolveEnemyTurnReady(ctx, encounterID)

	turn, err := s.store.ReseatTurn(ctx, params)
	if err != nil {
		return TurnInfo{}, fmt.Errorf("reseating turn: %w", err)
	}

	// Start-of-turn effects for the newly-seated combatant, mirroring the core
	// of createActiveTurn. Zone triggers, summon resets and readied-action
	// expiry are intentionally omitted: this corrects a seat before anyone has
	// acted (round start), where they are nil or already ran for the displaced
	// actor. Death-save / save-ends turn prompts (skipOrActivate) are likewise
	// out of scope — seat-repair targets a healthy first actor, not a downed one.
	s.clearUsedEffectsForCombatant(encounterID, target.ID)
	if _, err := s.ProcessTurnStartWithLog(ctx, encounterID, target, turn.RoundNumber, turn.ID); err != nil {
		return TurnInfo{}, fmt.Errorf("processing turn start conditions: %w", err)
	}
	s.refreshInitiativeTracker(ctx, encounterID, target.ID)
	s.notifyTurnStart(ctx, encounterID, target, turn, turn.RoundNumber)
	// If the target is an NPC, post its "Run Enemy Turn" prompt.
	s.postEnemyTurnReady(ctx, encounterID, target)

	return TurnInfo{Turn: turn, CombatantID: target.ID, RoundNumber: turn.RoundNumber}, nil
}
