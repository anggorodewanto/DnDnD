package messageplayer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeStore struct {
	inserted  []InsertParams
	insertErr error

	listArg struct {
		campaign uuid.UUID
		player   uuid.UUID
		limit    int
		offset   int
	}
	list    []Message
	listErr error
}

func (f *fakeStore) InsertDMMessage(ctx context.Context, p InsertParams) (Message, error) {
	if f.insertErr != nil {
		return Message{}, f.insertErr
	}
	f.inserted = append(f.inserted, p)
	return Message{
		ID:                uuid.New(),
		CampaignID:        p.CampaignID,
		PlayerCharacterID: p.PlayerCharacterID,
		AuthorUserID:      p.AuthorUserID,
		Body:              p.Body,
		DiscordMessageIDs: append([]string(nil), p.DiscordMessageIDs...),
		SentAt:            time.Now(),
	}, nil
}

func (f *fakeStore) ListDMMessages(ctx context.Context, campaignID, playerCharacterID uuid.UUID, limit, offset int) ([]Message, error) {
	f.listArg.campaign = campaignID
	f.listArg.player = playerCharacterID
	f.listArg.limit = limit
	f.listArg.offset = offset
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.list, nil
}

type fakeLookup struct {
	discordUserID string
	campaignID    uuid.UUID
	rowID         uuid.UUID
	err           error
	called        uuid.UUID
}

func (f *fakeLookup) LookupPlayer(ctx context.Context, characterID uuid.UUID) (PlayerInfo, error) {
	f.called = characterID
	if f.err != nil {
		return PlayerInfo{}, f.err
	}
	return PlayerInfo{DiscordUserID: f.discordUserID, CampaignID: f.campaignID, RowID: f.rowID}, nil
}

type fakeMessenger struct {
	calls []messengerCall
	ids   []string
	err   error
}

type messengerCall struct {
	userID string
	body   string
}

func (f *fakeMessenger) SendDirectMessage(discordUserID, body string) ([]string, error) {
	f.calls = append(f.calls, messengerCall{discordUserID, body})
	if f.err != nil {
		return nil, f.err
	}
	return f.ids, nil
}

func newSvc(store Store, lookup PlayerLookup, messenger Messenger) *Service {
	return NewService(store, lookup, messenger)
}

func TestService_Send_Success(t *testing.T) {
	campID := uuid.New()
	charID := uuid.New() // what the dashboard sends (characters.id)
	rowID := uuid.New()  // the resolved player_characters PK (FK target)
	store := &fakeStore{}
	lookup := &fakeLookup{discordUserID: "user-42", campaignID: campID, rowID: rowID}
	messenger := &fakeMessenger{ids: []string{"m1", "m2"}}

	svc := newSvc(store, lookup, messenger)
	msg, err := svc.SendMessage(context.Background(), SendMessageInput{
		CampaignID:        campID,
		PlayerCharacterID: charID,
		AuthorUserID:      "dm-1",
		Body:              "psst, you notice something",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(messenger.calls) != 1 || messenger.calls[0].userID != "user-42" {
		t.Fatalf("messenger = %+v", messenger.calls)
	}
	if lookup.called != charID {
		t.Fatalf("lookup should receive the character_id %s, got %s", charID, lookup.called)
	}
	if len(store.inserted) != 1 {
		t.Fatalf("expected 1 insert, got %d", len(store.inserted))
	}
	ins := store.inserted[0]
	// The log row must store the player_characters PK (FK target), not the
	// character_id the dashboard sent — else the insert violates the FK.
	if ins.CampaignID != campID || ins.PlayerCharacterID != rowID {
		t.Fatalf("ids mismatch: %+v (want player_character_id %s)", ins, rowID)
	}
	if len(ins.DiscordMessageIDs) != 2 || ins.DiscordMessageIDs[0] != "m1" {
		t.Fatalf("discord ids = %v", ins.DiscordMessageIDs)
	}
	if msg.Body != "psst, you notice something" {
		t.Fatalf("returned body = %q", msg.Body)
	}
}

func TestService_Send_RejectsEmptyBody(t *testing.T) {
	svc := newSvc(&fakeStore{}, &fakeLookup{}, &fakeMessenger{})
	_, err := svc.SendMessage(context.Background(), SendMessageInput{
		CampaignID:        uuid.New(),
		PlayerCharacterID: uuid.New(),
		AuthorUserID:      "dm-1",
		Body:              "   ",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestService_Send_RejectsEmptyAuthor(t *testing.T) {
	svc := newSvc(&fakeStore{}, &fakeLookup{}, &fakeMessenger{})
	_, err := svc.SendMessage(context.Background(), SendMessageInput{
		CampaignID:        uuid.New(),
		PlayerCharacterID: uuid.New(),
		AuthorUserID:      "",
		Body:              "hi",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestService_Send_RejectsNilCampaign(t *testing.T) {
	svc := newSvc(&fakeStore{}, &fakeLookup{}, &fakeMessenger{})
	_, err := svc.SendMessage(context.Background(), SendMessageInput{
		PlayerCharacterID: uuid.New(), AuthorUserID: "dm", Body: "hi",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestService_Send_RejectsNilPlayerCharacter(t *testing.T) {
	svc := newSvc(&fakeStore{}, &fakeLookup{}, &fakeMessenger{})
	_, err := svc.SendMessage(context.Background(), SendMessageInput{
		CampaignID: uuid.New(), AuthorUserID: "dm", Body: "hi",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestService_Send_NilMessengerReturnsUnavailable(t *testing.T) {
	svc := NewService(&fakeStore{}, &fakeLookup{}, nil)
	_, err := svc.SendMessage(context.Background(), SendMessageInput{
		CampaignID:        uuid.New(),
		PlayerCharacterID: uuid.New(),
		AuthorUserID:      "dm",
		Body:              "hi",
	})
	if !errors.Is(err, ErrMessengerUnavailable) {
		t.Fatalf("expected ErrMessengerUnavailable, got %v", err)
	}
}

func TestService_Send_PlayerNotFound(t *testing.T) {
	lookup := &fakeLookup{err: ErrPlayerNotFound}
	svc := newSvc(&fakeStore{}, lookup, &fakeMessenger{})
	_, err := svc.SendMessage(context.Background(), SendMessageInput{
		CampaignID:        uuid.New(),
		PlayerCharacterID: uuid.New(),
		AuthorUserID:      "dm",
		Body:              "hi",
	})
	if !errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("expected ErrPlayerNotFound, got %v", err)
	}
}

func TestService_Send_CampaignMismatchRejected(t *testing.T) {
	// Lookup returns a PC whose campaign doesn't match the request.
	lookup := &fakeLookup{discordUserID: "u", campaignID: uuid.New()}
	svc := newSvc(&fakeStore{}, lookup, &fakeMessenger{})
	_, err := svc.SendMessage(context.Background(), SendMessageInput{
		CampaignID:        uuid.New(),
		PlayerCharacterID: uuid.New(),
		AuthorUserID:      "dm",
		Body:              "hi",
	})
	if !errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("expected ErrPlayerNotFound for mismatch, got %v", err)
	}
}

func TestService_Send_MessengerFailureNotRecorded(t *testing.T) {
	campID := uuid.New()
	store := &fakeStore{}
	lookup := &fakeLookup{discordUserID: "user", campaignID: campID}
	messenger := &fakeMessenger{err: errors.New("discord down")}

	svc := newSvc(store, lookup, messenger)
	_, err := svc.SendMessage(context.Background(), SendMessageInput{
		CampaignID:        campID,
		PlayerCharacterID: uuid.New(),
		AuthorUserID:      "dm",
		Body:              "hi",
	})
	if err == nil || !strings.Contains(err.Error(), "discord down") {
		t.Fatalf("expected discord error, got %v", err)
	}
	if len(store.inserted) != 0 {
		t.Fatalf("must not record on send failure")
	}
}

func TestService_Send_StoreFailurePropagates(t *testing.T) {
	campID := uuid.New()
	store := &fakeStore{insertErr: errors.New("db down")}
	lookup := &fakeLookup{discordUserID: "user", campaignID: campID}
	messenger := &fakeMessenger{ids: []string{"m"}}
	svc := newSvc(store, lookup, messenger)
	_, err := svc.SendMessage(context.Background(), SendMessageInput{
		CampaignID:        campID,
		PlayerCharacterID: uuid.New(),
		AuthorUserID:      "dm",
		Body:              "hi",
	})
	if err == nil || !strings.Contains(err.Error(), "db down") {
		t.Fatalf("expected store error, got %v", err)
	}
}

func TestService_History_Delegates(t *testing.T) {
	campID := uuid.New()
	charID := uuid.New() // dashboard sends the character_id
	rowID := uuid.New()  // resolved player_characters PK (how rows are stored)
	store := &fakeStore{list: []Message{{ID: uuid.New(), Body: "h1"}}}
	lookup := &fakeLookup{campaignID: campID, rowID: rowID}
	svc := newSvc(store, lookup, &fakeMessenger{})

	got, err := svc.History(context.Background(), campID, charID, 10, 5)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if lookup.called != charID {
		t.Fatalf("history lookup should receive character_id %s, got %s", charID, lookup.called)
	}
	// History must query by the resolved PK, not the character_id.
	if store.listArg.campaign != campID || store.listArg.player != rowID ||
		store.listArg.limit != 10 || store.listArg.offset != 5 {
		t.Fatalf("list args wrong: %+v (want player %s)", store.listArg, rowID)
	}
	if len(got) != 1 || got[0].Body != "h1" {
		t.Fatalf("history = %+v", got)
	}
}

func TestService_History_PlayerNotFoundReturnsEmpty(t *testing.T) {
	store := &fakeStore{}
	lookup := &fakeLookup{err: ErrPlayerNotFound}
	svc := newSvc(store, lookup, &fakeMessenger{})
	got, err := svc.History(context.Background(), uuid.New(), uuid.New(), 10, 0)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no history for unknown player, got %+v", got)
	}
}

func TestService_History_DefaultsLimit(t *testing.T) {
	store := &fakeStore{}
	svc := newSvc(store, &fakeLookup{}, &fakeMessenger{})
	_, err := svc.History(context.Background(), uuid.New(), uuid.New(), -1, -1)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if store.listArg.limit != 20 || store.listArg.offset != 0 {
		t.Fatalf("expected normalized limit/offset, got %+v", store.listArg)
	}
}

func TestService_History_StoreError(t *testing.T) {
	store := &fakeStore{listErr: errors.New("db down")}
	svc := newSvc(store, &fakeLookup{}, &fakeMessenger{})
	_, err := svc.History(context.Background(), uuid.New(), uuid.New(), 5, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}
