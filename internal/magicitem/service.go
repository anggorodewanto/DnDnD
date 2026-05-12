// Package magicitem houses pure-logic helpers for converting magic-item
// inventory rows into combat feature effects, plus a thin stateful Service
// surface used by Phase 104b for encounter-publisher fan-out.
//
// The Service is intentionally minimal: it carries a publisher + encounter
// lookup so callers (the /attune handler, magic-item activation flows, the
// /rest dawn-recharge path) can refresh the dashboard snapshot for a
// character that is also a combatant in an active encounter
// (H-104b-rest-magicitem-publisher).
package magicitem

import (
	"context"
	"log"

	"github.com/google/uuid"
)

// EncounterPublisher fans out a fresh encounter snapshot over the dashboard
// WebSocket hub whenever a magic-item state mutation (attunement,
// activation, charge consumption) touches a character that is also a
// combatant in an active encounter (H-104b). The interface is injected
// (optionally) onto Service so the package stays decoupled from the
// concrete dashboard.Publisher.
type EncounterPublisher interface {
	PublishEncounterSnapshot(ctx context.Context, encounterID uuid.UUID) error
}

// EncounterLookup resolves the active encounter (if any) that currently
// contains the given character. Returns (encID, true, nil) when the
// character is a combatant in an active encounter; (uuid.Nil, false, nil)
// when not in combat; or a non-nil error on store failure.
type EncounterLookup interface {
	ActiveEncounterIDForCharacter(ctx context.Context, characterID uuid.UUID) (uuid.UUID, bool, error)
}

// Service is the stateful surface for magic-item operations. It currently
// only carries the publisher + encounter-lookup pair used by H-104b — the
// pure-logic helpers (ItemFeatures, CollectItemFeatures, ParsePassiveEffects)
// continue to live as package-level functions.
type Service struct {
	publisher EncounterPublisher
	lookup    EncounterLookup
}

// NewService creates a new magicitem Service. Both the publisher and the
// encounter lookup default to nil and can be wired later via SetPublisher.
func NewService() *Service {
	return &Service{}
}

// SetPublisher wires the optional dashboard publisher and encounter lookup
// (H-104b). A nil publisher is tolerated and disables fan-out. Publish
// errors are logged but never surfaced to callers so a dashboard hiccup
// cannot undo a committed magic-item write.
func (s *Service) SetPublisher(p EncounterPublisher, lookup EncounterLookup) {
	s.publisher = p
	s.lookup = lookup
}

// PublishForCharacter looks up the character's active encounter (if any)
// and fires the publisher. Silently no-ops when the character is not in
// combat, when the publisher is unset, or when the lookup/publish fails.
// Callers (the /attune Discord handler, magic-item activation flows)
// invoke this AFTER persisting a magic-item write so dashboard subscribers
// see the refreshed AC / attunement / charge state.
func (s *Service) PublishForCharacter(ctx context.Context, characterID uuid.UUID) {
	if s == nil || s.publisher == nil || s.lookup == nil {
		return
	}
	encID, ok, err := s.lookup.ActiveEncounterIDForCharacter(ctx, characterID)
	if err != nil {
		log.Printf("magicitem: active encounter lookup failed for %s: %v", characterID, err)
		return
	}
	if !ok {
		return
	}
	if err := s.publisher.PublishEncounterSnapshot(ctx, encID); err != nil {
		log.Printf("magicitem: encounter snapshot publish failed for %s: %v", encID, err)
	}
}
