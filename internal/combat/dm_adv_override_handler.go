package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// overrideAdvantageRequest is the JSON body for the DM dashboard
// "next-attack advantage / disadvantage" override endpoint.
//
// Mode legal values:
//
//	"advantage"    — next attack rolls with DM-granted advantage.
//	"disadvantage" — next attack rolls with DM-granted disadvantage.
//	"none" / ""    — clear any existing override.
//
// The override is persisted in combatants.next_attack_adv_override and
// consumed (cleared) by Service.Attack / OffhandAttack / MartialArtsBonusAttack
// the next time the targeted combatant rolls an attack — see
// Service.consumeDMAdvOverride. This implements the "DM override from
// dashboard" branch of Phase 35 (docs/phases.md lines 170-244).
type overrideAdvantageRequest struct {
	Mode   string `json:"mode"`
	Reason string `json:"reason"`
}

// OverrideCombatantNextAttackAdvantage handles
// POST /api/combat/{encounterID}/override/combatant/{combatantID}/advantage.
// The request body is overrideAdvantageRequest. The override is per-attack:
// consumed exactly once when the affected combatant rolls an attack.
func (h *DMDashboardHandler) OverrideCombatantNextAttackAdvantage(w http.ResponseWriter, r *http.Request) {
	encounterID, combatantID, err := parseCombatantOverrideIDs(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req overrideAdvantageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if err := validateOverrideMode(req.Mode); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	turn, err := h.svc.store.GetActiveTurnByEncounterID(r.Context(), encounterID)
	if err != nil {
		http.Error(w, "no active turn", http.StatusNotFound)
		return
	}

	if err := h.withTurnLock(r.Context(), turn.ID, func() error {
		return h.applyDMAdvOverride(r.Context(), encounterID, combatantID, turn.ID, req)
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// applyDMAdvOverride persists the override, logs the action, and posts the
// DM correction message. Splits out of OverrideCombatantNextAttackAdvantage
// so the lock wrapper stays tight.
func (h *DMDashboardHandler) applyDMAdvOverride(
	ctx context.Context,
	encounterID, combatantID, turnID uuid.UUID,
	req overrideAdvantageRequest,
) error {
	c, err := h.svc.store.GetCombatant(ctx, combatantID)
	if err != nil {
		return fmt.Errorf("combatant not found: %w", err)
	}

	before, _ := json.Marshal(map[string]any{
		"next_attack_adv_override": c.NextAttackAdvOverride,
	})
	if err := h.svc.setDMAdvOverride(ctx, combatantID, req.Mode); err != nil {
		return err
	}
	after, _ := json.Marshal(map[string]string{"next_attack_adv_override": req.Mode})

	h.logOverride(ctx, turnID, encounterID, combatantID, req.Reason, before, after)

	base := formatDMAdvOverrideMessage(c.DisplayName, req.Mode)
	h.postCorrection(ctx, encounterID, correctionMsg(base, req.Reason))
	return nil
}

// formatDMAdvOverrideMessage returns the DM correction message body
// posted to #combat-log when the override is applied or cleared.
func formatDMAdvOverrideMessage(name, mode string) string {
	if mode == "" || mode == "none" {
		return fmt.Sprintf("⚠️ **DM Correction:** %s next-attack advantage override cleared", name)
	}
	return fmt.Sprintf("⚠️ **DM Correction:** %s next attack rolls with %s (DM)", name, mode)
}

// validateOverrideMode returns an error for unknown modes. Legal modes are
// "advantage", "disadvantage", "none", and "" (treated as clear).
func validateOverrideMode(mode string) error {
	switch mode {
	case "", "none", dmAdvOverrideAdvantage, dmAdvOverrideDisadvantage:
		return nil
	}
	return fmt.Errorf("invalid mode %q (want advantage|disadvantage|none)", mode)
}
