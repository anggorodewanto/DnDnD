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
