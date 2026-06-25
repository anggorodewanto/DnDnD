package combat

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The DM dashboard "advance turn" path calls Service.AdvanceTurn directly (no
// /done handler), so before this fix a turn advanced from the dashboard posted
// nothing to #your-turn — the next player sat in silence. AdvanceTurn must fire
// the TurnStartNotifier for the newly-active combatant on every advance, not
// just the first turn of combat.
func TestAdvanceTurn_FiresTurnStartNotifier(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := advanceTurnStoreForKind(combatantID, "Goblin G2", true)

	notifier := &stubTurnStartNotifier{}
	svc := NewService(store)
	svc.SetTurnStartNotifier(notifier)

	info, err := svc.AdvanceTurn(ctx, encounterID)
	require.NoError(t, err)

	require.Len(t, notifier.calls, 1, "advance must fire the turn-start ping exactly once")
	assert.Equal(t, combatantID, notifier.calls[0].CombatantID)
	assert.Equal(t, info.Turn.ID, notifier.calls[0].Turn.ID)
	assert.Equal(t, int32(1), notifier.calls[0].RoundNumber)
	// No prior completed turn for this combatant → empty impact summary.
	assert.Empty(t, notifier.impacts[0])
}

// A nil TurnStartNotifier must never break a turn advance.
func TestAdvanceTurn_NilTurnStartNotifier_NoOp(t *testing.T) {
	ctx := context.Background()
	store := advanceTurnStoreForKind(uuid.New(), "Goblin", true)

	svc := NewService(store)
	// no SetTurnStartNotifier

	_, err := svc.AdvanceTurn(ctx, uuid.New())
	require.NoError(t, err)
}
