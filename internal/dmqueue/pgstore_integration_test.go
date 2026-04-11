package dmqueue_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

func seedCampaign(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := db.QueryRow(
		`INSERT INTO campaigns (guild_id, dm_user_id, name) VALUES ($1, $2, $3) RETURNING id`,
		"guild-"+uuid.NewString()[:8], "dm-user", "Test Campaign",
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func newPgStore(t *testing.T) (*dmqueue.PgStore, *sql.DB) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db := sharedDB.AcquireDB(t)
	q := refdata.New(db)
	return dmqueue.NewPgStore(q), db
}

func makeEvent(campaignID uuid.UUID) dmqueue.Event {
	return dmqueue.Event{
		Kind:        dmqueue.KindFreeformAction,
		PlayerName:  "Thorn",
		Summary:     `"flip the table"`,
		ResolvePath: "/dashboard/queue/x",
		GuildID:     "guild-1",
		CampaignID:  campaignID.String(),
	}
}

func TestPgStore_InsertAndGet(t *testing.T) {
	store, db := newPgStore(t)
	campaignID := seedCampaign(t, db)
	ctx := context.Background()

	id := dmqueue.NewItemID()
	posted := "🎭 **Action** — Thorn: \"flip the table\" — [Resolve →](/dashboard/queue/" + id + ")"
	got, err := store.Insert(ctx, id, makeEvent(campaignID), "chan-1", "msg-1", posted)
	require.NoError(t, err)
	assert.Equal(t, id, got.ID)
	assert.Equal(t, dmqueue.StatusPending, got.Status)
	assert.Equal(t, "chan-1", got.ChannelID)
	assert.Equal(t, "msg-1", got.MessageID)
	assert.Equal(t, posted, got.PostedText)
	assert.Equal(t, dmqueue.KindFreeformAction, got.Event.Kind)
	assert.Equal(t, campaignID.String(), got.Event.CampaignID)

	fetched, ok, err := store.Get(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, got.ID, fetched.ID)
	assert.Equal(t, posted, fetched.PostedText)
}

func TestPgStore_Get_NotFound(t *testing.T) {
	store, _ := newPgStore(t)
	ctx := context.Background()

	_, ok, err := store.Get(ctx, uuid.NewString())
	require.NoError(t, err)
	assert.False(t, ok)

	_, ok, err = store.Get(ctx, "not-a-uuid")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestPgStore_MarkResolved(t *testing.T) {
	store, db := newPgStore(t)
	campaignID := seedCampaign(t, db)
	ctx := context.Background()

	id := dmqueue.NewItemID()
	_, err := store.Insert(ctx, id, makeEvent(campaignID), "c", "m", "posted")
	require.NoError(t, err)

	updated, err := store.MarkResolved(ctx, id, "table is flipped")
	require.NoError(t, err)
	assert.Equal(t, dmqueue.StatusResolved, updated.Status)
	assert.Equal(t, "table is flipped", updated.Outcome)

	fetched, ok, err := store.Get(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, dmqueue.StatusResolved, fetched.Status)
	assert.Equal(t, "table is flipped", fetched.Outcome)
}

func TestPgStore_MarkResolved_NotFound(t *testing.T) {
	store, _ := newPgStore(t)
	ctx := context.Background()

	_, err := store.MarkResolved(ctx, uuid.NewString(), "x")
	assert.ErrorIs(t, err, dmqueue.ErrItemNotFound)

	_, err = store.MarkResolved(ctx, "garbage", "x")
	assert.ErrorIs(t, err, dmqueue.ErrItemNotFound)
}

func TestPgStore_MarkCancelled(t *testing.T) {
	store, db := newPgStore(t)
	campaignID := seedCampaign(t, db)
	ctx := context.Background()

	id := dmqueue.NewItemID()
	_, err := store.Insert(ctx, id, makeEvent(campaignID), "c", "m", "posted")
	require.NoError(t, err)

	updated, err := store.MarkCancelled(ctx, id, "Cancelled by player")
	require.NoError(t, err)
	assert.Equal(t, dmqueue.StatusCancelled, updated.Status)
	assert.Equal(t, "Cancelled by player", updated.Outcome)
}

func TestPgStore_MarkCancelled_NotFound(t *testing.T) {
	store, _ := newPgStore(t)
	_, err := store.MarkCancelled(context.Background(), uuid.NewString(), "x")
	assert.ErrorIs(t, err, dmqueue.ErrItemNotFound)
	_, err = store.MarkCancelled(context.Background(), "junk", "x")
	assert.ErrorIs(t, err, dmqueue.ErrItemNotFound)
}

func TestPgStore_ListPending(t *testing.T) {
	store, db := newPgStore(t)
	campaignID := seedCampaign(t, db)
	ctx := context.Background()

	id1 := dmqueue.NewItemID()
	id2 := dmqueue.NewItemID()
	id3 := dmqueue.NewItemID()
	_, err := store.Insert(ctx, id1, makeEvent(campaignID), "c", "m1", "posted-1")
	require.NoError(t, err)
	_, err = store.Insert(ctx, id2, makeEvent(campaignID), "c", "m2", "posted-2")
	require.NoError(t, err)
	_, err = store.Insert(ctx, id3, makeEvent(campaignID), "c", "m3", "posted-3")
	require.NoError(t, err)

	// Resolve one and cancel another so only id2 remains pending for this run.
	_, err = store.MarkResolved(ctx, id1, "done")
	require.NoError(t, err)
	_, err = store.MarkCancelled(ctx, id3, "nope")
	require.NoError(t, err)

	pending, err := store.ListPendingForCampaign(ctx, campaignID)
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, id2, pending[0].ID)

	// All pending across campaigns must include this one too.
	all, err := store.ListPending(ctx)
	require.NoError(t, err)
	found := false
	for _, item := range all {
		if item.ID == id2 {
			found = true
			break
		}
	}
	assert.True(t, found, "expected id2 in global pending list")
}

func TestPgStore_NotifierEndToEnd(t *testing.T) {
	store, db := newPgStore(t)
	campaignID := seedCampaign(t, db)

	sender := &recordingSender{}
	notifier := dmqueue.NewNotifierWithStore(
		sender,
		func(string) string { return "chan-1" },
		func(id string) string { return "/dashboard/queue/" + id },
		store,
	)

	itemID, err := notifier.Post(context.Background(), dmqueue.Event{
		Kind:       dmqueue.KindFreeformAction,
		PlayerName: "Thorn",
		Summary:    `"flip the table"`,
		GuildID:    "g1",
		CampaignID: campaignID.String(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, itemID)
	require.Len(t, sender.sends, 1)

	item, ok := notifier.Get(itemID)
	require.True(t, ok)
	assert.Equal(t, dmqueue.StatusPending, item.Status)

	require.NoError(t, notifier.Resolve(context.Background(), itemID, "table flipped"))
	require.Len(t, sender.edits, 1)
	item, _ = notifier.Get(itemID)
	assert.Equal(t, dmqueue.StatusResolved, item.Status)
	assert.Equal(t, "table flipped", item.Outcome)
}

type recordingSender struct {
	sends []sendCall
	edits []editCall
}

type sendCall struct{ ChannelID, Content string }
type editCall struct{ ChannelID, MessageID, Content string }

func (r *recordingSender) Send(ch, content string) (string, error) {
	r.sends = append(r.sends, sendCall{ch, content})
	return "msg-" + uuid.NewString()[:6], nil
}

func (r *recordingSender) Edit(ch, msg, content string) error {
	r.edits = append(r.edits, editCall{ch, msg, content})
	return nil
}
