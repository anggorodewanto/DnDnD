package combat

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/refdata"
)

// Notifier sends messages to Discord channels.
type Notifier interface {
	SendMessage(channelID, content string) error
}

// ConcentrationResolverFn fires after a pending CON save with
// `source = "concentration"` is resolved (success OR failure). The hook
// inspects the row and triggers the cleanup pipeline only on failure.
// Wiring is optional: tests for unrelated AutoResolve behavior set no
// resolver and the hook is skipped.
type ConcentrationResolverFn func(ctx context.Context, ps refdata.PendingSafe) error

// TurnTimer polls for turn timeouts and sends nudge/warning messages.
type TurnTimer struct {
	store                  Store
	notifier               Notifier
	interval               time.Duration
	stopCh                 chan struct{}
	stopOnce               sync.Once
	concentrationResolver  ConcentrationResolverFn
}

// NewTurnTimer creates a new TurnTimer.
func NewTurnTimer(store Store, notifier Notifier, interval time.Duration) *TurnTimer {
	return &TurnTimer{
		store:    store,
		notifier: notifier,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// SetConcentrationResolver wires the Phase 118 concentration save resolver.
// Production code passes Service.ResolveConcentrationSave; tests pass a
// stub. Calling with nil disables the hook.
func (t *TurnTimer) SetConcentrationResolver(fn ConcentrationResolverFn) {
	t.concentrationResolver = fn
}

// Start launches the polling goroutine.
func (t *TurnTimer) Start() {
	go t.run()
}

// Stop signals the polling goroutine to stop. Safe to call multiple times.
func (t *TurnTimer) Stop() {
	t.stopOnce.Do(func() { close(t.stopCh) })
}

func (t *TurnTimer) run() {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopCh:
			return
		case <-ticker.C:
			if err := t.PollOnce(context.Background()); err != nil {
				log.Printf("turn timer poll error: %v", err)
			}
		}
	}
}

// PollOnce performs one poll cycle: checks for nudges, warnings, timeouts, and DM auto-resolves.
func (t *TurnTimer) PollOnce(ctx context.Context) error {
	if err := t.processNudges(ctx); err != nil {
		return err
	}
	if err := t.processWarnings(ctx); err != nil {
		return err
	}
	if err := t.processTimeouts(ctx); err != nil {
		return err
	}
	return t.processDMAutoResolves(ctx)
}

func (t *TurnTimer) processNudges(ctx context.Context) error {
	turns, err := t.store.ListTurnsNeedingNudge(ctx)
	if err != nil {
		return err
	}

	for _, turn := range turns {
		if err := t.sendNudge(ctx, turn); err != nil {
			log.Printf("failed to send nudge for turn %s: %v", turn.ID, err)
			continue
		}
	}
	return nil
}

func (t *TurnTimer) processWarnings(ctx context.Context) error {
	turns, err := t.store.ListTurnsNeedingWarning(ctx)
	if err != nil {
		return err
	}

	for _, turn := range turns {
		if err := t.sendWarning(ctx, turn); err != nil {
			log.Printf("failed to send warning for turn %s: %v", turn.ID, err)
			continue
		}
	}
	return nil
}

func (t *TurnTimer) sendNudge(ctx context.Context, turn refdata.Turn) error {
	combatant, err := t.store.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		return err
	}

	channelID, err := t.getYourTurnChannel(ctx, turn.EncounterID)
	if err != nil {
		return err
	}

	if channelID != "" {
		remaining := clampDuration(time.Until(turn.TimeoutAt.Time))
		msg := FormatNudgeMessage(combatant.DisplayName, remaining)

		if err := t.notifier.SendMessage(channelID, msg); err != nil {
			return err
		}
	}

	_, err = t.store.UpdateTurnNudgeSent(ctx, turn.ID)
	return err
}

func (t *TurnTimer) sendWarning(ctx context.Context, turn refdata.Turn) error {
	combatant, err := t.store.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		return err
	}

	channelID, err := t.getYourTurnChannel(ctx, turn.EncounterID)
	if err != nil {
		return err
	}

	if channelID != "" {
		allCombatants, err := t.store.ListCombatantsByEncounterID(ctx, turn.EncounterID)
		if err != nil {
			return err
		}

		adjacentEnemies := findAdjacentEnemies(combatant, allCombatants)
		remaining := clampDuration(time.Until(turn.TimeoutAt.Time))
		msg := FormatTacticalSummary(combatant, turn, adjacentEnemies, remaining)

		if err := t.notifier.SendMessage(channelID, msg); err != nil {
			return err
		}
	}

	_, err = t.store.UpdateTurnWarningSent(ctx, turn.ID)
	return err
}

// clampDuration returns d if positive, otherwise 0.
func clampDuration(d time.Duration) time.Duration {
	if d < 0 {
		return 0
	}
	return d
}

func (t *TurnTimer) getYourTurnChannel(ctx context.Context, encounterID uuid.UUID) (string, error) {
	return t.getChannel(ctx, encounterID, "your-turn")
}

// getChannel returns a named channel ID from the campaign settings for the given encounter.
func (t *TurnTimer) getChannel(ctx context.Context, encounterID uuid.UUID, channelName string) (string, error) {
	camp, err := t.store.GetCampaignByEncounterID(ctx, encounterID)
	if err != nil {
		return "", err
	}

	if !camp.Settings.Valid {
		return "", nil
	}

	var settings campaign.Settings
	if err := json.Unmarshal(camp.Settings.RawMessage, &settings); err != nil {
		return "", err
	}

	return settings.ChannelIDs[channelName], nil
}

// findAdjacentEnemies returns enemy combatants within 1 tile (adjacent) of the target.
// Adjacent means Chebyshev distance <= 1 on the grid.
func findAdjacentEnemies(target refdata.Combatant, allCombatants []refdata.Combatant) []refdata.Combatant {
	targetCol := colToIndex(target.PositionCol)
	targetRow := int(target.PositionRow)

	var adjacent []refdata.Combatant
	for _, c := range allCombatants {
		if c.ID == target.ID || !c.IsAlive || c.IsNpc == target.IsNpc {
			continue
		}

		dCol := abs(colToIndex(c.PositionCol) - targetCol)
		dRow := abs(int(c.PositionRow) - targetRow)

		if dCol <= 1 && dRow <= 1 {
			adjacent = append(adjacent, c)
		}
	}
	return adjacent
}

