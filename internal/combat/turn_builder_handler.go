package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

// RegisterEnemyTurnRoutes mounts enemy turn plan routes on the router.
func (h *Handler) RegisterEnemyTurnRoutes(r chi.Router) {
	r.Get("/api/combat/{encounterID}/enemy-turn/{combatantID}/plan", h.GetEnemyTurnPlan)
	r.Post("/api/combat/{encounterID}/enemy-turn", h.ExecuteEnemyTurn)
}

// enemyTurnPlanResponse is the JSON response for a turn plan.
type enemyTurnPlanResponse struct {
	CombatantID string                  `json:"combatant_id"`
	DisplayName string                  `json:"display_name"`
	Steps       []turnStepResponse      `json:"steps"`
	Reactions   []reactionDeclarationResponse `json:"reactions"`
}

type turnStepResponse struct {
	Type      string             `json:"type"`
	Suggested bool               `json:"suggested"`
	Movement  *MovementStep      `json:"movement,omitempty"`
	Attack    *AttackStep        `json:"attack,omitempty"`
	Ability   *AbilityStep       `json:"ability,omitempty"`
}

func toTurnPlanResponse(plan *TurnPlan) enemyTurnPlanResponse {
	steps := make([]turnStepResponse, len(plan.Steps))
	for i, s := range plan.Steps {
		steps[i] = turnStepResponse{
			Type:      s.Type,
			Suggested: s.Suggested,
			Movement:  s.Movement,
			Attack:    s.Attack,
			Ability:   s.Ability,
		}
	}

	reactions := make([]reactionDeclarationResponse, len(plan.Reactions))
	for i, r := range plan.Reactions {
		reactions[i] = toReactionDeclarationResponse(r)
	}

	return enemyTurnPlanResponse{
		CombatantID: plan.CombatantID.String(),
		DisplayName: plan.DisplayName,
		Steps:       steps,
		Reactions:   reactions,
	}
}

// GetEnemyTurnPlan handles GET /api/combat/{encounterID}/enemy-turn/{combatantID}/plan.
func (h *Handler) GetEnemyTurnPlan(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	combatantID, err := uuid.Parse(chi.URLParam(r, "combatantID"))
	if err != nil {
		http.Error(w, "invalid combatant ID", http.StatusBadRequest)
		return
	}

	plan, err := h.svc.GenerateEnemyTurnPlan(r.Context(), encounterID, combatantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, toTurnPlanResponse(plan))
}

// executeEnemyTurnRequest is the JSON request body for executing an enemy turn.
type executeEnemyTurnRequest struct {
	CombatantID string            `json:"combatant_id"`
	Steps       []executeStepReq  `json:"steps"`
}

type executeStepReq struct {
	Type     string       `json:"type"`
	Movement *MovementStep `json:"movement,omitempty"`
	Attack   *AttackStep   `json:"attack,omitempty"`
	Ability  *AbilityStep  `json:"ability,omitempty"`
}

// executeEnemyTurnResponse is the JSON response for an executed enemy turn.
type executeEnemyTurnResponse struct {
	CombatLog string `json:"combat_log"`
	Success   bool   `json:"success"`
}

// ExecuteEnemyTurn handles POST /api/combat/{encounterID}/enemy-turn.
func (h *Handler) ExecuteEnemyTurn(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	var req executeEnemyTurnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	combatantID, err := uuid.Parse(req.CombatantID)
	if err != nil {
		http.Error(w, "invalid combatant_id", http.StatusBadRequest)
		return
	}

	// Convert request steps to TurnPlan
	plan := TurnPlan{
		CombatantID: combatantID,
		Steps:       make([]TurnStep, len(req.Steps)),
	}
	for i, s := range req.Steps {
		plan.Steps[i] = TurnStep{
			Type:     s.Type,
			Movement: s.Movement,
			Attack:   s.Attack,
			Ability:  s.Ability,
		}
	}

	result, err := h.svc.ExecuteEnemyTurn(r.Context(), encounterID, plan, h.roller)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, executeEnemyTurnResponse{
		CombatLog: result.CombatLog,
		Success:   true,
	})
}

// GenerateEnemyTurnPlan generates a suggested turn plan for an NPC combatant.
func (s *Service) GenerateEnemyTurnPlan(ctx context.Context, encounterID, combatantID uuid.UUID) (*TurnPlan, error) {
	combatant, err := s.store.GetCombatant(ctx, combatantID)
	if err != nil {
		return nil, fmt.Errorf("getting combatant: %w", err)
	}
	if !combatant.IsNpc {
		return nil, fmt.Errorf("combatant is not an NPC")
	}

	if !combatant.CreatureRefID.Valid {
		return nil, fmt.Errorf("combatant has no creature reference")
	}

	creature, err := s.store.GetCreature(ctx, combatant.CreatureRefID.String)
	if err != nil {
		return nil, fmt.Errorf("getting creature: %w", err)
	}

	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return nil, fmt.Errorf("listing combatants: %w", err)
	}

	reactions, err := s.store.ListActiveReactionDeclarationsByEncounter(ctx, encounterID)
	if err != nil {
		return nil, fmt.Errorf("listing reactions: %w", err)
	}

	speedFt := parseWalkSpeed(creature.Speed)

	// Build a simple grid for pathfinding — use a 20x20 default if no map available
	grid := buildDefaultGrid(20, 20, combatants, combatant.ID)

	return BuildTurnPlan(BuildTurnPlanInput{
		Combatant:  combatant,
		Creature:   creature,
		Combatants: combatants,
		Grid:       grid,
		Reactions:  reactions,
		SpeedFt:    speedFt,
	})
}

// ExecuteEnemyTurn executes a finalized enemy turn plan — applies movement, rolls attacks,
// applies damage, logs actions, and advances the turn.
func (s *Service) ExecuteEnemyTurn(ctx context.Context, encounterID uuid.UUID, plan TurnPlan, roller *dice.Roller) (*ExecuteTurnPlanResult, error) {
	combatant, err := s.store.GetCombatant(ctx, plan.CombatantID)
	if err != nil {
		return nil, fmt.Errorf("getting combatant: %w", err)
	}

	turn, err := s.store.GetActiveTurnByEncounterID(ctx, encounterID)
	if err != nil {
		return nil, fmt.Errorf("getting active turn: %w", err)
	}

	damageApplied := make(map[uuid.UUID]int32)

	// Process each step
	for i := range plan.Steps {
		step := &plan.Steps[i]
		switch step.Type {
		case StepTypeMovement:
			if step.Movement != nil && len(step.Movement.Path) > 0 {
				dest := step.Movement.Path[len(step.Movement.Path)-1]
				col := indexToColLabel(dest.Col)
				row := int32(dest.Row + 1) // 0-based to 1-based
				if _, err := s.store.UpdateCombatantPosition(ctx, refdata.UpdateCombatantPositionParams{
					ID:          combatant.ID,
					PositionCol: col,
					PositionRow: row,
					AltitudeFt:  combatant.AltitudeFt,
				}); err != nil {
					return nil, fmt.Errorf("updating position: %w", err)
				}
			}

		case StepTypeAttack:
			if step.Attack == nil {
				continue
			}
			// If no roll result provided, roll it now
			if step.Attack.RollResult == nil {
				target, err := s.store.GetCombatant(ctx, step.Attack.TargetID)
				if err != nil {
					continue
				}
				result := RollAttack(*step.Attack, int(target.Ac), roller)
				step.Attack.RollResult = &result
			}
			// Apply damage if hit
			if step.Attack.RollResult != nil && step.Attack.RollResult.Hit {
				dmg := int32(step.Attack.RollResult.DamageTotal)
				damageApplied[step.Attack.TargetID] += dmg
			}

		case StepTypeAbility:
			// Abilities are informational in the plan; DM resolves them
		}
	}

	// Apply accumulated damage
	for targetID, totalDmg := range damageApplied {
		target, err := s.store.GetCombatant(ctx, targetID)
		if err != nil {
			continue
		}
		newHP := target.HpCurrent - totalDmg
		if newHP < 0 {
			newHP = 0
		}
		isAlive := newHP > 0
		if _, err := s.store.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
			ID:        targetID,
			HpCurrent: newHP,
			TempHp:    target.TempHp,
			IsAlive:   isAlive,
		}); err != nil {
			return nil, fmt.Errorf("applying damage to %s: %w", targetID, err)
		}
	}

	// Create action log
	combatLog := FormatCombatLog(plan)
	plan.DisplayName = combatant.DisplayName
	if _, err := s.store.CreateActionLog(ctx, refdata.CreateActionLogParams{
		TurnID:      turn.ID,
		EncounterID: encounterID,
		ActionType:  "enemy_turn",
		ActorID:     combatant.ID,
		Description: nullString(combatLog),
	}); err != nil {
		return nil, fmt.Errorf("creating action log: %w", err)
	}

	// Mark turn resources as used
	turn.ActionUsed = true
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(turn)); err != nil {
		return nil, fmt.Errorf("updating turn actions: %w", err)
	}

	return &ExecuteTurnPlanResult{
		CombatLog:     combatLog,
		DamageApplied: damageApplied,
	}, nil
}

// buildDefaultGrid creates a simple open grid for NPC pathfinding.
func buildDefaultGrid(w, h int, combatants []refdata.Combatant, moverID uuid.UUID) *pathfinding.Grid {
	terrain := make([]renderer.TerrainType, w*h)
	var occupants []pathfinding.Occupant

	for _, c := range combatants {
		if c.ID == moverID || !c.IsAlive {
			continue
		}
		col, row := parsePosition(c)
		occupants = append(occupants, pathfinding.Occupant{
			Col:          col,
			Row:          row,
			IsAlly:       c.IsNpc,
			SizeCategory: pathfinding.SizeMedium,
		})
	}

	return &pathfinding.Grid{
		Width:     w,
		Height:    h,
		Terrain:   terrain,
		Occupants: occupants,
	}
}

// indexToColLabel converts a 0-based column index to a label (0 = "A", 25 = "Z", 26 = "AA").
func indexToColLabel(col int) string {
	result := ""
	col++ // convert to 1-based
	for col > 0 {
		col--
		result = string(rune('A'+col%26)) + result
		col /= 26
	}
	return result
}
