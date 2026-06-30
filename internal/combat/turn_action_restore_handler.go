package combat

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// ISSUE-049: DM dashboard endpoint to hand the active combatant back the action
// they spent this turn — the missing step when granting a player's undo of a
// misplayed action (e.g. an AoE cast whose pending saves were voided via
// ISSUE-048). Mounted in RegisterRoutes behind DM auth.

// RestoreTurnAction handles
// POST /api/combat/{encounterID}/combatants/{combatantID}/restore-action. It
// gives the active combatant back the action they spent this turn so they can
// act again. No HP is touched or surfaced.
func (h *DMDashboardHandler) RestoreTurnAction(w http.ResponseWriter, r *http.Request) {
	encounterID, combatantID, err := parseCombatantOverrideIDs(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := h.svc.RestoreTurnAction(r.Context(), encounterID, combatantID)
	if err != nil {
		writeRestoreActionError(w, err)
		return
	}

	line := formatRestoreActionLog(res)

	// #combat-log: the correction only (never any HP).
	if h.poster != nil {
		h.poster.PostCorrection(r.Context(), encounterID, line)
	}

	// Best-effort audit row, parented to the active turn (the one just restored).
	turnID := uuid.Nil
	if turn, terr := h.svc.store.GetActiveTurnByEncounterID(r.Context(), encounterID); terr == nil {
		turnID = turn.ID
	}
	h.svc.recordCombatAction(r.Context(), turnID, encounterID, res.CombatantID, uuid.NullUUID{}, "dm_restore_action", line)

	h.svc.publish(r.Context(), encounterID)

	writeJSON(w, http.StatusOK, map[string]any{
		"combatant_id":   res.CombatantID.String(),
		"combatant_name": res.CombatantName,
		"restored":       true,
	})
}

// formatRestoreActionLog renders the #combat-log / action_log line for a restored
// action. It names the combatant only — never any HP.
func formatRestoreActionLog(res ActionRestoration) string {
	return fmt.Sprintf("↩️ **DM Correction:** %s's action restored — they may take their action again this turn.", res.CombatantName)
}

// writeRestoreActionError maps the service's sentinel errors to HTTP statuses.
func writeRestoreActionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotActiveCombatant), errors.Is(err, ErrNoActionToRestore):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
