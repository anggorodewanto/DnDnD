package discord

import (
	"context"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"errors"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// makeRollInteraction builds a /roll command interaction. An empty dice or
// reason string omits nothing for dice (always present so the empty-expression
// path is exercised) and omits reason when blank.
func makeRollInteraction(diceExpr, reason string) *discordgo.Interaction {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "dice", Value: diceExpr, Type: discordgo.ApplicationCommandOptionString},
	}
	if reason != "" {
		opts = append(opts, &discordgo.ApplicationCommandInteractionDataOption{
			Name: "reason", Value: reason, Type: discordgo.ApplicationCommandOptionString,
		})
	}
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1", Username: "PlayerOne"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "roll",
			Options: opts,
		},
	}
}

// setupRollHandler wires a RollHandler with a deterministic roller (every die
// face returns rollVal) and the reusable check mocks. charErr non-nil makes
// the character lookup fail so the display-name fallback is exercised.
func setupRollHandler(sess *MockSession, charName string, charErr error, rollVal int) (*RollHandler, *mockCheckRollLogger) {
	campaignID := uuid.New()
	roller := dice.NewRoller(func(int) int { return rollVal })
	logger := &mockCheckRollLogger{}
	campProv := &mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
		return refdata.Campaign{ID: campaignID}, nil
	}}
	charLookup := &mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
		if charErr != nil {
			return refdata.Character{}, charErr
		}
		return refdata.Character{ID: uuid.New(), Name: charName}, nil
	}}
	h := NewRollHandler(sess, roller, campProv, charLookup, logger)
	return h, logger
}

func captureRollResponse(sess *MockSession, responded *string) {
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		*responded = resp.Data.Content
		return nil
	}
}

func TestRollHandler_RollsAndLogs(t *testing.T) {
	var responded string
	sess := newTestMock()
	captureRollResponse(sess, &responded)

	h, logger := setupRollHandler(sess, "Forge", nil, 14)
	h.Handle(makeRollInteraction("1d20+4", ""))

	if !strings.Contains(responded, "Forge") {
		t.Errorf("expected roller name in response, got: %s", responded)
	}
	if !strings.Contains(responded, "18") {
		t.Errorf("expected total 18 (14+4) in response, got: %s", responded)
	}
	if !strings.Contains(responded, "1d20+4") {
		t.Errorf("expected expression in response, got: %s", responded)
	}
	if len(logger.logged) != 1 {
		t.Fatalf("expected 1 roll logged, got %d", len(logger.logged))
	}
	entry := logger.logged[0]
	if entry.Roller != "Forge" {
		t.Errorf("expected logged roller Forge, got %q", entry.Roller)
	}
	if entry.Total != 18 {
		t.Errorf("expected logged total 18, got %d", entry.Total)
	}
	if entry.Expression != "1d20+4" {
		t.Errorf("expected logged expression 1d20+4, got %q", entry.Expression)
	}
}

func TestRollHandler_WithReason(t *testing.T) {
	var responded string
	sess := newTestMock()
	captureRollResponse(sess, &responded)

	h, logger := setupRollHandler(sess, "Forge", nil, 3)
	h.Handle(makeRollInteraction("2d6", "fire damage"))

	if !strings.Contains(responded, "fire damage") {
		t.Errorf("expected reason in response, got: %s", responded)
	}
	if !strings.Contains(responded, "6") {
		t.Errorf("expected total 6 (3+3) in response, got: %s", responded)
	}
	if len(logger.logged) != 1 || logger.logged[0].Purpose != "fire damage" {
		t.Errorf("expected logged purpose 'fire damage', got %+v", logger.logged)
	}
}

func TestRollHandler_InvalidExpression(t *testing.T) {
	var responded string
	sess := newTestMock()
	captureRollResponse(sess, &responded)

	h, logger := setupRollHandler(sess, "Forge", nil, 1)
	h.Handle(makeRollInteraction("banana", ""))

	if responded == "" {
		t.Fatal("expected an error response")
	}
	if !strings.Contains(strings.ToLower(responded), "1d20") {
		t.Errorf("expected examples in error response, got: %s", responded)
	}
	if len(logger.logged) != 0 {
		t.Errorf("expected no roll logged on parse error, got %d", len(logger.logged))
	}
}

func TestRollHandler_EmptyExpression(t *testing.T) {
	var responded string
	sess := newTestMock()
	captureRollResponse(sess, &responded)

	h, logger := setupRollHandler(sess, "Forge", nil, 1)
	h.Handle(makeRollInteraction("", ""))

	if responded == "" {
		t.Fatal("expected a usage response")
	}
	if len(logger.logged) != 0 {
		t.Errorf("expected no roll logged for empty expression, got %d", len(logger.logged))
	}
}

func TestRollHandler_TooManyDice(t *testing.T) {
	var responded string
	sess := newTestMock()
	captureRollResponse(sess, &responded)

	h, logger := setupRollHandler(sess, "Forge", nil, 1)
	h.Handle(makeRollInteraction("9999d20", ""))

	if !strings.Contains(strings.ToLower(responded), "too many dice") {
		t.Errorf("expected too-many-dice guard, got: %s", responded)
	}
	if len(logger.logged) != 0 {
		t.Errorf("expected no roll logged when over the dice cap, got %d", len(logger.logged))
	}
}

func TestRollHandler_TooManySides(t *testing.T) {
	var responded string
	sess := newTestMock()
	captureRollResponse(sess, &responded)

	h, logger := setupRollHandler(sess, "Forge", nil, 1)
	h.Handle(makeRollInteraction("1d99999", ""))

	if !strings.Contains(strings.ToLower(responded), "too many sides") {
		t.Errorf("expected too-many-sides guard, got: %s", responded)
	}
	if len(logger.logged) != 0 {
		t.Errorf("expected no roll logged when over the sides cap, got %d", len(logger.logged))
	}
}

func TestRollHandler_FallsBackToDisplayNameWhenNoCharacter(t *testing.T) {
	var responded string
	sess := newTestMock()
	captureRollResponse(sess, &responded)

	h, logger := setupRollHandler(sess, "", errors.New("no character"), 5)
	h.Handle(makeRollInteraction("1d6", ""))

	if !strings.Contains(responded, "PlayerOne") {
		t.Errorf("expected Discord display name fallback, got: %s", responded)
	}
	if len(logger.logged) != 1 || logger.logged[0].Roller != "PlayerOne" {
		t.Errorf("expected logged roller 'PlayerOne', got %+v", logger.logged)
	}
}

func TestRollHandler_NilDepsStillRolls(t *testing.T) {
	var responded string
	sess := newTestMock()
	captureRollResponse(sess, &responded)

	h := NewRollHandler(sess, dice.NewRoller(func(int) int { return 3 }), nil, nil, nil)
	h.Handle(makeRollInteraction("1d6", ""))

	if !strings.Contains(responded, "3") {
		t.Errorf("expected the roll to resolve with nil deps, got: %s", responded)
	}
	if !strings.Contains(responded, "PlayerOne") {
		t.Errorf("expected display-name fallback with nil deps, got: %s", responded)
	}
}
