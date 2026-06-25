package main

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// fakeTrackerCSP is a minimal CampaignSettingsProvider returning a fixed
// channel map for the #initiative-tracker lookups.
type fakeTrackerCSP struct {
	channels map[string]string
	err      error
}

func (f fakeTrackerCSP) GetChannelIDs(_ context.Context, _ uuid.UUID) (map[string]string, error) {
	return f.channels, f.err
}

// fakeTrackerStore records the persistence calls the notifier makes and lets a
// test seed a "previously persisted" message id (the restart case).
type fakeTrackerStore struct {
	getRow    refdata.InitiativeTrackerMessage
	getErr    error
	upserts   []refdata.UpsertInitiativeTrackerMessageParams
	deletes   []uuid.UUID
	upsertErr error
}

func (s *fakeTrackerStore) GetInitiativeTrackerMessage(_ context.Context, _ uuid.UUID) (refdata.InitiativeTrackerMessage, error) {
	return s.getRow, s.getErr
}

func (s *fakeTrackerStore) UpsertInitiativeTrackerMessage(_ context.Context, arg refdata.UpsertInitiativeTrackerMessageParams) error {
	s.upserts = append(s.upserts, arg)
	return s.upsertErr
}

func (s *fakeTrackerStore) DeleteInitiativeTrackerMessage(_ context.Context, encounterID uuid.UUID) error {
	s.deletes = append(s.deletes, encounterID)
	return nil
}

func trackerCSP() fakeTrackerCSP {
	return fakeTrackerCSP{channels: map[string]string{"initiative-tracker": "chan-1"}}
}

// The core regression: a notifier with NO in-memory state (fresh process after
// a restart) must still edit the persisted message in place rather than post a
// duplicate, because the (channel,message) pair now lives in the DB.
func TestInitiativeTrackerNotifier_UpdateEditsPersistedMessageAcrossRestart(t *testing.T) {
	enc := uuid.New()
	var editCh, editMsg, editContent string
	editCount, sendCount := 0, 0
	sess := &testSession{
		editFunc: func(channelID, messageID, content string) (*discordgo.Message, error) {
			editCount++
			editCh, editMsg, editContent = channelID, messageID, content
			return &discordgo.Message{ID: messageID}, nil
		},
		sendFunc: func(channelID, content string) (*discordgo.Message, error) {
			sendCount++
			return &discordgo.Message{ID: "should-not-happen"}, nil
		},
	}
	store := &fakeTrackerStore{getRow: refdata.InitiativeTrackerMessage{
		EncounterID: enc, ChannelID: "chan-1", MessageID: "msg-1",
	}}

	n := newInitiativeTrackerNotifier(sess, trackerCSP(), store)
	if n == nil {
		t.Fatal("expected non-nil notifier")
	}
	n.UpdateTracker(context.Background(), enc, "round 1 — Vale's turn")

	if editCount != 1 {
		t.Fatalf("expected exactly one edit, got %d", editCount)
	}
	if sendCount != 0 {
		t.Fatalf("expected no fresh post (no duplicate), got %d sends", sendCount)
	}
	if editCh != "chan-1" || editMsg != "msg-1" {
		t.Fatalf("edited wrong target: ch=%q msg=%q", editCh, editMsg)
	}
	if editContent != "round 1 — Vale's turn" {
		t.Fatalf("edited wrong content: %q", editContent)
	}
}

func TestInitiativeTrackerNotifier_UpdatePostsAndRecordsWhenNoPersistedMessage(t *testing.T) {
	enc := uuid.New()
	sendCount, editCount := 0, 0
	sess := &testSession{
		sendFunc: func(channelID, content string) (*discordgo.Message, error) {
			sendCount++
			return &discordgo.Message{ID: "msg-new"}, nil
		},
		editFunc: func(channelID, messageID, content string) (*discordgo.Message, error) {
			editCount++
			return &discordgo.Message{}, nil
		},
	}
	store := &fakeTrackerStore{getErr: sql.ErrNoRows}

	n := newInitiativeTrackerNotifier(sess, trackerCSP(), store)
	n.UpdateTracker(context.Background(), enc, "content")

	if editCount != 0 {
		t.Fatalf("expected no edit when nothing persisted, got %d", editCount)
	}
	if sendCount != 1 {
		t.Fatalf("expected one fresh post, got %d", sendCount)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("expected one upsert recording the new message, got %d", len(store.upserts))
	}
	if got := store.upserts[0]; got.EncounterID != enc || got.ChannelID != "chan-1" || got.MessageID != "msg-new" {
		t.Fatalf("upsert recorded wrong values: %+v", got)
	}
}

// If the persisted message was deleted out from under us, the edit fails and we
// re-post + re-record so the channel still reflects the live turn.
func TestInitiativeTrackerNotifier_UpdateRepostsWhenEditFails(t *testing.T) {
	enc := uuid.New()
	sendCount := 0
	sess := &testSession{
		editFunc: func(channelID, messageID, content string) (*discordgo.Message, error) {
			return nil, errors.New("unknown message")
		},
		sendFunc: func(channelID, content string) (*discordgo.Message, error) {
			sendCount++
			return &discordgo.Message{ID: "msg-2"}, nil
		},
	}
	store := &fakeTrackerStore{getRow: refdata.InitiativeTrackerMessage{
		EncounterID: enc, ChannelID: "chan-1", MessageID: "msg-gone",
	}}

	n := newInitiativeTrackerNotifier(sess, trackerCSP(), store)
	n.UpdateTracker(context.Background(), enc, "content")

	if sendCount != 1 {
		t.Fatalf("expected one re-post after a failed edit, got %d", sendCount)
	}
	if len(store.upserts) != 1 || store.upserts[0].MessageID != "msg-2" {
		t.Fatalf("expected upsert with the new message id, got %+v", store.upserts)
	}
}

func TestInitiativeTrackerNotifier_PostTrackerRecordsMessageID(t *testing.T) {
	enc := uuid.New()
	sess := &testSession{
		sendFunc: func(channelID, content string) (*discordgo.Message, error) {
			return &discordgo.Message{ID: "msg-1"}, nil
		},
	}
	store := &fakeTrackerStore{}

	n := newInitiativeTrackerNotifier(sess, trackerCSP(), store)
	n.PostTracker(context.Background(), enc, "content")

	if len(store.upserts) != 1 {
		t.Fatalf("expected one upsert, got %d", len(store.upserts))
	}
	if got := store.upserts[0]; got.EncounterID != enc || got.ChannelID != "chan-1" || got.MessageID != "msg-1" {
		t.Fatalf("upsert recorded wrong values: %+v", got)
	}
}

func TestInitiativeTrackerNotifier_PostCompletedDeletesMapping(t *testing.T) {
	enc := uuid.New()
	sendCount := 0
	sess := &testSession{
		sendFunc: func(channelID, content string) (*discordgo.Message, error) {
			sendCount++
			return &discordgo.Message{ID: "final"}, nil
		},
	}
	store := &fakeTrackerStore{}

	n := newInitiativeTrackerNotifier(sess, trackerCSP(), store)
	n.PostCompletedTracker(context.Background(), enc, "combat over")

	if sendCount != 1 {
		t.Fatalf("expected the final summary to post once, got %d", sendCount)
	}
	if len(store.deletes) != 1 || store.deletes[0] != enc {
		t.Fatalf("expected the mapping to be deleted, got %+v", store.deletes)
	}
}

func TestNewInitiativeTrackerNotifier_NilDepsReturnNil(t *testing.T) {
	sess := &testSession{}
	csp := trackerCSP()
	store := &fakeTrackerStore{}
	if newInitiativeTrackerNotifier(nil, csp, store) != nil {
		t.Error("nil session should yield nil notifier")
	}
	if newInitiativeTrackerNotifier(sess, nil, store) != nil {
		t.Error("nil csp should yield nil notifier")
	}
	if newInitiativeTrackerNotifier(sess, csp, nil) != nil {
		t.Error("nil store should yield nil notifier")
	}
}
