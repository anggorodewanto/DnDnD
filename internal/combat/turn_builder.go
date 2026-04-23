package combat

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

// Step type constants for TurnStep.
const (
	StepTypeMovement    = "movement"
	StepTypeAttack      = "attack"
	StepTypeMultiattack = "multiattack"
	StepTypeAbility     = "ability"
	StepTypeBonusAction = "bonus_action"
)

// TurnPlan is the multi-step plan for an NPC/enemy turn.
type TurnPlan struct {
	CombatantID uuid.UUID                     `json:"combatant_id"`
	DisplayName string                        `json:"display_name"`
	Steps       []TurnStep                    `json:"steps"`
	Reactions   []refdata.ReactionDeclaration `json:"reactions,omitempty"`
}

// TurnStep represents one step in a turn plan.
type TurnStep struct {
	Type      string        `json:"type"`
	Suggested bool          `json:"suggested"`
	Movement  *MovementStep `json:"movement,omitempty"`
	Attack    *AttackStep   `json:"attack,omitempty"`
	Ability   *AbilityStep  `json:"ability,omitempty"`
}

// MovementStep holds movement plan data.
type MovementStep struct {
	Path        []pathfinding.Point `json:"path"`
	TotalCostFt int                 `json:"total_cost_ft"`
	Destination pathfinding.Point   `json:"destination"`
}

// AttackStep holds attack plan data.
type AttackStep struct {
	WeaponName string            `json:"weapon_name"`
	ToHit      int               `json:"to_hit"`
	DamageDice string            `json:"damage_dice"`
	DamageType string            `json:"damage_type"`
	ReachFt    int               `json:"reach_ft"`
	TargetID   uuid.UUID         `json:"target_id"`
	TargetName string            `json:"target_name"`
	RollResult *AttackRollResult `json:"roll_result,omitempty"`
}

// AttackRollResult holds the results of an attack roll.
type AttackRollResult struct {
	ToHitRoll   int  `json:"to_hit_roll"`
	ToHitTotal  int  `json:"to_hit_total"`
	Hit         bool `json:"hit"`
	Critical    bool `json:"critical"`
	DamageRoll  int  `json:"damage_roll"`
	DamageTotal int  `json:"damage_total"`
}

// AbilityStep holds a special ability plan data.
type AbilityStep struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsRecharge  bool   `json:"is_recharge"`
	RechargeMin int    `json:"recharge_min,omitempty"`
}

// CreatureAbilityEntry represents one ability from a creature's abilities JSON.
type CreatureAbilityEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// BuildTurnPlanInput holds all data needed to build a turn plan.
type BuildTurnPlanInput struct {
	Combatant  refdata.Combatant
	Creature   refdata.Creature
	Combatants []refdata.Combatant
	Grid       *pathfinding.Grid
	Reactions  []refdata.ReactionDeclaration
	SpeedFt    int32
}

// BuildTurnPlan generates a suggested turn plan for an NPC combatant.
func BuildTurnPlan(input BuildTurnPlanInput) (*TurnPlan, error) {
	plan := &TurnPlan{
		CombatantID: input.Combatant.ID,
		DisplayName: input.Combatant.DisplayName,
		Reactions:   input.Reactions,
	}

	attacks, err := ParseCreatureAttacksWithSource(input.Creature.Attacks, input.Creature.Source)
	if err != nil {
		return nil, fmt.Errorf("parsing creature attacks: %w", err)
	}

	abilities := parseCreatureAbilitiesFromCreature(input.Creature)
	hasMultiattack := hasMultiattackAbility(abilities)

	// Find nearest hostile (PC) target
	nearestTarget, distFt := findNearestHostile(input.Combatant, input.Combatants)

	// Step 1: Movement — path toward nearest hostile
	if nearestTarget != nil && input.Grid != nil {
		movementStep := buildMovementStep(input.Combatant, *nearestTarget, input.Creature, input.Grid, input.SpeedFt, attacks)
		if movementStep != nil {
			plan.Steps = append(plan.Steps, *movementStep)
		}
	}

	// Step 2: Attacks
	if nearestTarget != nil && len(attacks) > 0 {
		if hasMultiattack {
			multiattackSteps := buildMultiattackSteps(attacks, *nearestTarget, abilities)
			plan.Steps = append(plan.Steps, multiattackSteps...)
		} else {
			plan.Steps = append(plan.Steps, buildAttackStep(attacks[0], *nearestTarget))
		}
	}

	// Step 3: Check for recharge abilities
	for _, ability := range abilities {
		if isRechargeAbility(ability.Name) {
			plan.Steps = append(plan.Steps, TurnStep{
				Type:      StepTypeAbility,
				Suggested: true,
				Ability: &AbilityStep{
					Name:        ability.Name,
					Description: ability.Description,
					IsRecharge:  true,
					RechargeMin: parseRechargeMin(ability.Name),
				},
			})
		}
	}

	// Step 4: Bonus actions
	bonusActions := ParseBonusActions(abilities)
	for _, ba := range bonusActions {
		plan.Steps = append(plan.Steps, TurnStep{
			Type:      StepTypeBonusAction,
			Suggested: true,
			Ability: &AbilityStep{
				Name:        ba.Name,
				Description: ba.Description,
			},
		})
	}

	_ = distFt

	return plan, nil
}

// findNearestHostile finds the nearest alive hostile combatant (PC for NPCs).
func findNearestHostile(mover refdata.Combatant, combatants []refdata.Combatant) (*refdata.Combatant, int) {
	moverCol, moverRow := parsePosition(mover)
	var nearest *refdata.Combatant
	bestDist := math.MaxInt32

	for i := range combatants {
		c := &combatants[i]
		if c.ID == mover.ID || c.IsNpc == mover.IsNpc || !c.IsAlive || c.HpCurrent <= 0 {
			continue
		}
		cCol, cRow := parsePosition(*c)
		dist := chebyshevDist(moverCol, moverRow, cCol, cRow) * 5
		if dist < bestDist {
			bestDist = dist
			nearest = c
		}
	}
	return nearest, bestDist
}

// parsePosition converts a combatant's position to 0-based col, row ints.
func parsePosition(c refdata.Combatant) (int, int) {
	coord := c.PositionCol + strconv.Itoa(int(c.PositionRow))
	col, row, err := renderer.ParseCoordinate(coord)
	if err != nil {
		return 0, 0
	}
	return col, row
}

// buildMovementStep creates a movement step if the NPC needs to move to reach its target.
func buildMovementStep(mover refdata.Combatant, target refdata.Combatant, creature refdata.Creature, grid *pathfinding.Grid, speedFt int32, attacks []CreatureAttackEntry) *TurnStep {
	startCol, startRow := parsePosition(mover)
	targetCol, targetRow := parsePosition(target)

	// Find best reach from attacks
	bestReach := 5
	for _, a := range attacks {
		if a.ReachFt > bestReach {
			bestReach = a.ReachFt
		}
	}

	// If already in range, no movement needed
	dist := chebyshevDist(startCol, startRow, targetCol, targetRow) * 5
	if dist <= bestReach {
		return nil
	}

	// Use A* to find path
	sizeCategory := pathfinding.ParseSizeCategory(creature.Size)
	result, err := pathfinding.FindPath(pathfinding.PathRequest{
		Start:        pathfinding.Point{Col: startCol, Row: startRow},
		End:          pathfinding.Point{Col: targetCol, Row: targetRow},
		SizeCategory: sizeCategory,
		Grid:         grid,
	})
	if err != nil || !result.Found {
		return nil
	}

	// Trim path to movement budget, but stop when within reach of target
	trimmedPath, totalCost := trimPathTobudget(result.Path, int(speedFt), targetCol, targetRow, bestReach, grid)
	if len(trimmedPath) <= 1 {
		return nil
	}

	dest := trimmedPath[len(trimmedPath)-1]
	return &TurnStep{
		Type:      StepTypeMovement,
		Suggested: true,
		Movement: &MovementStep{
			Path:        trimmedPath,
			TotalCostFt: totalCost,
			Destination: dest,
		},
	}
}

// trimPathTobudget trims a path to fit within the movement budget, stopping when
// within reach of the target.
func trimPathTobudget(path []pathfinding.Point, budgetFt int, targetCol, targetRow, reachFt int, grid *pathfinding.Grid) ([]pathfinding.Point, int) {
	if len(path) == 0 {
		return nil, 0
	}

	trimmed := []pathfinding.Point{path[0]}
	totalCost := 0

	for i := 1; i < len(path); i++ {
		p := path[i]
		cost := 5 // default tile cost
		if grid != nil {
			idx := p.Row*grid.Width + p.Col
			if idx >= 0 && idx < len(grid.Terrain) && grid.Terrain[idx] == renderer.TerrainDifficultTerrain {
				cost = 10
			}
		}

		if totalCost+cost > budgetFt {
			break
		}

		totalCost += cost
		trimmed = append(trimmed, p)

		// Stop if we're within reach of the target
		dist := chebyshevDist(p.Col, p.Row, targetCol, targetRow) * 5
		if dist <= reachFt {
			break
		}
	}

	return trimmed, totalCost
}

// buildAttackStep creates a single attack step.
func buildAttackStep(attack CreatureAttackEntry, target refdata.Combatant) TurnStep {
	return TurnStep{
		Type:      StepTypeAttack,
		Suggested: true,
		Attack: &AttackStep{
			WeaponName: attack.Name,
			ToHit:      attack.ToHit,
			DamageDice: attack.Damage,
			DamageType: attack.DamageType,
			ReachFt:    attack.ReachFt,
			TargetID:   target.ID,
			TargetName: target.DisplayName,
		},
	}
}

// buildMultiattackSteps creates attack steps for a multiattack sequence.
// It tries to parse the multiattack description to determine the sequence,
// but falls back to using all available attacks if parsing fails.
func buildMultiattackSteps(attacks []CreatureAttackEntry, target refdata.Combatant, abilities []CreatureAbilityEntry) []TurnStep {
	var steps []TurnStep

	// Find multiattack description
	multiattackDesc := ""
	for _, a := range abilities {
		if strings.EqualFold(a.Name, "Multiattack") {
			multiattackDesc = a.Description
			break
		}
	}

	// Try to parse multiattack description for attack sequence
	sequence := parseMultiattackSequence(multiattackDesc, attacks)
	if len(sequence) == 0 {
		// Fallback: use all attacks once each
		for _, a := range attacks {
			steps = append(steps, buildAttackStep(a, target))
		}
		return steps
	}

	for _, a := range sequence {
		steps = append(steps, buildAttackStep(a, target))
	}
	return steps
}

// parseMultiattackSequence attempts to parse a multiattack description to determine
// the attack sequence. Returns nil if parsing fails.
func parseMultiattackSequence(desc string, attacks []CreatureAttackEntry) []CreatureAttackEntry {
	if desc == "" {
		return nil
	}

	lower := strings.ToLower(desc)

	// Build attack name lookup
	attackMap := make(map[string]CreatureAttackEntry)
	for _, a := range attacks {
		attackMap[strings.ToLower(a.Name)] = a
	}

	// Pattern: "two with its scimitar and one with its dagger"
	// or "two melee attacks" etc.
	// Try to extract "NUMBER ATTACK_NAME" patterns
	var sequence []CreatureAttackEntry

	numberWords := map[string]int{
		"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	}

	// For each attack, check if the description mentions it with a count
	for _, a := range attacks {
		attackLower := strings.ToLower(a.Name)
		count := 0

		for word, n := range numberWords {
			// Check patterns like "two with its <weapon>" or "two <weapon>" etc.
			if strings.Contains(lower, word+" with its "+attackLower) ||
				strings.Contains(lower, word+" "+attackLower) {
				count = n
				break
			}
		}

		// Also try digit patterns like "2 scimitar attacks"
		if count == 0 {
			re := regexp.MustCompile(`(\d+)\s+(?:with\s+its\s+)?` + regexp.QuoteMeta(attackLower))
			if m := re.FindStringSubmatch(lower); m != nil {
				if n, err := strconv.Atoi(m[1]); err == nil {
					count = n
				}
			}
		}

		for i := 0; i < count; i++ {
			sequence = append(sequence, a)
		}
	}

	if len(sequence) == 0 {
		return nil
	}

	return sequence
}

// parseCreatureAbilities parses a creature's abilities JSON.
func parseCreatureAbilities(abilities json.RawMessage) []CreatureAbilityEntry {
	if len(abilities) == 0 {
		return nil
	}
	var entries []CreatureAbilityEntry
	if err := json.Unmarshal(abilities, &entries); err != nil {
		return nil
	}
	return entries
}

// open5eProseEntry matches the [{"name":"X","desc":"Y"}] shape Open5e
// returns for both actions and special_abilities. We map `desc` onto
// CreatureAbilityEntry.Description so the turn builder's downstream
// "bonus action" / "multiattack" heuristics keep working on the prose.
type open5eProseEntry struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

// parseOpen5eCreatureAbilities decodes Open5e-cached creature abilities
// AND actions into a single abilities list. Open5e's `actions` column
// cannot be parsed as structured attacks (see
// ParseCreatureAttacksWithSource), so we surface the action prose as
// abilities instead — that way the DM sees the full verbatim text and
// the planner simply emits no structured attack steps for Open5e NPCs.
func parseOpen5eCreatureAbilities(creature refdata.Creature) []CreatureAbilityEntry {
	var abilitiesRaw json.RawMessage
	if creature.Abilities.Valid {
		abilitiesRaw = creature.Abilities.RawMessage
	}
	out := appendOpen5eProse(nil, creature.Attacks)
	out = appendOpen5eProse(out, abilitiesRaw)
	if len(out) == 0 {
		return nil
	}
	return out
}

// appendOpen5eProse appends decoded {name, desc} entries from the Open5e
// prose payload raw to dst. Empty/malformed payloads and blank entries
// are silently skipped.
func appendOpen5eProse(dst []CreatureAbilityEntry, raw json.RawMessage) []CreatureAbilityEntry {
	if len(raw) == 0 {
		return dst
	}
	var entries []open5eProseEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return dst
	}
	for _, e := range entries {
		if strings.TrimSpace(e.Name) == "" && strings.TrimSpace(e.Desc) == "" {
			continue
		}
		dst = append(dst, CreatureAbilityEntry{Name: e.Name, Description: e.Desc})
	}
	return dst
}

// hasMultiattackAbility checks if the creature has a Multiattack ability.
func hasMultiattackAbility(abilities []CreatureAbilityEntry) bool {
	for _, a := range abilities {
		if strings.EqualFold(a.Name, "Multiattack") {
			return true
		}
	}
	return false
}

// ParseBonusActions filters abilities whose description mentions "bonus action" (case-insensitive).
// It excludes Multiattack and Recharge abilities.
func ParseBonusActions(abilities []CreatureAbilityEntry) []CreatureAbilityEntry {
	var result []CreatureAbilityEntry
	for _, a := range abilities {
		if strings.EqualFold(a.Name, "Multiattack") {
			continue
		}
		if isRechargeAbility(a.Name) {
			continue
		}
		if strings.Contains(strings.ToLower(a.Description), "bonus action") {
			result = append(result, a)
		}
	}
	return result
}

// isRechargeAbility checks if an ability name contains a recharge notation.
func isRechargeAbility(name string) bool {
	return strings.Contains(name, "Recharge")
}

// parseRechargeMin extracts the minimum recharge roll from a name like "Fire Breath (Recharge 5-6)".
func parseRechargeMin(name string) int {
	re := regexp.MustCompile(`Recharge (\d+)`)
	m := re.FindStringSubmatch(name)
	if len(m) < 2 {
		return 6
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 6
	}
	return n
}

// RollAttack rolls a to-hit and (if hit) damage for an attack step.
func RollAttack(attack AttackStep, targetAC int, roller *dice.Roller) AttackRollResult {
	d20, _ := roller.RollD20(attack.ToHit, dice.Normal)

	result := AttackRollResult{
		ToHitRoll:  d20.Chosen,
		ToHitTotal: d20.Total,
		Critical:   d20.CriticalHit,
	}

	// Natural 20 always hits, natural 1 always misses
	if d20.CriticalHit {
		result.Hit = true
	} else if d20.CriticalFail {
		result.Hit = false
	} else {
		result.Hit = d20.Total >= targetAC
	}

	if !result.Hit {
		return result
	}

	// Roll damage
	dmgResult, err := roller.RollDamage(attack.DamageDice, result.Critical)
	if err != nil {
		return result
	}
	result.DamageRoll = dmgResult.Total
	result.DamageTotal = dmgResult.Total
	return result
}

// ExecuteTurnPlanInput holds data needed to execute a finalized turn plan.
type ExecuteTurnPlanInput struct {
	Plan        TurnPlan
	EncounterID uuid.UUID
	TurnID      uuid.UUID
}

// ExecuteTurnPlanResult holds the results of executing a turn plan.
type ExecuteTurnPlanResult struct {
	CombatLog     string              `json:"combat_log"`
	DamageApplied map[uuid.UUID]int32 `json:"damage_applied"`
	MovedTo       *pathfinding.Point  `json:"moved_to,omitempty"`
}

// FormatCombatLog formats the combat log output for a completed enemy turn.
func FormatCombatLog(plan TurnPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**%s's Turn**\n", plan.DisplayName)

	for _, step := range plan.Steps {
		switch step.Type {
		case StepTypeMovement:
			if step.Movement != nil {
				fmt.Fprintf(&b, "\U0001f3c3 Moves %dft\n", step.Movement.TotalCostFt)
			}
		case StepTypeAttack:
			if step.Attack != nil && step.Attack.RollResult != nil {
				formatAttackLog(&b, step.Attack)
			}
		case StepTypeAbility:
			if step.Ability != nil {
				fmt.Fprintf(&b, "\u2728 Uses %s\n", step.Ability.Name)
			}
		case StepTypeBonusAction:
			if step.Ability != nil {
				fmt.Fprintf(&b, "\u26a1 Bonus Action: %s\n", step.Ability.Name)
			}
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func formatAttackLog(b *strings.Builder, attack *AttackStep) {
	r := attack.RollResult
	if r.Critical {
		fmt.Fprintf(b, "\u2694\ufe0f %s vs %s: \U0001f3af **CRITICAL HIT!** (%d) \u2014 %d %s damage\n",
			attack.WeaponName, attack.TargetName, r.ToHitTotal, r.DamageTotal, attack.DamageType)
		return
	}
	if r.Hit {
		fmt.Fprintf(b, "\u2694\ufe0f %s vs %s: %d to hit \u2014 **Hit!** %d %s damage\n",
			attack.WeaponName, attack.TargetName, r.ToHitTotal, r.DamageTotal, attack.DamageType)
		return
	}
	fmt.Fprintf(b, "\u2694\ufe0f %s vs %s: %d to hit \u2014 Miss\n",
		attack.WeaponName, attack.TargetName, r.ToHitTotal)
}
