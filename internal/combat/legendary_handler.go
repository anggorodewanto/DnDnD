package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// RegisterLegendaryRoutes mounts legendary and lair action routes on the router.
// The router should already be scoped to /api/combat.
func (h *Handler) RegisterLegendaryRoutes(r chi.Router) {
	r.Get("/{encounterID}/legendary/{combatantID}/plan", h.GetLegendaryActionPlan)
	r.Post("/{encounterID}/legendary", h.ExecuteLegendaryAction)
	r.Get("/{encounterID}/lair-action/plan", h.GetLairActionPlan)
	r.Post("/{encounterID}/lair-action", h.ExecuteLairAction)
	r.Get("/{encounterID}/turn-queue", h.GetTurnQueue)
}

// legendaryActionPlanResponse is the JSON response for a legendary action plan.
type legendaryActionPlanResponse struct {
	CreatureName     string                          `json:"creature_name"`
	CombatantID      string                          `json:"combatant_id"`
	BudgetTotal      int                             `json:"budget_total"`
	BudgetRemaining  int                             `json:"budget_remaining"`
	AvailableActions []legendaryActionOptionResponse `json:"available_actions"`
}

type legendaryActionOptionResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Cost        int    `json:"cost"`
	Affordable  bool   `json:"affordable"`
}

// GetLegendaryActionPlan handles GET /api/combat/{encounterID}/legendary/{combatantID}/plan.
func (h *Handler) GetLegendaryActionPlan(w http.ResponseWriter, r *http.Request) {
	combatantID, err := uuid.Parse(chi.URLParam(r, "combatantID"))
	if err != nil {
		http.Error(w, "invalid combatant ID", http.StatusBadRequest)
		return
	}

	combatant, err := h.svc.store.GetCombatant(r.Context(), combatantID)
	if err != nil {
		http.Error(w, "combatant not found", http.StatusNotFound)
		return
	}

	if !combatant.CreatureRefID.Valid {
		http.Error(w, "combatant has no creature reference", http.StatusBadRequest)
		return
	}

	creature, err := h.svc.store.GetCreature(r.Context(), combatant.CreatureRefID.String)
	if err != nil {
		http.Error(w, "creature not found", http.StatusNotFound)
		return
	}

	abilities := parseCreatureAbilitiesFromCreature(creature)
	info := ParseLegendaryInfo(abilities)
	if info == nil {
		http.Error(w, "creature has no legendary actions", http.StatusNotFound)
		return
	}

	budgetRemaining := info.Budget
	// Check query parameter for current budget
	if br := r.URL.Query().Get("budget_remaining"); br != "" {
		fmt.Sscanf(br, "%d", &budgetRemaining)
	}
	budget := LegendaryActionBudget{Total: info.Budget, Remaining: budgetRemaining}

	plan := BuildLegendaryActionPlan(combatant.DisplayName, info, budget)

	resp := legendaryActionPlanResponse{
		CreatureName:    plan.CreatureName,
		CombatantID:     combatantID.String(),
		BudgetTotal:     plan.BudgetTotal,
		BudgetRemaining: plan.BudgetRemaining,
	}
	for _, a := range plan.AvailableActions {
		resp.AvailableActions = append(resp.AvailableActions, legendaryActionOptionResponse{
			Name:        a.Name,
			Description: a.Description,
			Cost:        a.Cost,
			Affordable:  a.Affordable,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// executeLegendaryActionRequest is the JSON body for executing a legendary action.
type executeLegendaryActionRequest struct {
	CombatantID     string `json:"combatant_id"`
	ActionName      string `json:"action_name"`
	BudgetRemaining int    `json:"budget_remaining"`
}

// executeLegendaryActionResponse is the JSON response for an executed legendary action.
type executeLegendaryActionResponse struct {
	Success         bool   `json:"success"`
	CombatLog       string `json:"combat_log"`
	BudgetRemaining int    `json:"budget_remaining"`
}

// ExecuteLegendaryAction handles POST /api/combat/{encounterID}/legendary.
func (h *Handler) ExecuteLegendaryAction(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	var req executeLegendaryActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	combatantID, err := uuid.Parse(req.CombatantID)
	if err != nil {
		http.Error(w, "invalid combatant_id", http.StatusBadRequest)
		return
	}

	combatant, err := h.svc.store.GetCombatant(r.Context(), combatantID)
	if err != nil {
		http.Error(w, "combatant not found", http.StatusNotFound)
		return
	}

	if !combatant.CreatureRefID.Valid {
		http.Error(w, "combatant has no creature reference", http.StatusBadRequest)
		return
	}

	creature, err := h.svc.store.GetCreature(r.Context(), combatant.CreatureRefID.String)
	if err != nil {
		http.Error(w, "creature not found", http.StatusNotFound)
		return
	}

	abilities := parseCreatureAbilitiesFromCreature(creature)
	info := ParseLegendaryInfo(abilities)
	if info == nil {
		http.Error(w, "creature has no legendary actions", http.StatusNotFound)
		return
	}

	// Find the requested action
	var selectedAction *LegendaryAction
	for _, a := range info.Actions {
		if a.Name == req.ActionName {
			selectedAction = &a
			break
		}
	}
	if selectedAction == nil {
		http.Error(w, fmt.Sprintf("unknown legendary action: %s", req.ActionName), http.StatusBadRequest)
		return
	}

	// Check budget
	budget := LegendaryActionBudget{Total: info.Budget, Remaining: req.BudgetRemaining}

	// F-H06: Server-side budget enforcement
	serverBudget, _ := h.svc.store.GetLegendaryBudget(r.Context(), combatantID)
	if serverBudget >= 0 {
		// Server has tracked budget; use it instead of client-supplied value
		if serverBudget == 0 {
			http.Error(w, "legendary action budget exhausted", http.StatusBadRequest)
			return
		}
		budget.Remaining = serverBudget
	}

	newBudget, err := budget.Spend(selectedAction.Cost)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Decrement server-side budget
	h.svc.store.DecrementLegendaryBudget(r.Context(), combatantID)

	// Format combat log
	budgetSpent := budget.Total - newBudget.Remaining
	combatLog := FormatLegendaryActionLog(combatant.DisplayName, *selectedAction, budgetSpent, budget.Total)

	h.logActionAndNotify(r.Context(), encounterID, combatantID, "legendary_action", combatLog)

	writeJSON(w, http.StatusOK, executeLegendaryActionResponse{
		Success:         true,
		CombatLog:       combatLog,
		BudgetRemaining: newBudget.Remaining,
	})
}

// lairActionPlanResponse is the JSON response for a lair action plan.
type lairActionPlanResponse struct {
	CreatureName     string               `json:"creature_name"`
	AvailableActions []lairActionResponse `json:"available_actions"`
	DisabledActions  []lairActionResponse `json:"disabled_actions"`
}

type lairActionResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// GetLairActionPlan handles GET /api/combat/{encounterID}/lair-action/plan.
func (h *Handler) GetLairActionPlan(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	combatant, info, err := h.findLairCreature(r.Context(), encounterID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	lastUsed, _ := h.svc.store.GetLastLairAction(r.Context(), encounterID)
	if q := r.URL.Query().Get("last_used"); q != "" {
		lastUsed = q
	}
	tracker := LairActionTracker{LastUsedName: lastUsed}
	plan := BuildLairActionPlan(info, tracker)

	resp := lairActionPlanResponse{
		CreatureName: combatant.DisplayName,
	}
	for _, a := range plan.AvailableActions {
		resp.AvailableActions = append(resp.AvailableActions, lairActionResponse{
			Name:        a.Name,
			Description: a.Description,
		})
	}
	for _, a := range plan.DisabledActions {
		resp.DisabledActions = append(resp.DisabledActions, lairActionResponse{
			Name:        a.Name,
			Description: a.Description,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// executeLairActionRequest is the JSON body for executing a lair action.
type executeLairActionRequest struct {
	ActionName     string `json:"action_name"`
	LastUsedAction string `json:"last_used_action"`
}

// executeLairActionResponse is the JSON response for an executed lair action.
type executeLairActionResponse struct {
	Success        bool   `json:"success"`
	CombatLog      string `json:"combat_log"`
	LastUsedAction string `json:"last_used_action"`
}

// ExecuteLairAction handles POST /api/combat/{encounterID}/lair-action.
func (h *Handler) ExecuteLairAction(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	var req executeLairActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	combatant, info, err := h.findLairCreature(r.Context(), encounterID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Enforce no-repeat
	tracker := LairActionTracker{LastUsedName: req.LastUsedAction}
	if !tracker.CanUse(req.ActionName) {
		http.Error(w, "cannot repeat the same lair action on consecutive rounds", http.StatusBadRequest)
		return
	}

	// Validate action exists
	var selectedAction *LairAction
	for _, a := range info.Actions {
		if a.Name == req.ActionName {
			selectedAction = &a
			break
		}
	}
	if selectedAction == nil {
		http.Error(w, fmt.Sprintf("unknown lair action: %s", req.ActionName), http.StatusBadRequest)
		return
	}

	combatLog := FormatLairActionLog(*selectedAction)

	h.svc.store.SetLastLairAction(r.Context(), encounterID, req.ActionName)
	h.logActionAndNotify(r.Context(), encounterID, combatant.ID, "lair_action", combatLog)

	writeJSON(w, http.StatusOK, executeLairActionResponse{
		Success:        true,
		CombatLog:      combatLog,
		LastUsedAction: req.ActionName,
	})
}

// logActionAndNotify records a combat action to the action log and notifies Discord.
func (h *Handler) logActionAndNotify(ctx context.Context, encounterID, actorID uuid.UUID, actionType, combatLog string) {
	turn, err := h.svc.store.GetActiveTurnByEncounterID(ctx, encounterID)
	if err == nil {
		h.svc.store.CreateActionLog(ctx, refdata.CreateActionLogParams{
			TurnID:      turn.ID,
			EncounterID: encounterID,
			ActionType:  actionType,
			ActorID:     actorID,
			Description: nullString(combatLog),
		})
	}
	if h.enemyTurnNotifier != nil {
		go h.enemyTurnNotifier.NotifyEnemyTurnExecuted(ctx, encounterID, combatLog)
	}
}

// findLairCreature finds the NPC combatant in an encounter that has lair actions.
// Returns the combatant and its parsed LairInfo to avoid redundant re-parsing.
func (h *Handler) findLairCreature(ctx context.Context, encounterID uuid.UUID) (*refdata.Combatant, *LairInfo, error) {
	combatants, err := h.svc.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return nil, nil, fmt.Errorf("listing combatants: %w", err)
	}

	for _, c := range combatants {
		if !c.IsNpc || !c.IsAlive || !c.CreatureRefID.Valid {
			continue
		}
		creature, err := h.svc.store.GetCreature(ctx, c.CreatureRefID.String)
		if err != nil {
			continue
		}
		abilities := parseCreatureAbilitiesFromCreature(creature)
		info := ParseLairInfo(abilities)
		if info != nil {
			cc := c // copy to avoid closure issues
			return &cc, info, nil
		}
	}
	return nil, nil, fmt.Errorf("no creature with lair actions found in encounter")
}

// parseCreatureAbilitiesFromCreature extracts abilities from a Creature
// struct. For open5e:* rows the abilities column holds Open5e prose
// (shape [{name, desc}]) and the attacks column likewise holds prose —
// both are decoded via the Open5e-tolerant path so the DM sees the
// verbatim descriptions instead of silently-zeroed structured entries.
func parseCreatureAbilitiesFromCreature(creature refdata.Creature) []CreatureAbilityEntry {
	if isOpen5eSource(creature.Source) {
		return parseOpen5eCreatureAbilities(creature)
	}
	if !creature.Abilities.Valid {
		return nil
	}
	return parseCreatureAbilities(creature.Abilities.RawMessage)
}

// turnQueueResponse is the JSON response for the turn queue.
type turnQueueResponse struct {
	Entries []turnQueueEntryResponse `json:"entries"`
}

type turnQueueEntryResponse struct {
	Type        string `json:"type"`
	CombatantID string `json:"combatant_id,omitempty"`
	DisplayName string `json:"display_name"`
	Initiative  int32  `json:"initiative"`
	IsNPC       bool   `json:"is_npc"`
}

// GetTurnQueue handles GET /api/combat/{encounterID}/turn-queue.
func (h *Handler) GetTurnQueue(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	combatants, err := h.svc.store.ListCombatantsByEncounterID(r.Context(), encounterID)
	if err != nil {
		http.Error(w, "failed to list combatants", http.StatusInternalServerError)
		return
	}

	legendaryCreatures := make(map[uuid.UUID]string)
	lairCreatures := make(map[uuid.UUID]string)

	for _, c := range combatants {
		if !c.IsNpc || !c.IsAlive || !c.CreatureRefID.Valid {
			continue
		}
		creature, err := h.svc.store.GetCreature(r.Context(), c.CreatureRefID.String)
		if err != nil {
			continue
		}
		abilities := parseCreatureAbilitiesFromCreature(creature)
		if HasLegendaryActions(abilities) {
			legendaryCreatures[c.ID] = c.DisplayName
		}
		if HasLairActions(abilities) {
			lairCreatures[c.ID] = c.DisplayName
		}
	}

	entries := BuildTurnQueueEntries(combatants, legendaryCreatures, lairCreatures)

	resp := turnQueueResponse{}
	for _, e := range entries {
		cidStr := ""
		if e.CombatantID != uuid.Nil {
			cidStr = e.CombatantID.String()
		}
		resp.Entries = append(resp.Entries, turnQueueEntryResponse{
			Type:        e.Type,
			CombatantID: cidStr,
			DisplayName: e.DisplayName,
			Initiative:  e.Initiative,
			IsNPC:       e.IsNPC,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}
