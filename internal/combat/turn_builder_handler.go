package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

// EnemyTurnNotifier is called after a successful enemy turn execution to post
// the combat log to Discord and trigger map regeneration.
type EnemyTurnNotifier interface {
	NotifyEnemyTurnExecuted(ctx context.Context, encounterID uuid.UUID, combatLog string)
}

// SetEnemyTurnNotifier sets the optional notifier called after enemy turn execution.
func (h *Handler) SetEnemyTurnNotifier(n EnemyTurnNotifier) {
	h.enemyTurnNotifier = n
}

// RegisterEnemyTurnRoutes mounts enemy turn plan routes on the router.
func (h *Handler) RegisterEnemyTurnRoutes(r chi.Router) {
	r.Get("/api/combat/{encounterID}/enemy-turn/{combatantID}/plan", h.GetEnemyTurnPlan)
	r.Post("/api/combat/{encounterID}/enemy-turn", h.ExecuteEnemyTurn)
}

// enemyTurnPlanResponse is the JSON response for a turn plan.
type enemyTurnPlanResponse struct {
	CombatantID string                        `json:"combatant_id"`
	DisplayName string                        `json:"display_name"`
	Steps       []turnStepResponse            `json:"steps"`
	Reactions   []reactionDeclarationResponse `json:"reactions"`
}

type turnStepResponse struct {
	Type      string        `json:"type"`
	Suggested bool          `json:"suggested"`
	Movement  *MovementStep `json:"movement,omitempty"`
	Attack    *AttackStep   `json:"attack,omitempty"`
	Ability   *AbilityStep  `json:"ability,omitempty"`
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

	plan, err := h.svc.GenerateEnemyTurnPlan(r.Context(), encounterID, combatantID, h.roller)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, toTurnPlanResponse(plan))
}

// executeEnemyTurnRequest is the JSON request body for executing an enemy turn.
type executeEnemyTurnRequest struct {
	CombatantID string           `json:"combatant_id"`
	Steps       []executeStepReq `json:"steps"`
}

type executeStepReq struct {
	Type     string        `json:"type"`
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

	// Best-effort async notification to Discord (combat log + map regeneration).
	// Uses context.Background() so it isn't cancelled when the HTTP response completes.
	if h.enemyTurnNotifier != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("enemy turn notifier panic: %v\n", r)
				}
			}()
			h.enemyTurnNotifier.NotifyEnemyTurnExecuted(context.Background(), encounterID, result.CombatLog)
		}()
	}

	writeJSON(w, http.StatusOK, executeEnemyTurnResponse{
		CombatLog: result.CombatLog,
		Success:   true,
	})
}

// GenerateEnemyTurnPlan generates a suggested turn plan for an NPC combatant.
func (s *Service) GenerateEnemyTurnPlan(ctx context.Context, encounterID, combatantID uuid.UUID, roller *dice.Roller) (*TurnPlan, error) {
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

	speedFt := ParseWalkSpeed(creature.Speed)

	// F-12: Load the encounter's actual map for pathfinding; fall back to 20x20 open grid.
	// The map's static lighting layer also carries magical-darkness tiles, which
	// feed the planner's see-filter so an NPC can't target a PC it can't see.
	grid := buildDefaultGrid(20, 20, combatants, combatant.ID)
	var magicalDarknessTiles []renderer.GridPos
	enc, err := s.store.GetEncounter(ctx, encounterID)
	if err == nil && enc.MapID.Valid {
		m, merr := s.store.GetMapByIDUnchecked(ctx, enc.MapID.UUID)
		if merr == nil {
			md, perr := renderer.ParseTiledJSON(m.TiledJson, nil, nil)
			if perr == nil && md.Width > 0 && md.Height > 0 {
				grid = buildMapGrid(md, combatants, combatant.ID)
				magicalDarknessTiles = md.MagicalDarknessTiles
			}
		}
	}

	// Union live-cast Darkness (encounter_zones) with the map's static
	// magical-darkness lighting layer so a spell cast mid-combat also blinds the
	// planner, matching the player-facing fog path. Best-effort: a listing error
	// leaves the map-layer darkness in place.
	if zones, zerr := s.store.ListEncounterZonesByEncounterID(ctx, encounterID); zerr == nil {
		for _, z := range zones {
			if z.ZoneType != "magical_darkness" {
				continue
			}
			for _, t := range ZoneAffectedTilesFromShape(z.Shape, z.OriginCol, z.OriginRow, z.Dimensions) {
				magicalDarknessTiles = append(magicalDarknessTiles, renderer.GridPos{Col: t.Col, Row: t.Row})
			}
		}
	}

	plan, err := BuildTurnPlan(BuildTurnPlanInput{
		Combatant:            combatant,
		Creature:             creature,
		Combatants:           combatants,
		Grid:                 grid,
		Reactions:            reactions,
		SpeedFt:              speedFt,
		MagicalDarknessTiles: magicalDarknessTiles,
	})
	if err != nil {
		return nil, err
	}

	// F-22: Pre-roll attacks so the DM can see and fudge rolls in the review UI.
	// 5b: surface a targeted PC's available reactions so the DM can pick one in
	// the Turn Builder before executing (the choice raises the target's AC).
	for i := range plan.Steps {
		step := &plan.Steps[i]
		if step.Type != StepTypeAttack || step.Attack == nil {
			continue
		}
		target, terr := s.store.GetCombatant(ctx, step.Attack.TargetID)
		if terr != nil {
			continue
		}
		if step.Attack.RollResult == nil {
			result := RollAttack(*step.Attack, int(target.Ac), roller)
			step.Attack.RollResult = &result
		}
		if target.CharacterID.Valid {
			if opts, oerr := s.AvailableReactions(ctx, target, encounterID); oerr == nil {
				step.Attack.AvailableReactions = opts
			}
		}
	}

	return plan, nil
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
	// Per-attack damage queue. We apply damage per-hit (rather than
	// aggregating to a totals map) so that ApplyDamage can resolve R/I/V
	// against the actual damage type of each strike. Aggregation across
	// mixed types would silently drop the type info.
	type pendingHit struct {
		targetID   uuid.UUID
		amount     int32
		damageType string
		attack     *AttackStep
	}
	var hits []pendingHit

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
			// 5b: honor a DM-chosen pre-roll reaction only if the target still has
			// it available this round. A separate enemy earlier in the round (its
			// own ExecuteEnemyTurn) — or an earlier attack in this multiattack —
			// may have already spent it; drop a stale/duplicate choice so it can't
			// apply twice.
			if step.Attack.ChosenReaction != nil {
				if free, ferr := s.CanDeclareReaction(ctx, encounterID, step.Attack.TargetID); ferr != nil || !free {
					step.Attack.ChosenReaction = nil
				}
			}
			// A DM-chosen pre-roll reaction raises the target's AC, and so does a
			// Defensive Duelist bonus the target bought on an EARLIER attack —
			// 2024 PHB p.203 keeps that +PB up against melee attacks until the
			// start of the defender's next turn. The two never stack (same
			// effect), so take the larger. Fold the result into the hit check —
			// either at roll time (no pre-roll) or by recomputing the pre-rolled
			// hit against the boosted AC.
			target, targetErr := s.store.GetCombatant(ctx, step.Attack.TargetID)
			acBonus := 0
			if step.Attack.ChosenReaction != nil {
				acBonus = step.Attack.ChosenReaction.ACBonus
			}
			if targetErr == nil {
				if lingering := lingeringDefensiveDuelistAC(target, step.Attack); lingering > acBonus {
					acBonus = lingering
				}
			}
			// If no roll result provided, roll it now
			if step.Attack.RollResult == nil {
				if targetErr != nil {
					continue
				}
				result := RollAttack(*step.Attack, int(target.Ac)+acBonus, roller)
				step.Attack.RollResult = &result
			} else if acBonus > 0 && targetErr == nil {
				applyReactionToRoll(step.Attack.RollResult, int(target.Ac), acBonus)
			}
			// A post-hit damage-halving reaction (Uncanny Dodge) only triggers
			// "when an attacker hits you" — so on a miss it is neither consumed
			// nor announced. Drop it before the consume/log below. A +AC reaction
			// is left in place regardless: it was applied at roll time and is
			// spent whatever outcome it produced.
			if step.Attack.ChosenReaction != nil && step.Attack.ChosenReaction.HalveDamage &&
				(step.Attack.RollResult == nil || !step.Attack.RollResult.Hit) {
				step.Attack.ChosenReaction = nil
			}
			// Record the spent reaction so the PC can't react again this round.
			if step.Attack.ChosenReaction != nil {
				_ = s.markPCReactionUsed(ctx, encounterID, step.Attack.TargetID, *step.Attack.ChosenReaction)
				// Defensive Duelist alone lingers: the one Reaction just spent
				// buys +PB against every melee attack until the start of the
				// defender's next turn. Gated on the reaction ID so a post-hit
				// damage-halving reaction (Uncanny Dodge) never inherits it.
				if step.Attack.ChosenReaction.ID == defensiveDuelistReactionID {
					_ = s.applyLingeringDefensiveDuelistAC(ctx, step.Attack.TargetID, step.Attack.ChosenReaction.ACBonus)
				}
			}
			// Queue damage if hit, preserving per-attack damage type so the
			// ApplyDamage R/I/V resolution can match correctly.
			if step.Attack.RollResult != nil && step.Attack.RollResult.Hit {
				dmg := step.Attack.RollResult.DamageTotal
				// A damage-halving reaction reduces the pre-rolled damage BEFORE
				// it is written to HP, so nothing is resolved retroactively (no
				// full-damage-then-heal-back).
				if step.Attack.ChosenReaction != nil && step.Attack.ChosenReaction.HalveDamage {
					dmg = ApplyUncannyDodge(dmg)
				}
				hits = append(hits, pendingHit{
					targetID:   step.Attack.TargetID,
					amount:     int32(dmg),
					damageType: step.Attack.DamageType,
					attack:     step.Attack,
				})
			}

		case StepTypeAbility:
			// Abilities are informational in the plan; DM resolves them
		}
	}

	// Apply each hit through the ApplyDamage wrapper so Phase 42 (R/I/V,
	// temp HP, exhaustion level-6 death) and Phase 118
	// (concentration save, unconscious-at-0-HP) both fire per strike. The
	// returned summary aggregates the post-R/I/V damage per target.
	for _, hit := range hits {
		target, err := s.store.GetCombatant(ctx, hit.targetID)
		if err != nil {
			continue
		}
		dmgRes, err := s.ApplyDamage(ctx, ApplyDamageInput{
			EncounterID: target.EncounterID,
			Target:      target,
			RawDamage:   int(hit.amount),
			DamageType:  hit.damageType,
		})
		if err != nil {
			return nil, fmt.Errorf("applying damage to %s: %w", hit.targetID, err)
		}
		damageApplied[hit.targetID] += int32(dmgRes.FinalDamage)
		// Record the dealt amount on the attack step so FormatCombatLog reports
		// the post-resistance damage (e.g. a raging target's halved bite) rather
		// than the rolled total.
		if hit.attack != nil && hit.attack.RollResult != nil {
			hit.attack.RollResult.FinalDamage = dmgRes.FinalDamage
			hit.attack.RollResult.DamageResolved = true
		}
	}

	// Create action log. action_log.{before_state,after_state} are NOT NULL,
	// so capture the actor's combatant state before/after the turn (matching the
	// DM move/resolve action_log paths). Omitting these violated the constraint
	// and left combat stuck on the enemy's turn after damage was applied.
	plan.DisplayName = combatant.DisplayName
	combatLog := FormatCombatLog(plan)
	// The local `combatant` still holds the pre-turn position/state (the movement
	// step writes to the DB but never reassigns it), so it is the before-state.
	beforeState, _ := snapshotCombatantState(combatant)
	afterCombatant := combatant
	if updated, gerr := s.store.GetCombatant(ctx, combatant.ID); gerr == nil {
		afterCombatant = updated
	}
	afterState, _ := snapshotCombatantState(afterCombatant)
	if _, err := s.store.CreateActionLog(ctx, refdata.CreateActionLogParams{
		TurnID:      turn.ID,
		EncounterID: encounterID,
		ActionType:  "enemy_turn",
		ActorID:     combatant.ID,
		Description: nullString(combatLog),
		BeforeState: beforeState,
		AfterState:  afterState,
	}); err != nil {
		return nil, fmt.Errorf("creating action log: %w", err)
	}

	// Mark turn resources as used
	turn.ActionUsed = true
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(turn)); err != nil {
		return nil, fmt.Errorf("updating turn actions: %w", err)
	}

	// COV-19: end-of-turn repeat save. A creature held by a "save ends" condition
	// (e.g. paralyzed by Hold Person) re-rolls its save at the end of each of its
	// turns; for an NPC the DM triggers this by running its turn, so roll it here
	// (honest, server-side) and end the condition — dropping the caster's
	// concentration — on a success. A no-op for NPCs holding no save-ends condition.
	resave, rerr := s.rollNPCEndOfTurnResave(ctx, encounterID, combatant)
	if rerr != nil {
		return nil, fmt.Errorf("rolling end-of-turn re-save: %w", rerr)
	}
	if resave != nil {
		resaveLog := strings.Join(resave.Messages, "\n")
		if combatLog == "" {
			combatLog = resaveLog
		} else {
			combatLog = combatLog + "\n" + resaveLog
		}
		if lerr := s.logConditionMessages(ctx, resave.Messages, "condition_resave", combatant.ID, encounterID, turn.ID); lerr != nil {
			return nil, fmt.Errorf("logging end-of-turn re-save: %w", lerr)
		}
	}

	// ISSUE-057: this NPC's turn is now taken — cancel its enemy_turn_ready
	// #dm-queue prompt so the DM Console next_step stops pointing at it. Runs
	// after the turn state is persisted so a notifier hiccup can't undo it.
	s.resolveEnemyTurnReady(ctx, encounterID)

	// F-11: Publish WebSocket snapshot after all mutations complete.
	s.publish(ctx, encounterID)

	return &ExecuteTurnPlanResult{
		CombatLog:     combatLog,
		DamageApplied: damageApplied,
	}, nil
}

// buildMapGrid creates a pathfinding grid from parsed map data.
func buildMapGrid(md *renderer.MapData, combatants []refdata.Combatant, moverID uuid.UUID) *pathfinding.Grid {
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
		Width:     md.Width,
		Height:    md.Height,
		Terrain:   md.TerrainGrid,
		Walls:     md.Walls,
		Occupants: occupants,
	}
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
