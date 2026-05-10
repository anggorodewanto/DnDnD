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

// --- Mocks for /attack ---

type mockAttackCombatService struct {
	attackCalls  []combat.AttackCommand
	offhandCalls []combat.OffhandAttackCommand
	attackResult combat.AttackResult
	offhandResult combat.AttackResult
	attackErr    error
	offhandErr   error
}

func (m *mockAttackCombatService) Attack(_ context.Context, cmd combat.AttackCommand, _ *dice.Roller) (combat.AttackResult, error) {
	m.attackCalls = append(m.attackCalls, cmd)
	return m.attackResult, m.attackErr
}

func (m *mockAttackCombatService) OffhandAttack(_ context.Context, cmd combat.OffhandAttackCommand, _ *dice.Roller) (combat.AttackResult, error) {
	m.offhandCalls = append(m.offhandCalls, cmd)
	return m.offhandResult, m.offhandErr
}

type mockAttackProvider struct {
	encID      uuid.UUID
	turn       refdata.Turn
	attacker   refdata.Combatant
	target     refdata.Combatant
	enc        refdata.Encounter
	resolveErr error
	getEncErr  error
	getTurnErr error
	getCombErr error
	listErr    error
}

func (m *mockAttackProvider) ActiveEncounterForUser(_ context.Context, _, _ string) (uuid.UUID, error) {
	if m.resolveErr != nil {
		return uuid.Nil, m.resolveErr
	}
	return m.encID, nil
}

func (m *mockAttackProvider) GetEncounter(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
	if m.getEncErr != nil {
		return refdata.Encounter{}, m.getEncErr
	}
	return m.enc, nil
}

func (m *mockAttackProvider) GetCombatant(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
	if m.getCombErr != nil {
		return refdata.Combatant{}, m.getCombErr
	}
	if id == m.attacker.ID {
		return m.attacker, nil
	}
	return m.target, nil
}

func (m *mockAttackProvider) ListCombatantsByEncounterID(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return []refdata.Combatant{m.attacker, m.target}, nil
}

func (m *mockAttackProvider) GetTurn(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
	if m.getTurnErr != nil {
		return refdata.Turn{}, m.getTurnErr
	}
	return m.turn, nil
}

// --- Helpers ---

func makeAttackInteraction(opts map[string]any) *discordgo.Interaction {
	cmdOpts := []*discordgo.ApplicationCommandInteractionDataOption{}
	for name, val := range opts {
		switch v := val.(type) {
		case string:
			cmdOpts = append(cmdOpts, &discordgo.ApplicationCommandInteractionDataOption{
				Name: name, Value: v, Type: discordgo.ApplicationCommandOptionString,
			})
		case bool:
			cmdOpts = append(cmdOpts, &discordgo.ApplicationCommandInteractionDataOption{
				Name: name, Value: v, Type: discordgo.ApplicationCommandOptionBoolean,
			})
		}
	}
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "g1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "u1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "attack",
			Options: cmdOpts,
		},
	}
}

func setupAttackHandler() (*AttackHandler, *mockMoveSession, *mockAttackCombatService, *mockAttackProvider) {
	encID := uuid.New()
	turnID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()

	provider := &mockAttackProvider{
		encID: encID,
		enc: refdata.Encounter{
			ID:            encID,
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
		},
		turn: refdata.Turn{
			ID:               turnID,
			CombatantID:      attackerID,
			AttacksRemaining: 1,
		},
		attacker: refdata.Combatant{
			ID: attackerID, ShortID: "AR", DisplayName: "Aria",
			PositionCol: "A", PositionRow: 1,
		},
		target: refdata.Combatant{
			ID: targetID, ShortID: "OS", DisplayName: "Orc",
			PositionCol: "B", PositionRow: 1,
		},
	}
	combatSvc := &mockAttackCombatService{
		attackResult: combat.AttackResult{
			AttackerName: "Aria",
			TargetName:   "Orc",
			WeaponName:   "longsword",
			Hit:          true,
			IsMelee:      true,
			DistanceFt:   5,
		},
		offhandResult: combat.AttackResult{
			AttackerName: "Aria",
			TargetName:   "Orc",
			WeaponName:   "shortsword",
			Hit:          true,
			IsMelee:      true,
			DistanceFt:   5,
		},
	}
	sess := &mockMoveSession{}
	h := NewAttackHandler(sess, combatSvc, provider, dice.NewRoller(func(_ int) int { return 10 }))
	return h, sess, combatSvc, provider
}

// --- Tests ---

func TestAttackHandler_DispatchesAttackWithFlags(t *testing.T) {
	h, sess, svc, _ := setupAttackHandler()

	h.Handle(makeAttackInteraction(map[string]any{
		"target":       "OS",
		"weapon":       "greatsword",
		"gwm":          true,
		"sharpshooter": false,
		"reckless":     true,
		"twohanded":    true,
	}))

	if len(svc.attackCalls) != 1 {
		t.Fatalf("expected 1 attack call, got %d", len(svc.attackCalls))
	}
	got := svc.attackCalls[0]
	if got.WeaponOverride != "greatsword" {
		t.Errorf("expected weapon override 'greatsword', got %q", got.WeaponOverride)
	}
	if !got.GWM {
		t.Error("expected GWM=true")
	}
	if got.Sharpshooter {
		t.Error("expected Sharpshooter=false")
	}
	if !got.Reckless {
		t.Error("expected Reckless=true")
	}
	if !got.TwoHanded {
		t.Error("expected TwoHanded=true")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "attacks") {
		t.Errorf("expected attack log, got %q", sess.lastResponse.Data.Content)
	}
}

func TestAttackHandler_OffhandRoutesToOffhandService(t *testing.T) {
	h, _, svc, _ := setupAttackHandler()

	h.Handle(makeAttackInteraction(map[string]any{
		"target":  "OS",
		"offhand": true,
	}))

	if len(svc.offhandCalls) != 1 {
		t.Fatalf("expected 1 offhand call, got %d", len(svc.offhandCalls))
	}
	if len(svc.attackCalls) != 0 {
		t.Errorf("expected no main-hand calls when offhand=true, got %d", len(svc.attackCalls))
	}
}

func TestAttackHandler_TargetNotFound(t *testing.T) {
	h, sess, svc, _ := setupAttackHandler()

	h.Handle(makeAttackInteraction(map[string]any{"target": "ZZ"}))

	if len(svc.attackCalls) != 0 {
		t.Error("expected no attack call when target missing")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "not found") {
		t.Errorf("expected target-missing rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestAttackHandler_NoActiveEncounter(t *testing.T) {
	h, sess, _, provider := setupAttackHandler()
	provider.resolveErr = errors.New("no encounter")

	h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))

	if !strings.Contains(sess.lastResponse.Data.Content, "not in an active encounter") {
		t.Errorf("expected encounter rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestAttackHandler_TurnGate_RejectsWrongOwner(t *testing.T) {
	h, sess, svc, _ := setupAttackHandler()
	h.SetTurnGate(&stubTurnGate{err: &combat.ErrNotYourTurn{
		CurrentCharacterName: "Bob",
		CurrentDiscordUserID: "u-bob",
	}})

	h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))

	if len(svc.attackCalls) != 0 {
		t.Error("expected no service call when gate rejects")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Bob") {
		t.Errorf("expected wrong-owner rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestAttackHandler_PostsToCombatLog(t *testing.T) {
	h, _, _, _ := setupAttackHandler()
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

	h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))

	if len(captured) != 1 || !strings.HasPrefix(captured[0], "ch-cl:") {
		t.Errorf("expected combat-log post to ch-cl, got %v", captured)
	}
}

func TestAttackHandler_NoTarget(t *testing.T) {
	h, sess, _, _ := setupAttackHandler()

	h.Handle(makeAttackInteraction(map[string]any{}))

	if !strings.Contains(sess.lastResponse.Data.Content, "specify a target") {
		t.Errorf("expected target prompt, got %q", sess.lastResponse.Data.Content)
	}
}

func TestAttackHandler_ServiceError(t *testing.T) {
	h, sess, svc, _ := setupAttackHandler()
	svc.attackErr = errors.New("out of attacks")

	h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))

	if !strings.Contains(sess.lastResponse.Data.Content, "Attack failed") {
		t.Errorf("expected service-error rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestAttackHandler_OffhandServiceError(t *testing.T) {
	h, sess, svc, _ := setupAttackHandler()
	svc.offhandErr = errors.New("no off-hand")

	h.Handle(makeAttackInteraction(map[string]any{"target": "OS", "offhand": true}))

	if !strings.Contains(sess.lastResponse.Data.Content, "Off-hand attack failed") {
		t.Errorf("expected offhand-error rejection, got %q", sess.lastResponse.Data.Content)
	}
}

func TestAttackHandler_GetEncounterError(t *testing.T) {
	h, sess, _, provider := setupAttackHandler()
	provider.getEncErr = errors.New("boom")
	h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))
	if !strings.Contains(sess.lastResponse.Data.Content, "load encounter") {
		t.Errorf("expected load-encounter error, got %q", sess.lastResponse.Data.Content)
	}
}

func TestAttackHandler_NoCurrentTurn(t *testing.T) {
	h, sess, _, provider := setupAttackHandler()
	provider.enc.CurrentTurnID = uuid.NullUUID{Valid: false}
	h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))
	if !strings.Contains(sess.lastResponse.Data.Content, "No active turn") {
		t.Errorf("expected no-active-turn error, got %q", sess.lastResponse.Data.Content)
	}
}

func TestAttackHandler_GetTurnError(t *testing.T) {
	h, sess, _, provider := setupAttackHandler()
	provider.getTurnErr = errors.New("boom")
	h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))
	if !strings.Contains(sess.lastResponse.Data.Content, "load turn") {
		t.Errorf("expected load-turn error, got %q", sess.lastResponse.Data.Content)
	}
}

func TestAttackHandler_ListCombatantsError(t *testing.T) {
	h, sess, _, provider := setupAttackHandler()
	provider.listErr = errors.New("boom")
	h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))
	if !strings.Contains(sess.lastResponse.Data.Content, "list combatants") {
		t.Errorf("expected list-combatants error, got %q", sess.lastResponse.Data.Content)
	}
}
