package discord

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// --- Mocks shared by /deathsave + /interact + /attack tests ---

type mockEncounterResolver struct {
	fn func(ctx context.Context, guildID, userID string) (uuid.UUID, error)
}

func (m *mockEncounterResolver) ActiveEncounterForUser(ctx context.Context, guildID, userID string) (uuid.UUID, error) {
	return m.fn(ctx, guildID, userID)
}

type mockDeathSaveStore struct {
	updatedDS  *refdata.UpdateCombatantDeathSavesParams
	updatedHP  *refdata.UpdateCombatantHPParams
	dsError    error
	hpError    error
}

func (m *mockDeathSaveStore) UpdateCombatantDeathSaves(_ context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error) {
	if m.dsError != nil {
		return refdata.Combatant{}, m.dsError
	}
	m.updatedDS = &arg
	return refdata.Combatant{ID: arg.ID, DeathSaves: arg.DeathSaves}, nil
}

func (m *mockDeathSaveStore) UpdateCombatantHP(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
	if m.hpError != nil {
		return refdata.Combatant{}, m.hpError
	}
	m.updatedHP = &arg
	return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
}

type mockDeathSaveCSP struct {
	fn         func(ctx context.Context, encounterID uuid.UUID) (map[string]string, error)
}

func (m *mockDeathSaveCSP) GetChannelIDs(ctx context.Context, encounterID uuid.UUID) (map[string]string, error) {
	return m.fn(ctx, encounterID)
}

// --- Helpers ---

func makeDeathSaveInteraction() *discordgo.Interaction {
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "g1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "u1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "deathsave",
		},
	}
}

func setupDeathSaveHandler(t *testing.T, dyingCombatant refdata.Combatant) (*DeathSaveHandler, *mockMoveSession, *mockDeathSaveStore, uuid.UUID) {
	t.Helper()
	encID := uuid.New()
	charID := dyingCombatant.CharacterID.UUID

	sess := &mockMoveSession{}
	resolver := &mockEncounterResolver{
		fn: func(_ context.Context, _ string, _ string) (uuid.UUID, error) {
			return encID, nil
		},
	}
	combLookup := &mockCheckCombatantLookup{
		listFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{dyingCombatant}, nil
		},
	}
	store := &mockDeathSaveStore{}
	campProv := &mockCheckCampaignProvider{
		fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: uuid.New()}, nil
		},
	}
	charLookup := &mockCheckCharacterLookup{
		fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return refdata.Character{ID: charID}, nil
		},
	}
	roller := dice.NewRoller(func(_ int) int { return 14 }) // deterministic 14
	h := NewDeathSaveHandler(sess, roller, resolver, combLookup, store, campProv, charLookup)
	return h, sess, store, encID
}

func dyingCombatantWithSaves(t *testing.T, successes, failures int) refdata.Combatant {
	t.Helper()
	dsBytes, _ := json.Marshal(map[string]int{"successes": successes, "failures": failures})
	return refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		DisplayName: "Aria",
		HpCurrent:   0,
		IsAlive:     true,
		DeathSaves:  pqtype.NullRawMessage{RawMessage: dsBytes, Valid: true},
	}
}

// --- Tests ---

func TestDeathSaveHandler_RollsAndPersistsSuccess(t *testing.T) {
	combatant := dyingCombatantWithSaves(t, 0, 0)
	h, sess, store, _ := setupDeathSaveHandler(t, combatant)

	h.Handle(makeDeathSaveInteraction())

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Success") {
		t.Errorf("expected success message, got: %q", sess.lastResponse.Data.Content)
	}
	if store.updatedDS == nil {
		t.Fatal("expected death saves to be persisted")
	}
	// roll=14 with 0 prior successes -> 1 success, 0 failures
	var ds map[string]int
	if err := json.Unmarshal(store.updatedDS.DeathSaves.RawMessage, &ds); err != nil {
		t.Fatalf("unmarshal updated DS: %v", err)
	}
	if ds["successes"] != 1 {
		t.Errorf("expected 1 success, got %d", ds["successes"])
	}
}

func TestDeathSaveHandler_NotDying(t *testing.T) {
	combatant := refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		DisplayName: "Aria",
		HpCurrent:   10, // alive and standing
		IsAlive:     true,
	}
	h, sess, store, _ := setupDeathSaveHandler(t, combatant)

	h.Handle(makeDeathSaveInteraction())

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "not dying") {
		t.Errorf("expected 'not dying' rejection, got: %q", sess.lastResponse.Data.Content)
	}
	if store.updatedDS != nil || store.updatedHP != nil {
		t.Error("expected no persistence when combatant is not dying")
	}
}

func TestDeathSaveHandler_NoActiveEncounter(t *testing.T) {
	combatant := dyingCombatantWithSaves(t, 0, 0)
	h, sess, _, _ := setupDeathSaveHandler(t, combatant)
	h.resolver = &mockEncounterResolver{
		fn: func(_ context.Context, _ string, _ string) (uuid.UUID, error) {
			return uuid.Nil, errNoEncounter
		},
	}

	h.Handle(makeDeathSaveInteraction())

	if !strings.Contains(sess.lastResponse.Data.Content, "not in an active encounter") {
		t.Errorf("expected encounter rejection, got: %q", sess.lastResponse.Data.Content)
	}
}

func TestDeathSaveHandler_PostsToCombatLog(t *testing.T) {
	combatant := dyingCombatantWithSaves(t, 0, 0)
	h, sess, _, encID := setupDeathSaveHandler(t, combatant)
	h.SetChannelIDProvider(&mockDeathSaveCSP{
		fn: func(_ context.Context, gotEnc uuid.UUID) (map[string]string, error) {
			if gotEnc != encID {
				t.Errorf("expected encounter %s, got %s", encID, gotEnc)
			}
			return map[string]string{"combat-log": "ch-1"}, nil
		},
	})
	// override session to capture ChannelMessageSend
	sentChannels := []string{}
	sess.lastResponse = nil
	h.session = &capturingSession{
		mockMoveSession: sess,
		sendFn: func(channelID, _ string) {
			sentChannels = append(sentChannels, channelID)
		},
	}

	h.Handle(makeDeathSaveInteraction())

	if len(sentChannels) != 1 || sentChannels[0] != "ch-1" {
		t.Errorf("expected combat-log post to ch-1, got %v", sentChannels)
	}
}

// capturingSession wraps mockMoveSession to capture ChannelMessageSend calls.
type capturingSession struct {
	*mockMoveSession
	sendFn func(channelID, content string)
}

func (c *capturingSession) ChannelMessageSend(channelID, content string) (*discordgo.Message, error) {
	if c.sendFn != nil {
		c.sendFn(channelID, content)
	}
	return nil, nil
}

func TestDeathSaveHandler_Nat20HealsToOneHP(t *testing.T) {
	combatant := dyingCombatantWithSaves(t, 1, 2)
	h, sess, store, _ := setupDeathSaveHandler(t, combatant)
	// Override roller to deterministic 20.
	h.roller = dice.NewRoller(func(_ int) int { return 20 })

	h.Handle(makeDeathSaveInteraction())

	if !strings.Contains(sess.lastResponse.Data.Content, "NAT 20") {
		t.Errorf("expected NAT 20 message, got %q", sess.lastResponse.Data.Content)
	}
	if store.updatedHP == nil || store.updatedHP.HpCurrent != 1 {
		t.Errorf("expected HP=1 update, got %+v", store.updatedHP)
	}
	// Reviewer fix: nat-20 must reset the death-save tally so the next
	// drop-to-0 starts fresh. Otherwise the prior 1S/2F survives the heal.
	if store.updatedDS == nil {
		t.Fatalf("expected death-save reset write on nat-20, got nil")
	}
	reset, err := combat.ParseDeathSaves(store.updatedDS.DeathSaves.RawMessage)
	if err != nil {
		t.Fatalf("parse reset death saves: %v", err)
	}
	if reset.Successes != 0 || reset.Failures != 0 {
		t.Errorf("expected death saves reset to 0/0, got %dS/%dF", reset.Successes, reset.Failures)
	}
}

func TestDeathSaveHandler_ThreeFailuresKills(t *testing.T) {
	combatant := dyingCombatantWithSaves(t, 0, 2)
	h, sess, store, _ := setupDeathSaveHandler(t, combatant)
	// roll=5 -> failure -> 3rd failure -> dead
	h.roller = dice.NewRoller(func(_ int) int { return 5 })

	h.Handle(makeDeathSaveInteraction())

	if !strings.Contains(sess.lastResponse.Data.Content, "dead") {
		t.Errorf("expected dead outcome, got %q", sess.lastResponse.Data.Content)
	}
	if store.updatedHP == nil || store.updatedHP.IsAlive {
		t.Errorf("expected IsAlive=false update, got %+v", store.updatedHP)
	}
}

func TestDeathSaveHandler_NoCharLookup(t *testing.T) {
	combatant := dyingCombatantWithSaves(t, 0, 0)
	h, sess, _, _ := setupDeathSaveHandler(t, combatant)
	h.characterLookup = nil
	h.Handle(makeDeathSaveInteraction())
	if !strings.Contains(sess.lastResponse.Data.Content, "Could not find your character") {
		t.Errorf("expected character lookup failure, got %q", sess.lastResponse.Data.Content)
	}
}
