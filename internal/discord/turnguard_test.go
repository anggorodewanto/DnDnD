package discord

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// stubTurnGate is a TurnGate that returns a configured (info, err) tuple and
// records its call args so tests can assert the gate fired with the right
// inputs. Counters let tests verify the gate ran exactly once and that the
// downstream service calls were skipped on rejection.
type stubTurnGate struct {
	info       combat.TurnOwnerInfo
	err        error
	calls      int
	lastEnc    uuid.UUID
	lastUserID string
}

func (s *stubTurnGate) AcquireAndRelease(_ context.Context, encounterID uuid.UUID, discordUserID string) (combat.TurnOwnerInfo, error) {
	s.calls++
	s.lastEnc = encounterID
	s.lastUserID = discordUserID
	return s.info, s.err
}

// callCountingMoveService wraps mockMoveService and counts UpdateCombatantPosition
// calls, so a test can assert "the service write never fired".
type callCountingMoveService struct {
	*mockMoveService
	updatePosCalls int
}

func (c *callCountingMoveService) UpdateCombatantPosition(ctx context.Context, id uuid.UUID, col string, row, alt int32) (refdata.Combatant, error) {
	c.updatePosCalls++
	return c.mockMoveService.UpdateCombatantPosition(ctx, id, col, row, alt)
}

// --- /move turn-gate tests ---

func TestMoveHandler_TurnGate_RejectsWrongOwner(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)

	gate := &stubTurnGate{err: &combat.ErrNotYourTurn{
		CurrentCharacterName: "Aragorn",
		CurrentDiscordUserID: "owner-456",
	}}
	handler.SetTurnGate(gate)

	// Wrap the combat service so we can assert the write never fired.
	wrapped := &callCountingMoveService{mockMoveService: handler.combatService.(*mockMoveService)}
	handler.combatService = wrapped

	interaction := makeMoveInteraction("D1")
	handler.Handle(interaction)

	if gate.calls != 1 {
		t.Fatalf("expected gate to fire exactly once, got %d", gate.calls)
	}
	if sess.lastResponse == nil {
		t.Fatal("expected ephemeral response on rejection")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "not your turn") {
		t.Errorf("expected ErrNotYourTurn message, got: %s", content)
	}
	if !strings.Contains(content, "Aragorn") {
		t.Errorf("expected current owner's name in message, got: %s", content)
	}
	if wrapped.updatePosCalls != 0 {
		t.Errorf("expected zero position writes after rejection, got %d", wrapped.updatePosCalls)
	}
}

func TestMoveHandler_TurnGate_PassesEncounterAndUserID(t *testing.T) {
	sess := &mockMoveSession{}
	handler, encID, _, _ := setupMoveHandler(sess)

	gate := &stubTurnGate{} // success: zero info, nil err
	handler.SetTurnGate(gate)

	handler.Handle(makeMoveInteraction("D1"))

	if gate.calls != 1 {
		t.Fatalf("expected gate to fire once, got %d", gate.calls)
	}
	if gate.lastEnc != encID {
		t.Errorf("expected encounterID %s passed to gate, got %s", encID, gate.lastEnc)
	}
	if gate.lastUserID != "user1" {
		t.Errorf("expected discordUserID 'user1' passed to gate, got %q", gate.lastUserID)
	}
}

func TestMoveHandler_TurnGate_LockTimeoutMessage(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)

	handler.SetTurnGate(&stubTurnGate{err: combat.ErrLockTimeout})
	handler.Handle(makeMoveInteraction("D1"))

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "busy") {
		t.Errorf("expected busy/timeout message, got: %s", content)
	}
}

func TestMoveHandler_TurnGate_NoActiveTurnMessage(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)

	handler.SetTurnGate(&stubTurnGate{err: combat.ErrNoActiveTurn})
	handler.Handle(makeMoveInteraction("D1"))

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "No active turn") {
		t.Errorf("expected no-active-turn message, got: %s", content)
	}
}

func TestMoveHandler_TurnGate_NotInvokedInExploration(t *testing.T) {
	sess := &mockMoveSession{}
	handler, encID, _, _ := setupMoveHandler(sess)

	// Switch the encounter to exploration mode — the gate must NOT fire.
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:    encID,
				Mode:  "exploration",
				MapID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
			}, nil
		},
		getCombatant:       func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) { return refdata.Combatant{}, nil },
		listCombatants:     func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) { return nil, nil },
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) { return refdata.Combatant{}, nil },
	}
	handler.mapProvider = &mockMoveMapProvider{
		getByID: func(_ context.Context, _ uuid.UUID) (refdata.Map, error) {
			return refdata.Map{TiledJson: tiledJSON5x5()}, nil
		},
	}

	gate := &stubTurnGate{}
	handler.SetTurnGate(gate)
	handler.Handle(makeMoveInteraction("D1"))

	if gate.calls != 0 {
		t.Errorf("expected gate NOT to fire in exploration mode, got %d calls", gate.calls)
	}
}

// --- /fly turn-gate tests ---

func TestFlyHandler_TurnGate_RejectsWrongOwner(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupFlyHandler(sess)

	gate := &stubTurnGate{err: &combat.ErrNotYourTurn{
		CurrentCharacterName: "Goblin #1",
		CurrentDiscordUserID: "dm-1",
	}}
	handler.SetTurnGate(gate)

	wrapped := &callCountingMoveService{mockMoveService: handler.combatService.(*mockMoveService)}
	handler.combatService = wrapped

	handler.Handle(makeFlyInteraction(30))

	if gate.calls != 1 {
		t.Fatalf("expected gate to fire once, got %d", gate.calls)
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "not your turn") {
		t.Errorf("expected not-your-turn message, got: %s", content)
	}
	if wrapped.updatePosCalls != 0 {
		t.Errorf("expected zero position writes after rejection, got %d", wrapped.updatePosCalls)
	}
}

func TestFlyHandler_TurnGate_PassesEncounterAndUserID(t *testing.T) {
	sess := &mockMoveSession{}
	handler, encID, _, _ := setupFlyHandler(sess)

	gate := &stubTurnGate{}
	handler.SetTurnGate(gate)

	handler.Handle(makeFlyInteraction(30))

	if gate.calls != 1 {
		t.Fatalf("expected gate to fire once, got %d", gate.calls)
	}
	if gate.lastEnc != encID {
		t.Errorf("expected encounterID %s, got %s", encID, gate.lastEnc)
	}
	if gate.lastUserID != "user1" {
		t.Errorf("expected user 'user1', got %q", gate.lastUserID)
	}
}

// --- /distance turn-gate tests ---

func TestDistanceHandler_IsExempt_GateNotInvoked(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDistanceHandler(sess)

	// Even with a configured gate that would always reject, /distance should
	// not call it because IsExemptCommand("distance") == true. This proves
	// the handler honors the spec exemption for read-only commands.
	gate := &stubTurnGate{err: errors.New("gate must not fire")}
	handler.SetTurnGate(gate)

	handler.Handle(makeDistanceInteraction("G1"))

	if gate.calls != 0 {
		t.Errorf("expected gate to NOT fire for exempt /distance, got %d calls", gate.calls)
	}
	// Distance handler should still respond with the measured distance.
	if sess.lastResponse == nil {
		t.Fatal("expected distance response")
	}
}

// Sanity: confirm IsExemptCommand("distance") really is true so the test
// above is meaningful (not a false-positive from a typo in the handler).
func TestIsExemptCommand_DistanceListed(t *testing.T) {
	if !combat.IsExemptCommand("distance") {
		t.Fatal("IsExemptCommand(\"distance\") must be true so distance handler can branch on it")
	}
	if !combat.IsExemptCommand("reaction") {
		t.Fatal("IsExemptCommand contract regression: reaction should remain exempt")
	}
}

// --- formatTurnGateError exhaustive cases ---

func TestFormatTurnGateError_AllCases(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"not your turn", &combat.ErrNotYourTurn{
			CurrentCharacterName: "Bob",
			CurrentDiscordUserID: "u-123",
		}, "It's not your turn. Current turn: **Bob** (@u-123)"},
		{"no active turn", combat.ErrNoActiveTurn, "No active turn."},
		{"lock timeout", combat.ErrLockTimeout, "The server is busy processing the active turn — please try again."},
		{"turn changed", combat.ErrTurnChanged, "It's no longer your turn."},
		{"generic", errors.New("db down"), "Failed to validate turn ownership."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTurnGateError(tt.err)
			if got != tt.want {
				t.Errorf("formatTurnGateError(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}
