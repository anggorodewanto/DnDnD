package dmqueue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// pgQueries is the subset of *refdata.Queries the PgStore depends on. Kept
// as an interface so unit tests can supply a fake without spinning up
// PostgreSQL — integration coverage still uses the real *refdata.Queries.
type pgQueries interface {
	InsertDMQueueItem(ctx context.Context, arg refdata.InsertDMQueueItemParams) (refdata.DmQueueItem, error)
	UpdateDMQueueItemMessageID(ctx context.Context, arg refdata.UpdateDMQueueItemMessageIDParams) (refdata.DmQueueItem, error)
	GetDMQueueItem(ctx context.Context, id uuid.UUID) (refdata.DmQueueItem, error)
	ListAllPendingDMQueueItems(ctx context.Context) ([]refdata.DmQueueItem, error)
	ListPendingDMQueueItems(ctx context.Context, campaignID uuid.UUID) ([]refdata.DmQueueItem, error)
	MarkDMQueueItemResolved(ctx context.Context, arg refdata.MarkDMQueueItemResolvedParams) (refdata.DmQueueItem, error)
	MarkDMQueueItemCancelled(ctx context.Context, arg refdata.MarkDMQueueItemCancelledParams) (refdata.DmQueueItem, error)
}

// PgStore persists dm-queue items in PostgreSQL via the sqlc-generated
// *refdata.Queries handle. Use NewPgStore to construct one.
type PgStore struct {
	q pgQueries
}

// NewPgStore wraps a *refdata.Queries (or any pgQueries-compatible value)
// in a Store implementation suitable for production use.
func NewPgStore(q pgQueries) *PgStore {
	return &PgStore{q: q}
}

// Insert persists a new pending item. The id MUST be a valid UUID string;
// PgStore overrides the database default to keep the in-memory and DB
// stores byte-for-byte interchangeable from the caller's perspective.
//
// The Event must carry CampaignID and GuildID. PostedText is recorded as
// the canonical Discord render and used by Cancel/Resolve later.
func (p *PgStore) Insert(ctx context.Context, id string, e Event, channelID, messageID, postedText string) (Item, error) {
	itemUUID, err := uuid.Parse(id)
	if err != nil {
		return Item{}, fmt.Errorf("dmqueue pgstore: parse id: %w", err)
	}
	campaignUUID, err := uuid.Parse(e.CampaignID)
	if err != nil {
		return Item{}, fmt.Errorf("dmqueue pgstore: parse campaign id: %w", err)
	}
	extra, err := encodeExtra(e.ExtraMetadata, postedText)
	if err != nil {
		return Item{}, err
	}

	row, err := p.q.InsertDMQueueItem(ctx, refdata.InsertDMQueueItemParams{
		ID:          itemUUID,
		CampaignID:  campaignUUID,
		GuildID:     e.GuildID,
		ChannelID:   channelID,
		MessageID:   messageID,
		Kind:        string(e.Kind),
		PlayerName:  e.PlayerName,
		Summary:     e.Summary,
		ResolvePath: e.ResolvePath,
		Extra:       extra,
	})
	if err != nil {
		return Item{}, err
	}
	return rowToItem(row)
}

// SetMessageID updates the row's message_id (used by Notifier.Post after
// Sender.Send returns the real Discord message ID under the insert-then-send
// ordering — SR-002). Returns ErrItemNotFound for unknown ids.
func (p *PgStore) SetMessageID(ctx context.Context, id, messageID string) error {
	itemUUID, err := uuid.Parse(id)
	if err != nil {
		return ErrItemNotFound
	}
	_, err = p.q.UpdateDMQueueItemMessageID(ctx, refdata.UpdateDMQueueItemMessageIDParams{
		ID:        itemUUID,
		MessageID: messageID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrItemNotFound
		}
		return err
	}
	return nil
}

// Get returns the item by ID, or (zero, false, nil) if not found.
func (p *PgStore) Get(ctx context.Context, id string) (Item, bool, error) {
	itemUUID, err := uuid.Parse(id)
	if err != nil {
		return Item{}, false, nil
	}
	row, err := p.q.GetDMQueueItem(ctx, itemUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Item{}, false, nil
		}
		return Item{}, false, err
	}
	item, err := rowToItem(row)
	if err != nil {
		return Item{}, false, err
	}
	return item, true, nil
}

// MarkResolved transitions a pending item to resolved.
func (p *PgStore) MarkResolved(ctx context.Context, id, outcome string) (Item, error) {
	itemUUID, err := uuid.Parse(id)
	if err != nil {
		return Item{}, ErrItemNotFound
	}
	row, err := p.q.MarkDMQueueItemResolved(ctx, refdata.MarkDMQueueItemResolvedParams{
		ID:      itemUUID,
		Outcome: outcome,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Item{}, ErrItemNotFound
		}
		return Item{}, err
	}
	return rowToItem(row)
}

// MarkCancelled transitions a pending item to cancelled.
func (p *PgStore) MarkCancelled(ctx context.Context, id, reason string) (Item, error) {
	itemUUID, err := uuid.Parse(id)
	if err != nil {
		return Item{}, ErrItemNotFound
	}
	row, err := p.q.MarkDMQueueItemCancelled(ctx, refdata.MarkDMQueueItemCancelledParams{
		ID:      itemUUID,
		Outcome: reason,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Item{}, ErrItemNotFound
		}
		return Item{}, err
	}
	return rowToItem(row)
}

// ListPending returns all pending items across every campaign in created_at order.
func (p *PgStore) ListPending(ctx context.Context) ([]Item, error) {
	rows, err := p.q.ListAllPendingDMQueueItems(ctx)
	if err != nil {
		return nil, err
	}
	return rowsToItems(rows)
}

// ListPendingForCampaign filters pending items to a single campaign. Not
// part of the Store interface but exposed for the future dashboard inbox.
func (p *PgStore) ListPendingForCampaign(ctx context.Context, campaignID uuid.UUID) ([]Item, error) {
	rows, err := p.q.ListPendingDMQueueItems(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	return rowsToItems(rows)
}

// rowsToItems converts a slice of sqlc rows into dm-queue Items, propagating
// any decode error from the first offending row.
func rowsToItems(rows []refdata.DmQueueItem) ([]Item, error) {
	out := make([]Item, 0, len(rows))
	for _, r := range rows {
		item, err := rowToItem(r)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// extraEnvelope is the JSON shape we persist into dm_queue_items.extra. We
// stash the rendered Discord message under a reserved "_posted" key so
// FormatCancelled / FormatResolved can rebuild edits without re-running
// FormatEvent (which would lose any future formatting drift).
type extraEnvelope struct {
	Posted   string            `json:"_posted"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

func encodeExtra(metadata map[string]string, posted string) (json.RawMessage, error) {
	env := extraEnvelope{Posted: posted, Metadata: metadata}
	return json.Marshal(env)
}

func decodeExtra(raw json.RawMessage) (extraEnvelope, error) {
	if len(raw) == 0 {
		return extraEnvelope{}, nil
	}
	var env extraEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return extraEnvelope{}, err
	}
	return env, nil
}

func rowToItem(row refdata.DmQueueItem) (Item, error) {
	env, err := decodeExtra(row.Extra)
	if err != nil {
		return Item{}, err
	}
	return Item{
		ID: row.ID.String(),
		Event: Event{
			Kind:          EventKind(row.Kind),
			PlayerName:    row.PlayerName,
			Summary:       row.Summary,
			ResolvePath:   row.ResolvePath,
			GuildID:       row.GuildID,
			CampaignID:    row.CampaignID.String(),
			ExtraMetadata: env.Metadata,
		},
		ChannelID:  row.ChannelID,
		MessageID:  row.MessageID,
		PostedText: env.Posted,
		Status:     Status(row.Status),
		Outcome:    row.Outcome,
	}, nil
}
