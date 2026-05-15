package discord

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// stubTurnGate is a TurnGate that returns a configured (info, err) tuple and
// records its call args so tests can assert the gate fired with the right
// inputs. Counters let tests verify the gate ran exactly once and that the
// downstream service calls were skipped on rejection.
//
// runCalls counts AcquireAndRun invocations separately so tests can assert
// which gate method a handler used (F-4: writers must call AcquireAndRun so
// the lock stays held across the persistence step).
type stubTurnGate struct {
	info       combat.TurnOwnerInfo
	err        error
	calls      int
	runCalls   int
	lastEnc    uuid.UUID
	lastUserID string
	// runErr, when non-nil, is returned as the synthetic fn error inside
	// AcquireAndRun so tests can simulate a write failure WITHOUT making
	// validation itself fail. When nil and fn was supplied, fn is invoked
	// normally and its error (if any) propagates.
	runErr error
	// ranInsideLock is set true when AcquireAndRun ran fn (proves the gate
	// invoked the callback rather than short-circuiting on a gate error).
	ranInsideLock bool
}

func (s *stubTurnGate) AcquireAndRelease(_ context.Context, encounterID uuid.UUID, discordUserID string) (combat.TurnOwnerInfo, error) {
	s.calls++
	s.lastEnc = encounterID
	s.lastUserID = discordUserID
	return s.info, s.err
}

func (s *stubTurnGate) AcquireAndRun(ctx context.Context, encounterID uuid.UUID, discordUserID string, fn func(ctx context.Context) error) (combat.TurnOwnerInfo, error) {
	s.calls++
	s.runCalls++
	s.lastEnc = encounterID
	s.lastUserID = discordUserID
	if s.err != nil {
		return combat.TurnOwnerInfo{}, s.err
	}
	if fn != nil {
		s.ranInsideLock = true
		if runErr := fn(ctx); runErr != nil {
			return combat.TurnOwnerInfo{}, runErr
		}
	}
	if s.runErr != nil {
		return combat.TurnOwnerInfo{}, s.runErr
	}
	return s.info, nil
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

// --- F-4: HandleMoveConfirm uses AcquireAndRun (lock held across write) ---

func TestMoveHandler_HandleMoveConfirm_UsesAcquireAndRun(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupMoveHandler(sess)

	// Wrap service so we can assert the write fired AND happened inside fn.
	wrapped := &callCountingMoveService{mockMoveService: handler.combatService.(*mockMoveService)}
	handler.combatService = wrapped

	gate := &stubTurnGate{} // success
	handler.SetTurnGate(gate)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}
	handler.HandleMoveConfirm(interaction, turnID, combatantID, 3, 0, 15)

	if gate.runCalls != 1 {
		t.Fatalf("expected HandleMoveConfirm to call AcquireAndRun exactly once, got %d", gate.runCalls)
	}
	if gate.calls != 1 {
		t.Fatalf("expected gate.calls == 1, got %d", gate.calls)
	}
	if !gate.ranInsideLock {
		t.Error("expected fn to run inside AcquireAndRun (proves the write happened with lock held)")
	}
	if wrapped.updatePosCalls != 1 {
		t.Errorf("expected exactly one position write inside the gated fn, got %d", wrapped.updatePosCalls)
	}
}

func TestMoveHandler_HandleMoveConfirm_GateRejectsBeforeWrite(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupMoveHandler(sess)

	wrapped := &callCountingMoveService{mockMoveService: handler.combatService.(*mockMoveService)}
	handler.combatService = wrapped

	// Simulate "the turn ended between Handle (showing the button) and the
	// user clicking Confirm". The gate must reject and fn must NOT run.
	gate := &stubTurnGate{err: combat.ErrTurnChanged}
	handler.SetTurnGate(gate)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}
	handler.HandleMoveConfirm(interaction, turnID, combatantID, 3, 0, 15)

	if gate.runCalls != 1 {
		t.Fatalf("expected AcquireAndRun to fire even on rejection, got %d", gate.runCalls)
	}
	if gate.ranInsideLock {
		t.Error("fn must NOT run when gate rejects (validation failed)")
	}
	if wrapped.updatePosCalls != 0 {
		t.Errorf("expected zero writes when gate rejects, got %d", wrapped.updatePosCalls)
	}
	if sess.lastResponse == nil {
		t.Fatal("expected ephemeral rejection response")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "no longer your turn") {
		t.Errorf("expected ErrTurnChanged message, got: %s", sess.lastResponse.Data.Content)
	}
}

func TestMoveHandler_HandleMoveConfirm_NilGate_StillWrites(t *testing.T) {
	// Legacy / unit-test path: no gate wired. Writes must still happen so
	// the existing TestMoveHandler_HandleMoveConfirm (which doesn't wire a
	// gate) keeps passing. This is the explicit nil-gate contract.
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupMoveHandler(sess)
	wrapped := &callCountingMoveService{mockMoveService: handler.combatService.(*mockMoveService)}
	handler.combatService = wrapped

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}
	handler.HandleMoveConfirm(interaction, turnID, combatantID, 3, 0, 15)

	if wrapped.updatePosCalls != 1 {
		t.Errorf("expected position write to fire even without a gate, got %d", wrapped.updatePosCalls)
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

// --- F-4: /attack uses AcquireAndRun (lock held across mutation) ---

func TestAttackHandler_UsesAcquireAndRun(t *testing.T) {
	h, sess, _, _ := setupAttackHandler()
	gate := &stubTurnGate{}
	h.SetTurnGate(gate)

	h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))

	if gate.runCalls != 1 {
		t.Fatalf("expected AcquireAndRun to fire once, got %d", gate.runCalls)
	}
	if !gate.ranInsideLock {
		t.Error("expected fn to run inside AcquireAndRun")
	}
	if sess.lastResponse == nil {
		t.Fatal("expected a response")
	}
}

func TestAttackHandler_AcquireAndRun_RejectsWrongOwner(t *testing.T) {
	h, sess, _, _ := setupAttackHandler()
	gate := &stubTurnGate{err: &combat.ErrNotYourTurn{
		CurrentCharacterName: "Goblin",
		CurrentDiscordUserID: "dm-1",
	}}
	h.SetTurnGate(gate)

	h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))

	if gate.runCalls != 1 {
		t.Fatalf("expected AcquireAndRun to fire, got %d", gate.runCalls)
	}
	if gate.ranInsideLock {
		t.Error("fn must NOT run when gate rejects")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "not your turn") {
		t.Errorf("expected not-your-turn message, got: %s", content)
	}
}

// --- F-4: /bonus uses AcquireAndRun ---

func TestBonusHandler_UsesAcquireAndRun(t *testing.T) {
	h, sess, _, _ := setupBonusHandler()
	gate := &stubTurnGate{}
	h.SetTurnGate(gate)

	h.Handle(makeBonusInteraction("rage", ""))

	if gate.runCalls != 1 {
		t.Fatalf("expected AcquireAndRun to fire once, got %d", gate.runCalls)
	}
	if !gate.ranInsideLock {
		t.Error("expected fn to run inside AcquireAndRun")
	}
	if sess.lastResponse == nil {
		t.Fatal("expected a response")
	}
}

func TestBonusHandler_AcquireAndRun_RejectsWrongOwner(t *testing.T) {
	h, sess, _, _ := setupBonusHandler()
	gate := &stubTurnGate{err: &combat.ErrNotYourTurn{
		CurrentCharacterName: "Goblin",
		CurrentDiscordUserID: "dm-1",
	}}
	h.SetTurnGate(gate)

	h.Handle(makeBonusInteraction("rage", ""))

	if gate.runCalls != 1 {
		t.Fatalf("expected AcquireAndRun to fire, got %d", gate.runCalls)
	}
	if gate.ranInsideLock {
		t.Error("fn must NOT run when gate rejects")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "not your turn") {
		t.Errorf("expected not-your-turn message, got: %s", content)
	}
}

// --- F-4: /fly confirm uses AcquireAndRun ---

func TestFlyHandler_HandleFlyConfirm_UsesAcquireAndRun(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupFlyHandler(sess)

	gate := &stubTurnGate{}
	handler.SetTurnGate(gate)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}
	handler.HandleFlyConfirm(interaction, turnID, combatantID, 30, 10)

	if gate.runCalls != 1 {
		t.Fatalf("expected AcquireAndRun to fire once, got %d", gate.runCalls)
	}
	if !gate.ranInsideLock {
		t.Error("expected fn to run inside AcquireAndRun")
	}
}

func TestFlyHandler_HandleFlyConfirm_GateRejectsBeforeWrite(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupFlyHandler(sess)

	wrapped := &callCountingMoveService{mockMoveService: handler.combatService.(*mockMoveService)}
	handler.combatService = wrapped

	gate := &stubTurnGate{err: combat.ErrTurnChanged}
	handler.SetTurnGate(gate)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}
	handler.HandleFlyConfirm(interaction, turnID, combatantID, 30, 10)

	if gate.ranInsideLock {
		t.Error("fn must NOT run when gate rejects")
	}
	if wrapped.updatePosCalls != 0 {
		t.Errorf("expected zero writes when gate rejects, got %d", wrapped.updatePosCalls)
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "no longer your turn") {
		t.Errorf("expected ErrTurnChanged message, got: %s", content)
	}
}

// --- F-4: /interact uses AcquireAndRun ---

func TestInteractHandler_UsesAcquireAndRun(t *testing.T) {
	h, sess, _, _ := setupInteractHandler()
	gate := &stubTurnGate{}
	h.SetTurnGate(gate)

	h.Handle(makeInteractInteraction("draw longsword"))

	if gate.runCalls != 1 {
		t.Fatalf("expected AcquireAndRun to fire once, got %d", gate.runCalls)
	}
	if !gate.ranInsideLock {
		t.Error("expected fn to run inside AcquireAndRun")
	}
	if sess.lastResponse == nil {
		t.Fatal("expected a response")
	}
}

func TestInteractHandler_AcquireAndRun_RejectsWrongOwner(t *testing.T) {
	h, sess, _, _ := setupInteractHandler()
	gate := &stubTurnGate{err: &combat.ErrNotYourTurn{
		CurrentCharacterName: "Goblin",
		CurrentDiscordUserID: "dm-1",
	}}
	h.SetTurnGate(gate)

	h.Handle(makeInteractInteraction("draw longsword"))

	if gate.runCalls != 1 {
		t.Fatalf("expected AcquireAndRun to fire, got %d", gate.runCalls)
	}
	if gate.ranInsideLock {
		t.Error("fn must NOT run when gate rejects")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "not your turn") {
		t.Errorf("expected not-your-turn message, got: %s", content)
	}
}

// --- F-4: /cast uses AcquireAndRun ---

func TestCastHandler_UsesAcquireAndRun(t *testing.T) {
	h, sess, _, _ := setupCastHandler()
	gate := &stubTurnGate{}
	h.SetTurnGate(gate)

	h.Handle(makeCastInteraction(map[string]any{"spell": "fireball", "target": "OS"}))

	if gate.runCalls != 1 {
		t.Fatalf("expected AcquireAndRun to fire once, got %d", gate.runCalls)
	}
	if !gate.ranInsideLock {
		t.Error("expected fn to run inside AcquireAndRun")
	}
	if sess.lastResponse == nil {
		t.Fatal("expected a response")
	}
}

func TestCastHandler_AcquireAndRun_RejectsWrongOwner(t *testing.T) {
	h, sess, _, _ := setupCastHandler()
	gate := &stubTurnGate{err: &combat.ErrNotYourTurn{
		CurrentCharacterName: "Goblin",
		CurrentDiscordUserID: "dm-1",
	}}
	h.SetTurnGate(gate)

	h.Handle(makeCastInteraction(map[string]any{"spell": "fireball", "target": "OS"}))

	if gate.runCalls != 1 {
		t.Fatalf("expected AcquireAndRun to fire, got %d", gate.runCalls)
	}
	if gate.ranInsideLock {
		t.Error("fn must NOT run when gate rejects")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "not your turn") {
		t.Errorf("expected not-your-turn message, got: %s", content)
	}
}

// --- Finding 7: /action uses AcquireAndRun in combat mode ---

func TestActionHandler_UsesAcquireAndRun_CombatMode(t *testing.T) {
	h, sess := setupActionHandlerForGateTest()
	gate := &stubTurnGate{}
	h.SetTurnGate(gate)

	h.Handle(makeActionInteraction("g1", "u1", "flip the table", ""))

	if gate.runCalls != 1 {
		t.Fatalf("expected AcquireAndRun to fire once, got %d", gate.runCalls)
	}
	if !gate.ranInsideLock {
		t.Error("expected fn to run inside AcquireAndRun")
	}
	if sess.lastResponse == nil {
		t.Fatal("expected a response")
	}
}

func TestActionHandler_AcquireAndRun_RejectsWrongOwner(t *testing.T) {
	h, sess := setupActionHandlerForGateTest()
	gate := &stubTurnGate{err: &combat.ErrNotYourTurn{
		CurrentCharacterName: "Goblin",
		CurrentDiscordUserID: "dm-1",
	}}
	h.SetTurnGate(gate)

	h.Handle(makeActionInteraction("g1", "u1", "flip the table", ""))

	if gate.runCalls != 1 {
		t.Fatalf("expected AcquireAndRun to fire, got %d", gate.runCalls)
	}
	if gate.ranInsideLock {
		t.Error("fn must NOT run when gate rejects")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "not your turn") {
		t.Errorf("expected not-your-turn message, got: %s", content)
	}
}

func TestActionHandler_NoGate_ExplorationMode_SkipsGate(t *testing.T) {
	h, _ := setupActionHandlerForGateTest()
	// Switch to exploration mode
	h.combatService = &mockActionCombatServiceForGate{
		encounter: refdata.Encounter{
			ID:   uuid.New(),
			Mode: "exploration",
		},
	}
	gate := &stubTurnGate{err: errors.New("gate must not fire")}
	h.SetTurnGate(gate)

	h.Handle(makeActionInteraction("g1", "u1", "look around", ""))

	if gate.calls != 0 {
		t.Errorf("expected gate NOT to fire in exploration mode, got %d calls", gate.calls)
	}
}

// --- helpers for /action gate tests ---

// mockActionCombatServiceForGate is a minimal ActionCombatService that
// returns canned data for the gate tests. Only GetEncounter, GetCombatant,
// ListCombatantsByEncounterID, and FreeformAction are exercised.
type mockActionCombatServiceForGate struct {
	encounter refdata.Encounter
}

func (m *mockActionCombatServiceForGate) GetEncounter(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
	return m.encounter, nil
}
func (m *mockActionCombatServiceForGate) GetCombatant(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
	return refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true}}, nil
}
func (m *mockActionCombatServiceForGate) ListCombatantsByEncounterID(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
	return nil, nil
}
func (m *mockActionCombatServiceForGate) FreeformAction(_ context.Context, cmd combat.FreeformActionCommand) (combat.FreeformActionResult, error) {
	return combat.FreeformActionResult{CombatLog: "action queued"}, nil
}
func (m *mockActionCombatServiceForGate) CancelFreeformAction(_ context.Context, _ combat.CancelFreeformActionCommand) (combat.CancelFreeformActionResult, error) {
	return combat.CancelFreeformActionResult{}, nil
}
func (m *mockActionCombatServiceForGate) CancelExplorationFreeformAction(_ context.Context, _ uuid.UUID) (combat.CancelFreeformActionResult, error) {
	return combat.CancelFreeformActionResult{}, nil
}
func (m *mockActionCombatServiceForGate) ReadyAction(_ context.Context, _ combat.ReadyActionCommand) (combat.ReadyActionResult, error) {
	return combat.ReadyActionResult{}, nil
}
func (m *mockActionCombatServiceForGate) ActionSurge(_ context.Context, _ combat.ActionSurgeCommand) (combat.ActionSurgeResult, error) {
	return combat.ActionSurgeResult{}, nil
}
func (m *mockActionCombatServiceForGate) Dash(_ context.Context, _ combat.DashCommand) (combat.DashResult, error) {
	return combat.DashResult{}, nil
}
func (m *mockActionCombatServiceForGate) Disengage(_ context.Context, _ combat.DisengageCommand) (combat.DisengageResult, error) {
	return combat.DisengageResult{}, nil
}
func (m *mockActionCombatServiceForGate) Dodge(_ context.Context, _ combat.DodgeCommand) (combat.DodgeResult, error) {
	return combat.DodgeResult{}, nil
}
func (m *mockActionCombatServiceForGate) Help(_ context.Context, _ combat.HelpCommand) (combat.HelpResult, error) {
	return combat.HelpResult{}, nil
}
func (m *mockActionCombatServiceForGate) Hide(_ context.Context, _ combat.HideCommand, _ *dice.Roller) (combat.HideResult, error) {
	return combat.HideResult{}, nil
}
func (m *mockActionCombatServiceForGate) Stand(_ context.Context, _ combat.StandCommand) (combat.StandResult, error) {
	return combat.StandResult{}, nil
}
func (m *mockActionCombatServiceForGate) DropProne(_ context.Context, _ combat.DropProneCommand) (combat.DropProneResult, error) {
	return combat.DropProneResult{}, nil
}
func (m *mockActionCombatServiceForGate) Escape(_ context.Context, _ combat.EscapeCommand, _ *dice.Roller) (combat.EscapeResult, error) {
	return combat.EscapeResult{}, nil
}
func (m *mockActionCombatServiceForGate) Grapple(_ context.Context, _ combat.GrappleCommand, _ *dice.Roller) (combat.GrappleResult, error) {
	return combat.GrappleResult{}, nil
}
func (m *mockActionCombatServiceForGate) TurnUndead(_ context.Context, _ combat.TurnUndeadCommand, _ *dice.Roller) (combat.TurnUndeadResult, error) {
	return combat.TurnUndeadResult{}, nil
}
func (m *mockActionCombatServiceForGate) PreserveLife(_ context.Context, _ combat.PreserveLifeCommand) (combat.PreserveLifeResult, error) {
	return combat.PreserveLifeResult{}, nil
}
func (m *mockActionCombatServiceForGate) SacredWeapon(_ context.Context, _ combat.SacredWeaponCommand) (combat.SacredWeaponResult, error) {
	return combat.SacredWeaponResult{}, nil
}
func (m *mockActionCombatServiceForGate) VowOfEnmity(_ context.Context, _ combat.VowOfEnmityCommand) (combat.VowOfEnmityResult, error) {
	return combat.VowOfEnmityResult{}, nil
}
func (m *mockActionCombatServiceForGate) ChannelDivinityDMQueue(_ context.Context, _ combat.ChannelDivinityDMQueueCommand) (combat.DMQueueResult, error) {
	return combat.DMQueueResult{}, nil
}
func (m *mockActionCombatServiceForGate) LayOnHands(_ context.Context, _ combat.LayOnHandsCommand) (combat.LayOnHandsResult, error) {
	return combat.LayOnHandsResult{}, nil
}

type mockActionTurnProviderForGate struct {
	turn refdata.Turn
}

func (m *mockActionTurnProviderForGate) GetTurn(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
	return m.turn, nil
}

type mockActionCampaignProviderForGate struct {
	campaign refdata.Campaign
}

func (m *mockActionCampaignProviderForGate) GetCampaignByGuildID(_ context.Context, _ string) (refdata.Campaign, error) {
	return m.campaign, nil
}

type mockActionCharacterLookupForGate struct {
	char refdata.Character
}

func (m *mockActionCharacterLookupForGate) GetCharacterByCampaignAndDiscord(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
	return m.char, nil
}

func setupActionHandlerForGateTest() (*ActionHandler, *mockMoveSession) {
	encID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()

	sess := &mockMoveSession{}
	resolver := &fakeActionEncounterResolver{encounterID: encID}
	combatSvc := &mockActionCombatServiceForGate{
		encounter: refdata.Encounter{
			ID:            encID,
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			Mode:          "combat",
		},
	}
	turnProv := &mockActionTurnProviderForGate{
		turn: refdata.Turn{ID: turnID, CombatantID: combatantID},
	}
	campProv := &mockActionCampaignProviderForGate{
		campaign: refdata.Campaign{ID: uuid.New()},
	}
	charLookup := &mockActionCharacterLookupForGate{
		char: refdata.Character{ID: charID},
	}

	// Override GetCombatant to return a combatant with matching CharacterID
	combatSvc2 := &mockActionCombatServiceForGateWithChar{
		mockActionCombatServiceForGate: *combatSvc,
		combatant: refdata.Combatant{
			ID:          combatantID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		},
	}

	h := NewActionHandler(sess, resolver, combatSvc2, turnProv, campProv, charLookup, nil)
	return h, sess
}

// mockActionCombatServiceForGateWithChar extends the base mock to return
// a specific combatant with a CharacterID that matches the character lookup.
type mockActionCombatServiceForGateWithChar struct {
	mockActionCombatServiceForGate
	combatant refdata.Combatant
}

func (m *mockActionCombatServiceForGateWithChar) GetCombatant(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
	return m.combatant, nil
}
