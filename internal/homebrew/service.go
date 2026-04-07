// Package homebrew implements DM Dashboard Homebrew Content (Phase 99).
//
// It exposes campaign-scoped CRUD over the reference data tables for the
// types DMs are allowed to customize: creatures, spells, weapons, magic
// items, races, feats, and classes. Every entry is created with
// homebrew=true and source="homebrew", and is owned by exactly one
// campaign — never the global SRD scope.
//
// SRD entries (campaign_id IS NULL) are read-only here: every Update and
// Delete enforces that the target row both has homebrew=true AND its
// campaign_id matches the supplied campaign. Mismatches are reported as
// ErrNotFound rather than ErrForbidden so that callers cannot probe for
// the existence of other campaigns' content.
package homebrew

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// ErrNotFound is returned when the target row does not exist, is not
// homebrew, or is not owned by the requesting campaign.
var ErrNotFound = errors.New("homebrew entry not found")

// ErrInvalidInput is returned when validation fails (empty name, missing
// campaign id, etc).
var ErrInvalidInput = errors.New("invalid homebrew input")

// Store is the minimal subset of *refdata.Queries the homebrew service
// needs. It is defined as an interface so unit tests can swap in an
// in-memory fake.
type Store interface {
	// Creatures
	GetCreature(ctx context.Context, id string) (refdata.Creature, error)
	UpsertCreature(ctx context.Context, arg refdata.UpsertCreatureParams) error
	DeleteHomebrewCreature(ctx context.Context, arg refdata.DeleteHomebrewCreatureParams) (int64, error)

	// Spells
	GetSpell(ctx context.Context, id string) (refdata.Spell, error)
	UpsertSpell(ctx context.Context, arg refdata.UpsertSpellParams) error
	DeleteHomebrewSpell(ctx context.Context, arg refdata.DeleteHomebrewSpellParams) (int64, error)

	// Weapons
	GetWeapon(ctx context.Context, id string) (refdata.Weapon, error)
	UpsertWeapon(ctx context.Context, arg refdata.UpsertWeaponParams) error
	DeleteHomebrewWeapon(ctx context.Context, arg refdata.DeleteHomebrewWeaponParams) (int64, error)

	// Magic items
	GetMagicItem(ctx context.Context, id string) (refdata.MagicItem, error)
	UpsertMagicItem(ctx context.Context, arg refdata.UpsertMagicItemParams) error
	DeleteHomebrewMagicItem(ctx context.Context, arg refdata.DeleteHomebrewMagicItemParams) (int64, error)

	// Races
	GetRace(ctx context.Context, id string) (refdata.Race, error)
	UpsertRace(ctx context.Context, arg refdata.UpsertRaceParams) error
	DeleteHomebrewRace(ctx context.Context, arg refdata.DeleteHomebrewRaceParams) (int64, error)

	// Feats
	GetFeat(ctx context.Context, id string) (refdata.Feat, error)
	UpsertFeat(ctx context.Context, arg refdata.UpsertFeatParams) error
	DeleteHomebrewFeat(ctx context.Context, arg refdata.DeleteHomebrewFeatParams) (int64, error)

	// Classes
	GetClass(ctx context.Context, id string) (refdata.Class, error)
	UpsertClass(ctx context.Context, arg refdata.UpsertClassParams) error
	DeleteHomebrewClass(ctx context.Context, arg refdata.DeleteHomebrewClassParams) (int64, error)
}

// Service is the homebrew CRUD service.
type Service struct {
	store  Store
	idGen  func() string
	source string
}

// NewService constructs a Service backed by the given store.
func NewService(store Store) *Service {
	return &Service{store: store, idGen: defaultIDGen, source: "homebrew"}
}

// defaultIDGen returns a short, opaque random hex id prefixed for
// debuggability ("hb_" + 12 hex chars). Globally unique enough for our
// per-campaign namespace; collisions become ErrInvalidInput downstream.
func defaultIDGen() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "hb_" + hex.EncodeToString(b[:])
}

// --- shared validation helpers ---

// validateCampaign returns ErrInvalidInput if campaignID is the zero UUID.
func validateCampaign(campaignID uuid.UUID) error {
	if campaignID == uuid.Nil {
		return fmt.Errorf("%w: campaign_id required", ErrInvalidInput)
	}
	return nil
}

// validateName returns ErrInvalidInput if name is empty/whitespace.
func validateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: name required", ErrInvalidInput)
	}
	return nil
}

// requireCreate runs the standard pre-create validation.
func (s *Service) requireCreate(campaignID uuid.UUID, name string) error {
	if err := validateCampaign(campaignID); err != nil {
		return err
	}
	return validateName(name)
}

// requireUpdate runs the standard pre-update validation.
func (s *Service) requireUpdate(campaignID uuid.UUID, id, name string) error {
	if err := validateCampaign(campaignID); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: id required", ErrInvalidInput)
	}
	return validateName(name)
}

// requireDelete validates campaign+id before a delete.
func (s *Service) requireDelete(campaignID uuid.UUID, id string) error {
	if err := validateCampaign(campaignID); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: id required", ErrInvalidInput)
	}
	return nil
}

// homebrewCols builds the (campaign_id, homebrew, source) trio used by
// every Upsert.
func (s *Service) homebrewCols(campaignID uuid.UUID) (uuid.NullUUID, sql.NullBool, sql.NullString) {
	return uuid.NullUUID{UUID: campaignID, Valid: true},
		sql.NullBool{Bool: true, Valid: true},
		sql.NullString{String: s.source, Valid: true}
}

// ownsCreature returns nil if the creature exists, is homebrew, and is
// owned by the given campaign; otherwise ErrNotFound.
func ownsRow(homebrew sql.NullBool, owner uuid.NullUUID, campaignID uuid.UUID) error {
	if !homebrew.Valid || !homebrew.Bool {
		return ErrNotFound
	}
	if !owner.Valid || owner.UUID != campaignID {
		return ErrNotFound
	}
	return nil
}

// translateGetErr converts sql.ErrNoRows to ErrNotFound.
func translateGetErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
