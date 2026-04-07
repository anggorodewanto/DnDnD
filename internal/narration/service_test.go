package narration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// --- fakes ---

type fakeStore struct {
	inserted []InsertPostParams
	list     []Post
	insertErr error
	listErr  error
}

func (f *fakeStore) InsertNarrationPost(ctx context.Context, p InsertPostParams) (Post, error) {
	if f.insertErr != nil {
		return Post{}, f.insertErr
	}
	f.inserted = append(f.inserted, p)
	return Post{
		ID:                 uuid.New(),
		CampaignID:         p.CampaignID,
		AuthorUserID:       p.AuthorUserID,
		Body:               p.Body,
		AttachmentAssetIDs: append([]uuid.UUID(nil), p.AttachmentAssetIDs...),
		DiscordMessageIDs:  append([]string(nil), p.DiscordMessageIDs...),
		PostedAt:           time.Now(),
	}, nil
}

func (f *fakeStore) ListNarrationPostsByCampaign(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]Post, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.list, nil
}

type fakePoster struct {
	calls     []posterCall
	returnIDs []string
	err       error
}

type posterCall struct {
	guildID        string
	body           string
	embeds         []DiscordEmbed
	attachmentURLs []string
}

func (f *fakePoster) PostToStory(guildID, body string, embeds []DiscordEmbed, attachmentURLs []string) ([]string, error) {
	f.calls = append(f.calls, posterCall{guildID, body, embeds, attachmentURLs})
	if f.err != nil {
		return nil, f.err
	}
	return f.returnIDs, nil
}

type fakeAttachments struct {
	urls    map[uuid.UUID]string
	missing uuid.UUID
}

func (f *fakeAttachments) AttachmentURL(id uuid.UUID) (string, bool) {
	if id == f.missing {
		return "", false
	}
	u, ok := f.urls[id]
	return u, ok
}

type fakeCampaigns struct {
	guildID string
	err     error
}

func (f *fakeCampaigns) GuildIDForCampaign(ctx context.Context, id uuid.UUID) (string, error) {
	return f.guildID, f.err
}

// --- tests ---

func newSvc(store Store, poster Poster, att AttachmentResolver, camp CampaignResolver) *Service {
	return NewService(store, poster, att, camp)
}

func TestService_Post_RejectsEmptyBody(t *testing.T) {
	store := &fakeStore{}
	poster := &fakePoster{returnIDs: []string{"m1"}}
	svc := newSvc(store, poster, &fakeAttachments{}, &fakeCampaigns{guildID: "g1"})

	_, err := svc.Post(context.Background(), PostInput{
		CampaignID:   uuid.New(),
		AuthorUserID: "u1",
		Body:         "   ",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	if len(store.inserted) != 0 {
		t.Fatalf("nothing should be inserted on validation failure")
	}
	if len(poster.calls) != 0 {
		t.Fatalf("nothing should be posted on validation failure")
	}
}

func TestService_Post_RejectsMissingAttachment(t *testing.T) {
	missing := uuid.New()
	store := &fakeStore{}
	poster := &fakePoster{}
	att := &fakeAttachments{urls: map[uuid.UUID]string{}, missing: missing}
	svc := newSvc(store, poster, att, &fakeCampaigns{guildID: "g1"})

	_, err := svc.Post(context.Background(), PostInput{
		CampaignID:         uuid.New(),
		AuthorUserID:       "u1",
		Body:               "hello",
		AttachmentAssetIDs: []uuid.UUID{missing},
	})
	if !errors.Is(err, ErrAttachmentNotFound) {
		t.Fatalf("expected ErrAttachmentNotFound, got %v", err)
	}
	if len(poster.calls) != 0 {
		t.Fatalf("poster must not be called")
	}
}

func TestService_Post_DiscordFailurePropagatedAndNotRecorded(t *testing.T) {
	store := &fakeStore{}
	poster := &fakePoster{err: errors.New("discord down")}
	svc := newSvc(store, poster, &fakeAttachments{}, &fakeCampaigns{guildID: "g1"})

	_, err := svc.Post(context.Background(), PostInput{
		CampaignID:   uuid.New(),
		AuthorUserID: "u1",
		Body:         "hi",
	})
	if err == nil || !strings.Contains(err.Error(), "discord down") {
		t.Fatalf("expected discord error, got %v", err)
	}
	if len(store.inserted) != 0 {
		t.Fatalf("post must not be recorded on discord failure")
	}
}

func TestService_Post_SuccessRecordsPost(t *testing.T) {
	assetID := uuid.New()
	store := &fakeStore{}
	poster := &fakePoster{returnIDs: []string{"msg-1", "msg-2"}}
	att := &fakeAttachments{urls: map[uuid.UUID]string{assetID: "/api/assets/" + assetID.String()}}
	svc := newSvc(store, poster, att, &fakeCampaigns{guildID: "g1"})

	campID := uuid.New()
	post, err := svc.Post(context.Background(), PostInput{
		CampaignID:         campID,
		AuthorUserID:       "dm-42",
		Body:               "The party enters the tavern.",
		AttachmentAssetIDs: []uuid.UUID{assetID},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.inserted) != 1 {
		t.Fatalf("expected 1 insert, got %d", len(store.inserted))
	}
	ins := store.inserted[0]
	if ins.CampaignID != campID {
		t.Fatalf("campaign id mismatch")
	}
	if ins.AuthorUserID != "dm-42" {
		t.Fatalf("author id mismatch")
	}
	if len(ins.DiscordMessageIDs) != 2 || ins.DiscordMessageIDs[0] != "msg-1" {
		t.Fatalf("discord message ids = %v", ins.DiscordMessageIDs)
	}
	if len(poster.calls) != 1 || poster.calls[0].guildID != "g1" {
		t.Fatalf("poster call = %+v", poster.calls)
	}
	if len(poster.calls[0].attachmentURLs) != 1 {
		t.Fatalf("attachment URLs = %v", poster.calls[0].attachmentURLs)
	}
	if post.Body != "The party enters the tavern." {
		t.Fatalf("returned body = %q", post.Body)
	}
}

func TestService_Post_NilPosterReturnsErrPosterUnavailable(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store, nil, &fakeAttachments{}, &fakeCampaigns{guildID: "g1"})
	_, err := svc.Post(context.Background(), PostInput{
		CampaignID:   uuid.New(),
		AuthorUserID: "u1",
		Body:         "hi",
	})
	if !errors.Is(err, ErrPosterUnavailable) {
		t.Fatalf("expected ErrPosterUnavailable, got %v", err)
	}
	if len(store.inserted) != 0 {
		t.Fatalf("should not record post on unavailable poster")
	}
}

func TestService_Post_RejectsNilCampaign(t *testing.T) {
	svc := NewService(&fakeStore{}, &fakePoster{}, &fakeAttachments{}, &fakeCampaigns{})
	_, err := svc.Post(context.Background(), PostInput{AuthorUserID: "u", Body: "b"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestService_Post_RejectsEmptyAuthor(t *testing.T) {
	svc := NewService(&fakeStore{}, &fakePoster{}, &fakeAttachments{}, &fakeCampaigns{})
	_, err := svc.Post(context.Background(), PostInput{CampaignID: uuid.New(), Body: "b"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestService_Post_CampaignLookupError(t *testing.T) {
	svc := NewService(&fakeStore{}, &fakePoster{}, &fakeAttachments{}, &fakeCampaigns{err: errors.New("db down")})
	_, err := svc.Post(context.Background(), PostInput{CampaignID: uuid.New(), AuthorUserID: "u", Body: "b"})
	if err == nil || !strings.Contains(err.Error(), "db down") {
		t.Fatalf("expected campaign lookup error, got %v", err)
	}
}

func TestService_History_DefaultsLimitAndOffset(t *testing.T) {
	store := &fakeStore{list: []Post{}}
	svc := NewService(store, &fakePoster{}, &fakeAttachments{}, &fakeCampaigns{})
	// Negative limit/offset should be normalized.
	_, err := svc.History(context.Background(), uuid.New(), -5, -1)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestService_History_Delegates(t *testing.T) {
	campID := uuid.New()
	want := []Post{{ID: uuid.New(), CampaignID: campID, Body: "h1"}}
	store := &fakeStore{list: want}
	svc := newSvc(store, &fakePoster{}, &fakeAttachments{}, &fakeCampaigns{})

	got, err := svc.History(context.Background(), campID, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Body != "h1" {
		t.Fatalf("history = %+v", got)
	}
}

