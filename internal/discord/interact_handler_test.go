package discord

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// --- Mocks for /interact ---

type mockInteractProvider struct {
	encounterID  uuid.UUID
	encounter    refdata.Encounter
	combatant    refdata.Combatant
	resolveErr   error
	getEncErr    error
	getCombatErr error
}

func (m *mockInteractProvider) ActiveEncounterForUser(_ context.Context, _ string, _ string) (uuid.UUID, error) {
	if m.resolveErr != nil {
		return uuid.Nil, m.resolveErr
	}
	return m.encounterID, nil
}

func (m *mockInteractProvider) GetEncounter(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
	if m.getEncErr != nil {
		return refdata.Encounter{}, m.getEncErr
	}
	return m.encounter, nil
}

func (m *mockInteractProvider) GetCombatant(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
	if m.getCombatErr != nil {
		return refdata.Combatant{}, m.getCombatErr
	}
	return m.combatant, nil
}

type mockInteractTurnStore struct {
	turn        refdata.Turn
	updated     *refdata.UpdateTurnActionsParams
	getTurnErr  error
	updateErr   error
}

func (m *mockInteractTurnStore) GetTurn(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
	if m.getTurnErr != nil {
		return refdata.Turn{}, m.getTurnErr
	}
	return m.turn, nil
}

func (m *mockInteractTurnStore) UpdateTurnActions(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
	m.updated = &arg
	if m.updateErr != nil {
		return refdata.Turn{}, m.updateErr
	}
	return refdata.Turn{ID: arg.ID, FreeInteractUsed: arg.FreeInteractUsed}, nil
}

func makeInteractInteraction(desc string) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "g1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "u1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "interact",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: "description", Value: desc, Type: discordgo.ApplicationCommandOptionString},
			},
		},
	}
}

func setupInteractHandler() (*InteractHandler, *mockMoveSession, *mockInteractTurnStore, *mockInteractProvider) {
	encID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()

	provider := &mockInteractProvider{
		encounterID: encID,
		encounter: refdata.Encounter{
			ID:            encID,
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
		},
		combatant: refdata.Combatant{ID: combatantID, DisplayName: "Aria"},
	}
	turnStore := &mockInteractTurnStore{
		turn: refdata.Turn{
			ID:               turnID,
			CombatantID:      combatantID,
			FreeInteractUsed: false,
		},
	}
	sess := &mockMoveSession{}
	h := NewInteractHandler(sess, provider, turnStore)
	return h, sess, turnStore, provider
}

// --- Tests ---

func TestInteractHandler_MarksFreeInteractUsed(t *testing.T) {
	h, sess, turnStore, _ := setupInteractHandler()

	h.Handle(makeInteractInteraction("draw longsword"))

	if turnStore.updated == nil {
		t.Fatal("expected turn to be updated")
	}
	if !turnStore.updated.FreeInteractUsed {
		t.Error("expected FreeInteractUsed=true after /interact")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "draw longsword") {
		t.Errorf("expected description in response, got %q", sess.lastResponse.Data.Content)
	}
}

func TestInteractHandler_RejectsWhenAlreadyUsed(t *testing.T) {
	h, sess, turnStore, _ := setupInteractHandler()
	turnStore.turn.FreeInteractUsed = true

	h.Handle(makeInteractInteraction("draw longsword"))

	if turnStore.updated != nil {
		t.Error("expected no turn update when free interact already spent")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Cannot interact") {
		t.Errorf("expected rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestInteractHandler_NoEncounter(t *testing.T) {
	h, sess, _, provider := setupInteractHandler()
	provider.resolveErr = errNoEncounter

	h.Handle(makeInteractInteraction("draw longsword"))

	if !strings.Contains(sess.lastResponse.Data.Content, "not in an active encounter") {
		t.Errorf("expected no-encounter rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestInteractHandler_PostsToCombatLog(t *testing.T) {
	h, _, _, _ := setupInteractHandler()
	csp := &mockDeathSaveCSP{
		fn: func(_ context.Context, _ uuid.UUID) (map[string]string, error) {
			return map[string]string{"combat-log": "ch-1"}, nil
		},
	}
	h.SetChannelIDProvider(csp)

	captured := []string{}
	h.session = &capturingSession{
		mockMoveSession: &mockMoveSession{},
		sendFn: func(channelID, content string) {
			captured = append(captured, channelID+":"+content)
		},
	}

	h.Handle(makeInteractInteraction("flip lever"))

	if len(captured) != 1 {
		t.Fatalf("expected 1 combat-log post, got %d", len(captured))
	}
	if !strings.Contains(captured[0], "ch-1") || !strings.Contains(captured[0], "flip lever") {
		t.Errorf("expected combat-log post containing description, got %q", captured[0])
	}
}

func TestInteractHandler_TurnGate_RejectsWrongOwner(t *testing.T) {
	h, sess, turnStore, _ := setupInteractHandler()
	h.SetTurnGate(&stubTurnGate{err: &combat.ErrNotYourTurn{
		CurrentCharacterName: "Bob",
		CurrentDiscordUserID: "u-bob",
	}})

	h.Handle(makeInteractInteraction("draw"))

	if turnStore.updated != nil {
		t.Error("expected no update when gate rejects ownership")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Bob") {
		t.Errorf("expected wrong-owner rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestInteractHandler_NoCurrentTurn(t *testing.T) {
	h, sess, _, provider := setupInteractHandler()
	provider.encounter.CurrentTurnID = uuid.NullUUID{Valid: false}

	h.Handle(makeInteractInteraction("draw"))

	if !strings.Contains(sess.lastResponse.Data.Content, "No active turn") {
		t.Errorf("expected no-active-turn rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestInteractHandler_GetTurnError(t *testing.T) {
	h, sess, turnStore, _ := setupInteractHandler()
	turnStore.getTurnErr = errors.New("boom")

	h.Handle(makeInteractInteraction("draw"))

	if !strings.Contains(sess.lastResponse.Data.Content, "Failed to load turn") {
		t.Errorf("expected load-turn rejection, got %q", sess.lastResponse.Data.Content)
	}
}

// --- SR-005: combat.Interact wiring ---

// fakeInteractCombatService records the InteractCommand received and returns a
// canned InteractResult / error. Lets handler tests assert SR-005 routing
// (first/second/third /interact) without needing a full combat.Service.
type fakeInteractCombatService struct {
	calls  []combat.InteractCommand
	result combat.InteractResult
	err    error
}

func (f *fakeInteractCombatService) Interact(_ context.Context, cmd combat.InteractCommand) (combat.InteractResult, error) {
	f.calls = append(f.calls, cmd)
	if f.err != nil {
		return combat.InteractResult{}, f.err
	}
	return f.result, nil
}

// TestInteractHandler_SR005_FirstFreeInteractConsumesFreeFlag covers
// acceptance criterion (a): first /interact with combat service wired
// routes through combat.Interact, marks FreeInteractUsed, and leaves the
// action untouched.
func TestInteractHandler_SR005_FirstFreeInteractConsumesFreeFlag(t *testing.T) {
	h, sess, turnStore, _ := setupInteractHandler()

	fake := &fakeInteractCombatService{
		result: combat.InteractResult{
			Turn:         refdata.Turn{ID: turnStore.turn.ID, FreeInteractUsed: true},
			CombatLog:    "🤚 Aria: draw longsword",
			AutoResolved: true,
		},
	}
	h.SetCombatService(fake)

	h.Handle(makeInteractInteraction("draw longsword"))

	if len(fake.calls) != 1 {
		t.Fatalf("expected combat.Interact to be called once, got %d", len(fake.calls))
	}
	if fake.calls[0].Turn.FreeInteractUsed {
		t.Error("expected to pass un-consumed turn to combat.Interact")
	}
	if fake.calls[0].Description != "draw longsword" {
		t.Errorf("expected description forwarded, got %q", fake.calls[0].Description)
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "draw longsword") {
		t.Errorf("expected combat-log line in response, got %q", sess.lastResponse.Data.Content)
	}
	// SR-005: handler must no longer touch the legacy turn store when the
	// combat service is wired — combat.Interact owns the UpdateTurnActions
	// write.
	if turnStore.updated != nil {
		t.Error("expected handler to delegate turn update to combat.Interact")
	}
}

// TestInteractHandler_SR005_SecondInteractFallsBackToAction covers acceptance
// criterion (b): with FreeInteractUsed already true, /interact must call
// combat.Interact (which consumes the action and creates a pending_actions
// row); the player sees a "sent to DM queue" combat log instead of an error.
func TestInteractHandler_SR005_SecondInteractFallsBackToAction(t *testing.T) {
	h, sess, turnStore, _ := setupInteractHandler()
	turnStore.turn.FreeInteractUsed = true

	pendingID := uuid.New()
	fake := &fakeInteractCombatService{
		result: combat.InteractResult{
			Turn: refdata.Turn{
				ID:               turnStore.turn.ID,
				FreeInteractUsed: true,
				ActionUsed:       true,
			},
			PendingAction: refdata.PendingAction{
				ID:         pendingID,
				ActionText: "search the chest",
				Status:     "pending",
			},
			CombatLog:      `🤚 Aria: "search the chest" — sent to DM queue`,
			DMQueueMessage: `🤚 **Interact** — Aria: "search the chest"`,
		},
	}
	h.SetCombatService(fake)

	h.Handle(makeInteractInteraction("search the chest"))

	if len(fake.calls) != 1 {
		t.Fatalf("expected combat.Interact to be called once, got %d", len(fake.calls))
	}
	if !fake.calls[0].Turn.FreeInteractUsed {
		t.Error("expected handler to forward turn.FreeInteractUsed=true so service falls back to action")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "DM queue") {
		t.Errorf("expected DM-queue line in response, got %q", sess.lastResponse.Data.Content)
	}
	if strings.Contains(sess.lastResponse.Data.Content, "Cannot interact") {
		t.Errorf("second /interact must not be rejected, got %q", sess.lastResponse.Data.Content)
	}
}

// TestInteractHandler_SR005_ThirdInteractRejectedWhenActionSpent covers
// acceptance criterion (c): when both free interact and action are spent,
// combat.Interact returns the action-spent error and the handler surfaces it.
func TestInteractHandler_SR005_ThirdInteractRejectedWhenActionSpent(t *testing.T) {
	h, sess, turnStore, _ := setupInteractHandler()
	turnStore.turn.FreeInteractUsed = true
	turnStore.turn.ActionUsed = true

	fake := &fakeInteractCombatService{
		err: errors.New("Free interaction already used and action is spent"),
	}
	h.SetCombatService(fake)

	h.Handle(makeInteractInteraction("draw dagger"))

	if len(fake.calls) != 1 {
		t.Fatalf("expected combat.Interact to be called once, got %d", len(fake.calls))
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Cannot interact") {
		t.Errorf("expected action-spent rejection, got %q", sess.lastResponse.Data.Content)
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "action is spent") {
		t.Errorf("expected action-spent rejection wording, got %q", sess.lastResponse.Data.Content)
	}
	if turnStore.updated != nil {
		t.Error("expected no turn update when action is spent")
	}
}
