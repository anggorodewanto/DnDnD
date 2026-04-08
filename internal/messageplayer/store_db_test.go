package messageplayer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

type fakeRefdata struct {
	insertArg refdata.InsertDMPlayerMessageParams
	inserted  refdata.DmPlayerMessage
	insertErr error

	listArg refdata.ListDMPlayerMessagesParams
	list    []refdata.DmPlayerMessage
	listErr error
}

func (f *fakeRefdata) InsertDMPlayerMessage(ctx context.Context, arg refdata.InsertDMPlayerMessageParams) (refdata.DmPlayerMessage, error) {
	f.insertArg = arg
	if f.insertErr != nil {
		return refdata.DmPlayerMessage{}, f.insertErr
	}
	return f.inserted, nil
}

func (f *fakeRefdata) ListDMPlayerMessages(ctx context.Context, arg refdata.ListDMPlayerMessagesParams) ([]refdata.DmPlayerMessage, error) {
	f.listArg = arg
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.list, nil
}

func TestDBStore_Insert_MapsFieldsBothWays(t *testing.T) {
	campID := uuid.New()
	pcID := uuid.New()
	now := time.Now()
	fake := &fakeRefdata{inserted: refdata.DmPlayerMessage{
		ID:                uuid.New(),
		CampaignID:        campID,
		PlayerCharacterID: pcID,
		AuthorUserID:      "dm-1",
		Body:              "hi",
		DiscordMessageIds: []string{"m1"},
		SentAt:            now,
	}}
	store := NewDBStore(fake)

	msg, err := store.InsertDMMessage(context.Background(), InsertParams{
		CampaignID:        campID,
		PlayerCharacterID: pcID,
		AuthorUserID:      "dm-1",
		Body:              "hi",
		DiscordMessageIDs: []string{"m1"},
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if fake.insertArg.CampaignID != campID || fake.insertArg.PlayerCharacterID != pcID {
		t.Fatalf("arg ids wrong: %+v", fake.insertArg)
	}
	if len(fake.insertArg.DiscordMessageIds) != 1 || fake.insertArg.DiscordMessageIds[0] != "m1" {
		t.Fatalf("arg ids wrong: %+v", fake.insertArg.DiscordMessageIds)
	}
	if msg.Body != "hi" || msg.AuthorUserID != "dm-1" {
		t.Fatalf("msg = %+v", msg)
	}
	if !msg.SentAt.Equal(now) {
		t.Fatalf("sent_at mismatch")
	}
}

func TestDBStore_Insert_ErrorPropagates(t *testing.T) {
	fake := &fakeRefdata{insertErr: errors.New("boom")}
	store := NewDBStore(fake)
	_, err := store.InsertDMMessage(context.Background(), InsertParams{
		CampaignID: uuid.New(), PlayerCharacterID: uuid.New(), AuthorUserID: "u", Body: "b",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDBStore_List_MapsResultsAndParams(t *testing.T) {
	campID := uuid.New()
	pcID := uuid.New()
	fake := &fakeRefdata{list: []refdata.DmPlayerMessage{
		{ID: uuid.New(), CampaignID: campID, PlayerCharacterID: pcID, Body: "a", DiscordMessageIds: []string{"x"}},
		{ID: uuid.New(), CampaignID: campID, PlayerCharacterID: pcID, Body: "b", DiscordMessageIds: []string{}},
	}}
	store := NewDBStore(fake)

	got, err := store.ListDMMessages(context.Background(), campID, pcID, 10, 5)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if fake.listArg.CampaignID != campID || fake.listArg.PlayerCharacterID != pcID ||
		fake.listArg.Limit != 10 || fake.listArg.Offset != 5 {
		t.Fatalf("list args wrong: %+v", fake.listArg)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[0].Body != "a" {
		t.Fatalf("first body = %q", got[0].Body)
	}
	if got[1].DiscordMessageIDs == nil {
		t.Fatal("expected empty slice, got nil")
	}
}

func TestDBStore_List_ErrorPropagates(t *testing.T) {
	fake := &fakeRefdata{listErr: errors.New("boom")}
	store := NewDBStore(fake)
	_, err := store.ListDMMessages(context.Background(), uuid.New(), uuid.New(), 1, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}
