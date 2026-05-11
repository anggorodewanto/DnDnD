package discord

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

// captureSentComplex returns a session that records the last ChannelMessageSendComplex
// payload + a slice of all components captured across calls.
func captureSentComplex() (*MockSession, *[]*discordgo.MessageSend) {
	mock := newTestMock()
	var mu sync.Mutex
	var sent []*discordgo.MessageSend
	mock.ChannelMessageSendComplexFunc = func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
		mu.Lock()
		defer mu.Unlock()
		sent = append(sent, data)
		return &discordgo.Message{ID: "m-1", ChannelID: channelID}, nil
	}
	return mock, &sent
}

func TestReactionPromptStore_Post_SendsMessageWithButtons(t *testing.T) {
	mock, sent := captureSentComplex()
	store := NewReactionPromptStore(mock)

	_, err := store.Post(ReactionPromptPostArgs{
		ChannelID: "ch-1",
		Content:   "Counterspell?",
		Buttons: []ReactionPromptButton{
			{Label: "Slot 3", Choice: "3"},
			{Label: "Slot 5", Choice: "5"},
			{Label: "Pass", Choice: "pass", Style: discordgo.SecondaryButton},
		},
		OnChoice: func(ctx context.Context, _ *discordgo.Interaction, choice string) {},
	})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if len(*sent) != 1 {
		t.Fatalf("expected 1 send, got %d", len(*sent))
	}
	row, ok := (*sent)[0].Components[0].(discordgo.ActionsRow)
	if !ok || len(row.Components) != 3 {
		t.Fatalf("expected ActionsRow with 3 buttons, got %+v", (*sent)[0].Components)
	}
	for i, want := range []string{"3", "5", "pass"} {
		btn := row.Components[i].(discordgo.Button)
		if !strings.HasSuffix(btn.CustomID, ":"+want) {
			t.Errorf("button %d CustomID = %q, want suffix %q", i, btn.CustomID, want)
		}
		if !strings.HasPrefix(btn.CustomID, reactionPromptPrefix+":") {
			t.Errorf("button %d CustomID = %q, want prefix %q", i, btn.CustomID, reactionPromptPrefix+":")
		}
	}
	if row.Components[2].(discordgo.Button).Style != discordgo.SecondaryButton {
		t.Errorf("pass button style: got %v want SecondaryButton", row.Components[2].(discordgo.Button).Style)
	}
}

func TestReactionPromptStore_HandleComponent_RoutesChoice(t *testing.T) {
	mock, _ := captureSentComplex()
	store := NewReactionPromptStoreWithTTL(mock, time.Hour)

	var got atomic.Value
	id, err := store.Post(ReactionPromptPostArgs{
		ChannelID: "ch-1",
		Content:   "Pick a slot",
		Buttons:   []ReactionPromptButton{{Label: "3", Choice: "3"}},
		OnChoice: func(_ context.Context, _ *discordgo.Interaction, choice string) {
			got.Store(choice)
		},
	})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: reactionPromptPrefix + ":" + id.String() + ":3",
		},
	}
	if !store.HandleComponent(interaction) {
		t.Fatalf("HandleComponent returned false for known prompt")
	}
	if v := got.Load(); v == nil || v.(string) != "3" {
		t.Errorf("expected choice %q, got %v", "3", v)
	}

	// Second click with the same id is a no-op (prompt consumed).
	got.Store("")
	if !store.HandleComponent(interaction) {
		t.Fatalf("expected HandleComponent to still claim the prefix after consume")
	}
	if v := got.Load(); v.(string) != "" {
		t.Errorf("expected stale click to be ignored, got %q", v)
	}
}

func TestReactionPromptStore_HandleComponent_RejectsForeignPrefix(t *testing.T) {
	mock, _ := captureSentComplex()
	store := NewReactionPromptStore(mock)
	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: "move_confirm:abc"},
	}
	if store.HandleComponent(interaction) {
		t.Errorf("HandleComponent claimed a foreign customID")
	}
}

func TestReactionPromptStore_Forfeit_FiresAfterTTL(t *testing.T) {
	mock, _ := captureSentComplex()
	store := NewReactionPromptStoreWithTTL(mock, 5*time.Millisecond)

	done := make(chan struct{})
	_, err := store.Post(ReactionPromptPostArgs{
		ChannelID: "ch-1",
		Content:   "Stunning Strike?",
		Buttons:   []ReactionPromptButton{{Label: "Use Ki", Choice: "use"}},
		OnChoice:  func(context.Context, *discordgo.Interaction, string) {},
		OnForfeit: func(context.Context) { close(done) },
	})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("OnForfeit did not fire within TTL window")
	}
}

func TestReactionPromptStore_Forfeit_SkippedOnChoice(t *testing.T) {
	mock, _ := captureSentComplex()
	store := NewReactionPromptStoreWithTTL(mock, 30*time.Millisecond)

	forfeited := atomic.Bool{}
	id, err := store.Post(ReactionPromptPostArgs{
		ChannelID: "ch-1",
		Content:   "Smite?",
		Buttons:   []ReactionPromptButton{{Label: "Slot 1", Choice: "1"}},
		OnChoice:  func(context.Context, *discordgo.Interaction, string) {},
		OnForfeit: func(context.Context) { forfeited.Store(true) },
	})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}

	store.HandleComponent(&discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: reactionPromptPrefix + ":" + id.String() + ":1",
		},
	})

	time.Sleep(60 * time.Millisecond)
	if forfeited.Load() {
		t.Errorf("OnForfeit fired after a choice was made")
	}
}

func TestReactionPromptStore_Cancel_StopsForfeit(t *testing.T) {
	mock, _ := captureSentComplex()
	store := NewReactionPromptStoreWithTTL(mock, 30*time.Millisecond)

	forfeited := atomic.Bool{}
	id, err := store.Post(ReactionPromptPostArgs{
		ChannelID: "ch-1",
		Content:   "Uncanny Dodge?",
		Buttons:   []ReactionPromptButton{{Label: "Halve", Choice: "halve"}},
		OnChoice:  func(context.Context, *discordgo.Interaction, string) {},
		OnForfeit: func(context.Context) { forfeited.Store(true) },
	})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	store.Cancel(id)
	time.Sleep(60 * time.Millisecond)
	if forfeited.Load() {
		t.Errorf("Cancel did not stop the forfeit timer")
	}
}

func TestReactionPromptStore_Post_RejectsZeroButtons(t *testing.T) {
	mock, _ := captureSentComplex()
	store := NewReactionPromptStore(mock)
	if _, err := store.Post(ReactionPromptPostArgs{
		ChannelID: "ch-1",
		Content:   "x",
		Buttons:   nil,
		OnChoice:  func(context.Context, *discordgo.Interaction, string) {},
	}); err == nil {
		t.Errorf("expected error for zero buttons")
	}
}

func TestReactionPromptStore_Post_RejectsNilOnChoice(t *testing.T) {
	mock, _ := captureSentComplex()
	store := NewReactionPromptStore(mock)
	if _, err := store.Post(ReactionPromptPostArgs{
		ChannelID: "ch-1",
		Content:   "x",
		Buttons:   []ReactionPromptButton{{Label: "x", Choice: "x"}},
		OnChoice:  nil,
	}); err == nil {
		t.Errorf("expected error for nil OnChoice")
	}
}

func TestReactionPromptStore_HandleComponent_IgnoresBadID(t *testing.T) {
	mock, _ := captureSentComplex()
	store := NewReactionPromptStore(mock)
	cases := []string{
		reactionPromptPrefix + ":notauuid:choice",
		reactionPromptPrefix + ":missing-choice",
		reactionPromptPrefix + ":",
	}
	for _, customID := range cases {
		interaction := &discordgo.Interaction{
			Type: discordgo.InteractionMessageComponent,
			Data: discordgo.MessageComponentInteractionData{CustomID: customID},
		}
		// claimed=true is acceptable for malformed-within-prefix; we only care
		// the call does not panic.
		store.HandleComponent(interaction)
	}
}
