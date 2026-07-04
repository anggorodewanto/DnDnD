package combat

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ISSUE-043: DM dashboard endpoints to surface and resolve a monster's pending
// AoE saving throw. Mounted in RegisterRoutes behind DM auth.

// pendingSaveResponse is one unresolved monster AoE save the DM UI renders.
type pendingSaveResponse struct {
	ID            string `json:"id"`
	CombatantID   string `json:"combatant_id"`
	CombatantName string `json:"combatant_name"`
	Ability       string `json:"ability"`
	DC            int    `json:"dc"`
	Source        string `json:"source"`
	SpellID       string `json:"spell_id"`
	CoverBonus    int    `json:"cover_bonus"`
}

// monsterSaveResolveResponse is the result of resolving a monster's AoE save.
// It deliberately omits any combatant HP — enemy HP is secret — surfacing only
// the save roll and the damage dealt.
type monsterSaveResolveResponse struct {
	SaveID        string             `json:"save_id"`
	CombatantID   string             `json:"combatant_id"`
	CombatantName string             `json:"combatant_name"`
	Ability       string             `json:"ability"`
	DC            int                `json:"dc"`
	NaturalRoll   int                `json:"natural_roll"`
	SaveBonus     int                `json:"save_bonus"`
	CoverBonus    int                `json:"cover_bonus"`
	Total         int                `json:"total"`
	Success       bool               `json:"success"`
	Damage        *aoeDamageResponse `json:"damage,omitempty"`
}

// aoeDamageResponse summarizes the AoE damage outcome. Per-target HP is omitted
// on purpose so the endpoint never leaks a monster's HP total.
type aoeDamageResponse struct {
	TotalDamage int                        `json:"total_damage"`
	Targets     []aoeTargetOutcomeResponse `json:"targets"`
}

type aoeTargetOutcomeResponse struct {
	CombatantID string `json:"combatant_id"`
	DisplayName string `json:"display_name"`
	SaveSuccess bool   `json:"save_success"`
	DamageDealt int    `json:"damage_dealt"`
}

// ListPendingSaves handles GET /api/combat/{encounterID}/pending-saves. It
// returns every unresolved AoE pending save in the encounter (the DM-actionable
// monster saves), regardless of whether a turn is active.
func (h *DMDashboardHandler) ListPendingSaves(w http.ResponseWriter, r *http.Request) {
	encounterID, err := parseEncounterID(r)
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	rows, err := h.svc.store.ListPendingSavesByEncounter(r.Context(), encounterID)
	if err != nil {
		http.Error(w, "failed to list pending saves", http.StatusInternalServerError)
		return
	}

	resp := make([]pendingSaveResponse, 0, len(rows))
	for _, row := range rows {
		if row.Status != "pending" {
			continue
		}
		if !IsAoEPendingSaveSource(row.Source) {
			continue
		}
		name := ""
		if c, cerr := h.svc.store.GetCombatant(r.Context(), row.CombatantID); cerr == nil {
			name = c.DisplayName
		}
		resp = append(resp, pendingSaveResponse{
			ID:            row.ID.String(),
			CombatantID:   row.CombatantID.String(),
			CombatantName: name,
			Ability:       row.Ability,
			DC:            int(row.Dc),
			Source:        row.Source,
			SpellID:       SpellIDFromAoEPendingSaveSource(row.Source),
			CoverBonus:    int(row.CoverBonus),
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// ResolveMonsterPendingSave handles
// POST /api/combat/{encounterID}/pending-saves/{saveID}/resolve.
func (h *DMDashboardHandler) ResolveMonsterPendingSave(w http.ResponseWriter, r *http.Request) {
	encounterID, saveID, err := parsePendingSaveIDs(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := h.svc.ResolveMonsterPendingSave(r.Context(), encounterID, saveID)
	if err != nil {
		writeSaveResolveError(w, err)
		return
	}

	line := formatMonsterSaveLog(res)

	// #combat-log: the save result + damage dealt only (enemy HP is secret).
	if h.poster != nil {
		h.poster.PostCorrection(r.Context(), encounterID, line)
	}

	// action_log row, best-effort parented to the active turn. Saves may be
	// resolved outside an active turn — recordCombatAction skips the write when
	// no turn parent exists rather than failing the request.
	turnID := uuid.Nil
	if turn, terr := h.svc.store.GetActiveTurnByEncounterID(r.Context(), encounterID); terr == nil {
		turnID = turn.ID
	}
	h.svc.recordCombatAction(r.Context(), turnID, encounterID, res.CombatantID, uuid.NullUUID{}, "dm_resolve_save", line)

	h.svc.publish(r.Context(), encounterID)

	writeJSON(w, http.StatusOK, toMonsterSaveResolveResponse(res))
}

// CancelAoEPendingSave handles
// POST /api/combat/{encounterID}/pending-saves/{saveID}/cancel. It voids the
// whole AoE cast the clicked save belongs to (every same-source pending save),
// so a DM can grant a player's undo of a misplaced AoE spell in one click. No
// damage lands and no HP is touched or surfaced — enemy HP stays secret.
func (h *DMDashboardHandler) CancelAoEPendingSave(w http.ResponseWriter, r *http.Request) {
	encounterID, saveID, err := parsePendingSaveIDs(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := h.svc.CancelAoEPendingSave(r.Context(), encounterID, saveID)
	if err != nil {
		writeSaveResolveError(w, err)
		return
	}

	line := formatAoESaveCancelLog(res)

	// #combat-log: the cancellation only (never any HP).
	if h.poster != nil {
		h.poster.PostCorrection(r.Context(), encounterID, line)
	}

	// Best-effort audit row, parented to the active turn when one exists.
	turnID := uuid.Nil
	if turn, terr := h.svc.store.GetActiveTurnByEncounterID(r.Context(), encounterID); terr == nil {
		turnID = turn.ID
	}
	h.svc.recordCombatAction(r.Context(), turnID, encounterID, res.CombatantID, uuid.NullUUID{}, "dm_cancel_aoe", line)

	h.svc.publish(r.Context(), encounterID)

	writeJSON(w, http.StatusOK, map[string]any{
		"save_id":  res.SaveID.String(),
		"spell_id": res.SpellID,
		"canceled": res.Canceled,
	})
}

// formatAoESaveCancelLog renders the #combat-log / action_log line for a voided
// AoE cast. It names the spell and the count of saves cleared, never any HP.
func formatAoESaveCancelLog(res AoESaveCancellation) string {
	spell := res.SpellID
	if spell == "" {
		spell = "AoE spell"
	}
	noun := "save"
	if res.Canceled != 1 {
		noun = "saves"
	}
	return fmt.Sprintf("\U0001f6e1️ **DM Correction:** %s cast canceled — %d pending %s voided, no damage applied.",
		spell, res.Canceled, noun)
}

// writeSaveResolveError maps the resolver's sentinel errors to HTTP statuses.
func writeSaveResolveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrPendingSaveNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, ErrSaveWrongEncounter):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, ErrSaveAlreadyResolved), errors.Is(err, ErrPlayerSaveViaDiscord):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, ErrSaveNotAoE):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// formatMonsterSaveLog renders the #combat-log / action_log line for a resolved
// monster save. It reports the save roll vs DC and the damage dealt to the
// monster — never its HP (enemy HP is secret).
func formatMonsterSaveLog(res MonsterSaveResolution) string {
	outcome := "Failure"
	if res.Success {
		outcome = "Success"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\U0001f6e1\ufe0f %s %s save: %d vs DC %d — %s!",
		res.CombatantName, strings.ToUpper(res.Ability), res.Total, res.DC, outcome)
	if dmg, ok := res.selfDamage(); ok {
		fmt.Fprintf(&b, " Takes %d damage", dmg)
		if res.Success && dmg > 0 {
			b.WriteString(" (halved on save)")
		}
	}
	// COV-2: append condition lines for save-or-suck spells (e.g. paralyzed on a
	// failed Hold Person). Present only on the drive that completes the save set
	// — same surfacing point as the damage above.
	if res.Damage != nil {
		for _, msg := range res.Damage.ConditionMessages {
			fmt.Fprintf(&b, "\n%s", msg)
		}
	}
	return b.String()
}

// toMonsterSaveResolveResponse maps the domain resolution to its JSON shape.
func toMonsterSaveResolveResponse(res MonsterSaveResolution) monsterSaveResolveResponse {
	resp := monsterSaveResolveResponse{
		SaveID:        res.SaveID.String(),
		CombatantID:   res.CombatantID.String(),
		CombatantName: res.CombatantName,
		Ability:       res.Ability,
		DC:            res.DC,
		NaturalRoll:   res.NaturalRoll,
		SaveBonus:     res.SaveBonus,
		CoverBonus:    res.CoverBonus,
		Total:         res.Total,
		Success:       res.Success,
	}
	if res.Damage == nil {
		return resp
	}
	damage := &aoeDamageResponse{TotalDamage: res.Damage.TotalDamage}
	for _, t := range res.Damage.Targets {
		damage.Targets = append(damage.Targets, aoeTargetOutcomeResponse{
			CombatantID: t.CombatantID.String(),
			DisplayName: t.DisplayName,
			SaveSuccess: t.SaveSuccess,
			DamageDealt: t.DamageDealt,
		})
	}
	resp.Damage = damage
	return resp
}

// parsePendingSaveIDs returns the parsed encounter and pending-save UUIDs.
func parsePendingSaveIDs(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	encounterID, err := parseEncounterID(r)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid encounter ID")
	}
	saveID, err := uuid.Parse(chi.URLParam(r, "saveID"))
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid save ID")
	}
	return encounterID, saveID, nil
}
