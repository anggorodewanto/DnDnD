package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// fakeEconomyCSP is a CampaignSettingsProvider whose channel map / error are
// fully controllable, so the poster's skip branches can be exercised.
type fakeEconomyCSP struct {
	channels map[string]string
	err      error
}

func (f fakeEconomyCSP) GetChannelIDs(_ context.Context, _ uuid.UUID) (map[string]string, error) {
	return f.channels, f.err
}

// fakeEconomyStore satisfies turnEconomyStore for the poster tests.
type fakeEconomyStore struct {
	combatant refdata.Combatant
	err       error
}

func (f fakeEconomyStore) GetCombatant(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
	return f.combatant, f.err
}

func yourTurnCSP() fakeEconomyCSP {
	return fakeEconomyCSP{channels: map[string]string{"your-turn": "yt-chan"}}
}

func pcCombatant() refdata.Combatant {
	return refdata.Combatant{DisplayName: "Vale", IsNpc: false}
}

// A live PC turn re-posts the remaining-economy line to #your-turn.
func TestTurnEconomyPoster_PostsRemainingForPC(t *testing.T) {
	var sentCh, sentContent string
	sess := &testSession{
		sendFunc: func(channelID, content string) (*discordgo.Message, error) {
			sentCh, sentContent = channelID, content
			return &discordgo.Message{ID: "m1"}, nil
		},
	}
	p := newTurnEconomyPoster(sess, yourTurnCSP(), fakeEconomyStore{combatant: pcCombatant()})
	if p == nil {
		t.Fatal("expected non-nil poster")
	}

	p.postSync(context.Background(), refdata.Turn{MovementRemainingFt: 15, AttacksRemaining: 1})

	if sentCh != "yt-chan" {
		t.Fatalf("posted to wrong channel: %q", sentCh)
	}
	// Reuses combat.FormatRemainingResources so wording matches #combat-log.
	if !strings.Contains(sentContent, "Remaining") || !strings.Contains(sentContent, "15ft") {
		t.Errorf("expected remaining-economy line, got: %s", sentContent)
	}
}

// When the active combatant has spent everything, the poster still posts the
// "all actions spent" nudge.
func TestTurnEconomyPoster_PostsAllSpent(t *testing.T) {
	var sentContent string
	sess := &testSession{
		sendFunc: func(_, content string) (*discordgo.Message, error) {
			sentContent = content
			return &discordgo.Message{}, nil
		},
	}
	p := newTurnEconomyPoster(sess, yourTurnCSP(), fakeEconomyStore{combatant: pcCombatant()})

	// No movement, no attacks, every boolean resource used.
	p.postSync(context.Background(), refdata.Turn{
		BonusActionUsed:  true,
		FreeInteractUsed: true,
		ReactionUsed:     true,
	})

	if !strings.Contains(sentContent, "All actions spent") {
		t.Errorf("expected all-spent nudge, got: %s", sentContent)
	}
}

// NPC economy changes (a monster spending its action on its own turn) stay
// silent — #your-turn is the player channel.
func TestTurnEconomyPoster_SkipsNPC(t *testing.T) {
	sent := 0
	sess := &testSession{
		sendFunc: func(_, _ string) (*discordgo.Message, error) {
			sent++
			return &discordgo.Message{}, nil
		},
	}
	npc := refdata.Combatant{DisplayName: "Ghoul", IsNpc: true}
	p := newTurnEconomyPoster(sess, yourTurnCSP(), fakeEconomyStore{combatant: npc})

	p.postSync(context.Background(), refdata.Turn{MovementRemainingFt: 30})

	if sent != 0 {
		t.Fatalf("NPC economy change must not post, got %d sends", sent)
	}
}

// No #your-turn channel configured → no post (and no panic).
func TestTurnEconomyPoster_NoYourTurnChannel(t *testing.T) {
	sent := 0
	sess := &testSession{sendFunc: func(_, _ string) (*discordgo.Message, error) { sent++; return &discordgo.Message{}, nil }}
	csp := fakeEconomyCSP{channels: map[string]string{}} // no your-turn key
	p := newTurnEconomyPoster(sess, csp, fakeEconomyStore{combatant: pcCombatant()})

	p.postSync(context.Background(), refdata.Turn{MovementRemainingFt: 30})

	if sent != 0 {
		t.Fatalf("missing #your-turn must not post, got %d sends", sent)
	}
}

// A channel-lookup error is swallowed (best-effort).
func TestTurnEconomyPoster_CSPError(t *testing.T) {
	sent := 0
	sess := &testSession{sendFunc: func(_, _ string) (*discordgo.Message, error) { sent++; return &discordgo.Message{}, nil }}
	csp := fakeEconomyCSP{err: errors.New("boom")}
	p := newTurnEconomyPoster(sess, csp, fakeEconomyStore{combatant: pcCombatant()})

	p.postSync(context.Background(), refdata.Turn{MovementRemainingFt: 30})

	if sent != 0 {
		t.Fatalf("csp error must not post, got %d sends", sent)
	}
}

// A combatant-lookup error is swallowed (best-effort).
func TestTurnEconomyPoster_CombatantError(t *testing.T) {
	sent := 0
	sess := &testSession{sendFunc: func(_, _ string) (*discordgo.Message, error) { sent++; return &discordgo.Message{}, nil }}
	p := newTurnEconomyPoster(sess, yourTurnCSP(), fakeEconomyStore{err: errors.New("no combatant")})

	p.postSync(context.Background(), refdata.Turn{MovementRemainingFt: 30})

	if sent != 0 {
		t.Fatalf("combatant lookup error must not post, got %d sends", sent)
	}
}

// Post() detaches and runs asynchronously; the send still lands.
func TestTurnEconomyPoster_Post_Async(t *testing.T) {
	done := make(chan string, 1)
	sess := &testSession{
		sendFunc: func(_, content string) (*discordgo.Message, error) {
			done <- content
			return &discordgo.Message{}, nil
		},
	}
	p := newTurnEconomyPoster(sess, yourTurnCSP(), fakeEconomyStore{combatant: pcCombatant()})

	p.Post(context.Background(), refdata.Turn{MovementRemainingFt: 30, AttacksRemaining: 1})

	select {
	case content := <-done:
		if !strings.Contains(content, "Remaining") {
			t.Errorf("unexpected async content: %s", content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Post did not send asynchronously")
	}
}

func TestNewTurnEconomyPoster_NilDepsReturnNil(t *testing.T) {
	sess := &testSession{}
	csp := yourTurnCSP()
	store := fakeEconomyStore{}
	if newTurnEconomyPoster(nil, csp, store) != nil {
		t.Error("nil session should yield nil poster")
	}
	if newTurnEconomyPoster(sess, nil, store) != nil {
		t.Error("nil csp should yield nil poster")
	}
	if newTurnEconomyPoster(sess, csp, nil) != nil {
		t.Error("nil store should yield nil poster")
	}
}

// --- decorator: combat.Store ---

// stubCombatStore embeds the (nil) combat.Store interface and overrides only
// UpdateTurnActions, so it satisfies the huge interface without implementing
// every method (the decorator only calls UpdateTurnActions).
type stubCombatStore struct {
	combat.Store
	turn refdata.Turn
	err  error
	hits int
}

func (s *stubCombatStore) UpdateTurnActions(_ context.Context, _ refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
	s.hits++
	return s.turn, s.err
}

func TestEconomyNotifyingStore_PostsOnSuccess(t *testing.T) {
	want := refdata.Turn{ID: uuid.New(), MovementRemainingFt: 10}
	inner := &stubCombatStore{turn: want}
	var posted *refdata.Turn
	store := newEconomyNotifyingStore(inner, func(_ context.Context, tn refdata.Turn) { posted = &tn })

	got, err := store.UpdateTurnActions(context.Background(), refdata.UpdateTurnActionsParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("turn not passed through: got %v want %v", got.ID, want.ID)
	}
	if posted == nil || posted.ID != want.ID {
		t.Errorf("post not fired with persisted turn")
	}
}

func TestEconomyNotifyingStore_SkipsOnError(t *testing.T) {
	inner := &stubCombatStore{err: errors.New("db down")}
	posted := false
	store := newEconomyNotifyingStore(inner, func(context.Context, refdata.Turn) { posted = true })

	if _, err := store.UpdateTurnActions(context.Background(), refdata.UpdateTurnActionsParams{}); err == nil {
		t.Fatal("expected error to propagate")
	}
	if posted {
		t.Error("post must not fire when persistence fails")
	}
}

func TestNewEconomyNotifyingStore_NilPassThrough(t *testing.T) {
	inner := &stubCombatStore{}
	if newEconomyNotifyingStore(inner, nil) != combat.Store(inner) {
		t.Error("nil post should return the inner store unwrapped")
	}
	if newEconomyNotifyingStore(nil, func(context.Context, refdata.Turn) {}) != nil {
		t.Error("nil inner should return nil")
	}
}

// --- decorator: move/fly/interact turn provider ---

type stubTurnStore struct {
	turn       refdata.Turn
	fetched    refdata.Turn
	updateErr  error
	updateHits int
}

func (s *stubTurnStore) GetTurn(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
	return s.fetched, nil
}

func (s *stubTurnStore) UpdateTurnActions(_ context.Context, _ refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
	s.updateHits++
	return s.turn, s.updateErr
}

func TestEconomyTurnProvider_PostsOnSuccess(t *testing.T) {
	want := refdata.Turn{ID: uuid.New()}
	inner := &stubTurnStore{turn: want, fetched: refdata.Turn{ID: uuid.New()}}
	posted := false
	p := newEconomyTurnProvider(inner, func(context.Context, refdata.Turn) { posted = true })

	// GetTurn delegates straight through.
	if _, err := p.GetTurn(context.Background(), uuid.New()); err != nil {
		t.Fatalf("GetTurn error: %v", err)
	}

	got, err := p.UpdateTurnActions(context.Background(), refdata.UpdateTurnActionsParams{})
	if err != nil {
		t.Fatalf("UpdateTurnActions error: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("turn not passed through")
	}
	if !posted {
		t.Error("post not fired on success")
	}
}

func TestEconomyTurnProvider_SkipsOnError(t *testing.T) {
	inner := &stubTurnStore{updateErr: errors.New("boom")}
	posted := false
	p := newEconomyTurnProvider(inner, func(context.Context, refdata.Turn) { posted = true })

	if _, err := p.UpdateTurnActions(context.Background(), refdata.UpdateTurnActionsParams{}); err == nil {
		t.Fatal("expected error")
	}
	if posted {
		t.Error("post must not fire on error")
	}
}

// --- decorator: use/give provider ---

type stubUseGiveProvider struct {
	turn      refdata.Turn
	inCombat  bool
	updateErr error
}

func (s *stubUseGiveProvider) GetActiveTurnForCharacter(_ context.Context, _ string, _ uuid.UUID) (refdata.Turn, bool, error) {
	return s.turn, s.inCombat, nil
}

func (s *stubUseGiveProvider) UpdateTurnActions(_ context.Context, _ refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
	return s.turn, s.updateErr
}

func TestEconomyUseGiveProvider_PostsOnSuccess(t *testing.T) {
	want := refdata.Turn{ID: uuid.New()}
	inner := &stubUseGiveProvider{turn: want, inCombat: true}
	posted := false
	p := &economyUseGiveProvider{UseCombatProvider: inner, post: func(context.Context, refdata.Turn) { posted = true }}

	// Delegation of the lookup method.
	if _, ok, _ := p.GetActiveTurnForCharacter(context.Background(), "g", uuid.New()); !ok {
		t.Error("GetActiveTurnForCharacter should delegate")
	}

	if _, err := p.UpdateTurnActions(context.Background(), refdata.UpdateTurnActionsParams{}); err != nil {
		t.Fatalf("UpdateTurnActions error: %v", err)
	}
	if !posted {
		t.Error("post not fired on success")
	}
}

func TestEconomyUseGiveProvider_SkipsOnError(t *testing.T) {
	inner := &stubUseGiveProvider{updateErr: errors.New("boom")}
	posted := false
	p := &economyUseGiveProvider{UseCombatProvider: inner, post: func(context.Context, refdata.Turn) { posted = true }}

	if _, err := p.UpdateTurnActions(context.Background(), refdata.UpdateTurnActionsParams{}); err == nil {
		t.Fatal("expected error")
	}
	if posted {
		t.Error("post must not fire on error")
	}
}
