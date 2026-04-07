package narration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

type fakeRefdata struct {
	insertArg refdata.InsertNarrationPostParams
	insertErr error
	inserted  refdata.NarrationPost
	listArg   refdata.ListNarrationPostsByCampaignParams
	list      []refdata.NarrationPost
	listErr   error
}

func (f *fakeRefdata) InsertNarrationPost(ctx context.Context, arg refdata.InsertNarrationPostParams) (refdata.NarrationPost, error) {
	f.insertArg = arg
	if f.insertErr != nil {
		return refdata.NarrationPost{}, f.insertErr
	}
	return f.inserted, nil
}

func (f *fakeRefdata) ListNarrationPostsByCampaign(ctx context.Context, arg refdata.ListNarrationPostsByCampaignParams) ([]refdata.NarrationPost, error) {
	f.listArg = arg
	return f.list, f.listErr
}

func TestDBStore_InsertNarrationPost_MapsFieldsBothWays(t *testing.T) {
	now := time.Now()
	campID := uuid.New()
	assetID := uuid.New()
	fake := &fakeRefdata{
		inserted: refdata.NarrationPost{
			ID:                 uuid.New(),
			CampaignID:         campID,
			AuthorUserID:       "dm-1",
			Body:               "Hi",
			AttachmentAssetIds: []uuid.UUID{assetID},
			DiscordMessageIds:  []string{"m1"},
			PostedAt:           now,
		},
	}
	store := NewDBStore(fake)

	p, err := store.InsertNarrationPost(context.Background(), InsertPostParams{
		CampaignID:         campID,
		AuthorUserID:       "dm-1",
		Body:               "Hi",
		AttachmentAssetIDs: []uuid.UUID{assetID},
		DiscordMessageIDs:  []string{"m1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify outbound refdata args
	if fake.insertArg.CampaignID != campID {
		t.Fatalf("arg campaign mismatch")
	}
	if len(fake.insertArg.AttachmentAssetIds) != 1 || fake.insertArg.AttachmentAssetIds[0] != assetID {
		t.Fatalf("arg attachments mismatch: %v", fake.insertArg.AttachmentAssetIds)
	}
	if len(fake.insertArg.DiscordMessageIds) != 1 || fake.insertArg.DiscordMessageIds[0] != "m1" {
		t.Fatalf("arg discord ids mismatch: %v", fake.insertArg.DiscordMessageIds)
	}

	// Verify returned Post
	if p.CampaignID != campID || p.AuthorUserID != "dm-1" || p.Body != "Hi" {
		t.Fatalf("post mismatch: %+v", p)
	}
	if len(p.AttachmentAssetIDs) != 1 || p.AttachmentAssetIDs[0] != assetID {
		t.Fatalf("post attachments mismatch")
	}
	if !p.PostedAt.Equal(now) {
		t.Fatalf("posted_at mismatch")
	}
}

func TestDBStore_InsertNarrationPost_ErrorPropagates(t *testing.T) {
	fake := &fakeRefdata{insertErr: errors.New("boom")}
	store := NewDBStore(fake)
	_, err := store.InsertNarrationPost(context.Background(), InsertPostParams{
		CampaignID: uuid.New(), AuthorUserID: "u", Body: "b",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestDBStore_List_MapsResultsAndParams(t *testing.T) {
	campID := uuid.New()
	fake := &fakeRefdata{
		list: []refdata.NarrationPost{
			{ID: uuid.New(), CampaignID: campID, Body: "a", AttachmentAssetIds: []uuid.UUID{}, DiscordMessageIds: []string{"x"}},
			{ID: uuid.New(), CampaignID: campID, Body: "b", AttachmentAssetIds: []uuid.UUID{}, DiscordMessageIds: []string{}},
		},
	}
	store := NewDBStore(fake)
	got, err := store.ListNarrationPostsByCampaign(context.Background(), campID, 10, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.listArg.Limit != 10 || fake.listArg.Offset != 5 || fake.listArg.CampaignID != campID {
		t.Fatalf("list args wrong: %+v", fake.listArg)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[0].Body != "a" {
		t.Fatalf("first post body = %q", got[0].Body)
	}
}

func TestDBStore_List_ErrorPropagates(t *testing.T) {
	fake := &fakeRefdata{listErr: errors.New("boom")}
	store := NewDBStore(fake)
	_, err := store.ListNarrationPostsByCampaign(context.Background(), uuid.New(), 1, 0)
	if err == nil {
		t.Fatalf("expected error")
	}
}
