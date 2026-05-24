package combat

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

// applyMasteryEffects resolves the target-side consequence of a 2024 Weapon
// Mastery property that fired on an attack. It is a no-op unless
// result.MasteryProperty is set (which ResolveAttack only does when the
// attacker knows the weapon's mastery), so existing non-mastery attacks are
// completely unaffected.
//
//   - "graze": a miss carrying DamageTotal > 0 deals that flat ability-modifier
//     damage to the target through the standard ApplyDamage pipeline so
//     resistance / immunity / vulnerability and temp HP still apply.
//   - "topple": a hit forces the target to make a CON save vs
//     result.MasteryToppleSaveDC; on a failure the Prone condition is applied.
//   - "vex": a hit applies a vex_advantage condition to the ATTACKER, scoped to
//     the target, granting advantage on the attacker's next attack vs that same
//     target (mirrors the Help action's help_advantage; single-shot consume).
//   - "sap": a hit applies a sap_disadvantage condition to the TARGET, imposing
//     disadvantage on the target's next attack (single-shot consume).
func (s *Service) applyMasteryEffects(ctx context.Context, attacker, target refdata.Combatant, result *AttackResult, roller *dice.Roller) error {
	switch result.MasteryProperty {
	case "graze":
		return s.applyGrazeDamage(ctx, target, result)
	case "topple":
		return s.applyToppleSave(ctx, attacker, target, result, roller)
	case "vex":
		return s.applyVexAdvantage(ctx, attacker, target)
	case "sap":
		return s.applySapDisadvantage(ctx, attacker, target)
	case "slow":
		return s.applySlowedCondition(ctx, attacker, target)
	case "push":
		return s.applyPushEffect(ctx, attacker, target)
	default:
		return nil
	}
}

// applySlowedCondition applies a "slowed" condition to the target it just hit.
// The 2024 Slow mastery reduces the target's Speed by 10 ft until the start of
// the attacker's next turn; the speed penalty itself is applied in
// EffectiveSpeed. The condition lives on the target with the attacker as source
// so it self-expires at the start of the attacker's next turn, matching the
// reckless/help/vex single-round convention.
func (s *Service) applySlowedCondition(ctx context.Context, attacker, target refdata.Combatant) error {
	cond := CombatCondition{
		Condition:         "slowed",
		DurationRounds:    1,
		SourceCombatantID: attacker.ID.String(),
		ExpiresOn:         "start_of_turn",
	}
	if _, _, err := s.ApplyCondition(ctx, target.ID, cond); err != nil {
		return fmt.Errorf("applying slowed condition: %w", err)
	}
	return nil
}

// applyPushEffect moves a Large-or-smaller target 10 ft (2 squares) straight
// away from the attacker. Huge/Gargantuan targets are not pushed. The target
// is moved square-by-square along the away vector, clamped to the encounter
// map bounds and stopping before the first occupied square (reusing the
// UpdateCombatantPosition store method the /shove push path already uses).
func (s *Service) applyPushEffect(ctx context.Context, attacker, target refdata.Combatant) error {
	targetSize, err := s.resolveCombatantSize(ctx, target)
	if err != nil {
		return fmt.Errorf("resolving push target size: %w", err)
	}
	if targetSize >= pathfinding.SizeHuge {
		return nil // Huge / Gargantuan targets are not pushed
	}

	width, height, err := s.resolveMapBounds(ctx, target.EncounterID)
	if err != nil {
		return fmt.Errorf("resolving map bounds for push: %w", err)
	}

	occupied, err := s.occupiedSquares(ctx, target.EncounterID, target.ID)
	if err != nil {
		return fmt.Errorf("resolving occupied squares for push: %w", err)
	}

	destCol, destRow := computePushSquares(
		colToInt(attacker.PositionCol), int(attacker.PositionRow),
		colToInt(target.PositionCol), int(target.PositionRow),
		2, width, height, occupied,
	)

	// No movement possible (blocked immediately or already at the edge).
	if destCol == colToInt(target.PositionCol) && destRow == int(target.PositionRow) {
		return nil
	}

	if _, err := s.store.UpdateCombatantPosition(ctx, refdata.UpdateCombatantPositionParams{
		ID:          target.ID,
		PositionCol: colIntToLabel(destCol),
		PositionRow: int32(destRow),
		AltitudeFt:  target.AltitudeFt,
	}); err != nil {
		return fmt.Errorf("updating pushed target position: %w", err)
	}
	return nil
}

// resolveMapBounds returns the encounter map's width/height in squares. When
// the encounter has no map (or it cannot be loaded), bounds of 0 are returned
// and computePushSquares treats them as "unbounded".
func (s *Service) resolveMapBounds(ctx context.Context, encounterID uuid.UUID) (width, height int, err error) {
	enc, err := s.store.GetEncounter(ctx, encounterID)
	if err != nil {
		return 0, 0, fmt.Errorf("getting encounter: %w", err)
	}
	if !enc.MapID.Valid {
		return 0, 0, nil
	}
	m, err := s.store.GetMapByIDUnchecked(ctx, enc.MapID.UUID)
	if err != nil {
		return 0, 0, nil // graceful: treat as unbounded if the map can't be loaded
	}
	return int(m.WidthSquares), int(m.HeightSquares), nil
}

// occupiedSquares returns the set of grid squares occupied by other alive
// combatants in the encounter (excluding the combatant being moved).
func (s *Service) occupiedSquares(ctx context.Context, encounterID, excludeID uuid.UUID) (map[[2]int]bool, error) {
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return nil, fmt.Errorf("listing combatants: %w", err)
	}
	occupied := make(map[[2]int]bool, len(combatants))
	for _, c := range combatants {
		if c.ID == excludeID || !c.IsAlive {
			continue
		}
		occupied[[2]int{colToInt(c.PositionCol), int(c.PositionRow)}] = true
	}
	return occupied, nil
}

// computePushSquares walks the target away from the attacker by up to `squares`
// 5ft steps along the (clamped) away vector. Each step is rejected if it leaves
// the map (when width/height > 0) or lands on an occupied square; the walk stops
// at the last valid square reached. Returns the final (col, row) — equal to the
// target's start when no step was possible.
func computePushSquares(attackerCol, attackerRow, targetCol, targetRow, squares, width, height int, occupied map[[2]int]bool) (int, int) {
	dc := sign(targetCol - attackerCol)
	dr := sign(targetRow - attackerRow)
	if dc == 0 && dr == 0 {
		return targetCol, targetRow // co-located: no defined away vector
	}

	col, row := targetCol, targetRow
	for i := 0; i < squares; i++ {
		nextCol, nextRow := col+dc, row+dr
		if width > 0 && (nextCol < 1 || nextCol > width) {
			break
		}
		if height > 0 && (nextRow < 1 || nextRow > height) {
			break
		}
		if occupied != nil && occupied[[2]int{nextCol, nextRow}] {
			break
		}
		col, row = nextCol, nextRow
	}
	return col, row
}

// applyVexAdvantage applies a vex_advantage condition to the attacker, scoped
// to the target it just hit. It reuses the same CombatCondition shape and
// single-shot/expiry convention as the Help action's help_advantage so the
// existing target-scoping (DetectAdvantage) and consume machinery applies. The
// next attack vs that target spends the grant; the condition also self-expires
// at the start of the attacker's next turn.
func (s *Service) applyVexAdvantage(ctx context.Context, attacker, target refdata.Combatant) error {
	cond := CombatCondition{
		Condition:         "vex_advantage",
		DurationRounds:    1,
		SourceCombatantID: attacker.ID.String(),
		TargetCombatantID: target.ID.String(),
		ExpiresOn:         "start_of_turn",
	}
	if _, _, err := s.ApplyCondition(ctx, attacker.ID, cond); err != nil {
		return fmt.Errorf("applying vex advantage: %w", err)
	}
	return nil
}

// applySapDisadvantage applies a sap_disadvantage condition to the target it
// just hit. The condition lives on the target so that when the target later
// makes an attack (where it is the attacker), DetectAdvantage adds disadvantage.
// It uses the same single-shot/expiry convention as the reckless/help markers:
// it self-expires at the start of the sapped creature's next turn and the
// next attack spends it.
func (s *Service) applySapDisadvantage(ctx context.Context, _ refdata.Combatant, target refdata.Combatant) error {
	cond := CombatCondition{
		Condition:         "sap_disadvantage",
		DurationRounds:    1,
		SourceCombatantID: target.ID.String(),
		ExpiresOn:         "start_of_turn",
	}
	if _, _, err := s.ApplyCondition(ctx, target.ID, cond); err != nil {
		return fmt.Errorf("applying sap disadvantage: %w", err)
	}
	return nil
}

// applyGrazeDamage applies the Graze miss-damage to the target. The damage was
// computed in ResolveAttack (ability modifier, min 0) and carried on
// result.DamageTotal. A zero total is a no-op.
func (s *Service) applyGrazeDamage(ctx context.Context, target refdata.Combatant, result *AttackResult) error {
	if result.DamageTotal <= 0 {
		return nil
	}
	if _, err := s.ApplyDamage(ctx, ApplyDamageInput{
		EncounterID: target.EncounterID,
		Target:      target,
		RawDamage:   result.DamageTotal,
		DamageType:  result.DamageType,
	}); err != nil {
		return fmt.Errorf("applying graze damage: %w", err)
	}
	return nil
}

// applyToppleSave resolves the target's CON save against the Topple DC and
// applies the Prone condition on a failure. The save uses the target's CON
// modifier (creature saving-throw bonus when present, else the CON ability
// modifier) via the shared resolveTargetConSave helper.
func (s *Service) applyToppleSave(ctx context.Context, attacker, target refdata.Combatant, result *AttackResult, roller *dice.Roller) error {
	conSaveBonus, err := s.resolveTargetConSave(ctx, target)
	if err != nil {
		return fmt.Errorf("resolving topple CON save: %w", err)
	}

	d20Result, err := roller.RollD20(conSaveBonus, dice.Normal)
	if err != nil {
		return fmt.Errorf("rolling topple CON save: %w", err)
	}

	if d20Result.Total >= result.MasteryToppleSaveDC {
		return nil // save succeeds → no Prone
	}

	prone := CombatCondition{
		Condition:         "prone",
		DurationRounds:    0,
		SourceCombatantID: attacker.ID.String(),
	}
	if _, _, err := s.ApplyCondition(ctx, target.ID, prone); err != nil {
		return fmt.Errorf("applying prone from topple: %w", err)
	}
	return nil
}
