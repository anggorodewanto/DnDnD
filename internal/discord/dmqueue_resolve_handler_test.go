package discord

import (
	"context"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"

	"github.com/ab/dndnd/internal/dmqueue"
)

// resolveRecordingNotifier records which resolve path the handler dispatched
// to and the args it passed.
type resolveRecordingNotifier struct {
	item       dmqueue.Item
	found      bool
	resolved   string // "resolve" | "whisper" | "narration"
	gotID      string
	gotText    string
	resolveErr error
}

func (n *resolveRecordingNotifier) Post(context.Context, dmqueue.Event) (string, error) {
	return "", nil
}
func (n *resolveRecordingNotifier) Cancel(context.Context, string, string) error { return nil }
func (n *resolveRecordingNotifier) Resolve(_ context.Context, id, text string) error {
	n.resolved, n.gotID, n.gotText = "resolve", id, text
	return n.resolveErr
}
func (n *resolveRecordingNotifier) ResolveWhisper(_ context.Context, id, text string) error {
	n.resolved, n.gotID, n.gotText = "whisper", id, text
	return n.resolveErr
}
func (n *resolveRecordingNotifier) ResolveSkillCheckNarration(_ context.Context, id, text string) error {
	n.resolved, n.gotID, n.gotText = "narration", id, text
	return n.resolveErr
}
func (n *resolveRecordingNotifier) Get(string) (dmqueue.Item, bool) {
	if !n.found {
		return dmqueue.Item{}, false
	}
	return n.item, true
}
func (n *resolveRecordingNotifier) ListPending() []dmqueue.Item { return nil }

func dmqueueModalSubmit(inputID, value string) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type: discordgo.InteractionModalSubmit,
		Data: discordgo.ModalSubmitInteractionData{
			Components: []discordgo.MessageComponent{
				&discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					&discordgo.TextInput{CustomID: inputID, Value: value},
				}},
			},
		},
	}
}

func TestDMQueueResolve_ShowModal_Pending(t *testing.T) {
	var resp *discordgo.InteractionResponse
	mock := newTestMock()
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, r *discordgo.InteractionResponse) error {
		resp = r
		return nil
	}
	n := &resolveRecordingNotifier{found: true, item: dmqueue.Item{
		ID: "q1", Status: dmqueue.StatusPending,
		Event: dmqueue.Event{Kind: dmqueue.KindFreeformAction},
	}}
	h := NewDMQueueResolveHandler(mock, n)
	h.ShowResolveModal(&discordgo.Interaction{Type: discordgo.InteractionMessageComponent}, "q1")

	if resp == nil || resp.Type != discordgo.InteractionResponseModal {
		t.Fatalf("expected modal response, got %+v", resp)
	}
	if resp.Data.CustomID != dmQueueResolveModalPrefix+"q1" {
		t.Errorf("modal CustomID = %q want %q", resp.Data.CustomID, dmQueueResolveModalPrefix+"q1")
	}
}

func TestDMQueueResolve_ShowModal_Gone(t *testing.T) {
	var content string
	mock := newTestMock()
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, r *discordgo.InteractionResponse) error {
		content = r.Data.Content
		return nil
	}
	h := NewDMQueueResolveHandler(mock, &resolveRecordingNotifier{found: false})
	h.ShowResolveModal(&discordgo.Interaction{}, "missing")
	if !strings.Contains(strings.ToLower(content), "no longer available") {
		t.Errorf("got %q", content)
	}
}

func TestDMQueueResolve_ShowModal_AlreadyHandled(t *testing.T) {
	var content string
	mock := newTestMock()
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, r *discordgo.InteractionResponse) error {
		content = r.Data.Content
		return nil
	}
	n := &resolveRecordingNotifier{found: true, item: dmqueue.Item{ID: "q1", Status: dmqueue.StatusResolved}}
	h := NewDMQueueResolveHandler(mock, n)
	h.ShowResolveModal(&discordgo.Interaction{}, "q1")
	if !strings.Contains(strings.ToLower(content), "already") {
		t.Errorf("got %q", content)
	}
}

func TestDMQueueResolve_Submit_DispatchesByKind(t *testing.T) {
	cases := []struct {
		kind dmqueue.EventKind
		want string
	}{
		{dmqueue.KindPlayerWhisper, "whisper"},
		{dmqueue.KindSkillCheckNarration, "narration"},
		{dmqueue.KindFreeformAction, "resolve"},
	}
	for _, tc := range cases {
		var content string
		mock := newTestMock()
		mock.InteractionRespondFunc = func(_ *discordgo.Interaction, r *discordgo.InteractionResponse) error {
			content = r.Data.Content
			return nil
		}
		n := &resolveRecordingNotifier{found: true, item: dmqueue.Item{
			ID: "q1", Status: dmqueue.StatusPending, Event: dmqueue.Event{Kind: tc.kind},
		}}
		h := NewDMQueueResolveHandler(mock, n)
		h.HandleResolveSubmit(dmqueueModalSubmit(dmQueueResolveInputID, "the outcome text"), "q1")

		if n.resolved != tc.want {
			t.Errorf("kind %s: dispatched %q want %q", tc.kind, n.resolved, tc.want)
		}
		if n.gotID != "q1" || n.gotText != "the outcome text" {
			t.Errorf("kind %s: got id=%q text=%q", tc.kind, n.gotID, n.gotText)
		}
		if !strings.Contains(content, "Resolved") {
			t.Errorf("kind %s: confirmation = %q", tc.kind, content)
		}
	}
}

func TestRouter_RoutesDMQueueResolveButton(t *testing.T) {
	var resp *discordgo.InteractionResponse
	mock := newTestMock()
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, r *discordgo.InteractionResponse) error {
		resp = r
		return nil
	}
	bot := &Bot{session: mock}
	router := NewCommandRouter(bot, nil)
	n := &resolveRecordingNotifier{found: true, item: dmqueue.Item{
		ID: "q1", Status: dmqueue.StatusPending, Event: dmqueue.Event{Kind: dmqueue.KindFreeformAction},
	}}
	router.SetDMQueueResolveHandler(NewDMQueueResolveHandler(mock, n))

	router.Handle(&discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{CustomID: dmqueue.ResolveButtonCustomIDPrefix + "q1"},
	})

	if resp == nil || resp.Type != discordgo.InteractionResponseModal {
		t.Fatalf("expected modal from button dispatch, got %+v", resp)
	}
	if resp.Data.CustomID != dmQueueResolveModalPrefix+"q1" {
		t.Errorf("modal CustomID = %q want %q", resp.Data.CustomID, dmQueueResolveModalPrefix+"q1")
	}
}

func TestRouter_RoutesDMQueueResolveModalSubmit(t *testing.T) {
	var content string
	mock := newTestMock()
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, r *discordgo.InteractionResponse) error {
		content = r.Data.Content
		return nil
	}
	bot := &Bot{session: mock}
	router := NewCommandRouter(bot, nil)
	n := &resolveRecordingNotifier{found: true, item: dmqueue.Item{
		ID: "q1", Status: dmqueue.StatusPending, Event: dmqueue.Event{Kind: dmqueue.KindSkillCheckNarration},
	}}
	router.SetDMQueueResolveHandler(NewDMQueueResolveHandler(mock, n))

	router.Handle(&discordgo.Interaction{
		Type: discordgo.InteractionModalSubmit,
		Data: discordgo.ModalSubmitInteractionData{
			CustomID: dmQueueResolveModalPrefix + "q1",
			Components: []discordgo.MessageComponent{
				&discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					&discordgo.TextInput{CustomID: dmQueueResolveInputID, Value: "you find a key"},
				}},
			},
		},
	})

	if n.resolved != "narration" {
		t.Errorf("expected narration dispatch, got %q", n.resolved)
	}
	if n.gotText != "you find a key" {
		t.Errorf("text = %q", n.gotText)
	}
	if !strings.Contains(content, "Resolved") {
		t.Errorf("confirmation = %q", content)
	}
}

func TestDMQueueResolve_Submit_EmptyTextRejected(t *testing.T) {
	var content string
	mock := newTestMock()
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, r *discordgo.InteractionResponse) error {
		content = r.Data.Content
		return nil
	}
	n := &resolveRecordingNotifier{found: true, item: dmqueue.Item{
		ID: "q1", Status: dmqueue.StatusPending, Event: dmqueue.Event{Kind: dmqueue.KindFreeformAction},
	}}
	h := NewDMQueueResolveHandler(mock, n)
	h.HandleResolveSubmit(dmqueueModalSubmit(dmQueueResolveInputID, "   "), "q1")

	if n.resolved != "" {
		t.Errorf("should not dispatch on empty text, dispatched %q", n.resolved)
	}
	if !strings.Contains(strings.ToLower(content), "required") {
		t.Errorf("expected 'required' message, got %q", content)
	}
}
