package discordfake_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/ab/dndnd/internal/testutil/discordfake"
)

func TestFake_RecordsChannelMessageSend(t *testing.T) {
	f := discordfake.New()

	if _, err := f.ChannelMessageSend("chan-1", "hello world"); err != nil {
		t.Fatalf("ChannelMessageSend returned error: %v", err)
	}

	transcript := f.Transcript()
	if len(transcript) != 1 {
		t.Fatalf("expected 1 entry in transcript, got %d", len(transcript))
	}
	got := transcript[0]
	if got.Kind != discordfake.KindChannelMessage {
		t.Fatalf("expected KindChannelMessage, got %v", got.Kind)
	}
	if got.ChannelID != "chan-1" {
		t.Fatalf("expected channel chan-1, got %q", got.ChannelID)
	}
	if got.Content != "hello world" {
		t.Fatalf("expected content 'hello world', got %q", got.Content)
	}
}

func TestFake_RecordsChannelMessageSendComplex(t *testing.T) {
	f := discordfake.New()

	_, err := f.ChannelMessageSendComplex("chan-2", &discordgo.MessageSend{
		Content: "complex content",
		Embeds:  []*discordgo.MessageEmbed{{Title: "embed-title"}},
	})
	if err != nil {
		t.Fatalf("ChannelMessageSendComplex returned error: %v", err)
	}

	transcript := f.Transcript()
	if len(transcript) != 1 {
		t.Fatalf("expected 1 entry in transcript, got %d", len(transcript))
	}
	got := transcript[0]
	if got.Kind != discordfake.KindChannelMessage {
		t.Fatalf("expected KindChannelMessage, got %v", got.Kind)
	}
	if got.ChannelID != "chan-2" {
		t.Fatalf("expected channel chan-2, got %q", got.ChannelID)
	}
	if got.Content != "complex content" {
		t.Fatalf("expected content 'complex content', got %q", got.Content)
	}
	if len(got.Embeds) != 1 || got.Embeds[0].Title != "embed-title" {
		t.Fatalf("expected one embed with title 'embed-title', got %+v", got.Embeds)
	}
}

func TestFake_RecordsInteractionRespondAsEphemeralWhenFlagged(t *testing.T) {
	f := discordfake.New()

	interaction := &discordgo.Interaction{
		ID:        "i-1",
		ChannelID: "chan-3",
	}
	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "ephemeral reply",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}
	if err := f.InteractionRespond(interaction, resp); err != nil {
		t.Fatalf("InteractionRespond returned error: %v", err)
	}

	transcript := f.Transcript()
	if len(transcript) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(transcript))
	}
	got := transcript[0]
	if got.Kind != discordfake.KindInteractionResponse {
		t.Fatalf("expected KindInteractionResponse, got %v", got.Kind)
	}
	if !got.Ephemeral {
		t.Fatalf("expected ephemeral=true; got false")
	}
	if got.Content != "ephemeral reply" {
		t.Fatalf("expected content 'ephemeral reply', got %q", got.Content)
	}
}

func TestFake_TranscriptIsGoroutineSafe(t *testing.T) {
	f := discordfake.New()

	const writers = 8
	const perWriter = 25

	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perWriter; j++ {
				_, _ = f.ChannelMessageSend("chan", "msg")
			}
		}()
	}
	wg.Wait()

	if got := len(f.Transcript()); got != writers*perWriter {
		t.Fatalf("expected %d transcript entries, got %d", writers*perWriter, got)
	}
}

func TestFake_InjectInteractionInvokesRegisteredHandler(t *testing.T) {
	f := discordfake.New()

	called := make(chan *discordgo.Interaction, 1)
	f.SetInteractionHandler(func(i *discordgo.Interaction) {
		called <- i
	})

	want := &discordgo.Interaction{ID: "i-2"}
	f.InjectInteraction(want)

	select {
	case got := <-called:
		if got != want {
			t.Fatalf("handler received %v, want %v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("handler was not invoked within 1s")
	}
}

func TestFake_WaitForReturnsWhenPredicateMatches(t *testing.T) {
	f := discordfake.New()

	go func() {
		time.Sleep(10 * time.Millisecond)
		_, _ = f.ChannelMessageSend("chan-A", "the quick brown fox")
	}()

	entry, err := f.WaitFor(func(e discordfake.Entry) bool {
		return e.ChannelID == "chan-A"
	}, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitFor returned error: %v", err)
	}
	if entry.ChannelID != "chan-A" {
		t.Fatalf("expected channel chan-A, got %q", entry.ChannelID)
	}
}

func TestFake_WaitForTimesOut(t *testing.T) {
	f := discordfake.New()

	_, err := f.WaitFor(func(e discordfake.Entry) bool { return false }, 50*time.Millisecond)
	if !errors.Is(err, discordfake.ErrWaitTimeout) {
		t.Fatalf("expected ErrWaitTimeout, got %v", err)
	}
}

func TestFake_GuildChannelsReturnsRegistered(t *testing.T) {
	f := discordfake.New()
	f.AddGuildChannel("guild-1", &discordgo.Channel{ID: "ch-1", Name: "general"})
	f.AddGuildChannel("guild-1", &discordgo.Channel{ID: "ch-2", Name: "dm-queue"})

	chans, err := f.GuildChannels("guild-1")
	if err != nil {
		t.Fatalf("GuildChannels error: %v", err)
	}
	if len(chans) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(chans))
	}
}

func TestFake_ResetClearsTranscript(t *testing.T) {
	f := discordfake.New()
	_, _ = f.ChannelMessageSend("c", "m")
	f.Reset()
	if got := len(f.Transcript()); got != 0 {
		t.Fatalf("expected empty transcript after Reset, got %d entries", got)
	}
}

func TestKind_String(t *testing.T) {
	cases := []struct {
		k    discordfake.Kind
		want string
	}{
		{discordfake.KindChannelMessage, "channel_message"},
		{discordfake.KindInteractionResponse, "interaction_response"},
		{discordfake.KindInteractionEdit, "interaction_edit"},
		{discordfake.KindChannelMessageEdit, "channel_message_edit"},
		{discordfake.KindDirectMessage, "direct_message"},
		{discordfake.Kind(99), "unknown"},
	}
	for _, c := range cases {
		if got := c.k.String(); got != c.want {
			t.Fatalf("Kind(%d).String() = %q, want %q", c.k, got, c.want)
		}
	}
}

func TestFake_SetClockTimestampsTranscript(t *testing.T) {
	f := discordfake.New()
	stamp := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	f.SetClock(func() time.Time { return stamp })

	_, _ = f.ChannelMessageSend("c", "m")
	got := f.Transcript()
	if len(got) != 1 {
		t.Fatalf("expected 1 transcript entry, got %d", len(got))
	}
	if !got[0].Timestamp.Equal(stamp) {
		t.Fatalf("expected fixed timestamp %v, got %v", stamp, got[0].Timestamp)
	}
}

func TestFake_InjectInteractionWithoutHandlerIsNoOp(t *testing.T) {
	f := discordfake.New()
	// Must not panic when no handler is registered.
	f.InjectInteraction(&discordgo.Interaction{ID: "noop"})
	if got := len(f.Transcript()); got != 0 {
		t.Fatalf("expected empty transcript when no handler is set, got %d entries", got)
	}
}

func TestFake_UserChannelCreateMemoizesAndRecordsAsDM(t *testing.T) {
	f := discordfake.New()
	ch1, err := f.UserChannelCreate("user-1")
	if err != nil {
		t.Fatalf("UserChannelCreate: %v", err)
	}
	ch2, err := f.UserChannelCreate("user-1")
	if err != nil {
		t.Fatalf("UserChannelCreate (2nd call): %v", err)
	}
	if ch1.ID != ch2.ID {
		t.Fatalf("expected memoized channel id, got %q vs %q", ch1.ID, ch2.ID)
	}

	// A subsequent send to the DM channel should be tagged KindDirectMessage.
	_, _ = f.ChannelMessageSend(ch1.ID, "private hello")
	tx := f.Transcript()
	if len(tx) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(tx))
	}
	if tx[0].Kind != discordfake.KindDirectMessage {
		t.Fatalf("expected KindDirectMessage for DM channel, got %v", tx[0].Kind)
	}
}

func TestFake_GuildChannelCreateComplexRegistersChannel(t *testing.T) {
	f := discordfake.New()
	ch, err := f.GuildChannelCreateComplex("guild-Z", discordgo.GuildChannelCreateData{Name: "general"})
	if err != nil {
		t.Fatalf("GuildChannelCreateComplex: %v", err)
	}
	if ch.Name != "general" {
		t.Fatalf("expected channel name 'general', got %q", ch.Name)
	}
	chans, _ := f.GuildChannels("guild-Z")
	if len(chans) != 1 || chans[0].ID != ch.ID {
		t.Fatalf("expected created channel to be findable in GuildChannels, got %+v", chans)
	}
}

func TestFake_InteractionResponseEditRecordsContent(t *testing.T) {
	f := discordfake.New()
	content := "edited body"
	_, err := f.InteractionResponseEdit(&discordgo.Interaction{ID: "i-edit", ChannelID: "c-edit"},
		&discordgo.WebhookEdit{Content: &content})
	if err != nil {
		t.Fatalf("InteractionResponseEdit: %v", err)
	}
	tx := f.Transcript()
	if len(tx) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(tx))
	}
	if tx[0].Kind != discordfake.KindInteractionEdit || tx[0].Content != content {
		t.Fatalf("expected interaction-edit with content %q, got %+v", content, tx[0])
	}
}

func TestFake_ChannelMessageEditRecorded(t *testing.T) {
	f := discordfake.New()
	if _, err := f.ChannelMessageEdit("c", "m-1", "patched"); err != nil {
		t.Fatalf("ChannelMessageEdit: %v", err)
	}
	tx := f.Transcript()
	if len(tx) != 1 || tx[0].Kind != discordfake.KindChannelMessageEdit || tx[0].MessageID != "m-1" {
		t.Fatalf("unexpected transcript entry: %+v", tx)
	}
}

func TestFake_ApplicationCommandStubsReturnSafeDefaults(t *testing.T) {
	f := discordfake.New()
	cmds := []*discordgo.ApplicationCommand{{Name: "ping"}}
	got, err := f.ApplicationCommandBulkOverwrite("app", "guild", cmds)
	if err != nil || len(got) != 1 || got[0].Name != "ping" {
		t.Fatalf("ApplicationCommandBulkOverwrite returned %+v, %v", got, err)
	}
	gotCmds, err := f.ApplicationCommands("app", "guild")
	if err != nil || gotCmds != nil {
		t.Fatalf("ApplicationCommands returned %+v, %v", gotCmds, err)
	}
	if err := f.ApplicationCommandDelete("app", "guild", "cmd-id"); err != nil {
		t.Fatalf("ApplicationCommandDelete returned err: %v", err)
	}
}

func TestFake_GetStateAndSetState(t *testing.T) {
	f := discordfake.New()
	if s := f.GetState(); s == nil {
		t.Fatal("expected non-nil default state")
	}
	custom := &discordgo.State{Ready: discordgo.Ready{User: &discordgo.User{ID: "custom"}}}
	f.SetState(custom)
	if got := f.GetState(); got != custom {
		t.Fatalf("expected custom state to round-trip, got %+v", got)
	}
}
