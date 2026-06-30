package combat

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// ISSUE-048: DM-side cancellation of a mid-flight AoE cast's pending saves.
//
// When a player misplaces an AoE spell (e.g. a Shatter whose blast catches an
// ally) and the DM grants their /undo, the cast's pending_saves must be voided
// so no damage ever lands at the wrong origin. Resolving them would APPLY
// damage; there was previously no reachable path to cancel them (the
// ForfeitPendingSave / CancelAllPendingSavesByCombatant queries existed but had
// no service method, handler, or button). CancelAoEPendingSave fills that gap.

// AoESaveCancellation is the outcome of voiding an AoE cast's pending saves. It
// deliberately surfaces no HP — enemy HP is secret — only what the #combat-log
// correction needs: the spell and how many saves were voided.
type AoESaveCancellation struct {
	SaveID      uuid.UUID
	CombatantID uuid.UUID // the clicked save's combatant — parents the audit row
	SpellID     string
	Source      string
	Canceled    int
}

// CancelAoEPendingSave forfeits every not-yet-applied pending save belonging to
// the same AoE cast as saveID — matched by Source (e.g. "aoe:shatter:s2c3"), so
// a single click voids the whole blast (every target's save), not just the one
// the DM clicked. The cast lands no damage. Returns ErrSaveAlreadyResolved when
// the cast's damage has already applied (nothing left to cleanly undo).
func (s *Service) CancelAoEPendingSave(ctx context.Context, encounterID, saveID uuid.UUID) (AoESaveCancellation, error) {
	row, err := s.store.GetPendingSave(ctx, saveID)
	if err != nil {
		return AoESaveCancellation{}, fmt.Errorf("%w: %w", ErrPendingSaveNotFound, err)
	}
	if row.EncounterID != encounterID {
		return AoESaveCancellation{}, ErrSaveWrongEncounter
	}
	if !IsAoEPendingSaveSource(row.Source) {
		return AoESaveCancellation{}, ErrSaveNotAoE
	}
	if row.Status == pendingSaveStatusApplied {
		return AoESaveCancellation{}, ErrSaveAlreadyResolved
	}

	// Every save sharing this cast's source is part of the same blast; forfeit
	// the ones that have not already landed damage. ListSavesByEncounter is the
	// all-status query (the pending-only list hides 'rolled' rows).
	rows, err := s.store.ListSavesByEncounter(ctx, encounterID)
	if err != nil {
		return AoESaveCancellation{}, fmt.Errorf("listing saves: %w", err)
	}

	canceled := 0
	for _, sv := range rows {
		if sv.Source != row.Source {
			continue
		}
		if sv.Status == pendingSaveStatusApplied || sv.Status == pendingSaveStatusForfeited {
			continue
		}
		if _, err := s.store.ForfeitPendingSave(ctx, sv.ID); err != nil {
			return AoESaveCancellation{}, fmt.Errorf("forfeiting save %s: %w", sv.ID, err)
		}
		canceled++
	}

	return AoESaveCancellation{
		SaveID:      saveID,
		CombatantID: row.CombatantID,
		SpellID:     SpellIDFromAoEPendingSaveSource(row.Source),
		Source:      row.Source,
		Canceled:    canceled,
	}, nil
}
