package channelpost

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/narration"
)

type fakeLookup struct {
	channels map[string]string
	err      error
}

func (f fakeLookup) GetChannelIDsForCampaign(_ context.Context, _ uuid.UUID) (map[string]string, error) {
	return f.channels, f.err
}

type fakePoster struct {
	channelID string
	body      string
	embeds    []narration.DiscordEmbed
	ids       []string
	err       error
	calls     int
}

func (f *fakePoster) PostToChannel(channelID, body string, embeds []narration.DiscordEmbed) ([]string, error) {
	f.calls++
	f.channelID = channelID
	f.body = body
	f.embeds = embeds
	if f.err != nil {
		return nil, f.err
	}
	return f.ids, nil
}

func newSvc(channels map[string]string, poster Poster) *Service {
	return NewService(fakeLookup{channels: channels}, poster)
}

func TestService_Post_Success(t *testing.T) {
	poster := &fakePoster{ids: []string{"m-1"}}
	svc := newSvc(map[string]string{"in-character": "c-1"}, poster)
	res, err := svc.Post(context.Background(), PostInput{CampaignID: uuid.New(), Channel: "in-character", Body: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if poster.channelID != "c-1" {
		t.Fatalf("poster got channelID %q", poster.channelID)
	}
	if res.ChannelID != "c-1" || res.Channel != "in-character" {
		t.Fatalf("res = %+v", res)
	}
	if len(res.DiscordMessageIDs) != 1 || res.DiscordMessageIDs[0] != "m-1" {
		t.Fatalf("ids = %v", res.DiscordMessageIDs)
	}
}

func TestService_Post_RendersReadAloud(t *testing.T) {
	poster := &fakePoster{ids: []string{"m-1"}}
	svc := newSvc(map[string]string{"the-story": "c-story"}, poster)
	_, err := svc.Post(context.Background(), PostInput{
		CampaignID: uuid.New(), Channel: "the-story",
		Body: "intro\n:::read-aloud\nBoxed text.\n:::",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(poster.embeds) != 1 {
		t.Fatalf("expected 1 read-aloud embed, got %d", len(poster.embeds))
	}
	if poster.embeds[0].Description == "" {
		t.Fatalf("read-aloud embed rendered empty")
	}
}

func TestService_Post_UnknownChannel(t *testing.T) {
	svc := newSvc(map[string]string{"in-character": "c-1"}, &fakePoster{})
	_, err := svc.Post(context.Background(), PostInput{CampaignID: uuid.New(), Channel: "combat-log", Body: "hi"})
	if !errors.Is(err, ErrUnknownChannel) {
		t.Fatalf("expected ErrUnknownChannel, got %v", err)
	}
}

func TestService_Post_EmptyChannelIDIsUnknown(t *testing.T) {
	svc := newSvc(map[string]string{"in-character": ""}, &fakePoster{})
	_, err := svc.Post(context.Background(), PostInput{CampaignID: uuid.New(), Channel: "in-character", Body: "hi"})
	if !errors.Is(err, ErrUnknownChannel) {
		t.Fatalf("expected ErrUnknownChannel for empty id, got %v", err)
	}
}

func TestService_Post_Validation(t *testing.T) {
	svc := newSvc(map[string]string{"in-character": "c-1"}, &fakePoster{})
	cases := []PostInput{
		{Channel: "in-character", Body: "hi"},                        // nil campaign
		{CampaignID: uuid.New(), Body: "hi"},                         // empty channel
		{CampaignID: uuid.New(), Channel: "in-character", Body: " "}, // blank body
	}
	for i, in := range cases {
		if _, err := svc.Post(context.Background(), in); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("case %d: expected ErrInvalidInput, got %v", i, err)
		}
	}
}

func TestService_Post_NilPoster(t *testing.T) {
	svc := NewService(fakeLookup{channels: map[string]string{"x": "c"}}, nil)
	_, err := svc.Post(context.Background(), PostInput{CampaignID: uuid.New(), Channel: "x", Body: "hi"})
	if !errors.Is(err, ErrPosterUnavailable) {
		t.Fatalf("expected ErrPosterUnavailable, got %v", err)
	}
}

func TestService_Post_LookupError(t *testing.T) {
	svc := NewService(fakeLookup{err: errors.New("db down")}, &fakePoster{})
	if _, err := svc.Post(context.Background(), PostInput{CampaignID: uuid.New(), Channel: "x", Body: "hi"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestService_Post_PosterError(t *testing.T) {
	svc := newSvc(map[string]string{"x": "c"}, &fakePoster{err: errors.New("send failed")})
	if _, err := svc.Post(context.Background(), PostInput{CampaignID: uuid.New(), Channel: "x", Body: "hi"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestService_Channels_SortedSkipsEmpty(t *testing.T) {
	svc := newSvc(map[string]string{"the-story": "c1", "in-character": "c2", "blank": ""}, &fakePoster{})
	got, err := svc.Channels(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"in-character", "the-story"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestService_Channels_NilCampaign(t *testing.T) {
	svc := newSvc(map[string]string{}, &fakePoster{})
	if _, err := svc.Channels(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}
