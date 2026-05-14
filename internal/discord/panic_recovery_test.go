package discord

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"

	"github.com/ab/dndnd/internal/errorlog"
)

// panicHandler is a CommandHandler whose Handle method always panics. It
// exists so TestCommandRouter_PanicRecovery_RepliesFriendly and
// TestCommandRouter_PanicRecovery_RecordsError can verify panics never
// bubble out of the router and are always surfaced to the player as a
// friendly ephemeral, matching spec lines 2960-2972.
type panicHandler struct{ msg string }

func (h *panicHandler) Handle(_ *discordgo.Interaction) { panic(h.msg) }

func TestCommandRouter_PanicRecovery_RepliesFriendly(t *testing.T) {
	mock := newTestMock()
	var respondedContent string
	var respondedFlags discordgo.MessageFlags
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Data != nil {
			respondedContent = resp.Data.Content
			respondedFlags = resp.Data.Flags
		}
		return nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)
	// Swap in a handler that panics.
	router.handlers["attack"] = &panicHandler{msg: "boom"}

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{Name: "attack"},
		Member: &discordgo.Member{User: &discordgo.User{ID: "user-99"}},
	}

	// Must not re-panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected panic to be recovered, got: %v", r)
		}
	}()

	router.Handle(interaction)

	if respondedContent == "" {
		t.Fatal("expected a friendly ephemeral response, got empty")
	}
	if respondedFlags != discordgo.MessageFlagsEphemeral {
		t.Errorf("expected ephemeral flag, got %d", respondedFlags)
	}
	if !strings.Contains(strings.ToLower(respondedContent), "something went wrong") {
		t.Errorf("expected friendly wording, got: %s", respondedContent)
	}
}

func TestCommandRouter_PanicRecovery_RecordsError(t *testing.T) {
	mock := newTestMock()
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, _ *discordgo.InteractionResponse) error {
		return nil
	}
	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)
	store := errorlog.NewMemoryStore(nil)
	router.SetErrorRecorder(store)
	router.handlers["cast"] = &panicHandler{msg: "db timeout"}

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{Name: "cast"},
		Member: &discordgo.Member{User: &discordgo.User{ID: "player-7"}},
	}

	router.Handle(interaction)

	entries, err := store.ListRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 recorded error, got %d", len(entries))
	}
	e := entries[0]
	if e.Command != "cast" {
		t.Errorf("expected command=cast, got %q", e.Command)
	}
	if e.UserID != "player-7" {
		t.Errorf("expected userID=player-7, got %q", e.UserID)
	}
	if !strings.Contains(e.Summary, "db timeout") {
		t.Errorf("expected summary to include panic message, got %q", e.Summary)
	}
}

func TestCommandRouter_PanicRecovery_NoRecorderStillReplies(t *testing.T) {
	mock := newTestMock()
	var responded bool
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, _ *discordgo.InteractionResponse) error {
		responded = true
		return nil
	}
	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)
	router.handlers["attack"] = &panicHandler{msg: "boom"}

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{Name: "attack"},
	}

	router.Handle(interaction)
	if !responded {
		t.Fatal("expected friendly ephemeral even without recorder configured")
	}
}

// errorReturningHandler implements ErrorReportingHandler — its HandleCommand
// returns a non-nil error to simulate a normal (non-panic) failure path.
type errorReturningHandler struct {
	err error
}

func (h *errorReturningHandler) Handle(_ *discordgo.Interaction) {}
func (h *errorReturningHandler) HandleCommand(_ *discordgo.Interaction) error {
	return h.err
}

func TestCommandRouter_ErrorReportingHandler_RecordsError(t *testing.T) {
	mock := newTestMock()
	var respondedContent string
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Data != nil {
			respondedContent = resp.Data.Content
		}
		return nil
	}
	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)
	store := errorlog.NewMemoryStore(nil)
	router.SetErrorRecorder(store)

	handlerErr := fmt.Errorf("DB timeout fetching spell slots")
	router.handlers["cast"] = &errorReturningHandler{err: handlerErr}

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{Name: "cast"},
		Member: &discordgo.Member{User: &discordgo.User{ID: "player-42"}},
	}

	router.Handle(interaction)

	// Verify error was recorded to errorlog.
	entries, err := store.ListRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 recorded error, got %d", len(entries))
	}
	e := entries[0]
	if e.Command != "cast" {
		t.Errorf("expected command=cast, got %q", e.Command)
	}
	if e.UserID != "player-42" {
		t.Errorf("expected userID=player-42, got %q", e.UserID)
	}
	if !strings.Contains(e.Summary, "DB timeout") {
		t.Errorf("expected summary to contain error message, got %q", e.Summary)
	}

	// Verify friendly ephemeral was sent to the player.
	if !strings.Contains(strings.ToLower(respondedContent), "something went wrong") {
		t.Errorf("expected friendly ephemeral, got %q", respondedContent)
	}
}

func TestCommandRouter_ErrorReportingHandler_NilError_NoRecord(t *testing.T) {
	mock := newTestMock()
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, _ *discordgo.InteractionResponse) error {
		return nil
	}
	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)
	store := errorlog.NewMemoryStore(nil)
	router.SetErrorRecorder(store)

	// Handler returns nil — no error to record.
	router.handlers["cast"] = &errorReturningHandler{err: nil}

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{Name: "cast"},
		Member: &discordgo.Member{User: &discordgo.User{ID: "player-42"}},
	}

	router.Handle(interaction)

	entries, err := store.ListRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 recorded errors for nil return, got %d", len(entries))
	}
}
