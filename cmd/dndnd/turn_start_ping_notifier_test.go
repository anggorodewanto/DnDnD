package main

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

// fakeTurnStartStore satisfies turnStartPingStore for the notifier tests.
type fakeTurnStartStore struct {
	combatant refdata.Combatant
	encounter refdata.Encounter
	pc        refdata.PlayerCharacter
	pcErr     error
}

func (f *fakeTurnStartStore) GetCombatant(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
	return f.combatant, nil
}

func (f *fakeTurnStartStore) GetEncounter(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
	return f.encounter, nil
}

func (f *fakeTurnStartStore) GetPlayerCharacterByCharacter(_ context.Context, _ refdata.GetPlayerCharacterByCharacterParams) (refdata.PlayerCharacter, error) {
	return f.pc, f.pcErr
}

func turnStartCSP() fakeTrackerCSP {
	return fakeTrackerCSP{channels: map[string]string{"your-turn": "yt-chan"}}
}

// A linked PC gets a real <@id> mention and the "since your last turn" impact
// line, posted to #your-turn.
func TestTurnStartPingNotifier_PostsMentionAndImpactForPC(t *testing.T) {
	charID := uuid.New()
	var sentCh, sentContent string
	sess := &testSession{
		sendFunc: func(channelID, content string) (*discordgo.Message, error) {
			sentCh, sentContent = channelID, content
			return &discordgo.Message{ID: "m1"}, nil
		},
	}
	store := &fakeTurnStartStore{
		combatant: refdata.Combatant{
			DisplayName: "Vale",
			IsNpc:       false,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		},
		encounter: refdata.Encounter{Name: "Crypt", CampaignID: uuid.New()},
		pc:        refdata.PlayerCharacter{DiscordUserID: "user-1"},
	}

	n := newTurnStartPingNotifier(sess, turnStartCSP(), store)
	if n == nil {
		t.Fatal("expected non-nil notifier")
	}
	n.NotifyTurnStart(context.Background(), uuid.New(),
		combat.TurnInfo{RoundNumber: 2}, "⚠️ Since your last turn: took 5 damage.")

	if sentCh != "yt-chan" {
		t.Fatalf("posted to wrong channel: %q", sentCh)
	}
	if !strings.Contains(sentContent, "<@user-1>") {
		t.Errorf("expected real <@id> mention, got: %s", sentContent)
	}
	if !strings.Contains(sentContent, "Since your last turn: took 5 damage") {
		t.Errorf("expected impact summary line, got: %s", sentContent)
	}
}

// An NPC (or unlinked combatant) keeps a plain @name ping — no real mention.
func TestTurnStartPingNotifier_NPCKeepsPlainName(t *testing.T) {
	var sentContent string
	sess := &testSession{
		sendFunc: func(_, content string) (*discordgo.Message, error) {
			sentContent = content
			return &discordgo.Message{ID: "m1"}, nil
		},
	}
	store := &fakeTurnStartStore{
		combatant: refdata.Combatant{DisplayName: "Ghoul", IsNpc: true},
		encounter: refdata.Encounter{Name: "Crypt", CampaignID: uuid.New()},
	}

	n := newTurnStartPingNotifier(sess, turnStartCSP(), store)
	n.NotifyTurnStart(context.Background(), uuid.New(), combat.TurnInfo{RoundNumber: 1}, "")

	if strings.Contains(sentContent, "<@") {
		t.Errorf("NPC must not get a real mention, got: %s", sentContent)
	}
	if !strings.Contains(sentContent, "@Ghoul") {
		t.Errorf("expected plain @name ping for NPC, got: %s", sentContent)
	}
}

// A failed player-character lookup falls back to the plain-name ping.
func TestTurnStartPingNotifier_LookupErrorKeepsPlainName(t *testing.T) {
	var sentContent string
	sess := &testSession{
		sendFunc: func(_, content string) (*discordgo.Message, error) {
			sentContent = content
			return &discordgo.Message{ID: "m1"}, nil
		},
	}
	store := &fakeTurnStartStore{
		combatant: refdata.Combatant{
			DisplayName: "Vale",
			IsNpc:       false,
			CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		},
		encounter: refdata.Encounter{Name: "Crypt", CampaignID: uuid.New()},
		pcErr:     errors.New("not found"),
	}

	n := newTurnStartPingNotifier(sess, turnStartCSP(), store)
	n.NotifyTurnStart(context.Background(), uuid.New(), combat.TurnInfo{RoundNumber: 1}, "")

	if strings.Contains(sentContent, "<@") {
		t.Errorf("lookup error must fall back to plain name, got: %s", sentContent)
	}
}

// No #your-turn channel configured → no post (and no panic).
func TestTurnStartPingNotifier_NoYourTurnChannel_NoPost(t *testing.T) {
	sent := 0
	sess := &testSession{
		sendFunc: func(_, _ string) (*discordgo.Message, error) {
			sent++
			return &discordgo.Message{}, nil
		},
	}
	store := &fakeTurnStartStore{
		combatant: refdata.Combatant{DisplayName: "Vale"},
		encounter: refdata.Encounter{Name: "Crypt"},
	}
	csp := fakeTrackerCSP{channels: map[string]string{}} // no your-turn key

	n := newTurnStartPingNotifier(sess, csp, store)
	n.NotifyTurnStart(context.Background(), uuid.New(), combat.TurnInfo{RoundNumber: 1}, "")

	if sent != 0 {
		t.Fatalf("expected no post when #your-turn is unconfigured, got %d", sent)
	}
}

func TestNewTurnStartPingNotifier_NilDepsReturnNil(t *testing.T) {
	sess := &testSession{}
	csp := turnStartCSP()
	store := &fakeTurnStartStore{}
	if newTurnStartPingNotifier(nil, csp, store) != nil {
		t.Error("nil session should yield nil notifier")
	}
	if newTurnStartPingNotifier(sess, nil, store) != nil {
		t.Error("nil csp should yield nil notifier")
	}
	if newTurnStartPingNotifier(sess, csp, nil) != nil {
		t.Error("nil store should yield nil notifier")
	}
}
