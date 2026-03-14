package combat

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/refdata"
)

// Notifier sends messages to Discord channels.
type Notifier interface {
	SendMessage(channelID, content string) error
}

// TurnTimer polls for turn timeouts and sends nudge/warning messages.
type TurnTimer struct {
	store    Store
	notifier Notifier
	interval time.Duration
	stopCh   chan struct{}
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

// Start launches the polling goroutine.
func (t *TurnTimer) Start() {
	go t.run()
}

// Stop signals the polling goroutine to stop.
func (t *TurnTimer) Stop() {
	close(t.stopCh)
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

// PollOnce performs one poll cycle: checks for turns needing nudge or warning.
func (t *TurnTimer) PollOnce(ctx context.Context) error {
	if err := t.processNudges(ctx); err != nil {
		return err
	}
	return t.processWarnings(ctx)
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
		timeRemaining := time.Until(turn.TimeoutAt.Time)
		if timeRemaining < 0 {
			timeRemaining = 0
		}
		msg := FormatNudgeMessage(combatant.DisplayName, timeRemaining)

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

		timeRemaining := time.Until(turn.TimeoutAt.Time)
		if timeRemaining < 0 {
			timeRemaining = 0
		}
		msg := FormatTacticalSummary(combatant, turn, adjacentEnemies, timeRemaining)

		if err := t.notifier.SendMessage(channelID, msg); err != nil {
			return err
		}
	}

	_, err = t.store.UpdateTurnWarningSent(ctx, turn.ID)
	return err
}

func (t *TurnTimer) getYourTurnChannel(ctx context.Context, encounterID uuid.UUID) (string, error) {
	camp, err := t.store.GetCampaignByEncounterID(ctx, encounterID)
	if err != nil {
		return "", err
	}

	var settings campaign.Settings
	if camp.Settings.Valid {
		if err := json.Unmarshal(camp.Settings.RawMessage, &settings); err != nil {
			return "", err
		}
	}

	channelID := settings.ChannelIDs["your-turn"]
	if channelID == "" {
		return "", nil // no channel configured, silently skip
	}
	return channelID, nil
}

// findAdjacentEnemies returns enemy combatants within 1 tile (adjacent) of the target.
// Adjacent means Chebyshev distance <= 1 on the grid.
func findAdjacentEnemies(target refdata.Combatant, allCombatants []refdata.Combatant) []refdata.Combatant {
	targetCol := colToIndex(target.PositionCol)
	targetRow := int(target.PositionRow)

	var adjacent []refdata.Combatant
	for _, c := range allCombatants {
		if c.ID == target.ID {
			continue
		}
		if !c.IsAlive {
			continue
		}
		// "Enemy" means NPC if target is PC, or PC if target is NPC
		if c.IsNpc == target.IsNpc {
			continue
		}

		col := colToIndex(c.PositionCol)
		row := int(c.PositionRow)

		dCol := abs(col - targetCol)
		dRow := abs(row - targetRow)

		if dCol <= 1 && dRow <= 1 {
			adjacent = append(adjacent, c)
		}
	}
	return adjacent
}

