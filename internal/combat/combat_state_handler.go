package combat

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// combatStateResponse is the read-side tracker snapshot returned by
// GET /api/combat/{encounterID}/state (APP-8): the round, the current-turn
// combatant, and each combatant's initiative / hp / active flag — the state the
// DM previously had to derive by querying Postgres directly.
type combatStateResponse struct {
	EncounterID        string                 `json:"encounter_id"`
	Status             string                 `json:"status"`
	RoundNumber        int32                  `json:"round_number"`
	CurrentTurnID      *string                `json:"current_turn_id"`
	CurrentCombatantID *string                `json:"current_combatant_id"`
	Combatants         []combatStateCombatant `json:"combatants"`
}

// combatStateCombatant is one row of the tracker snapshot, ordered by
// initiative. It carries full hp/ac for every combatant (NPCs included) because
// this endpoint is mounted alongside the other DM-only dashboard routes — it is
// the DM's own view, not a player-facing one.
type combatStateCombatant struct {
	ID              string `json:"id"`
	ShortID         string `json:"short_id"`
	DisplayName     string `json:"display_name"`
	InitiativeRoll  int32  `json:"initiative_roll"`
	InitiativeOrder int32  `json:"initiative_order"`
	HpCurrent       int32  `json:"hp_current"`
	HpMax           int32  `json:"hp_max"`
	TempHp          int32  `json:"temp_hp"`
	Ac              int32  `json:"ac"`
	PositionCol     string `json:"position_col"`
	PositionRow     int32  `json:"position_row"`
	IsNpc           bool   `json:"is_npc"`
	IsAlive         bool   `json:"is_alive"`
	IsActive        bool   `json:"is_active"`
}

// CombatState handles GET /api/combat/{encounterID}/state (APP-8).
func (h *DMDashboardHandler) CombatState(w http.ResponseWriter, r *http.Request) {
	encounterID, err := parseEncounterID(r)
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	enc, err := h.svc.store.GetEncounter(r.Context(), encounterID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "encounter not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load encounter", http.StatusInternalServerError)
		return
	}

	combatants, err := h.svc.store.ListCombatantsByEncounterID(r.Context(), encounterID)
	if err != nil {
		http.Error(w, "failed to list combatants", http.StatusInternalServerError)
		return
	}

	// Only read the active turn when the encounter actually has one; a
	// best-effort read (error → no active combatant) keeps the snapshot usable
	// even if the turn row is momentarily inconsistent.
	var activeCombatantID uuid.UUID
	if enc.CurrentTurnID.Valid {
		if turn, terr := h.svc.store.GetActiveTurnByEncounterID(r.Context(), encounterID); terr == nil {
			activeCombatantID = turn.CombatantID
		}
	}

	writeJSON(w, http.StatusOK, buildCombatStateResponse(enc, combatants, activeCombatantID))
}

// buildCombatStateResponse assembles the tracker snapshot. activeCombatantID is
// uuid.Nil when the encounter has no live turn.
func buildCombatStateResponse(enc refdata.Encounter, combatants []refdata.Combatant, activeCombatantID uuid.UUID) combatStateResponse {
	resp := combatStateResponse{
		EncounterID: enc.ID.String(),
		Status:      enc.Status,
		RoundNumber: enc.RoundNumber,
		Combatants:  make([]combatStateCombatant, len(combatants)),
	}
	if enc.CurrentTurnID.Valid {
		id := enc.CurrentTurnID.UUID.String()
		resp.CurrentTurnID = &id
	}
	hasActiveCombatant := activeCombatantID != uuid.Nil
	if hasActiveCombatant {
		id := activeCombatantID.String()
		resp.CurrentCombatantID = &id
	}
	for i, c := range combatants {
		resp.Combatants[i] = combatStateCombatant{
			ID:              c.ID.String(),
			ShortID:         c.ShortID,
			DisplayName:     c.DisplayName,
			InitiativeRoll:  c.InitiativeRoll,
			InitiativeOrder: c.InitiativeOrder,
			HpCurrent:       c.HpCurrent,
			HpMax:           c.HpMax,
			TempHp:          c.TempHp,
			Ac:              c.Ac,
			PositionCol:     c.PositionCol,
			PositionRow:     c.PositionRow,
			IsNpc:           c.IsNpc,
			IsAlive:         c.IsAlive,
			IsActive:        hasActiveCombatant && c.ID == activeCombatantID,
		}
	}
	return resp
}
