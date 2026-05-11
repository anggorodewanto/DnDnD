package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// ammoKey uniquely identifies a (encounter, combatant, ammo-type) tuple
// for the spent-ammunition tracker. The encounter scope keeps state
// isolated between simultaneous encounters; the ammo name lets the
// recovery loop produce one entry per ammunition type per character.
type ammoKey struct {
	encounterID uuid.UUID
	combatantID uuid.UUID
	ammoName    string
}

// AmmoSpentTracker is an in-memory counter for ammunition spent per
// (encounter, combatant, ammo name) used to drive the post-combat
// half-recovery hook (C-37 / Phase 37). It is thread-safe; the combat
// service holds a single instance for the process lifetime. The tracker
// is cleared whenever an encounter ends so a fresh encounter starts at
// zero. Persisting recovered ammunition is the caller's job — this
// struct only tracks deltas.
type AmmoSpentTracker struct {
	mu     sync.Mutex
	counts map[ammoKey]int
}

// NewAmmoSpentTracker returns a fresh in-memory tracker.
func NewAmmoSpentTracker() *AmmoSpentTracker {
	return &AmmoSpentTracker{counts: make(map[ammoKey]int)}
}

// Record bumps the spent counter for the given (encounter, combatant,
// ammoName) tuple by `count` shots. Negative counts are clamped to zero
// (the spend path never refunds — that's what half-recovery is for).
func (t *AmmoSpentTracker) Record(encounterID, combatantID uuid.UUID, ammoName string, count int) {
	if count <= 0 || ammoName == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counts[ammoKey{encounterID, combatantID, ammoName}] += count
}

// Snapshot returns a copy of all spent counts for the given encounter,
// grouped by (combatantID, ammoName). The map is keyed by combatantID
// with each entry mapping ammoName → count. Used by EndCombat to drive
// per-character recovery.
func (t *AmmoSpentTracker) Snapshot(encounterID uuid.UUID) map[uuid.UUID]map[string]int {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make(map[uuid.UUID]map[string]int)
	for k, v := range t.counts {
		if k.encounterID != encounterID {
			continue
		}
		if _, ok := out[k.combatantID]; !ok {
			out[k.combatantID] = make(map[string]int)
		}
		out[k.combatantID][k.ammoName] = v
	}
	return out
}

// ClearEncounter drops all spent counters for the given encounter. Called
// by EndCombat after recovery is persisted so a re-run encounter never
// double-recovers from stale state.
func (t *AmmoSpentTracker) ClearEncounter(encounterID uuid.UUID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for k := range t.counts {
		if k.encounterID == encounterID {
			delete(t.counts, k)
		}
	}
}

// RecordAmmoSpent is the public Service hook used by the attack pipeline
// (and tests) to mark one shot of `ammoName` as expended by `combatantID`
// in the given encounter. Wired from Service.Attack whenever a weapon with
// the "ammunition" property fires.
func (s *Service) RecordAmmoSpent(encounterID, combatantID uuid.UUID, ammoName string, count int) {
	if s.ammoTracker == nil {
		return
	}
	s.ammoTracker.Record(encounterID, combatantID, ammoName, count)
}

// recordAmmoForAttack derives the ammunition name from the weapon and
// records one shot in the tracker. Used by Service.Attack right after the
// ammunition-property deduction path so the spent count is kept in sync
// with the inventory write.
func (s *Service) recordAmmoForAttack(encounterID, combatantID uuid.UUID, weapon refdata.Weapon) {
	if !HasProperty(weapon, "ammunition") {
		return
	}
	s.RecordAmmoSpent(encounterID, combatantID, GetAmmunitionName(weapon), 1)
}

// recoverEncounterAmmunition iterates the encounter's tracker snapshot and
// adds RecoverAmmunition(spent) entries back to each PC's inventory.
// Persists via UpdateCharacterInventory. NPC combatants have no character
// row and are silently skipped — they don't loot their own arrows. Errors
// are non-fatal and bubble up so EndCombat callers can surface them.
//
// Schema note (C-37): no persistent ammo-spent column exists yet, so the
// tracker is in-memory only. A future migration can promote it.
func (s *Service) recoverEncounterAmmunition(ctx context.Context, encounterID uuid.UUID, combatants []refdata.Combatant) error {
	if s.ammoTracker == nil {
		return nil
	}
	snap := s.ammoTracker.Snapshot(encounterID)
	if len(snap) == 0 {
		return nil
	}

	for _, c := range combatants {
		if !c.CharacterID.Valid {
			continue
		}
		spentByAmmo, ok := snap[c.ID]
		if !ok || len(spentByAmmo) == 0 {
			continue
		}

		char, err := s.store.GetCharacter(ctx, c.CharacterID.UUID)
		if err != nil {
			return fmt.Errorf("recovering ammunition for %s: %w", c.DisplayName, err)
		}
		items, err := ParseInventory(char.Inventory.RawMessage)
		if err != nil {
			return fmt.Errorf("parsing inventory for %s: %w", c.DisplayName, err)
		}
		for ammoName, spent := range spentByAmmo {
			items = RecoverAmmunition(items, ammoName, spent)
		}
		invJSON, err := json.Marshal(items)
		if err != nil {
			return fmt.Errorf("marshaling inventory for %s: %w", c.DisplayName, err)
		}
		if err := s.store.UpdateCharacterInventory(ctx, char.ID, pqtype.NullRawMessage{RawMessage: invJSON, Valid: true}); err != nil {
			return fmt.Errorf("persisting recovered ammunition for %s: %w", c.DisplayName, err)
		}
	}

	s.ammoTracker.ClearEncounter(encounterID)
	return nil
}
