package discord

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// --- Mock InitiativeStagingStore ---

type mockInitiativeStore struct {
	upsertFn func(ctx context.Context, campaignID, characterID uuid.UUID, roll int32, dmQueueItemID string) error
	getFn    func(ctx context.Context, campaignID, characterID uuid.UUID) (int32, string, bool, error)
	deleteFn func(ctx context.Context, campaignID, characterID uuid.UUID) error

	upsertedRoll   int32
	upsertedItemID string
	upsertCalls    int
	deleteCalls    int
}

func (m *mockInitiativeStore) UpsertPendingInitiative(ctx context.Context, campaignID, characterID uuid.UUID, roll int32, dmQueueItemID string) error {
	m.upsertCalls++
	m.upsertedRoll = roll
	m.upsertedItemID = dmQueueItemID
	if m.upsertFn != nil {
		return m.upsertFn(ctx, campaignID, characterID, roll, dmQueueItemID)
	}
	return nil
}

func (m *mockInitiativeStore) GetPendingInitiative(ctx context.Context, campaignID, characterID uuid.UUID) (int32, string, bool, error) {
	if m.getFn != nil {
		return m.getFn(ctx, campaignID, characterID)
	}
	return 0, "", false, nil
}

func (m *mockInitiativeStore) DeletePendingInitiative(ctx context.Context, campaignID, characterID uuid.UUID) error {
	m.deleteCalls++
	if m.deleteFn != nil {
		return m.deleteFn(ctx, campaignID, characterID)
	}
	return nil
}

// --- Interaction builder ---

func makeInitiativeInteraction(opts map[string]any) *discordgo.Interaction {
	var dopts []*discordgo.ApplicationCommandInteractionDataOption
	if v, ok := opts["roll"]; ok {
		dopts = append(dopts, &discordgo.ApplicationCommandInteractionDataOption{
			Name: "roll", Value: float64(v.(int)), Type: discordgo.ApplicationCommandOptionInteger,
		})
	}
	if v, ok := opts["clear"]; ok {
		dopts = append(dopts, &discordgo.ApplicationCommandInteractionDataOption{
			Name: "clear", Value: v.(bool), Type: discordgo.ApplicationCommandOptionBoolean,
		})
	}
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "initiative",
			Options: dopts,
		},
	}
}

func setupInitiativeHandler(sess *MockSession, store InitiativeStagingStore) *InitiativeHandler {
	campaignID := uuid.New()
	char := makeTestCharacter()
	char.CampaignID = campaignID
	return NewInitiativeHandler(
		sess,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return char, nil
		}},
		store,
	)
}

func TestInitiativeHandler_RecordsRoll(t *testing.T) {
	sess := newTestMock()
	rc := captureResponse(sess)

	store := &mockInitiativeStore{}
	h := setupInitiativeHandler(sess, store)
	h.Handle(makeInitiativeInteraction(map[string]any{"roll": 17}))

	if store.upsertCalls != 1 {
		t.Fatalf("expected 1 upsert, got %d", store.upsertCalls)
	}
	if store.upsertedRoll != 17 {
		t.Errorf("expected staged roll 17, got %d", store.upsertedRoll)
	}
	if !strings.Contains(rc.Content, "17") || !strings.Contains(rc.Content, "Aria") {
		t.Errorf("expected confirmation with roll + name, got: %s", rc.Content)
	}
	if rc.Flags&discordgo.MessageFlagsEphemeral != 0 {
		t.Errorf("expected the recorded-initiative confirmation to be public (non-ephemeral), got flags %d", rc.Flags)
	}
}

func TestInitiativeHandler_ClearRemovesStagedValue(t *testing.T) {
	sess := newTestMock()
	rc := captureResponse(sess)

	store := &mockInitiativeStore{}
	h := setupInitiativeHandler(sess, store)
	h.Handle(makeInitiativeInteraction(map[string]any{"clear": true}))

	if store.deleteCalls != 1 {
		t.Fatalf("expected 1 delete, got %d", store.deleteCalls)
	}
	if store.upsertCalls != 0 {
		t.Errorf("clear must not upsert, got %d upserts", store.upsertCalls)
	}
	if !strings.Contains(strings.ToLower(rc.Content), "cleared") {
		t.Errorf("expected a cleared confirmation, got: %s", rc.Content)
	}
	if rc.Flags&discordgo.MessageFlagsEphemeral != 0 {
		t.Errorf("expected the cleared confirmation to be public (non-ephemeral), got flags %d", rc.Flags)
	}
}

func TestInitiativeHandler_ShowsCurrentWhenStaged(t *testing.T) {
	sess := newTestMock()
	rc := captureResponse(sess)

	store := &mockInitiativeStore{
		getFn: func(_ context.Context, _, _ uuid.UUID) (int32, string, bool, error) { return 14, "", true, nil },
	}
	h := setupInitiativeHandler(sess, store)
	h.Handle(makeInitiativeInteraction(map[string]any{}))

	if store.upsertCalls != 0 || store.deleteCalls != 0 {
		t.Errorf("bare /initiative must not write; got %d upserts, %d deletes", store.upsertCalls, store.deleteCalls)
	}
	if !strings.Contains(rc.Content, "14") {
		t.Errorf("expected the staged value 14 echoed, got: %s", rc.Content)
	}
	if rc.Flags&discordgo.MessageFlagsEphemeral != 0 {
		t.Errorf("expected the staged-value echo to be public (non-ephemeral), got flags %d", rc.Flags)
	}
}

func TestInitiativeHandler_PromptsWhenNoneStaged(t *testing.T) {
	sess := newTestMock()
	rc := captureResponse(sess)

	store := &mockInitiativeStore{
		getFn: func(_ context.Context, _, _ uuid.UUID) (int32, string, bool, error) { return 0, "", false, nil },
	}
	h := setupInitiativeHandler(sess, store)
	h.Handle(makeInitiativeInteraction(map[string]any{}))

	if !strings.Contains(strings.ToLower(rc.Content), "haven't staged") {
		t.Errorf("expected a prompt to stage, got: %s", rc.Content)
	}
	if rc.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Errorf("expected the empty-state prompt to stay ephemeral, got flags %d", rc.Flags)
	}
}

func TestInitiativeHandler_RejectsBothRollAndClear(t *testing.T) {
	sess := newTestMock()
	rc := captureResponse(sess)

	store := &mockInitiativeStore{}
	h := setupInitiativeHandler(sess, store)
	h.Handle(makeInitiativeInteraction(map[string]any{"roll": 17, "clear": true}))

	if store.upsertCalls != 0 || store.deleteCalls != 0 {
		t.Errorf("a conflicting request must write nothing; got %d upserts, %d deletes", store.upsertCalls, store.deleteCalls)
	}
	if !strings.Contains(rc.Content, "not both") {
		t.Errorf("expected a both-flags rejection, got: %s", rc.Content)
	}
	if rc.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Errorf("expected a validation error to stay ephemeral, got flags %d", rc.Flags)
	}
}

func TestInitiativeHandler_RejectsOutOfRange(t *testing.T) {
	sess := newTestMock()
	rc := captureResponse(sess)

	store := &mockInitiativeStore{}
	h := setupInitiativeHandler(sess, store)
	h.Handle(makeInitiativeInteraction(map[string]any{"roll": 99}))

	if store.upsertCalls != 0 {
		t.Errorf("an out-of-range total must not be staged, got %d upserts", store.upsertCalls)
	}
	if !strings.Contains(rc.Content, "between") {
		t.Errorf("expected a range error, got: %s", rc.Content)
	}
	if rc.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Errorf("expected a range error to stay ephemeral, got flags %d", rc.Flags)
	}
}

// --- APP-5 pre-combat DM-queue surfacing ---

// A fresh stage posts one initiative_staged item to #dm-queue and stores the
// returned item id on the staging row.
func TestInitiativeHandler_StagePostsQueueItem(t *testing.T) {
	sess := newTestMock()
	captureResponse(sess)

	store := &mockInitiativeStore{}
	h := setupInitiativeHandler(sess, store)
	n := &cancelRecordingNotifier{nextItemID: "queued-1"}
	h.SetNotifier(n)

	h.Handle(makeInitiativeInteraction(map[string]any{"roll": 12}))

	if len(n.posted) != 1 {
		t.Fatalf("expected exactly 1 dm-queue post, got %d", len(n.posted))
	}
	if n.posted[0].Kind != dmqueue.KindInitiativeStaged {
		t.Errorf("expected kind %q, got %q", dmqueue.KindInitiativeStaged, n.posted[0].Kind)
	}
	if !strings.Contains(n.posted[0].Summary, "12") {
		t.Errorf("expected the posted summary to carry the roll, got %q", n.posted[0].Summary)
	}
	if n.posted[0].CampaignID == "" {
		t.Errorf("expected CampaignID on the event so PgStore.Insert can persist it")
	}
	if len(n.cancelCalls) != 0 {
		t.Errorf("a fresh stage must not cancel anything, got %d cancels", len(n.cancelCalls))
	}
	if store.upsertedItemID != "queued-1" {
		t.Errorf("expected the posted item id stored on the row, got %q", store.upsertedItemID)
	}
}

// Re-rolling cancels the prior staged item, then posts a fresh one.
func TestInitiativeHandler_ReRollCancelsPriorThenPosts(t *testing.T) {
	sess := newTestMock()
	captureResponse(sess)

	store := &mockInitiativeStore{
		getFn: func(_ context.Context, _, _ uuid.UUID) (int32, string, bool, error) {
			return 8, "old-item", true, nil
		},
	}
	h := setupInitiativeHandler(sess, store)
	n := &cancelRecordingNotifier{nextItemID: "queued-2"}
	h.SetNotifier(n)

	h.Handle(makeInitiativeInteraction(map[string]any{"roll": 19}))

	if len(n.cancelCalls) != 1 || n.cancelCalls[0].itemID != "old-item" {
		t.Fatalf("expected the prior item 'old-item' cancelled, got %+v", n.cancelCalls)
	}
	if len(n.posted) != 1 {
		t.Fatalf("expected a fresh post after the cancel, got %d posts", len(n.posted))
	}
	if store.upsertedItemID != "queued-2" {
		t.Errorf("expected the NEW item id stored on the row, got %q", store.upsertedItemID)
	}
}

// Clearing cancels the prior staged item and does not post.
func TestInitiativeHandler_ClearCancelsQueueItem(t *testing.T) {
	sess := newTestMock()
	captureResponse(sess)

	store := &mockInitiativeStore{
		getFn: func(_ context.Context, _, _ uuid.UUID) (int32, string, bool, error) {
			return 8, "old-item", true, nil
		},
	}
	h := setupInitiativeHandler(sess, store)
	n := &cancelRecordingNotifier{}
	h.SetNotifier(n)

	h.Handle(makeInitiativeInteraction(map[string]any{"clear": true}))

	if len(n.cancelCalls) != 1 || n.cancelCalls[0].itemID != "old-item" {
		t.Fatalf("expected the staged item 'old-item' cancelled on clear, got %+v", n.cancelCalls)
	}
	if len(n.posted) != 0 {
		t.Errorf("clear must not post a new item, got %d posts", len(n.posted))
	}
	if store.deleteCalls != 1 {
		t.Errorf("expected the staging row deleted, got %d deletes", store.deleteCalls)
	}
}

func TestInitiativeHandler_NoCharacter(t *testing.T) {
	sess := newTestMock()
	rc := captureResponse(sess)

	campaignID := uuid.New()
	store := &mockInitiativeStore{}
	h := NewInitiativeHandler(
		sess,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return refdata.Character{}, errors.New("not found")
		}},
		store,
	)
	h.Handle(makeInitiativeInteraction(map[string]any{"roll": 17}))

	if store.upsertCalls != 0 {
		t.Errorf("no character → no write, got %d upserts", store.upsertCalls)
	}
	if !strings.Contains(strings.ToLower(rc.Content), "character") {
		t.Errorf("expected a missing-character error, got: %s", rc.Content)
	}
	if rc.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Errorf("expected a lookup error to stay ephemeral, got flags %d", rc.Flags)
	}
}
