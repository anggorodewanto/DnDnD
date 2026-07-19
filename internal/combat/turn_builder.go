package combat

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

// enemyBaseSightTiles is the NPC planner's base (non-magical) sight radius in
// tiles — 60ft / 12 tiles, matching the PC fog path's defaultBaseSightTiles in
// cmd/dndnd. It is the floor a sighted creature can see in a lit area even with
// no darkvision; darkvision/blindsight/truesight extend it via the parsed senses.
const enemyBaseSightTiles = 12

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
	// AvailableReactions lists the pre-roll reactions the PC target may use
	// against this attack. Populated by GenerateEnemyTurnPlan only when the
	// target is a PC with a free, qualifying reaction; the DM picks one in the
	// Turn Builder.
	AvailableReactions []ReactionOption `json:"available_reactions,omitempty"`
	// ChosenReaction is the reaction the DM applied to this attack at execute
	// time (nil = none). Its ACBonus is folded into the target's AC before the
	// hit is (re)evaluated, mirroring the pre-declared /attack reaction window.
	ChosenReaction *ReactionOption `json:"chosen_reaction,omitempty"`
}

// AttackRollResult holds the results of an attack roll.
type AttackRollResult struct {
	ToHitRoll   int  `json:"to_hit_roll"`
	ToHitTotal  int  `json:"to_hit_total"`
	Hit         bool `json:"hit"`
	Critical    bool `json:"critical"`
	DamageRoll  int  `json:"damage_roll"`
	DamageTotal int  `json:"damage_total"`
	// FinalDamage is the damage actually dealt after the target's
	// resistance/immunity/vulnerability is applied. Only meaningful once
	// DamageResolved is true (i.e. after the turn executes and damage lands).
	FinalDamage int `json:"final_damage,omitempty"`
	// DamageResolved is set once damage has been applied to the target, so the
	// combat log can report the dealt amount (FinalDamage) rather than the
	// rolled total. While false (plan preview), the log falls back to DamageTotal.
	DamageResolved bool `json:"damage_resolved,omitempty"`
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
	// MagicalDarknessTiles is the union of every magical-darkness tile on the
	// map (static lighting layer + live-cast Darkness zones). The planner's
	// see-filter uses it (with Grid.Walls) so an NPC only targets PCs it can
	// actually see: darkvision is demoted inside these tiles, mirroring the
	// player-facing fog model in renderer.ComputeVisibilityWithZones.
	MagicalDarknessTiles []renderer.GridPos
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

	// Best reach across this creature's attacks (used by both the movement trim
	// and the reach-filter below).
	bestReach := 5
	for _, a := range attacks {
		if a.ReachFt > bestReach {
			bestReach = a.ReachFt
		}
	}

	// See-filter: an NPC may only target PCs it can actually see. Compute the
	// NPC's own vision (base sight + parsed senses) against the map walls and
	// magical-darkness tiles, then keep only candidates standing on a Visible
	// tile. This stops the planner from swinging at a PC hidden behind a wall
	// or lost in magical darkness. findNearestHostile still applies its own
	// dead/self/faction filters on top.
	var fow *renderer.FogOfWar
	candidates := input.Combatants
	if input.Grid != nil {
		fow = computeNPCVisibility(input)
		candidates = visibleCombatants(input.Combatants, fow)
	}

	// Find nearest hostile (PC) target among the ones we can see.
	nearestTarget, distFt := findNearestHostile(input.Combatant, candidates)

	// Step 1: Movement — path toward nearest hostile.
	var movementStep *TurnStep
	if nearestTarget != nil && input.Grid != nil {
		movementStep = buildMovementStep(input.Combatant, *nearestTarget, input.Creature, input.Grid, input.SpeedFt, attacks)
		if movementStep != nil {
			plan.Steps = append(plan.Steps, *movementStep)
		}
	}

	// Step 2: Attacks — reach-filter. Only emit attacks if, from the NPC's END
	// position (after any planned movement), the target is actually reachable
	// this turn: within melee reach, or within a ranged attack's range while
	// still visible. Otherwise the turn is a hold (no movement/attack was worth
	// emitting) — ExecuteEnemyTurn treats an attack-less plan as a valid pass.
	if nearestTarget != nil && len(attacks) > 0 &&
		npcCanReachTarget(input.Combatant, *nearestTarget, movementStep, bestReach, attacks, fow) {
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

	// Step 4: Bonus actions. F-78c: prefer the structured bonus_actions
	// column when present; legacy rows fall back to scanning the abilities
	// blob via ParseBonusActions.
	bonusActions := ResolveBonusActions(input.Creature, abilities)
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

// npcSenses is the subset of a creature's `senses` JSONB the planner cares
// about, in feet: {"darkvision":60,"blindsight":10,"truesight":120,...}.
type npcSenses struct {
	Darkvision int `json:"darkvision"`
	Blindsight int `json:"blindsight"`
	Truesight  int `json:"truesight"`
}

// parseNPCSenses decodes a creature's senses JSONB. A null/empty/malformed
// payload yields the zero value (base sight only).
func parseNPCSenses(raw pqtype.NullRawMessage) npcSenses {
	var s npcSenses
	if !raw.Valid || len(raw.RawMessage) == 0 {
		return s
	}
	_ = json.Unmarshal(raw.RawMessage, &s)
	return s
}

// sensesTilesFromFeet converts a senses range in feet to tile units (5ft per
// tile). Local to the combat package because cmd/dndnd's tilesFromFeet is not
// importable. Non-positive input clamps to 0.
func sensesTilesFromFeet(ft int) int {
	if ft <= 0 {
		return 0
	}
	return ft / 5
}

// computeNPCVisibility builds the fog-of-war the NPC sees, seeded from a single
// VisionSource at the NPC's tile (base sight + darkvision/blindsight/truesight)
// evaluated against the map walls and magical-darkness tiles. Grid must be
// non-nil (callers guard this).
func computeNPCVisibility(input BuildTurnPlanInput) *renderer.FogOfWar {
	npcCol, npcRow := parsePosition(input.Combatant)
	senses := parseNPCSenses(input.Creature.Senses)
	src := renderer.VisionSource{
		Col:             npcCol,
		Row:             npcRow,
		RangeTiles:      enemyBaseSightTiles,
		DarkvisionTiles: sensesTilesFromFeet(senses.Darkvision),
		BlindsightTiles: sensesTilesFromFeet(senses.Blindsight),
		TruesightTiles:  sensesTilesFromFeet(senses.Truesight),
	}
	return renderer.ComputeVisibilityWithZones(
		[]renderer.VisionSource{src}, nil,
		input.Grid.Walls, input.MagicalDarknessTiles,
		input.Grid.Width, input.Grid.Height,
	)
}

// visibleCombatants keeps only the combatants an NPC can actually perceive as
// attack targets: on a Visible tile in fow AND not hidden. A combatant with
// IsVisible=false took the Hide action and won its Stealth-vs-Perception contest
// (that's what set the flag), so it can stand on a lit, line-of-sight tile yet
// still be unseen — the fog check (tile) and the hidden check (creature) are
// independent, and a target must pass both. Uses the same 0-based (col,row)
// parsePosition convention buildMapGrid uses for occupants, so the tile a
// combatant occupies in the grid is the tile checked against the fog.
func visibleCombatants(combatants []refdata.Combatant, fow *renderer.FogOfWar) []refdata.Combatant {
	visible := make([]refdata.Combatant, 0, len(combatants))
	for i := range combatants {
		if !combatants[i].IsVisible {
			continue // hidden (Hide action) — unseen, not a valid target
		}
		col, row := parsePosition(combatants[i])
		if fow.StateAt(col, row) == renderer.Visible {
			visible = append(visible, combatants[i])
		}
	}
	return visible
}

// npcCanReachTarget reports whether the NPC can strike target this turn from the
// position it ends on (movementStep's Destination if it moved, else its start
// tile): within melee reach, or within a ranged attack's range while the target
// is still visible. fow may be nil (no grid) — then the ranged visibility gate
// is skipped (best-effort).
func npcCanReachTarget(mover, target refdata.Combatant, movementStep *TurnStep, bestReach int, attacks []CreatureAttackEntry, fow *renderer.FogOfWar) bool {
	endCol, endRow := parsePosition(mover)
	if movementStep != nil && movementStep.Movement != nil {
		endCol = movementStep.Movement.Destination.Col
		endRow = movementStep.Movement.Destination.Row
	}
	targetCol, targetRow := parsePosition(target)
	distFt := chebyshevDist(endCol, endRow, targetCol, targetRow) * 5
	if distFt <= bestReach {
		return true
	}
	// Ranged: within a ranged attack's max range AND still visible.
	bestRange := 0
	for _, a := range attacks {
		if a.RangeFt > bestRange {
			bestRange = a.RangeFt
		}
	}
	if bestRange == 0 || distFt > bestRange {
		return false
	}
	if fow == nil {
		return true
	}
	return fow.StateAt(targetCol, targetRow) == renderer.Visible
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

// gridWithoutOccupantAt returns a shallow copy of grid whose Occupants exclude
// any occupant standing on (col,row). Walls/terrain/dimensions are shared. Used
// so A* can path toward a target tile that the target itself occupies; the
// approach path is trimmed to stop within reach, so the mover never enters it.
// Returns grid unchanged when nothing occupies (col,row).
func gridWithoutOccupantAt(grid *pathfinding.Grid, col, row int) *pathfinding.Grid {
	if grid == nil {
		return nil
	}
	filtered := make([]pathfinding.Occupant, 0, len(grid.Occupants))
	removed := false
	for _, o := range grid.Occupants {
		if o.Col == col && o.Row == row {
			removed = true
			continue
		}
		filtered = append(filtered, o)
	}
	if !removed {
		return grid
	}
	clone := *grid
	clone.Occupants = filtered
	return &clone
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

	// Use A* to find path. The target combatant occupies the destination tile,
	// and FindPath refuses to path onto an occupied tile — so we path against a
	// grid copy with the target's own occupant removed. trimPathTobudget below
	// stops the mover once it is within reach (i.e. adjacent), so it never
	// actually enters the target's tile; this only lets A* find the approach.
	pathGrid := gridWithoutOccupantAt(grid, targetCol, targetRow)
	sizeCategory := pathfinding.ParseSizeCategory(creature.Size)
	result, err := pathfinding.FindPath(pathfinding.PathRequest{
		Start:           pathfinding.Point{Col: startCol, Row: startRow},
		End:             pathfinding.Point{Col: targetCol, Row: targetRow},
		SizeCategory:    sizeCategory,
		Grid:            pathGrid,
		MoverAltitudeFt: int(mover.AltitudeFt),
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

// ResolveBonusActions returns the creature's bonus action ability entries.
// F-78c: prefer the structured creatures.bonus_actions JSONB column (data
// model promotion of what was previously a runtime scan). When that column
// is unset or unparseable, fall back to ParseBonusActions scanning the
// abilities blob so legacy creature rows imported before the column existed
// continue to surface Goblin Nimble Escape / Gnoll Rampage / etc.
func ResolveBonusActions(creature refdata.Creature, abilities []CreatureAbilityEntry) []CreatureAbilityEntry {
	if creature.BonusActions.Valid && len(creature.BonusActions.RawMessage) > 0 {
		var structured []CreatureAbilityEntry
		if err := json.Unmarshal(creature.BonusActions.RawMessage, &structured); err == nil && len(structured) > 0 {
			return structured
		}
	}
	return ParseBonusActions(abilities)
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
	// 5b: announce a DM-chosen pre-roll reaction before the attack line, so both
	// #combat-log and the DM action log show e.g. "🛡️ Windreth uses Defensive
	// Duelist — +3 AC" ahead of the swing it turned into a miss.
	if attack.ChosenReaction != nil {
		fmt.Fprintf(b, "%s\n", FormatReactionDeclared(attack.TargetName, *attack.ChosenReaction))
	}
	r := attack.RollResult
	dmg := attackDamagePhrase(attack)
	if r.Critical {
		fmt.Fprintf(b, "\u2694\ufe0f %s vs %s: \U0001f3af **CRITICAL HIT!** (%d) \u2014 %s\n",
			attack.WeaponName, attack.TargetName, r.ToHitTotal, dmg)
		return
	}
	if r.Hit {
		fmt.Fprintf(b, "\u2694\ufe0f %s vs %s: %d to hit \u2014 **Hit!** %s\n",
			attack.WeaponName, attack.TargetName, r.ToHitTotal, dmg)
		return
	}
	fmt.Fprintf(b, "\u2694\ufe0f %s vs %s: %d to hit \u2014 Miss\n",
		attack.WeaponName, attack.TargetName, r.ToHitTotal)
}

// attackDamagePhrase renders the damage portion of an attack log line. Before
// damage is applied (plan preview, DamageResolved=false) it reports the rolled
// total. Once resolved it reports the amount actually dealt (FinalDamage),
// annotating when the target's resistance, immunity, or vulnerability changed
// it from the rolled total.
func attackDamagePhrase(attack *AttackStep) string {
	r := attack.RollResult
	dealt := r.DamageTotal
	if r.DamageResolved {
		dealt = r.FinalDamage
	}
	phrase := fmt.Sprintf("%d %s damage", dealt, attack.DamageType)
	if !r.DamageResolved || r.FinalDamage == r.DamageTotal {
		return phrase
	}
	switch {
	case r.FinalDamage == 0:
		return fmt.Sprintf("%s (immune \u2014 %d negated)", phrase, r.DamageTotal)
	case r.FinalDamage < r.DamageTotal:
		return fmt.Sprintf("%s (resisted \u2014 halved from %d)", phrase, r.DamageTotal)
	default:
		return fmt.Sprintf("%s (vulnerable \u2014 doubled from %d)", phrase, r.DamageTotal)
	}
}
