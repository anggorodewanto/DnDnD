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

// --- Mocks for /shove ---

type mockShoveCombatService struct {
	shoveCalls   []combat.ShoveCommand
	grappleCalls []combat.GrappleCommand
	shoveResult  combat.ShoveResult
	grappleResult combat.GrappleResult
	shoveErr     error
	grappleErr   error
}

func (m *mockShoveCombatService) Shove(_ context.Context, cmd combat.ShoveCommand, _ *dice.Roller) (combat.ShoveResult, error) {
	m.shoveCalls = append(m.shoveCalls, cmd)
	return m.shoveResult, m.shoveErr
}

func (m *mockShoveCombatService) Grapple(_ context.Context, cmd combat.GrappleCommand, _ *dice.Roller) (combat.GrappleResult, error) {
	m.grappleCalls = append(m.grappleCalls, cmd)
	return m.grappleResult, m.grappleErr
}

type mockShoveProvider struct {
	encID       uuid.UUID
	turn        refdata.Turn
	shover      refdata.Combatant
	target      refdata.Combatant
	enc         refdata.Encounter
	resolveErr  error
	getEncErr   error
	getTurnErr  error
	getCombErr  error
	listErr     error
}

func (m *mockShoveProvider) ActiveEncounterForUser(_ context.Context, _, _ string) (uuid.UUID, error) {
	if m.resolveErr != nil {
		return uuid.Nil, m.resolveErr
	}
	return m.encID, nil
}

func (m *mockShoveProvider) GetEncounter(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
	if m.getEncErr != nil {
		return refdata.Encounter{}, m.getEncErr
	}
	return m.enc, nil
}

func (m *mockShoveProvider) GetCombatant(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
	if m.getCombErr != nil {
		return refdata.Combatant{}, m.getCombErr
	}
	if id == m.shover.ID {
		return m.shover, nil
	}
	return m.target, nil
}

func (m *mockShoveProvider) ListCombatantsByEncounterID(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return []refdata.Combatant{m.shover, m.target}, nil
}

func (m *mockShoveProvider) GetTurn(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
	if m.getTurnErr != nil {
		return refdata.Turn{}, m.getTurnErr
	}
	return m.turn, nil
}

func makeShoveInteraction(target string, mode string) *discordgo.Interaction {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "target", Value: target, Type: discordgo.ApplicationCommandOptionString},
	}
	if mode != "" {
		opts = append(opts, &discordgo.ApplicationCommandInteractionDataOption{
			Name: "mode", Value: mode, Type: discordgo.ApplicationCommandOptionString,
		})
	}
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "g1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "u1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "shove",
			Options: opts,
		},
	}
}

func setupShoveHandler() (*ShoveHandler, *mockMoveSession, *mockShoveCombatService, *mockShoveProvider) {
	encID := uuid.New()
	turnID := uuid.New()
	shoverID := uuid.New()
	targetID := uuid.New()

	provider := &mockShoveProvider{
		encID: encID,
		enc: refdata.Encounter{
			ID:            encID,
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
		},
		turn: refdata.Turn{
			ID:          turnID,
			CombatantID: shoverID,
		},
		shover: refdata.Combatant{
			ID: shoverID, ShortID: "AR", DisplayName: "Aria",
			PositionCol: "A", PositionRow: 1,
		},
		target: refdata.Combatant{
			ID: targetID, ShortID: "OS", DisplayName: "Orc",
			PositionCol: "B", PositionRow: 1,
		},
	}
	combatSvc := &mockShoveCombatService{
		shoveResult:   combat.ShoveResult{CombatLog: "📌 Aria shoves Orc — Pushed!"},
		grappleResult: combat.GrappleResult{CombatLog: "🤼 Aria grapples Orc!"},
	}
	sess := &mockMoveSession{}
	h := NewShoveHandler(sess, combatSvc, provider, dice.NewRoller(func(_ int) int { return 10 }))
	return h, sess, combatSvc, provider
}

// --- Tests ---

func TestShoveHandler_DefaultModeIsPush(t *testing.T) {
	h, sess, svc, _ := setupShoveHandler()

	h.Handle(makeShoveInteraction("OS", ""))

	if len(svc.shoveCalls) != 1 {
		t.Fatalf("expected 1 shove call, got %d", len(svc.shoveCalls))
	}
	if svc.shoveCalls[0].Mode != combat.ShovePush {
		t.Errorf("expected push mode, got %v", svc.shoveCalls[0].Mode)
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Pushed") {
		t.Errorf("expected push log, got %q", sess.lastResponse.Data.Content)
	}
}

func TestShoveHandler_ProneMode(t *testing.T) {
	h, _, svc, _ := setupShoveHandler()

	h.Handle(makeShoveInteraction("OS", "prone"))

	if len(svc.shoveCalls) != 1 {
		t.Fatalf("expected 1 shove call, got %d", len(svc.shoveCalls))
	}
	if svc.shoveCalls[0].Mode != combat.ShoveProne {
		t.Errorf("expected prone mode, got %v", svc.shoveCalls[0].Mode)
	}
}

func TestShoveHandler_GrappleMode(t *testing.T) {
	h, sess, svc, _ := setupShoveHandler()

	h.Handle(makeShoveInteraction("OS", "grapple"))

	if len(svc.grappleCalls) != 1 {
		t.Fatalf("expected 1 grapple call, got %d", len(svc.grappleCalls))
	}
	if len(svc.shoveCalls) != 0 {
		t.Errorf("expected no shove calls when grapple mode, got %d", len(svc.shoveCalls))
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "grapples") {
		t.Errorf("expected grapple log, got %q", sess.lastResponse.Data.Content)
	}
}

func TestShoveHandler_TargetNotFound(t *testing.T) {
	h, sess, svc, _ := setupShoveHandler()

	h.Handle(makeShoveInteraction("ZZ", ""))

	if len(svc.shoveCalls) != 0 {
		t.Error("expected no shove calls when target missing")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "not found") {
		t.Errorf("expected target-missing rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestShoveHandler_NoEncounter(t *testing.T) {
	h, sess, _, provider := setupShoveHandler()
	provider.resolveErr = errors.New("no encounter")

	h.Handle(makeShoveInteraction("OS", ""))

	if !strings.Contains(sess.lastResponse.Data.Content, "not in an active encounter") {
		t.Errorf("expected encounter rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestShoveHandler_TurnGate_RejectsWrongOwner(t *testing.T) {
	h, sess, svc, _ := setupShoveHandler()
	h.SetTurnGate(&stubTurnGate{err: &combat.ErrNotYourTurn{
		CurrentCharacterName: "Bob",
		CurrentDiscordUserID: "u-bob",
	}})

	h.Handle(makeShoveInteraction("OS", ""))

	if len(svc.shoveCalls) != 0 {
		t.Error("expected no service call when gate rejects")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Bob") {
		t.Errorf("expected wrong-owner rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestShoveHandler_UnknownMode(t *testing.T) {
	h, sess, svc, _ := setupShoveHandler()

	h.Handle(makeShoveInteraction("OS", "yeet"))

	if len(svc.shoveCalls) != 0 || len(svc.grappleCalls) != 0 {
		t.Error("expected no service call for unknown mode")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Unknown mode") {
		t.Errorf("expected unknown-mode rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestShoveHandler_PostsToCombatLog(t *testing.T) {
	h, _, _, _ := setupShoveHandler()
	captured := []string{}
	h.session = &capturingSession{
		mockMoveSession: &mockMoveSession{},
		sendFn: func(channelID, content string) {
			captured = append(captured, channelID+":"+content)
		},
	}
	h.SetChannelIDProvider(&mockDeathSaveCSP{
		fn: func(_ context.Context, _ uuid.UUID) (map[string]string, error) {
			return map[string]string{"combat-log": "ch-cl"}, nil
		},
	})

	h.Handle(makeShoveInteraction("OS", ""))

	if len(captured) != 1 || !strings.HasPrefix(captured[0], "ch-cl:") {
		t.Errorf("expected combat-log post to ch-cl, got %v", captured)
	}
}
