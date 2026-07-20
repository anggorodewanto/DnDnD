package combat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/ab/dndnd/internal/httpjson"
	"github.com/ab/dndnd/internal/refdata"
)

// DM override for a combatant's turn action economy.
//
// The other DM overrides reach HP, position, conditions, initiative,
// exhaustion, slots and feature uses — but nothing reached the `turns` row, so
// a mis-adjudicated action/bonus action/reaction/movement/attack count could
// only be repaired with a raw DB write. This endpoint closes that gap.
//
// It deliberately targets the combatant's OWN active turn only. A completed
// turn's flags are mechanically meaningless (the next turn re-initialises
// them), so writing to one would look like a success while changing nothing a
// player can use — the endpoint returns 409 instead.

// overrideTurnResourcesRequest is the JSON body for
// POST .../override/combatant/{combatantID}/turn-resources.
//
// Every resource field is a pointer so an omitted field (nil) is
// distinguishable from an explicit zero value: `"action_used": false` refunds
// the action, while omitting the key carries the turn's current value through
// untouched. At least one must be present. Reason is mandatory — this writes
// the action economy out from under the player, so the audit row must say why.
type overrideTurnResourcesRequest struct {
	ActionUsed          *bool  `json:"action_used"`
	BonusActionUsed     *bool  `json:"bonus_action_used"`
	ReactionUsed        *bool  `json:"reaction_used"`
	MovementRemainingFt *int32 `json:"movement_remaining_ft"`
	AttacksRemaining    *int32 `json:"attacks_remaining"`
	Reason              string `json:"reason"`
}

// hasResourceField reports whether the body asks for at least one change.
func (req overrideTurnResourcesRequest) hasResourceField() bool {
	return req.ActionUsed != nil ||
		req.BonusActionUsed != nil ||
		req.ReactionUsed != nil ||
		req.MovementRemainingFt != nil ||
		req.AttacksRemaining != nil
}

// validate returns the 400-worthy problem with a well-formed body, or nil.
func (req overrideTurnResourcesRequest) validate() error {
	if !req.hasResourceField() {
		return errors.New("at least one turn resource field is required")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return errors.New("reason is required")
	}
	if req.MovementRemainingFt != nil && *req.MovementRemainingFt < 0 {
		return errors.New("movement_remaining_ft must not be negative")
	}
	if req.AttacksRemaining != nil && *req.AttacksRemaining < 0 {
		return errors.New("attacks_remaining must not be negative")
	}
	return nil
}

// turnResourcesSnapshot is the before/after audit shape for a turn-resources
// override: the five DM-editable action-economy fields.
type turnResourcesSnapshot struct {
	ActionUsed          bool  `json:"action_used"`
	BonusActionUsed     bool  `json:"bonus_action_used"`
	ReactionUsed        bool  `json:"reaction_used"`
	MovementRemainingFt int32 `json:"movement_remaining_ft"`
	AttacksRemaining    int32 `json:"attacks_remaining"`
}

// snapshotTurnResources captures a turn's action economy for the audit trail.
func snapshotTurnResources(turn refdata.Turn) turnResourcesSnapshot {
	return turnResourcesSnapshot{
		ActionUsed:          turn.ActionUsed,
		BonusActionUsed:     turn.BonusActionUsed,
		ReactionUsed:        turn.ReactionUsed,
		MovementRemainingFt: turn.MovementRemainingFt,
		AttacksRemaining:    turn.AttacksRemaining,
	}
}

// marshalTurnResources renders a snapshot as the action_log before/after JSON.
func marshalTurnResources(snap turnResourcesSnapshot) json.RawMessage {
	b, _ := json.Marshal(snap)
	return b
}

// applyTurnResources returns turn with each explicitly-supplied resource field
// replaced. Omitted (nil) fields are left as they are.
func applyTurnResources(turn refdata.Turn, req overrideTurnResourcesRequest) refdata.Turn {
	if req.ActionUsed != nil {
		turn.ActionUsed = *req.ActionUsed
	}
	if req.BonusActionUsed != nil {
		turn.BonusActionUsed = *req.BonusActionUsed
	}
	if req.ReactionUsed != nil {
		turn.ReactionUsed = *req.ReactionUsed
	}
	if req.MovementRemainingFt != nil {
		turn.MovementRemainingFt = *req.MovementRemainingFt
	}
	if req.AttacksRemaining != nil {
		turn.AttacksRemaining = *req.AttacksRemaining
	}
	return turn
}

// describeTurnResourceChanges names each resource that actually changed with
// its before→after values, e.g. "action_used true→false, attacks_remaining 0→1".
// Returns "" when the override was a no-op (the DM re-asserted the current
// state), which is allowed and simply audits as such.
func describeTurnResourceChanges(before, after turnResourcesSnapshot) string {
	var parts []string
	if before.ActionUsed != after.ActionUsed {
		parts = append(parts, fmt.Sprintf("action_used %t→%t", before.ActionUsed, after.ActionUsed))
	}
	if before.BonusActionUsed != after.BonusActionUsed {
		parts = append(parts, fmt.Sprintf("bonus_action_used %t→%t", before.BonusActionUsed, after.BonusActionUsed))
	}
	if before.ReactionUsed != after.ReactionUsed {
		parts = append(parts, fmt.Sprintf("reaction_used %t→%t", before.ReactionUsed, after.ReactionUsed))
	}
	if before.MovementRemainingFt != after.MovementRemainingFt {
		parts = append(parts, fmt.Sprintf("movement_remaining_ft %d→%d", before.MovementRemainingFt, after.MovementRemainingFt))
	}
	if before.AttacksRemaining != after.AttacksRemaining {
		parts = append(parts, fmt.Sprintf("attacks_remaining %d→%d", before.AttacksRemaining, after.AttacksRemaining))
	}
	if len(parts) == 0 {
		return "no change"
	}
	return strings.Join(parts, ", ")
}

// OverrideCombatantTurnResources handles
// POST /api/combat/{encounterID}/override/combatant/{combatantID}/turn-resources.
// It rewrites the combatant's own active turn's action economy (action, bonus
// action, reaction, movement, attacks) and audits the change as a dm_override
// (action_log + #combat-log) naming every changed resource before→after.
//
// Body shape is validated BEFORE the turn lock so malformed/incomplete
// requests return 400. Targeting a combatant who has no active turn — either
// the encounter has none, or the active turn belongs to someone else — returns
// 409 rather than silently writing to a completed turn row whose flags the next
// turn will re-initialise. Only real DB failures map to 500.
func (h *DMDashboardHandler) OverrideCombatantTurnResources(w http.ResponseWriter, r *http.Request) {
	encounterID, combatantID, err := parseCombatantOverrideIDs(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req overrideTurnResourcesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, httpjson.DecodeError(err), http.StatusBadRequest)
		return
	}
	if err := req.validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	turn, err := h.svc.store.GetActiveTurnByEncounterID(r.Context(), encounterID)
	if err != nil {
		http.Error(w, "no active turn in this encounter — turn resources exist only for an active turn", http.StatusConflict)
		return
	}
	if turn.CombatantID != combatantID {
		http.Error(w, "combatant is not the active turn — turn resources can only be corrected on the combatant's own active turn", http.StatusConflict)
		return
	}

	if err := h.withTurnLock(r.Context(), turn.ID, func() error {
		combatant, err := h.svc.store.GetCombatant(r.Context(), combatantID)
		if err != nil {
			return fmt.Errorf("getting combatant: %w", err)
		}

		before := snapshotTurnResources(turn)
		updated := applyTurnResources(turn, req)
		if _, err := h.svc.store.UpdateTurnActions(r.Context(), TurnToUpdateParams(updated)); err != nil {
			return fmt.Errorf("updating turn resources: %w", err)
		}
		after := snapshotTurnResources(updated)

		changes := describeTurnResourceChanges(before, after)
		h.logOverride(r.Context(), turn.ID, encounterID, combatantID,
			correctionMsg("turn resources: "+changes, req.Reason),
			marshalTurnResources(before), marshalTurnResources(after))

		base := fmt.Sprintf("⚠️ **DM Correction:** %s turn resources adjusted — %s", combatant.DisplayName, changes)
		h.postCorrection(r.Context(), encounterID, correctionMsg(base, req.Reason))
		return nil
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.svc.publish(r.Context(), encounterID)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
