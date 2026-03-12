package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// HasProperty checks whether a weapon has the specified property (e.g. "finesse", "light").
func HasProperty(weapon refdata.Weapon, prop string) bool {
	for _, p := range weapon.Properties {
		if strings.EqualFold(p, prop) {
			return true
		}
	}
	return false
}

// IsRangedWeapon returns true if the weapon type indicates a ranged weapon.
func IsRangedWeapon(weapon refdata.Weapon) bool {
	return strings.HasSuffix(weapon.WeaponType, "_ranged")
}

// UnarmedStrike returns the pseudo-weapon for unarmed strikes.
// Damage is "0" because unarmed strike damage = 1 + STR modifier (flat, not a die roll).
func UnarmedStrike() refdata.Weapon {
	return refdata.Weapon{
		ID:         "unarmed-strike",
		Name:       "Unarmed Strike",
		Damage:     "0",
		DamageType: "bludgeoning",
		WeaponType: "simple_melee",
	}
}

// abilityModForWeapon returns the appropriate ability modifier for a weapon.
// Finesse weapons use the higher of STR/DEX; ranged weapons use DEX; melee uses STR.
func abilityModForWeapon(scores AbilityScores, weapon refdata.Weapon) int {
	strMod := AbilityModifier(scores.Str)
	dexMod := AbilityModifier(scores.Dex)

	if HasProperty(weapon, "finesse") {
		return max(strMod, dexMod)
	}
	if IsRangedWeapon(weapon) {
		return dexMod
	}
	return strMod
}

// AttackModifier returns the total attack modifier: ability mod + proficiency bonus.
func AttackModifier(scores AbilityScores, weapon refdata.Weapon, profBonus int) int {
	return abilityModForWeapon(scores, weapon) + profBonus
}

// DamageModifier returns the ability modifier added to damage rolls.
func DamageModifier(scores AbilityScores, weapon refdata.Weapon) int {
	return abilityModForWeapon(scores, weapon)
}

// MaxRange returns the maximum range of a weapon in feet.
// Melee weapons default to 5ft reach. Ranged weapons use long range if available, else normal range.
func MaxRange(weapon refdata.Weapon) int {
	if !IsRangedWeapon(weapon) {
		return 5
	}
	if weapon.RangeLongFt.Valid {
		return int(weapon.RangeLongFt.Int32)
	}
	if weapon.RangeNormalFt.Valid {
		return int(weapon.RangeNormalFt.Int32)
	}
	return 5
}

// NormalRange returns the normal range of a weapon. For melee, it's the reach (5ft).
// For ranged, it's the normal range value.
func NormalRange(weapon refdata.Weapon) int {
	if !IsRangedWeapon(weapon) {
		return 5
	}
	if weapon.RangeNormalFt.Valid {
		return int(weapon.RangeNormalFt.Int32)
	}
	return 5
}

// IsInLongRange returns true if distance > normal range but <= long range.
func IsInLongRange(weapon refdata.Weapon, distFt int) bool {
	if !IsRangedWeapon(weapon) {
		return false
	}
	normal := NormalRange(weapon)
	maxR := MaxRange(weapon)
	return distFt > normal && distFt <= maxR
}

// DamageExpression builds the damage dice expression for a weapon attack.
// For unarmed strikes returns empty string (damage computed differently).
// For normal weapons, returns "NdM+mod".
func DamageExpression(weapon refdata.Weapon, abilityMod int) string {
	if weapon.ID == "unarmed-strike" {
		return ""
	}
	return appendModifier(weapon.Damage, abilityMod)
}

// appendModifier appends a signed modifier to a dice expression string.
// Zero modifiers are omitted; negative modifiers include their own minus sign.
func appendModifier(base string, mod int) string {
	if mod == 0 {
		return base
	}
	if mod > 0 {
		return fmt.Sprintf("%s+%d", base, mod)
	}
	return fmt.Sprintf("%s%d", base, mod)
}

// AttackInput holds all inputs for resolving a single attack (pure function).
type AttackInput struct {
	AttackerName   string
	TargetName     string
	TargetAC       int
	Weapon         refdata.Weapon
	Scores         AbilityScores
	ProfBonus      int
	DistanceFt     int
	Cover          CoverLevel
	AutoCrit       bool
	AutoCritReason string
}

// AttackResult holds the full result of an attack resolution.
type AttackResult struct {
	AttackerName   string
	TargetName     string
	WeaponName     string
	DistanceFt     int
	IsMelee        bool
	Hit            bool
	CriticalHit    bool
	AutoCrit       bool
	AutoCritReason string
	D20Roll        dice.D20Result
	EffectiveAC    int
	DamageTotal    int
	DamageType     string
	DamageDice     string
	DamageRoll     *dice.RollResult
	InLongRange    bool
	Cover          CoverLevel
	RemainingTurn  *refdata.Turn
}

// AttackCommand holds the service-level inputs for an attack.
type AttackCommand struct {
	Attacker       refdata.Combatant
	Target         refdata.Combatant
	Turn           refdata.Turn
	WeaponOverride string
}

// ResolveAttack resolves a single attack using pure inputs. Returns an error if the target
// is out of range.
func ResolveAttack(input AttackInput, roller *dice.Roller) (AttackResult, error) {
	isMelee := !IsRangedWeapon(input.Weapon)
	maxR := MaxRange(input.Weapon)

	// Range validation
	if input.DistanceFt > maxR {
		return AttackResult{}, fmt.Errorf("out of range: %dft away (max %dft)", input.DistanceFt, maxR)
	}

	effectiveAC := EffectiveAC(input.TargetAC, input.Cover)
	atkMod := AttackModifier(input.Scores, input.Weapon, input.ProfBonus)
	dmgMod := DamageModifier(input.Scores, input.Weapon)

	result := AttackResult{
		AttackerName: input.AttackerName,
		TargetName:   input.TargetName,
		WeaponName:   input.Weapon.Name,
		DistanceFt:   input.DistanceFt,
		IsMelee:      isMelee,
		EffectiveAC:  effectiveAC,
		DamageType:   input.Weapon.DamageType,
		Cover:        input.Cover,
		InLongRange:  IsInLongRange(input.Weapon, input.DistanceFt),
	}

	// Auto-crit: skip attack roll, auto-hit and auto-crit
	if input.AutoCrit {
		result.Hit = true
		result.CriticalHit = true
		result.AutoCrit = true
		result.AutoCritReason = input.AutoCritReason
		dmg, damageDice, dmgRoll := resolveWeaponDamage(input.Weapon, dmgMod, true, roller)
		result.DamageTotal = dmg
		result.DamageDice = damageDice
		result.DamageRoll = dmgRoll
		return result, nil
	}

	// Roll attack
	d20, err := roller.RollD20(atkMod, dice.Normal)
	if err != nil {
		return AttackResult{}, fmt.Errorf("rolling attack: %w", err)
	}
	result.D20Roll = d20

	// Nat 20 always hits and crits; nat 1 always misses
	if d20.CriticalHit {
		result.Hit = true
		result.CriticalHit = true
	} else if d20.CriticalFail {
		result.Hit = false
	} else {
		result.Hit = d20.Total >= effectiveAC
	}

	if !result.Hit {
		return result, nil
	}

	// Roll damage
	dmg, damageDice, dmgRoll := resolveWeaponDamage(input.Weapon, dmgMod, result.CriticalHit, roller)
	result.DamageTotal = dmg
	result.DamageDice = damageDice
	result.DamageRoll = dmgRoll

	return result, nil
}

// resolveWeaponDamage handles damage calculation for both normal weapons and unarmed strikes.
func resolveWeaponDamage(weapon refdata.Weapon, dmgMod int, critical bool, roller *dice.Roller) (int, string, *dice.RollResult) {
	if weapon.ID == "unarmed-strike" {
		base := 1
		if critical {
			base = 2
		}
		total := max(base+dmgMod, 0)
		return total, fmt.Sprintf("%d", total), nil
	}

	expr := DamageExpression(weapon, dmgMod)
	rollResult, err := roller.RollDamage(expr, critical)
	if err != nil {
		return 0, expr, nil
	}

	if critical {
		return rollResult.Total, buildCritDiceDisplay(weapon, dmgMod), &rollResult
	}
	return rollResult.Total, expr, &rollResult
}

// buildCritDiceDisplay builds the display string for critical hit damage dice.
// Doubles the dice count but not the modifier.
func buildCritDiceDisplay(weapon refdata.Weapon, dmgMod int) string {
	expr, err := dice.ParseExpression(weapon.Damage)
	if err != nil {
		return weapon.Damage
	}

	var parts []string
	for _, g := range expr.Groups {
		parts = append(parts, fmt.Sprintf("%dd%d", g.Count*2, g.Sides))
	}
	return appendModifier(strings.Join(parts, "+"), dmgMod)
}

// CheckAutoCrit checks if an attack should auto-crit based on target conditions and distance.
// Returns (autoCrit bool, reason string).
func CheckAutoCrit(conditions json.RawMessage, distFt int, isMelee bool) (bool, string) {
	if !isMelee || distFt > 5 {
		return false, ""
	}

	var conds []CombatCondition
	if err := json.Unmarshal(conditions, &conds); err != nil {
		return false, ""
	}

	for _, c := range conds {
		switch c.Condition {
		case "paralyzed":
			return true, "target paralyzed within 5ft"
		case "unconscious":
			return true, "target unconscious within 5ft"
		}
	}
	return false, ""
}

// FormatAttackLog formats the combat log output for an attack result.
func FormatAttackLog(result AttackResult) string {
	var b strings.Builder

	// Header line
	fmt.Fprintf(&b, "\u2694\ufe0f  %s attacks %s with %s", result.AttackerName, result.TargetName, result.WeaponName)
	if !result.IsMelee || result.DistanceFt > 5 {
		fmt.Fprintf(&b, " (%dft)", result.DistanceFt)
	}

	if result.AutoCrit {
		fmt.Fprintf(&b, " (auto-crit \u2014 %s)", result.AutoCritReason)
		fmt.Fprintf(&b, "\n    \u2192 Damage: %d %s (doubled dice: %s)", result.DamageTotal, result.DamageType, result.DamageDice)
		return b.String()
	}

	// Attack roll line
	rollStr := fmt.Sprintf("%d (%d + %d)", result.D20Roll.Total, result.D20Roll.Chosen, result.D20Roll.Modifier)
	if result.CriticalHit {
		b.WriteString("\n    \u2192 Roll to hit: \U0001f3af NAT 20 \u2014 CRITICAL HIT!")
	} else if result.Hit {
		fmt.Fprintf(&b, "\n    \u2192 Roll to hit: %s \u2014 HIT", rollStr)
	} else {
		fmt.Fprintf(&b, "\n    \u2192 Roll to hit: %s \u2014 MISS", rollStr)
	}

	// Damage line (only on hit)
	if result.Hit {
		diceLabel := result.DamageDice
		if result.CriticalHit {
			diceLabel = "doubled dice: " + result.DamageDice
		}
		fmt.Fprintf(&b, "\n    \u2192 Damage: %d %s (%s)", result.DamageTotal, result.DamageType, diceLabel)
	}

	return b.String()
}

// Attack is the service-level method that orchestrates a full attack:
// weapon resolution, range calculation, auto-crit check, attack resolution,
// and turn resource tracking.
func (s *Service) Attack(ctx context.Context, cmd AttackCommand, roller *dice.Roller) (AttackResult, error) {
	// Validate attack resource
	if err := ValidateResource(cmd.Turn, ResourceAttack); err != nil {
		return AttackResult{}, err
	}

	// Resolve weapon
	weapon, scores, profBonus, err := s.resolveAttackWeapon(ctx, cmd)
	if err != nil {
		return AttackResult{}, fmt.Errorf("resolving weapon: %w", err)
	}

	// Calculate distance
	distFt := Distance3D(
		colToIndex(cmd.Attacker.PositionCol), int(cmd.Attacker.PositionRow)-1, int(cmd.Attacker.AltitudeFt),
		colToIndex(cmd.Target.PositionCol), int(cmd.Target.PositionRow)-1, int(cmd.Target.AltitudeFt),
	)

	isMelee := !IsRangedWeapon(weapon)

	// Check auto-crit
	autoCrit, autoCritReason := CheckAutoCrit(cmd.Target.Conditions, distFt, isMelee)

	input := AttackInput{
		AttackerName:   cmd.Attacker.DisplayName,
		TargetName:     cmd.Target.DisplayName,
		TargetAC:       int(cmd.Target.Ac),
		Weapon:         weapon,
		Scores:         scores,
		ProfBonus:      profBonus,
		DistanceFt:     distFt,
		AutoCrit:       autoCrit,
		AutoCritReason: autoCritReason,
	}

	result, err := ResolveAttack(input, roller)
	if err != nil {
		return AttackResult{}, err
	}

	// Deduct attack
	updatedTurn, err := UseAttack(cmd.Turn)
	if err != nil {
		return AttackResult{}, fmt.Errorf("using attack resource: %w", err)
	}

	// Persist turn resource update
	_, err = s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn))
	if err != nil {
		return AttackResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	result.RemainingTurn = &updatedTurn
	return result, nil
}

// resolveAttackWeapon resolves the weapon, ability scores, and proficiency bonus for an attack.
func (s *Service) resolveAttackWeapon(ctx context.Context, cmd AttackCommand) (refdata.Weapon, AbilityScores, int, error) {
	if !cmd.Attacker.CharacterID.Valid {
		// NPC combatant — for now, use unarmed strike as fallback
		return UnarmedStrike(), AbilityScores{Str: 10, Dex: 10}, 2, nil
	}

	char, err := s.store.GetCharacter(ctx, cmd.Attacker.CharacterID.UUID)
	if err != nil {
		return refdata.Weapon{}, AbilityScores{}, 0, fmt.Errorf("getting character: %w", err)
	}

	scores, err := ParseAbilityScores(char.AbilityScores)
	if err != nil {
		return refdata.Weapon{}, AbilityScores{}, 0, fmt.Errorf("parsing ability scores: %w", err)
	}

	profBonus := int(char.ProficiencyBonus)

	// Determine weapon ID: override > equipped main hand > unarmed
	weaponID := cmd.WeaponOverride
	if weaponID == "" && char.EquippedMainHand.Valid {
		weaponID = char.EquippedMainHand.String
	}
	if weaponID == "" {
		return UnarmedStrike(), scores, profBonus, nil
	}

	weapon, err := s.store.GetWeapon(ctx, weaponID)
	if err != nil {
		return refdata.Weapon{}, AbilityScores{}, 0, fmt.Errorf("getting weapon %q: %w", weaponID, err)
	}

	return weapon, scores, profBonus, nil
}

// colToIndex converts a column letter (A-Z) to a 0-based index.
func colToIndex(col string) int {
	if len(col) == 0 {
		return 0
	}
	c := strings.ToUpper(col)[0]
	return int(c - 'A')
}
