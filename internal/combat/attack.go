package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// CharacterFeature represents a single feature entry in a character's Features JSON array.
type CharacterFeature struct {
	Name             string `json:"name"`
	MechanicalEffect string `json:"mechanical_effect"`
}

// hasFeatureEffect checks whether a character's features include one with the
// given mechanical_effect value. This is the shared implementation for
// HasFightingStyle and HasFeat.
func hasFeatureEffect(features pqtype.NullRawMessage, effectID string) bool {
	if !features.Valid || len(features.RawMessage) == 0 {
		return false
	}
	var feats []CharacterFeature
	if err := json.Unmarshal(features.RawMessage, &feats); err != nil {
		return false
	}
	for _, f := range feats {
		if strings.EqualFold(f.MechanicalEffect, effectID) {
			return true
		}
	}
	return false
}

// HasFightingStyle checks whether a character's features include a fighting style
// with the given mechanical effect name (e.g., "two_weapon_fighting").
func HasFightingStyle(features pqtype.NullRawMessage, styleName string) bool {
	return hasFeatureEffect(features, styleName)
}

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
// Monk weapons (when monkLevel > 0) use the higher of STR/DEX like finesse.
func abilityModForWeapon(scores AbilityScores, weapon refdata.Weapon, monkLevel ...int) int {
	strMod := AbilityModifier(scores.Str)
	dexMod := AbilityModifier(scores.Dex)

	if HasProperty(weapon, "finesse") {
		return max(strMod, dexMod)
	}

	// Monk martial arts: use higher of STR/DEX for monk weapons
	ml := 0
	if len(monkLevel) > 0 {
		ml = monkLevel[0]
	}
	if ml > 0 && IsMonkWeapon(weapon) {
		return max(strMod, dexMod)
	}

	if IsRangedWeapon(weapon) {
		return dexMod
	}
	return strMod
}

// AttackModifier returns the total attack modifier: ability mod + proficiency bonus.
func AttackModifier(scores AbilityScores, weapon refdata.Weapon, profBonus int, monkLevel ...int) int {
	return abilityModForWeapon(scores, weapon, monkLevel...) + profBonus
}

// DamageModifier returns the ability modifier added to damage rolls.
func DamageModifier(scores AbilityScores, weapon refdata.Weapon, monkLevel ...int) int {
	return abilityModForWeapon(scores, weapon, monkLevel...)
}

// MaxRange returns the maximum range of a weapon in feet.
// Melee weapons default to 5ft reach (10ft with reach property).
// Ranged weapons use long range if available, else normal range.
func MaxRange(weapon refdata.Weapon) int {
	if !IsRangedWeapon(weapon) {
		if HasProperty(weapon, "reach") {
			return 10
		}
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

// NormalRange returns the normal range of a weapon. For melee, it's the reach (5ft, or 10ft with reach).
// For ranged, it's the normal range value.
func NormalRange(weapon refdata.Weapon) int {
	if !IsRangedWeapon(weapon) {
		if HasProperty(weapon, "reach") {
			return 10
		}
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

// InventoryItem represents a single item in a character's inventory JSON array.
type InventoryItem struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	Type     string `json:"type"`
}

// ParseInventory parses the inventory JSON array into InventoryItem slice.
func ParseInventory(raw json.RawMessage) ([]InventoryItem, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var items []InventoryItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("parsing inventory: %w", err)
	}
	return items, nil
}

// DeductAmmunition decrements the quantity of the named ammo by 1.
// Returns an error if the ammo is not found or has 0 quantity.
func DeductAmmunition(items []InventoryItem, ammoName string) ([]InventoryItem, error) {
	for i := range items {
		if strings.EqualFold(items[i].Name, ammoName) && items[i].Type == "ammunition" {
			if items[i].Quantity <= 0 {
				return items, fmt.Errorf("No %s remaining.", strings.ToLower(ammoName))
			}
			items[i].Quantity--
			return items, nil
		}
	}
	return items, fmt.Errorf("No %s remaining.", strings.ToLower(ammoName))
}

// RecoverAmmunition adds back half (rounded down) of spent ammunition after combat.
func RecoverAmmunition(items []InventoryItem, ammoName string, spent int) []InventoryItem {
	recovered := spent / 2
	for i := range items {
		if strings.EqualFold(items[i].Name, ammoName) && items[i].Type == "ammunition" {
			items[i].Quantity += recovered
			return items
		}
	}
	return items
}

// GetAmmunitionName returns the conventional ammunition name for a weapon.
// Crossbows use "Bolts", other ammunition weapons use "Arrows".
func GetAmmunitionName(weapon refdata.Weapon) string {
	if strings.Contains(strings.ToLower(weapon.Name), "crossbow") {
		return "Bolts"
	}
	return "Arrows"
}

// HasFeat checks whether a character's features include a feat with the given
// mechanical_effect ID (e.g., "crossbow-expert", "tavern-brawler").
func HasFeat(features pqtype.NullRawMessage, featID string) bool {
	return hasFeatureEffect(features, featID)
}

// HasBarbarianClass checks whether a character's classes JSON includes a Barbarian entry.
func HasBarbarianClass(classesJSON json.RawMessage) bool {
	return ClassLevelFromJSON(classesJSON, "Barbarian") > 0
}

// VersatileDamageExpression builds the damage expression using versatile_damage if available.
func VersatileDamageExpression(weapon refdata.Weapon, abilityMod int) string {
	if weapon.VersatileDamage.Valid && weapon.VersatileDamage.String != "" {
		return appendModifier(weapon.VersatileDamage.String, abilityMod)
	}
	return appendModifier(weapon.Damage, abilityMod)
}

// ImprovisedWeapon returns the pseudo-weapon for improvised weapon attacks.
func ImprovisedWeapon() refdata.Weapon {
	return refdata.Weapon{
		ID:         "improvised-weapon",
		Name:       "Improvised Weapon",
		Damage:     "1d4",
		DamageType: "bludgeoning",
		WeaponType: "simple_melee",
	}
}

// ApplyLoadingLimit caps attacks remaining to 1 for loading weapons,
// unless the character has the Crossbow Expert feat.
func ApplyLoadingLimit(attacks int32, isLoading, hasCrossbowExpert bool) int32 {
	if isLoading && !hasCrossbowExpert {
		return 1
	}
	return attacks
}

// ThrownMaxRange returns the max range for a thrown melee weapon.
func ThrownMaxRange(weapon refdata.Weapon) int {
	if weapon.RangeLongFt.Valid {
		return int(weapon.RangeLongFt.Int32)
	}
	if weapon.RangeNormalFt.Valid {
		return int(weapon.RangeNormalFt.Int32)
	}
	return 5
}

// ThrownNormalRange returns the normal range for a thrown melee weapon.
func ThrownNormalRange(weapon refdata.Weapon) int {
	if weapon.RangeNormalFt.Valid {
		return int(weapon.RangeNormalFt.Int32)
	}
	return 5
}

// IsThrownInLongRange returns true if the distance is beyond normal range but within long range
// for a thrown weapon.
func IsThrownInLongRange(weapon refdata.Weapon, distFt int) bool {
	normal := ThrownNormalRange(weapon)
	maxR := ThrownMaxRange(weapon)
	return distFt > normal && distFt <= maxR
}

// AttackInput holds all inputs for resolving a single attack (pure function).
type AttackInput struct {
	AttackerName        string
	TargetName          string
	TargetAC            int
	Weapon              refdata.Weapon
	Scores              AbilityScores
	ProfBonus           int
	DistanceFt          int
	Cover               CoverLevel
	AutoCrit            bool
	AutoCritReason      string
	AttackerConditions  []CombatCondition
	TargetConditions    []CombatCondition
	HostileNearAttacker bool
	AttackerSize        string
	DMAdvantage         bool
	DMDisadvantage      bool
	OverrideDmgMod      *int // If set, overrides the normal ability modifier for damage
	TwoHanded           bool // Use versatile damage die (requires free off-hand)
	OffHandOccupied     bool // True if off-hand is occupied (blocks two-handed)
	Thrown               bool // Melee weapon thrown at range
	IsImprovised        bool // Improvised weapon (no proficiency unless Tavern Brawler)
	ImprovisedThrown    bool // Improvised weapon thrown (range 20/60)
	HasCrossbowExpert   bool // Character has Crossbow Expert feat
	HasTavernBrawler    bool // Character has Tavern Brawler feat
	GWM                 bool // Great Weapon Master: -5 hit, +10 damage (heavy melee)
	Sharpshooter        bool // Sharpshooter: -5 hit, +10 damage (ranged)
	Reckless            bool // Reckless Attack: advantage on melee STR attacks (Barbarian)
	MonkLevel           int  // Monk level (0 = not a monk)
	AttackerHidden      bool // Attacker is hidden (not visible)
	TargetHidden        bool // Target is hidden (not visible)
	AttackerObscurement ObscurementLevel // Effective obscurement for attacker
	TargetObscurement   ObscurementLevel // Effective obscurement for target
}

// AttackResult holds the full result of an attack resolution.
type AttackResult struct {
	AttackerName        string
	TargetName          string
	WeaponName          string
	DistanceFt          int
	IsMelee             bool
	Hit                 bool
	CriticalHit         bool
	AutoCrit            bool
	AutoCritReason      string
	D20Roll             dice.D20Result
	EffectiveAC         int
	DamageTotal         int
	DamageType          string
	DamageDice          string
	DamageRoll          *dice.RollResult
	InLongRange         bool
	Cover               CoverLevel
	RemainingTurn       *refdata.Turn
	RollMode            dice.RollMode
	AdvantageReasons    []string
	DisadvantageReasons []string
	GWM                 bool
	Sharpshooter        bool
	Reckless            bool
	AttackerRevealed    bool // True if a hidden attacker was revealed by this attack
}

// OffhandAttackCommand holds the service-level inputs for an off-hand attack (bonus action).
type OffhandAttackCommand struct {
	Attacker            refdata.Combatant
	Target              refdata.Combatant
	Turn                refdata.Turn
	HostileNearAttacker bool
	AttackerSize        string
	DMAdvantage         bool
	DMDisadvantage      bool
	AttackerVision      VisionCapabilities
	TargetVision        VisionCapabilities
}

// AttackCommand holds the service-level inputs for an attack.
type AttackCommand struct {
	Attacker            refdata.Combatant
	Target              refdata.Combatant
	Turn                refdata.Turn
	WeaponOverride      string
	HostileNearAttacker bool
	AttackerSize        string
	DMAdvantage         bool
	DMDisadvantage      bool
	TwoHanded           bool // Use versatile two-handed grip
	IsImprovised        bool // Improvised weapon attack
	ImprovisedThrown    bool // Improvised weapon thrown
	Thrown               bool // Throw a melee weapon with "thrown" property
	GWM                  bool // Great Weapon Master flag
	Sharpshooter         bool // Sharpshooter flag
	Reckless             bool // Reckless Attack flag
	AttackerVision       VisionCapabilities // Vision capabilities of the attacker
	TargetVision         VisionCapabilities // Vision capabilities of the target
}

// ResolveAttack resolves a single attack using pure inputs. Returns an error if the target
// is out of range.
func ResolveAttack(input AttackInput, roller *dice.Roller) (AttackResult, error) {
	isMelee := !IsRangedWeapon(input.Weapon)

	// Versatile two-handed: reject if off-hand is occupied
	if input.TwoHanded && input.OffHandOccupied {
		return AttackResult{}, fmt.Errorf("cannot use two-handed grip: off-hand is occupied")
	}

	// Determine effective range based on thrown / improvised thrown
	maxR := MaxRange(input.Weapon)
	if input.Thrown && isMelee && HasProperty(input.Weapon, "thrown") {
		maxR = ThrownMaxRange(input.Weapon)
	}
	if input.ImprovisedThrown {
		maxR = 60 // improvised thrown: 20/60
	}

	// Range validation
	if input.DistanceFt > maxR {
		return AttackResult{}, fmt.Errorf("out of range: %dft away (max %dft)", input.DistanceFt, maxR)
	}

	effectiveAC := EffectiveAC(input.TargetAC, input.Cover)

	// Improvised weapons: no proficiency bonus (unless Tavern Brawler)
	profBonus := input.ProfBonus
	if input.IsImprovised && !input.HasTavernBrawler {
		profBonus = 0
	}

	// GWM validation: requires heavy melee weapon
	if input.GWM && (IsRangedWeapon(input.Weapon) || !HasProperty(input.Weapon, "heavy")) {
		return AttackResult{}, fmt.Errorf("Great Weapon Master requires a heavy melee weapon")
	}

	// Sharpshooter validation: requires ranged weapon
	if input.Sharpshooter && !IsRangedWeapon(input.Weapon) {
		return AttackResult{}, fmt.Errorf("Sharpshooter requires a ranged weapon")
	}

	// Reckless validation: requires melee STR-based attack
	if input.Reckless && IsRangedWeapon(input.Weapon) {
		return AttackResult{}, fmt.Errorf("Reckless Attack requires a melee weapon")
	}
	usesDEX := HasProperty(input.Weapon, "finesse") && AbilityModifier(input.Scores.Dex) > AbilityModifier(input.Scores.Str)
	if input.Reckless && usesDEX {
		return AttackResult{}, fmt.Errorf("Reckless Attack requires a STR-based attack (finesse weapon using DEX)")
	}

	atkMod := AttackModifier(input.Scores, input.Weapon, profBonus, input.MonkLevel)
	dmgMod := DamageModifier(input.Scores, input.Weapon, input.MonkLevel)
	if input.OverrideDmgMod != nil {
		dmgMod = *input.OverrideDmgMod
	}

	// GWM / Sharpshooter: -5 to hit, +10 to damage
	gwmSharpshooterBonus := 0
	if input.GWM || input.Sharpshooter {
		atkMod -= 5
		gwmSharpshooterBonus = 10
	}

	result := AttackResult{
		AttackerName: input.AttackerName,
		TargetName:   input.TargetName,
		WeaponName:   input.Weapon.Name,
		DistanceFt:   input.DistanceFt,
		IsMelee:      isMelee,
		EffectiveAC:  effectiveAC,
		DamageType:   input.Weapon.DamageType,
		Cover:        input.Cover,
		InLongRange:  resolveInLongRange(input),
		GWM:          input.GWM,
		Sharpshooter: input.Sharpshooter,
		Reckless:     input.Reckless,
	}

	// Detect advantage/disadvantage
	advInput := AdvantageInput{
		AttackerConditions:  input.AttackerConditions,
		TargetConditions:    input.TargetConditions,
		Weapon:              input.Weapon,
		DistanceFt:          input.DistanceFt,
		HostileNearAttacker: input.HostileNearAttacker,
		AttackerSize:        input.AttackerSize,
		DMAdvantage:         input.DMAdvantage,
		DMDisadvantage:      input.DMDisadvantage,
		Reckless:            input.Reckless,
		AttackerHidden:      input.AttackerHidden,
		TargetHidden:        input.TargetHidden,
		AttackerObscurement: input.AttackerObscurement,
		TargetObscurement:   input.TargetObscurement,
	}
	rollMode, advReasons, disadvReasons := DetectAdvantage(advInput)
	// Thrown/improvised-thrown in long range: add disadvantage
	if result.InLongRange && !IsRangedWeapon(input.Weapon) {
		disadvReasons = append(disadvReasons, "long range")
		rollMode = resolveMode(advReasons, disadvReasons)
	}
	result.RollMode = rollMode
	result.AdvantageReasons = advReasons
	result.DisadvantageReasons = disadvReasons

	// Auto-crit: skip attack roll, auto-hit and auto-crit
	if input.AutoCrit {
		result.Hit = true
		result.CriticalHit = true
		result.AutoCrit = true
		result.AutoCritReason = input.AutoCritReason
		dmg, damageDice, dmgRoll := resolveWeaponDamage(input.Weapon, dmgMod, true, input.TwoHanded, roller, input.MonkLevel)
		result.DamageTotal = dmg + gwmSharpshooterBonus
		result.DamageDice = damageDice
		result.DamageRoll = dmgRoll
		return result, nil
	}

	// Roll attack
	d20, err := roller.RollD20(atkMod, rollMode)
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
	dmg, damageDice, dmgRoll := resolveWeaponDamage(input.Weapon, dmgMod, result.CriticalHit, input.TwoHanded, roller, input.MonkLevel)
	result.DamageTotal = dmg + gwmSharpshooterBonus
	result.DamageDice = damageDice
	result.DamageRoll = dmgRoll

	return result, nil
}

// resolveInLongRange determines if the attack is in long range, handling
// thrown and improvised thrown weapons in addition to ranged weapons.
func resolveInLongRange(input AttackInput) bool {
	if input.Thrown && !IsRangedWeapon(input.Weapon) {
		return IsThrownInLongRange(input.Weapon, input.DistanceFt)
	}
	if input.ImprovisedThrown {
		return input.DistanceFt > 20 && input.DistanceFt <= 60
	}
	return IsInLongRange(input.Weapon, input.DistanceFt)
}

// resolveWeaponDamage handles damage calculation for both normal weapons and unarmed strikes.
// monkLevel is optional: if > 0, monk martial arts die is used for monk weapons/unarmed strikes.
func resolveWeaponDamage(weapon refdata.Weapon, dmgMod int, critical bool, twoHanded bool, roller *dice.Roller, monkLevel ...int) (int, string, *dice.RollResult) {
	ml := 0
	if len(monkLevel) > 0 {
		ml = monkLevel[0]
	}

	// Monk unarmed strike: use martial arts die instead of flat damage
	if weapon.ID == "unarmed-strike" && ml > 0 {
		maDie := MartialArtsDie(ml)
		expr := appendModifier(maDie, dmgMod)
		rollResult, err := roller.RollDamage(expr, critical)
		if err != nil {
			return 0, expr, nil
		}
		if critical {
			return rollResult.Total, appendModifier(fmt.Sprintf("2d%d", MartialArtsDieSides(ml)), dmgMod), &rollResult
		}
		return rollResult.Total, expr, &rollResult
	}

	if weapon.ID == "unarmed-strike" {
		base := 1
		if critical {
			base = 2
		}
		total := max(base+dmgMod, 0)
		return total, fmt.Sprintf("%d", total), nil
	}

	// Monk weapon: upgrade damage die if martial arts die is higher
	if ml > 0 && IsMonkWeapon(weapon) {
		weapon.Damage = MonkDamageExpression(weapon, ml)
	}

	useVersatile := twoHanded && weapon.VersatileDamage.Valid && weapon.VersatileDamage.String != ""

	var expr string
	if useVersatile {
		expr = VersatileDamageExpression(weapon, dmgMod)
	} else {
		expr = DamageExpression(weapon, dmgMod)
	}

	rollResult, err := roller.RollDamage(expr, critical)
	if err != nil {
		return 0, expr, nil
	}

	if critical {
		critWeapon := weapon
		if useVersatile {
			critWeapon.Damage = weapon.VersatileDamage.String
		}
		return rollResult.Total, buildCritDiceDisplay(critWeapon, dmgMod), &rollResult
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
func CheckAutoCrit(conditions json.RawMessage, distFt int) (bool, string) {
	if distFt > 5 {
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

	// Modifier flags annotation
	var flags []string
	if result.GWM {
		flags = append(flags, "GWM -5/+10")
	}
	if result.Sharpshooter {
		flags = append(flags, "Sharpshooter -5/+10")
	}
	if result.Reckless {
		flags = append(flags, "Reckless Attack")
	}
	if len(flags) > 0 {
		fmt.Fprintf(&b, " [%s]", strings.Join(flags, ", "))
	}

	// Advantage/disadvantage annotation
	switch result.RollMode {
	case dice.Advantage:
		fmt.Fprintf(&b, " (advantage \u2014 %s)", strings.Join(result.AdvantageReasons, ", "))
	case dice.Disadvantage:
		fmt.Fprintf(&b, " (disadvantage \u2014 %s)", strings.Join(result.DisadvantageReasons, ", "))
	case dice.AdvantageAndDisadvantage:
		b.WriteString(" (advantage + disadvantage cancel \u2014 normal roll)")
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

// resolveAndPersistAttack resolves an attack from the given input, persists the
// turn resource update, and returns the result with the updated turn attached.
func (s *Service) resolveAndPersistAttack(ctx context.Context, input AttackInput, updatedTurn refdata.Turn, attacker refdata.Combatant, roller *dice.Roller) (AttackResult, error) {
	result, err := ResolveAttack(input, roller)
	if err != nil {
		return AttackResult{}, err
	}

	_, err = s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn))
	if err != nil {
		return AttackResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	// Auto-reveal: hidden attacker is revealed after attacking (hit or miss)
	if !attacker.IsVisible {
		if _, err := s.store.UpdateCombatantVisibility(ctx, refdata.UpdateCombatantVisibilityParams{
			ID:        attacker.ID,
			IsVisible: true,
		}); err != nil {
			return AttackResult{}, fmt.Errorf("revealing attacker: %w", err)
		}
		result.AttackerRevealed = true
	}

	result.RemainingTurn = &updatedTurn
	return result, nil
}

// Attack is the service-level method that orchestrates a full attack:
// weapon resolution, range calculation, auto-crit check, attack resolution,
// and turn resource tracking.
func (s *Service) Attack(ctx context.Context, cmd AttackCommand, roller *dice.Roller) (AttackResult, error) {
	if err := ValidateResource(cmd.Turn, ResourceAttack); err != nil {
		return AttackResult{}, err
	}

	// Improvised weapon: use improvised pseudo-weapon
	if cmd.IsImprovised {
		return s.attackImprovised(ctx, cmd, roller)
	}

	weapon, scores, profBonus, char, err := s.resolveAttackWeaponFull(ctx, cmd)
	if err != nil {
		return AttackResult{}, fmt.Errorf("resolving weapon: %w", err)
	}

	// Validate modifier flag prerequisites
	if cmd.GWM && (char == nil || !HasFeat(char.Features, "great-weapon-master")) {
		return AttackResult{}, fmt.Errorf("Great Weapon Master requires the feat")
	}
	if cmd.Sharpshooter && (char == nil || !HasFeat(char.Features, "sharpshooter")) {
		return AttackResult{}, fmt.Errorf("Sharpshooter requires the feat")
	}
	if cmd.Reckless && (char == nil || !HasBarbarianClass(char.Classes)) {
		return AttackResult{}, fmt.Errorf("Reckless Attack requires Barbarian class")
	}

	// Loading weapons: limit to 1 attack per action
	hasCrossbowExpert := false
	hasTavernBrawler := false
	if char != nil {
		hasCrossbowExpert = HasFeat(char.Features, "crossbow-expert")
		hasTavernBrawler = HasFeat(char.Features, "tavern-brawler")
	}

	// Apply loading limit
	turn := cmd.Turn
	if HasProperty(weapon, "loading") {
		turn.AttacksRemaining = ApplyLoadingLimit(turn.AttacksRemaining, true, hasCrossbowExpert)
	}

	updatedTurn, err := UseAttack(turn)
	if err != nil {
		return AttackResult{}, fmt.Errorf("using attack resource: %w", err)
	}

	// Ammunition: deduct from inventory
	if HasProperty(weapon, "ammunition") && char != nil {
		ammoName := GetAmmunitionName(weapon)
		items, err := ParseInventory(char.Inventory.RawMessage)
		if err != nil {
			return AttackResult{}, fmt.Errorf("parsing inventory: %w", err)
		}
		items, err = DeductAmmunition(items, ammoName)
		if err != nil {
			return AttackResult{}, err
		}
		invJSON, err := json.Marshal(items)
		if err != nil {
			return AttackResult{}, fmt.Errorf("marshaling inventory: %w", err)
		}
		if err := s.store.UpdateCharacterInventory(ctx, char.ID, pqtype.NullRawMessage{RawMessage: invJSON, Valid: true}); err != nil {
			return AttackResult{}, fmt.Errorf("updating inventory: %w", err)
		}
	}

	offHandOccupied := char != nil && char.EquippedOffHand.Valid && char.EquippedOffHand.String != ""

	distFt := combatantDistance(cmd.Attacker, cmd.Target)
	input := buildAttackInput(
		cmd.Attacker, cmd.Target, weapon, scores, profBonus, distFt,
		cmd.HostileNearAttacker, cmd.AttackerSize,
		cmd.DMAdvantage, cmd.DMDisadvantage, nil,
	)
	input.TwoHanded = cmd.TwoHanded
	input.OffHandOccupied = offHandOccupied
	input.Thrown = cmd.Thrown
	input.HasCrossbowExpert = hasCrossbowExpert
	input.HasTavernBrawler = hasTavernBrawler
	input.GWM = cmd.GWM
	input.Sharpshooter = cmd.Sharpshooter
	input.Reckless = cmd.Reckless

	// Monk martial arts: set monk level for DEX/STR auto-select and die upgrade
	if char != nil {
		input.MonkLevel = ClassLevelFromJSON(char.Classes, "Monk")
	}

	// Obscurement from encounter zones
	attackerObs, targetObs, err := s.resolveObscurement(ctx, cmd)
	if err != nil {
		return AttackResult{}, err
	}
	input.AttackerObscurement = attackerObs
	input.TargetObscurement = targetObs

	return s.resolveAndPersistAttack(ctx, input, updatedTurn, cmd.Attacker, roller)
}

// attackImprovised handles improvised weapon attacks at the service level.
func (s *Service) attackImprovised(ctx context.Context, cmd AttackCommand, roller *dice.Roller) (AttackResult, error) {
	weapon := ImprovisedWeapon()
	scores := AbilityScores{Str: 10, Dex: 10}
	profBonus := 2
	hasTavernBrawler := false

	if cmd.Attacker.CharacterID.Valid {
		char, err := s.store.GetCharacter(ctx, cmd.Attacker.CharacterID.UUID)
		if err != nil {
			return AttackResult{}, fmt.Errorf("getting character: %w", err)
		}
		s, err := ParseAbilityScores(char.AbilityScores)
		if err != nil {
			return AttackResult{}, fmt.Errorf("parsing ability scores: %w", err)
		}
		scores = s
		profBonus = int(char.ProficiencyBonus)
		hasTavernBrawler = HasFeat(char.Features, "tavern-brawler")
	}

	updatedTurn, err := UseAttack(cmd.Turn)
	if err != nil {
		return AttackResult{}, fmt.Errorf("using attack resource: %w", err)
	}

	distFt := combatantDistance(cmd.Attacker, cmd.Target)
	input := buildAttackInput(
		cmd.Attacker, cmd.Target, weapon, scores, profBonus, distFt,
		cmd.HostileNearAttacker, cmd.AttackerSize,
		cmd.DMAdvantage, cmd.DMDisadvantage, nil,
	)
	input.IsImprovised = true
	input.ImprovisedThrown = cmd.ImprovisedThrown
	input.HasTavernBrawler = hasTavernBrawler

	// Obscurement from encounter zones
	attackerObs, targetObs, err := s.resolveObscurement(ctx, cmd)
	if err != nil {
		return AttackResult{}, err
	}
	input.AttackerObscurement = attackerObs
	input.TargetObscurement = targetObs

	return s.resolveAndPersistAttack(ctx, input, updatedTurn, cmd.Attacker, roller)
}

// OffhandAttack is the service-level method for a two-weapon fighting off-hand attack.
// It uses the bonus action, validates both weapons are light, and resolves the off-hand attack
// with 0 damage modifier (unless the character has the Two-Weapon Fighting fighting style).
func (s *Service) OffhandAttack(ctx context.Context, cmd OffhandAttackCommand, roller *dice.Roller) (AttackResult, error) {
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return AttackResult{}, err
	}

	if !cmd.Attacker.CharacterID.Valid {
		return AttackResult{}, fmt.Errorf("off-hand attack requires a character (not NPC)")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Attacker.CharacterID.UUID)
	if err != nil {
		return AttackResult{}, fmt.Errorf("getting character: %w", err)
	}

	scores, err := ParseAbilityScores(char.AbilityScores)
	if err != nil {
		return AttackResult{}, fmt.Errorf("parsing ability scores: %w", err)
	}

	// Validate main hand weapon exists and is light
	if !char.EquippedMainHand.Valid || char.EquippedMainHand.String == "" {
		return AttackResult{}, fmt.Errorf("no main hand weapon equipped")
	}
	mainWeapon, err := s.store.GetWeapon(ctx, char.EquippedMainHand.String)
	if err != nil {
		return AttackResult{}, fmt.Errorf("getting main hand weapon: %w", err)
	}
	if !HasProperty(mainWeapon, "light") {
		return AttackResult{}, fmt.Errorf("main hand weapon %q is not light", mainWeapon.Name)
	}

	// Validate off-hand weapon exists and is light
	if !char.EquippedOffHand.Valid || char.EquippedOffHand.String == "" {
		return AttackResult{}, fmt.Errorf("no off-hand weapon equipped")
	}
	offWeapon, err := s.store.GetWeapon(ctx, char.EquippedOffHand.String)
	if err != nil {
		return AttackResult{}, fmt.Errorf("getting off-hand weapon: %w", err)
	}
	if !HasProperty(offWeapon, "light") {
		return AttackResult{}, fmt.Errorf("off-hand weapon %q is not light", offWeapon.Name)
	}

	// Off-hand attacks use 0 damage modifier unless the character has TWF fighting style
	dmgMod := 0
	if HasFightingStyle(char.Features, "two_weapon_fighting") {
		dmgMod = DamageModifier(scores, offWeapon)
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return AttackResult{}, fmt.Errorf("using bonus action: %w", err)
	}

	distFt := combatantDistance(cmd.Attacker, cmd.Target)
	input := buildAttackInput(
		cmd.Attacker, cmd.Target, offWeapon, scores, int(char.ProficiencyBonus), distFt,
		cmd.HostileNearAttacker, cmd.AttackerSize,
		cmd.DMAdvantage, cmd.DMDisadvantage, &dmgMod,
	)

	// Obscurement from encounter zones
	zones, err := s.ListZonesForEncounter(ctx, cmd.Attacker.EncounterID)
	if err != nil {
		return AttackResult{}, fmt.Errorf("listing zones for obscurement: %w", err)
	}
	if len(zones) > 0 {
		attackerCol := colToIndex(cmd.Attacker.PositionCol)
		attackerRow := int(cmd.Attacker.PositionRow) - 1
		targetCol := colToIndex(cmd.Target.PositionCol)
		targetRow := int(cmd.Target.PositionRow) - 1
		input.AttackerObscurement = CombatantObscurement(attackerCol, attackerRow, zones, cmd.AttackerVision)
		input.TargetObscurement = CombatantObscurement(targetCol, targetRow, zones, cmd.TargetVision)
	}

	return s.resolveAndPersistAttack(ctx, input, updatedTurn, cmd.Attacker, roller)
}

// resolveAttackWeaponFull resolves weapon, scores, proficiency, and optionally the character.
func (s *Service) resolveAttackWeaponFull(ctx context.Context, cmd AttackCommand) (refdata.Weapon, AbilityScores, int, *refdata.Character, error) {
	if !cmd.Attacker.CharacterID.Valid {
		// NPC combatant — for now, use unarmed strike as fallback
		return UnarmedStrike(), AbilityScores{Str: 10, Dex: 10}, 2, nil, nil
	}

	char, err := s.store.GetCharacter(ctx, cmd.Attacker.CharacterID.UUID)
	if err != nil {
		return refdata.Weapon{}, AbilityScores{}, 0, nil, fmt.Errorf("getting character: %w", err)
	}

	scores, err := ParseAbilityScores(char.AbilityScores)
	if err != nil {
		return refdata.Weapon{}, AbilityScores{}, 0, nil, fmt.Errorf("parsing ability scores: %w", err)
	}

	profBonus := int(char.ProficiencyBonus)

	// Determine weapon ID: override > equipped main hand > unarmed
	weaponID := cmd.WeaponOverride
	if weaponID == "" && char.EquippedMainHand.Valid {
		weaponID = char.EquippedMainHand.String
	}
	if weaponID == "" {
		return UnarmedStrike(), scores, profBonus, &char, nil
	}

	weapon, err := s.store.GetWeapon(ctx, weaponID)
	if err != nil {
		return refdata.Weapon{}, AbilityScores{}, 0, nil, fmt.Errorf("getting weapon %q: %w", weaponID, err)
	}

	return weapon, scores, profBonus, &char, nil
}

// combatantDistance returns the 3D distance in feet between two combatants.
func combatantDistance(a, b refdata.Combatant) int {
	return Distance3D(
		colToIndex(a.PositionCol), int(a.PositionRow)-1, int(a.AltitudeFt),
		colToIndex(b.PositionCol), int(b.PositionRow)-1, int(b.AltitudeFt),
	)
}

// buildAttackInput constructs the common AttackInput fields shared by Attack and OffhandAttack.
func buildAttackInput(
	attacker, target refdata.Combatant,
	weapon refdata.Weapon,
	scores AbilityScores,
	profBonus int,
	distFt int,
	hostileNear bool,
	attackerSize string,
	dmAdvantage, dmDisadvantage bool,
	overrideDmgMod *int,
) AttackInput {
	autoCrit, autoCritReason := CheckAutoCrit(target.Conditions, distFt)
	attackerConds, _ := parseConditions(attacker.Conditions)
	targetConds, _ := parseConditions(target.Conditions)

	return AttackInput{
		AttackerName:        attacker.DisplayName,
		TargetName:          target.DisplayName,
		TargetAC:            int(target.Ac),
		Weapon:              weapon,
		Scores:              scores,
		ProfBonus:           profBonus,
		DistanceFt:          distFt,
		AutoCrit:            autoCrit,
		AutoCritReason:      autoCritReason,
		AttackerConditions:  attackerConds,
		TargetConditions:    targetConds,
		HostileNearAttacker: hostileNear,
		AttackerSize:        attackerSize,
		DMAdvantage:         dmAdvantage,
		DMDisadvantage:      dmDisadvantage,
		OverrideDmgMod:      overrideDmgMod,
		AttackerHidden:      !attacker.IsVisible,
		TargetHidden:        !target.IsVisible,
	}
}

// resolveObscurement looks up encounter zones and computes effective obscurement
// for both attacker and target based on their positions and vision capabilities.
func (s *Service) resolveObscurement(ctx context.Context, cmd AttackCommand) (ObscurementLevel, ObscurementLevel, error) {
	zones, err := s.ListZonesForEncounter(ctx, cmd.Attacker.EncounterID)
	if err != nil {
		return NotObscured, NotObscured, fmt.Errorf("listing zones for obscurement: %w", err)
	}
	if len(zones) == 0 {
		return NotObscured, NotObscured, nil
	}

	attackerCol := colToIndex(cmd.Attacker.PositionCol)
	attackerRow := int(cmd.Attacker.PositionRow) - 1
	targetCol := colToIndex(cmd.Target.PositionCol)
	targetRow := int(cmd.Target.PositionRow) - 1

	attackerObs := CombatantObscurement(attackerCol, attackerRow, zones, cmd.AttackerVision)
	targetObs := CombatantObscurement(targetCol, targetRow, zones, cmd.TargetVision)

	return attackerObs, targetObs, nil
}

// colToIndex converts a column letter (A-Z) to a 0-based index.
func colToIndex(col string) int {
	if len(col) == 0 {
		return 0
	}
	c := strings.ToUpper(col)[0]
	return int(c - 'A')
}
