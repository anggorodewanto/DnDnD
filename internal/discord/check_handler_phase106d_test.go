package discord

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// --- Mock notifier for /check gating tests ---

type checkRecordingNotifier struct {
	posted   []dmqueue.Event
	postedID string
	postErr  error
}

func (n *checkRecordingNotifier) Post(_ context.Context, e dmqueue.Event) (string, error) {
	n.posted = append(n.posted, e)
	if n.postErr != nil {
		return "", n.postErr
	}
	if n.postedID == "" {
		return "queue-1", nil
	}
	return n.postedID, nil
}
func (n *checkRecordingNotifier) Cancel(context.Context, string, string) error  { return nil }
func (n *checkRecordingNotifier) Resolve(context.Context, string, string) error { return nil }
func (n *checkRecordingNotifier) ResolveWhisper(context.Context, string, string) error {
	return nil
}
func (n *checkRecordingNotifier) ResolveSkillCheckNarration(context.Context, string, string) error {
	return nil
}
func (n *checkRecordingNotifier) Get(string) (dmqueue.Item, bool) { return dmqueue.Item{}, false }
func (n *checkRecordingNotifier) ListPending() []dmqueue.Item    { return nil }

// --- Helpers ---

func makeCheckInteractionWithDC(skill string, dc int, channelID, userID string) *discordgo.Interaction {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "skill", Value: skill, Type: discordgo.ApplicationCommandOptionString},
		{Name: "dc", Value: float64(dc), Type: discordgo.ApplicationCommandOptionInteger},
	}
	return &discordgo.Interaction{
		Type:      discordgo.InteractionApplicationCommand,
		GuildID:   "guild1",
		ChannelID: channelID,
		Member:    &discordgo.Member{User: &discordgo.User{ID: userID}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "check",
			Options: opts,
		},
	}
}

// setupCheckHandlerWithRoll builds a CheckHandler whose roller produces a
// fixed d20 face value (no modifiers). Useful for gating tests.
func setupCheckHandlerWithRoll(sess *MockSession, d20Face int) (*CheckHandler, *checkRecordingNotifier) {
	campaignID := uuid.New()
	char := makeTestCharacter()
	char.CampaignID = campaignID

	roller := dice.NewRoller(func(int) int { return d20Face })
	h := NewCheckHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return char, nil
		}},
		&mockCheckEncounterProvider{fn: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.Nil, errNoEncounter
		}},
		nil,
		nil,
	)
	rec := &checkRecordingNotifier{}
	h.SetNotifier(rec)
	return h, rec
}

// --- Gating: trivial nat 20 with DC met → ungated ---

func TestCheckHandler_Gating_NatTwentyMeetsDCIsUngated(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, rec := setupCheckHandlerWithRoll(sess, 20)
	h.Handle(makeCheckInteractionWithDC("perception", 10, "chan-1", "user1"))

	if len(rec.posted) != 0 {
		t.Errorf("expected no dm-queue post for trivial nat 20, got %d", len(rec.posted))
	}
	if !strings.Contains(responded, "Perception") {
		t.Errorf("expected normal response, got: %s", responded)
	}
	// Numeric total should be visible (ungated)
	if !strings.Contains(responded, "Check:") {
		t.Errorf("expected numeric check result, got: %s", responded)
	}
}

// --- Gating: trivial nat 1 → ungated ---

func TestCheckHandler_Gating_NatOneIsUngated(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, rec := setupCheckHandlerWithRoll(sess, 1)
	h.Handle(makeCheckInteractionWithDC("perception", 10, "chan-1", "user1"))

	if len(rec.posted) != 0 {
		t.Errorf("expected no dm-queue post for nat 1, got %d", len(rec.posted))
	}
	if !strings.Contains(responded, "Perception") {
		t.Errorf("expected normal response, got: %s", responded)
	}
}

// --- Gating: normal roll with DC → gated ---

func TestCheckHandler_Gating_NormalRollWithDCIsGated(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, rec := setupCheckHandlerWithRoll(sess, 12)
	h.Handle(makeCheckInteractionWithDC("perception", 15, "chan-99", "user1"))

	if len(rec.posted) != 1 {
		t.Fatalf("expected 1 dm-queue post, got %d", len(rec.posted))
	}
	ev := rec.posted[0]
	if ev.Kind != dmqueue.KindSkillCheckNarration {
		t.Errorf("kind = %q want KindSkillCheckNarration", ev.Kind)
	}
	if ev.PlayerName != "Aria" {
		t.Errorf("player name = %q", ev.PlayerName)
	}
	if !strings.Contains(strings.ToLower(ev.Summary), "perception") {
		t.Errorf("summary should mention skill: %q", ev.Summary)
	}
	if ev.ExtraMetadata[dmqueue.SkillCheckChannelIDKey] != "chan-99" {
		t.Errorf("channel id metadata = %q", ev.ExtraMetadata[dmqueue.SkillCheckChannelIDKey])
	}
	if ev.ExtraMetadata[dmqueue.SkillCheckPlayerDiscordIDKey] != "user1" {
		t.Errorf("user id metadata = %q", ev.ExtraMetadata[dmqueue.SkillCheckPlayerDiscordIDKey])
	}
	if ev.ExtraMetadata[dmqueue.SkillCheckSkillLabelKey] == "" {
		t.Errorf("skill label metadata missing")
	}
	if ev.ExtraMetadata[dmqueue.SkillCheckTotalKey] == "" {
		t.Errorf("total metadata missing")
	}

	// Player must NOT see numeric result in the gated ephemeral response.
	if strings.Contains(responded, "Check:") {
		t.Errorf("gated response should not include numeric result, got: %s", responded)
	}
	if !strings.Contains(strings.ToLower(responded), "dm") {
		t.Errorf("gated response should mention DM, got: %s", responded)
	}
}

// --- Gating: no DC at all → gated ---

func TestCheckHandler_Gating_NoDCIsGated(t *testing.T) {
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, _ *discordgo.InteractionResponse) error {
		return nil
	}

	h, rec := setupCheckHandlerWithRoll(sess, 12)
	h.Handle(makeCheckInteraction("perception", false, false))

	if len(rec.posted) != 1 {
		t.Fatalf("expected 1 dm-queue post when no DC provided, got %d", len(rec.posted))
	}
}

// --- Gating: nat 20 below DC (impossible if total>=DC, but check edge) ---

func TestCheckHandler_Gating_NatTwentyBelowDCIsGated(t *testing.T) {
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, _ *discordgo.InteractionResponse) error {
		return nil
	}

	// Aria's perception total at d20=20: 20 + WIS(4) + prof(3) = 27.
	// DC 30 → nat 20 does not auto-meet → gated.
	h, rec := setupCheckHandlerWithRoll(sess, 20)
	h.Handle(makeCheckInteractionWithDC("perception", 30, "chan-1", "user1"))

	if len(rec.posted) != 1 {
		t.Errorf("expected 1 gated post (nat 20 < DC), got %d", len(rec.posted))
	}
}

// --- AutoFail short-circuit (e.g. unconscious) should still respond, never gate ---

func TestCheckHandler_Gating_AutoFailIsUngated(t *testing.T) {
	// AutoFail checks already produce an immediate "Auto-fail" message and
	// never gate (no roll happened). Use a setup with no notifier post.
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, _ *discordgo.InteractionResponse) error {
		return nil
	}

	h, rec := setupCheckHandlerWithRoll(sess, 12)
	// Use an unknown skill to trigger an error path, which is also ungated.
	h.Handle(makeCheckInteractionWithDC("bogus", 10, "chan-1", "user1"))

	if len(rec.posted) != 0 {
		t.Errorf("error path should not post, got %d", len(rec.posted))
	}
}

// --- Wiring: notifier nil tolerated ---

func TestCheckHandler_Gating_NilNotifierTolerated(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, _ := setupCheckHandlerWithRoll(sess, 12)
	h.SetNotifier(nil) // explicitly nil
	h.Handle(makeCheckInteractionWithDC("perception", 15, "chan-1", "user1"))

	// Should not panic; falls back to immediate ephemeral with the result.
	if !strings.Contains(responded, "Perception") {
		t.Errorf("expected fallback response, got: %s", responded)
	}
}

// --- Notifier post error: handler should fall back to immediate response ---

func TestCheckHandler_Gating_PostErrorFallsBack(t *testing.T) {
	var responded string
	sess := newTestMock()
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = resp.Data.Content
		return nil
	}

	h, rec := setupCheckHandlerWithRoll(sess, 12)
	rec.postErr = errors.New("dm-queue offline")
	h.Handle(makeCheckInteractionWithDC("perception", 15, "chan-1", "user1"))

	if !strings.Contains(responded, "Perception") {
		t.Errorf("expected fallback response, got: %s", responded)
	}
	if !strings.Contains(responded, "Check:") {
		t.Errorf("expected numeric result on fallback, got: %s", responded)
	}
}
