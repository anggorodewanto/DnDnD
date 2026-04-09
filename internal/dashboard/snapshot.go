package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// SnapshotStore defines the read-only DB operations the SnapshotBuilder
// needs to assemble a full encounter snapshot. It is deliberately narrow
// so tests can use a fake implementation.
type SnapshotStore interface {
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
}

// EncounterSnapshot is the full state snapshot pushed over the WebSocket on
// every encounter mutation. Phase 103 is "snapshot-always": no deltas, no
// sequence numbers — every push is a complete view of the encounter.
type EncounterSnapshot struct {
	Type        string             `json:"type"`
	EncounterID string             `json:"encounter_id"`
	Encounter   refdata.Encounter  `json:"encounter"`
	Combatants  []refdata.Combatant `json:"combatants"`
	CurrentTurn *refdata.Turn      `json:"current_turn"`
	ServerTime  time.Time          `json:"server_time"`
}

// SnapshotBuilder assembles EncounterSnapshot values from the database.
type SnapshotBuilder struct {
	store SnapshotStore
	now   func() time.Time
}

// NewSnapshotBuilder creates a new builder. If nowFn is nil, time.Now is used.
func NewSnapshotBuilder(store SnapshotStore, nowFn func() time.Time) *SnapshotBuilder {
	if nowFn == nil {
		nowFn = time.Now
	}
	return &SnapshotBuilder{store: store, now: nowFn}
}

// Build loads the encounter, combatants, and active turn (if any) and
// returns a serializable snapshot. Any DB error is returned untouched so
// the caller can decide whether to log, retry, or suppress.
func (b *SnapshotBuilder) Build(ctx context.Context, encounterID uuid.UUID) (EncounterSnapshot, error) {
	enc, err := b.store.GetEncounter(ctx, encounterID)
	if err != nil {
		return EncounterSnapshot{}, fmt.Errorf("get encounter: %w", err)
	}

	combatants, err := b.store.ListCombatantsByEncounterID(ctx, enc.ID)
	if err != nil {
		return EncounterSnapshot{}, fmt.Errorf("list combatants: %w", err)
	}
	if combatants == nil {
		combatants = []refdata.Combatant{}
	}

	var currentTurn *refdata.Turn
	if enc.CurrentTurnID.Valid {
		turn, err := b.store.GetTurn(ctx, enc.CurrentTurnID.UUID)
		if err != nil {
			return EncounterSnapshot{}, fmt.Errorf("get current turn: %w", err)
		}
		currentTurn = &turn
	}

	return EncounterSnapshot{
		Type:        "encounter_snapshot",
		EncounterID: enc.ID.String(),
		Encounter:   enc,
		Combatants:  combatants,
		CurrentTurn: currentTurn,
		ServerTime:  b.now(),
	}, nil
}

// Publisher pushes full-state encounter snapshots over a Hub. Any service
// that mutates encounter state should call PublishEncounterSnapshot after
// committing its transaction.
type Publisher struct {
	hub     *Hub
	builder *SnapshotBuilder
}

// NewPublisher creates a new Publisher. A nil hub is tolerated so callers
// can use the publisher in contexts where WebSocket fan-out is disabled
// (e.g., CLI tools, tests); in that case PublishEncounterSnapshot still
// builds the snapshot (to surface DB errors) but skips the broadcast.
func NewPublisher(hub *Hub, builder *SnapshotBuilder) *Publisher {
	return &Publisher{hub: hub, builder: builder}
}

// PublishEncounterSnapshot assembles a full snapshot for encounterID and
// broadcasts it to every subscribed WebSocket client.
func (p *Publisher) PublishEncounterSnapshot(ctx context.Context, encounterID uuid.UUID) error {
	snap, err := p.builder.Build(ctx, encounterID)
	if err != nil {
		return err
	}
	msg, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	if p.hub == nil {
		return nil
	}
	p.hub.BroadcastEncounter(snap.EncounterID, msg)
	return nil
}
