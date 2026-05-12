package dmqueue

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// fakePgQueries is a unit-test double for pgQueries that exercises pgstore.go
// error branches without spinning up PostgreSQL.
type fakePgQueries struct {
	insertErr error
	getErr    error
	getRow    refdata.DmQueueItem
	resolveErr error
	cancelErr  error
	listErr    error
	updateMsgErr error
}

func (f *fakePgQueries) InsertDMQueueItem(_ context.Context, arg refdata.InsertDMQueueItemParams) (refdata.DmQueueItem, error) {
	if f.insertErr != nil {
		return refdata.DmQueueItem{}, f.insertErr
	}
	return refdata.DmQueueItem{
		ID:          arg.ID,
		CampaignID:  arg.CampaignID,
		GuildID:     arg.GuildID,
		ChannelID:   arg.ChannelID,
		MessageID:   arg.MessageID,
		Kind:        arg.Kind,
		PlayerName:  arg.PlayerName,
		Summary:     arg.Summary,
		ResolvePath: arg.ResolvePath,
		Status:      string(StatusPending),
		Extra:       arg.Extra,
	}, nil
}

func (f *fakePgQueries) GetDMQueueItem(_ context.Context, _ uuid.UUID) (refdata.DmQueueItem, error) {
	if f.getErr != nil {
		return refdata.DmQueueItem{}, f.getErr
	}
	return f.getRow, nil
}

func (f *fakePgQueries) ListAllPendingDMQueueItems(_ context.Context) ([]refdata.DmQueueItem, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return nil, nil
}

func (f *fakePgQueries) ListPendingDMQueueItems(_ context.Context, _ uuid.UUID) ([]refdata.DmQueueItem, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return nil, nil
}

func (f *fakePgQueries) MarkDMQueueItemResolved(_ context.Context, _ refdata.MarkDMQueueItemResolvedParams) (refdata.DmQueueItem, error) {
	if f.resolveErr != nil {
		return refdata.DmQueueItem{}, f.resolveErr
	}
	return refdata.DmQueueItem{ID: uuid.New(), Status: string(StatusResolved)}, nil
}

func (f *fakePgQueries) MarkDMQueueItemCancelled(_ context.Context, _ refdata.MarkDMQueueItemCancelledParams) (refdata.DmQueueItem, error) {
	if f.cancelErr != nil {
		return refdata.DmQueueItem{}, f.cancelErr
	}
	return refdata.DmQueueItem{ID: uuid.New(), Status: string(StatusCancelled)}, nil
}

func (f *fakePgQueries) UpdateDMQueueItemMessageID(_ context.Context, arg refdata.UpdateDMQueueItemMessageIDParams) (refdata.DmQueueItem, error) {
	if f.updateMsgErr != nil {
		return refdata.DmQueueItem{}, f.updateMsgErr
	}
	return refdata.DmQueueItem{ID: arg.ID, MessageID: arg.MessageID, Status: string(StatusPending)}, nil
}

func TestPgStore_Insert_BadIDOrCampaign(t *testing.T) {
	store := NewPgStore(&fakePgQueries{})

	_, err := store.Insert(context.Background(), "not-a-uuid", Event{CampaignID: uuid.NewString()}, "c", "m", "p")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse id")

	_, err = store.Insert(context.Background(), uuid.NewString(), Event{CampaignID: "bad"}, "c", "m", "p")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse campaign id")
}

func TestPgStore_Insert_DBError(t *testing.T) {
	fake := &fakePgQueries{insertErr: errors.New("boom")}
	store := NewPgStore(fake)
	_, err := store.Insert(context.Background(), uuid.NewString(), Event{CampaignID: uuid.NewString()}, "c", "m", "p")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestPgStore_Get_DBError(t *testing.T) {
	fake := &fakePgQueries{getErr: errors.New("db down")}
	store := NewPgStore(fake)
	_, _, err := store.Get(context.Background(), uuid.NewString())
	require.Error(t, err)
}

func TestPgStore_Get_NoRowsAsNotFound(t *testing.T) {
	fake := &fakePgQueries{getErr: sql.ErrNoRows}
	store := NewPgStore(fake)
	_, ok, err := store.Get(context.Background(), uuid.NewString())
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestPgStore_MarkResolved_DBError(t *testing.T) {
	fake := &fakePgQueries{resolveErr: errors.New("db down")}
	store := NewPgStore(fake)
	_, err := store.MarkResolved(context.Background(), uuid.NewString(), "x")
	require.Error(t, err)
}

func TestPgStore_MarkCancelled_DBError(t *testing.T) {
	fake := &fakePgQueries{cancelErr: errors.New("db down")}
	store := NewPgStore(fake)
	_, err := store.MarkCancelled(context.Background(), uuid.NewString(), "x")
	require.Error(t, err)
}

func TestPgStore_ListPending_DBError(t *testing.T) {
	fake := &fakePgQueries{listErr: errors.New("db down")}
	store := NewPgStore(fake)
	_, err := store.ListPending(context.Background())
	require.Error(t, err)
	_, err = store.ListPendingForCampaign(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestPgStore_RowToItem_BadExtra(t *testing.T) {
	row := refdata.DmQueueItem{
		ID:         uuid.New(),
		CampaignID: uuid.New(),
		Extra:      []byte("not-json"),
	}
	_, err := rowToItem(row)
	require.Error(t, err)
}

func TestPgStore_DecodeExtra_Empty(t *testing.T) {
	env, err := decodeExtra(nil)
	require.NoError(t, err)
	assert.Equal(t, "", env.Posted)
}

func TestNotifier_StoreErrorOnInsert(t *testing.T) {
	sender := &fakeSender{nextMsgID: "m"}
	store := &errorStore{insertErr: errors.New("insert fail")}
	n := NewNotifierWithStore(sender, staticChannelResolver("c"), func(id string) string { return "/x/" + id }, store)
	_, err := n.Post(context.Background(), Event{Kind: KindRestRequest, PlayerName: "A", Summary: "rests"})
	require.Error(t, err)
}

func TestNotifier_StoreErrorOnGet(t *testing.T) {
	sender := &fakeSender{}
	store := &errorStore{getErr: errors.New("boom")}
	n := NewNotifierWithStore(sender, staticChannelResolver("c"), func(id string) string { return "/x/" + id }, store)
	require.ErrorContains(t, n.Cancel(context.Background(), "id", ""), "boom")
	require.ErrorContains(t, n.Resolve(context.Background(), "id", "x"), "boom")
	_, ok := n.Get("id")
	assert.False(t, ok)
}

func TestNotifier_StoreErrorOnList(t *testing.T) {
	store := &errorStore{listErr: errors.New("nope")}
	n := NewNotifierWithStore(&fakeSender{}, staticChannelResolver("c"), func(id string) string { return "/x/" + id }, store)
	assert.Nil(t, n.ListPending())
}

// errorStore is an in-memory store wrapper with injectable failures.
type errorStore struct {
	insertErr error
	getErr    error
	listErr   error
}

func (e *errorStore) Insert(_ context.Context, _ string, _ Event, _, _, _ string) (Item, error) {
	if e.insertErr != nil {
		return Item{}, e.insertErr
	}
	return Item{}, nil
}

func (e *errorStore) Get(_ context.Context, _ string) (Item, bool, error) {
	if e.getErr != nil {
		return Item{}, false, e.getErr
	}
	return Item{ID: "id", Status: StatusPending}, true, nil
}

func (e *errorStore) SetMessageID(_ context.Context, _, _ string) error { return nil }

func (e *errorStore) MarkResolved(_ context.Context, _, _ string) (Item, error) { return Item{}, nil }
func (e *errorStore) MarkCancelled(_ context.Context, _, _ string) (Item, error) {
	return Item{}, nil
}
func (e *errorStore) ListPending(_ context.Context) ([]Item, error) {
	if e.listErr != nil {
		return nil, e.listErr
	}
	return nil, nil
}
