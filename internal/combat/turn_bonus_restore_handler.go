package combat

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// ISSUE-049 (bonus-action variant): DM dashboard endpoint to hand the active
// combatant back the bonus action they spent this turn — the sibling of
// RestoreTurnAction for the main action. Mounted in RegisterRoutes behind DM auth.

// RestoreTurnBonusAction handles
// POST /api/combat/{encounterID}/combatants/{combatantID}/restore-bonus-action.
// It gives the active combatant back the bonus action they spent this turn so
// they can use it again. No HP is touched or surfaced.
func (h *DMDashboardHandler) RestoreTurnBonusAction(w http.ResponseWriter, r *http.Request) {
	encounterID, combatantID, err := parseCombatantOverrideIDs(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := h.svc.RestoreTurnBonusAction(r.Context(), encounterID, combatantID)
	if err != nil {
		writeRestoreBonusActionError(w, err)
		return
	}

	line := formatRestoreBonusActionLog(res)

	// #combat-log: the correction only (never any HP).
	if h.poster != nil {
		h.poster.PostCorrection(r.Context(), encounterID, line)
	}

	// Best-effort audit row, parented to the active turn (the one just restored).
	turnID := uuid.Nil
	if turn, terr := h.svc.store.GetActiveTurnByEncounterID(r.Context(), encounterID); terr == nil {
		turnID = turn.ID
	}
	h.svc.recordCombatAction(r.Context(), turnID, encounterID, res.CombatantID, uuid.NullUUID{}, "dm_restore_bonus_action", line)

	h.svc.publish(r.Context(), encounterID)

	writeJSON(w, http.StatusOK, map[string]any{
		"combatant_id":   res.CombatantID.String(),
		"combatant_name": res.CombatantName,
		"restored":       true,
	})
}

// formatRestoreBonusActionLog renders the #combat-log / action_log line for a
// restored bonus action. It names the combatant only — never any HP.
func formatRestoreBonusActionLog(res ActionRestoration) string {
	return fmt.Sprintf("↩️ **DM Correction:** %s's bonus action restored — they may take their bonus action again this turn.", res.CombatantName)
}

// writeRestoreBonusActionError maps the service's sentinel errors to HTTP statuses.
func writeRestoreBonusActionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotActiveCombatant), errors.Is(err, ErrNoBonusActionToRestore):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
