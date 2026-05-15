package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/gamemap/renderer"
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
	OverrideDmgMod      *int                // If set, overrides the normal ability modifier for damage
	TwoHanded           bool                // Use versatile damage die (requires free off-hand)
	OffHandOccupied     bool                // True if off-hand is occupied (blocks two-handed)
	Thrown              bool                // Melee weapon thrown at range
	IsImprovised        bool                // Improvised weapon (no proficiency unless Tavern Brawler)
	ImprovisedThrown    bool                // Improvised weapon thrown (range 20/60)
	HasCrossbowExpert   bool                // Character has Crossbow Expert feat
	HasTavernBrawler    bool                // Character has Tavern Brawler feat
	GWM                 bool                // Great Weapon Master: -5 hit, +10 damage (heavy melee)
	Sharpshooter        bool                // Sharpshooter: -5 hit, +10 damage (ranged)
	Reckless            bool                // Reckless Attack: advantage on melee STR attacks (Barbarian)
	MonkLevel           int                 // Monk level (0 = not a monk)
	AttackerHidden      bool                // Attacker is hidden (not visible)
	TargetHidden        bool                // Target is hidden (not visible)
	AttackerObscurement ObscurementLevel    // Effective obscurement for attacker
	TargetObscurement   ObscurementLevel    // Effective obscurement for target
	// TargetCombatantID is the ID of the combatant currently being attacked.
	// SR-018: piped into DetectAdvantage so target-scoped attacker conditions
	// (help_advantage) only fire when the named target is the one under attack.
	TargetCombatantID string
	Features            []FeatureDefinition // Feature Effect System definitions (magic items, etc.)
	IsRaging            bool                // Attacker is currently raging (Phase 46)
	WearingArmor        bool                // Attacker is wearing armor (Defense fighting style)
	OneHandedMeleeOnly  bool                // Wielding a one-handed melee weapon with no off-hand weapon (Dueling)
	AllyWithinFt        int                 // Distance to nearest ally relative to target (Pack Tactics, Sneak Attack)
	AbilityUsed         string              // "str" or "dex" — which ability mod was chosen for this attack
	UsedThisTurn        map[string]bool     // Per-turn feature usage tracking (Sneak Attack OncePerTurn)
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
	InvisibilityBroken  bool // True if standard Invisibility condition was broken by this attack

	// Class-feature post-hit prompt eligibility hints (D-46/D-48b/D-49/D-51).
	// The combat service surfaces these flags so the Discord layer can fire
	// the corresponding ReactionPromptStore posts (PromptStunningStrike /
	// PromptDivineSmite / PromptBardicInspiration). Pure data — the service
	// does not itself post Discord UI.
	PromptStunningStrikeEligible    bool   // Monk melee hit with ki remaining
	PromptStunningStrikeKiAvailable int    // current ki points the monk can spend
	PromptDivineSmiteEligible       bool   // Paladin melee hit with at least one slot available
	PromptDivineSmiteSlots          []int  // sorted ascending list of available slot levels
	PromptBardicInspirationEligible bool   // Attacker holds an un-expired Bardic Inspiration die
	PromptBardicInspirationDie      string // die expression (d6/d8/d10/d12)

	// SR-010: list of EffectType strings whose conditions included
	// OncePerTurn:true and which actually fired (passed condition filtering)
	// in the on_damage_roll pass. Service.Attack reads this to mark the
	// effect types used so subsequent attacks by the same combatant — same
	// turn or a reaction on another creature's turn — skip them until the
	// combatant's own turn starts again.
	OncePerTurnEffectsFired []string
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
	// Walls are encounter-map wall segments used to compute attacker→target
	// cover (Phase 33 / C-33). A nil/empty slice degrades to "no wall cover";
	// creature-granted cover still applies via the encounter's combatant list.
	Walls []renderer.WallSegment
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
	TwoHanded           bool               // Use versatile two-handed grip
	IsImprovised        bool               // Improvised weapon attack
	ImprovisedThrown    bool               // Improvised weapon thrown
	Thrown              bool               // Throw a melee weapon with "thrown" property
	GWM                 bool               // Great Weapon Master flag
	Sharpshooter        bool               // Sharpshooter flag
	Reckless            bool               // Reckless Attack flag
	AttackerVision      VisionCapabilities // Vision capabilities of the attacker
	TargetVision        VisionCapabilities // Vision capabilities of the target
	// Walls are encounter-map wall segments used to compute attacker→target
	// cover (Phase 33 / C-33). A nil/empty slice degrades to "no wall cover";
	// creature-granted cover still applies via the encounter's combatant list.
	Walls []renderer.WallSegment
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

	// Detect advantage/disadvantage BEFORE the Feature Effect System runs so
	// effects with HasAdvantage / AdvantageOrAllyWithin filters (Sneak Attack,
	// Pack Tactics) can read the resolved roll mode.
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
		AbilityUsed:         input.AbilityUsed,
		TargetCombatantID:   input.TargetCombatantID,
		HasCrossbowExpert:   input.HasCrossbowExpert,
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

	// Apply Feature Effect System bonuses (class features, fighting styles,
	// magic items). Only post-cancellation Advantage counts as "has advantage"
	// for FES purposes — cancelled adv+disadv = normal roll.
	var fesDamageDice []string
	if len(input.Features) > 0 {
		attackCtx := BuildAttackEffectContext(AttackEffectInput{
			Weapon:             input.Weapon,
			HasAdvantage:       rollMode == dice.Advantage,
			AllyWithinFt:       input.AllyWithinFt,
			WearingArmor:       input.WearingArmor,
			OneHandedMeleeOnly: input.OneHandedMeleeOnly,
			IsRaging:           input.IsRaging,
			AbilityUsed:        input.AbilityUsed,
			UsedThisTurn:       input.UsedThisTurn,
		})
		atkResult := ProcessEffects(input.Features, TriggerOnAttackRoll, attackCtx)
		atkMod += atkResult.FlatModifier

		dmgResult := ProcessEffects(input.Features, TriggerOnDamageRoll, attackCtx)
		dmgMod += dmgResult.FlatModifier
		fesDamageDice = dmgResult.ExtraDice
		// SR-010: surface once-per-turn effect types that actually fired
		// so the service layer can mark them used for this combatant's
		// "turn window" (since their own turn last started). The damage
		// trigger is where Sneak Attack's extra_damage_dice lives.
		for _, re := range dmgResult.AppliedEffects {
			if re.Effect.Conditions.OncePerTurn {
				result.OncePerTurnEffectsFired = append(result.OncePerTurnEffectsFired, string(re.Effect.Type))
			}
		}
	}

	// Auto-crit: skip attack roll, auto-hit and auto-crit
	if input.AutoCrit {
		result.Hit = true
		result.CriticalHit = true
		result.AutoCrit = true
		result.AutoCritReason = input.AutoCritReason
		dmg, damageDice, dmgRoll := resolveWeaponDamage(input.Weapon, dmgMod, true, input.TwoHanded, roller, input.MonkLevel)
		extra := rollFESExtraDice(fesDamageDice, true, roller)
		result.DamageTotal = dmg + gwmSharpshooterBonus + extra
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
	extra := rollFESExtraDice(fesDamageDice, result.CriticalHit, roller)
	result.DamageTotal = dmg + gwmSharpshooterBonus + extra
	result.DamageDice = damageDice
	result.DamageRoll = dmgRoll

	return result, nil
}

// rollFESExtraDice rolls each dice expression collected from the Feature
// Effect System (Sneak Attack, smite-style on-hit dice, etc.) and returns
// the summed total. On a critical hit each dice group's count is doubled
// (per Roller.RollDamage). Empty input returns 0.
func rollFESExtraDice(exprs []string, critical bool, roller *dice.Roller) int {
	total := 0
	for _, expr := range exprs {
		r, err := roller.RollDamage(expr, critical)
		if err != nil {
			continue
		}
		total += r.Total
	}
	return total
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
func CheckAutoCrit(conditions json.RawMessage, distFt int, weapon refdata.Weapon) (bool, string) {
	if distFt > 5 {
		return false, ""
	}
	if IsRangedWeapon(weapon) {
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

	if result.InvisibilityBroken {
		fmt.Fprintf(&b, "\n    \u2192 \U0001f441\ufe0f Invisibility ends \u2014 %s is visible again.", result.AttackerName)
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

	// Break standard Invisibility (not Greater) on attack per 5e rules.
	broken, err := s.breakInvisibilityAndPersist(ctx, attacker)
	if err != nil {
		return AttackResult{}, err
	}
	result.InvisibilityBroken = broken

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

	// C-40: a charmed combatant cannot make attacks against the source of
	// the charm. Check before consuming any resource or rolling.
	if err := validateCharmedAttack(cmd.Attacker, cmd.Target); err != nil {
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

	// Phase 38 / SR-011: Reckless is declared on the FIRST attack of the
	// action per RAW (the carry-through is implicit because the attacker
	// keeps the transient `reckless` condition applied below). The previous
	// gate read `cmd.Turn.ActionUsed`, but Service.Attack never sets that
	// field for multi-attack — only `AttacksRemaining` decrements. Compare
	// `AttacksRemaining` against the character's resolved max attacks per
	// action; equality means no attack has been spent yet this turn. Reckless
	// requires Barbarian class (gated above) so `char` is always non-nil
	// here, but stay defensive in case future callers diverge.
	if cmd.Reckless && char != nil {
		maxAttacks := int32(s.resolveAttacksPerAction(ctx, *char))
		if cmd.Turn.AttacksRemaining < maxAttacks {
			return AttackResult{}, fmt.Errorf("Reckless Attack must be declared on the first attack of the turn")
		}
	}

	// Phase 33 / C-33: compute attacker→target cover BEFORE burning the
	// attack resource or deducting ammo. Total cover short-circuits via
	// ErrTargetFullyCovered; half / three-quarters cover are forwarded to
	// ResolveAttack via AttackInput.Cover (applied below).
	coverLevel, err := s.resolveAttackCover(ctx, cmd.Attacker, cmd.Target, cmd.Walls)
	if err != nil {
		return AttackResult{}, err
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

	// C-35 — consume any DM-set per-attack advantage/disadvantage override
	// for this attacker. Override ORs into the caller flags (so a Reckless
	// or other context-derived flag stays in effect even when the override
	// is cleared) and the persistent row is deleted single-use.
	dmAdv, dmDisadv := s.consumeDMAdvOverride(ctx, cmd.Attacker, cmd.DMAdvantage, cmd.DMDisadvantage)

	input := buildAttackInput(
		cmd.Attacker, cmd.Target, weapon, scores, profBonus, distFt,
		cmd.HostileNearAttacker, cmd.AttackerSize,
		dmAdv, dmDisadv, nil,
	)
	// C-35 — auto-populate attacker-size / hostile-near context so the
	// heavy-weapon and ranged-adjacent disadvantage paths fire end-to-end.
	// Command-supplied values still win over the auto-detected ones.
	s.populateAttackContext(ctx, &input, cmd.Attacker)
	input.TwoHanded = cmd.TwoHanded
	input.OffHandOccupied = offHandOccupied
	input.Thrown = cmd.Thrown
	input.HasCrossbowExpert = hasCrossbowExpert
	input.HasTavernBrawler = hasTavernBrawler
	input.GWM = cmd.GWM
	input.Sharpshooter = cmd.Sharpshooter
	input.Reckless = cmd.Reckless
	input.Cover = coverLevel

	// Monk martial arts: set monk level for DEX/STR auto-select and die upgrade
	if char != nil {
		input.MonkLevel = ClassLevelFromJSON(char.Classes, "Monk")
	}

	// Obscurement from encounter zones
	attackerObs, targetObs, err := s.resolveObscurement(ctx, cmd.Attacker.EncounterID, cmd.Attacker, cmd.Target, cmd.AttackerVision, cmd.TargetVision)
	if err != nil {
		return AttackResult{}, err
	}
	input.AttackerObscurement = attackerObs
	input.TargetObscurement = targetObs

	// Wire Feature Effect System: builds class/fighting-style features and
	// populates EffectContext flags (IsRaging, AllyWithinFt, AbilityUsed,
	// WearingArmor, OneHandedMeleeOnly) so Sneak Attack, Rage damage,
	// Pack Tactics, etc. actually fire end-to-end.
	if err := s.populateAttackFES(ctx, &input, cmd, char, weapon, scores); err != nil {
		return AttackResult{}, err
	}

	result, err := s.resolveAndPersistAttack(ctx, input, updatedTurn, cmd.Attacker, roller)
	if err != nil {
		return result, err
	}

	// SR-010: persist the once-per-turn effect-type bits so subsequent
	// attacks by the same attacker (whether on the same turn or as a
	// reaction during an enemy turn) skip those effects. Runs after the
	// attack actually resolved so a miss / out-of-range error doesn't
	// burn the once-per-turn slot.
	s.markUsedEffects(cmd.Attacker.EncounterID, cmd.Attacker.ID, result.OncePerTurnEffectsFired)

	// D-46-rage-auto-end-quiet — mark the raging attacker so the
	// no-attack-no-damage auto-end check at end-of-turn sees the activity.
	s.markRageAttacked(ctx, cmd.Attacker)

	// D-48b / D-49 / D-51: surface class-feature post-hit prompt eligibility
	// hints on the AttackResult. The Discord handler reads these to decide
	// whether to post the Stunning Strike / Divine Smite / Bardic Inspiration
	// reaction prompts. No DB writes happen here — pure data shaping.
	s.populatePostHitPrompts(ctx, &result, cmd.Attacker, char)

	// C-38 — Reckless Attack's target-side half: apply a transient `reckless`
	// marker on the attacker until the start of their next turn. While
	// active, DetectAdvantage grants advantage to attackers targeting them.
	if cmd.Reckless {
		s.applyRecklessMarker(ctx, cmd.Attacker)
	}

	// SR-018 — clear any help_advantage scoped to this target. The grant is
	// single-shot: the next attack vs the named target spends it. Runs after
	// resolveAndPersistAttack so a range/cover/charmed rejection does not
	// burn the Help.
	s.consumeHelpAdvantage(ctx, cmd.Attacker, cmd.Target)

	// C-37 — track ammunition spent for post-combat half-recovery (Phase 37).
	if char != nil && HasProperty(weapon, "ammunition") {
		s.recordAmmoForAttack(cmd.Attacker.EncounterID, cmd.Attacker.ID, weapon)
	}

	// Phase 37 thrown weapon: the weapon leaves the attacker's hand. Clear
	// EquippedMainHand so subsequent attacks/bonus actions can't keep
	// hitting with a thrown javelin. Inventory retains the item so the
	// player can re-equip from a stack on their next turn.
	if cmd.Thrown && char != nil && char.EquippedMainHand.Valid && char.EquippedMainHand.String != "" {
		if _, equipErr := s.store.UpdateCharacterEquipment(ctx, refdata.UpdateCharacterEquipmentParams{
			ID:               char.ID,
			EquippedMainHand: sql.NullString{},
			EquippedOffHand:  char.EquippedOffHand,
			EquippedArmor:    char.EquippedArmor,
			Ac:               char.Ac,
		}); equipErr != nil {
			// Silently swallow — the attack already landed; a follow-up
			// /equip will recover. Do not break the combat log.
			_ = equipErr
		}
	}

	return result, nil
}

// attackImprovised handles improvised weapon attacks at the service level.
func (s *Service) attackImprovised(ctx context.Context, cmd AttackCommand, roller *dice.Roller) (AttackResult, error) {
	weapon := ImprovisedWeapon()
	scores := AbilityScores{Str: 10, Dex: 10}
	profBonus := 2
	hasTavernBrawler := false
	var charPtr *refdata.Character

	if cmd.Attacker.CharacterID.Valid {
		char, err := s.store.GetCharacter(ctx, cmd.Attacker.CharacterID.UUID)
		if err != nil {
			return AttackResult{}, fmt.Errorf("getting character: %w", err)
		}
		parsed, err := ParseAbilityScores(char.AbilityScores)
		if err != nil {
			return AttackResult{}, fmt.Errorf("parsing ability scores: %w", err)
		}
		// SR-022: druid Wild Shaped into a beast swings with beast STR/DEX
		// even on improvised attacks. ResolveAttackerScores degrades to
		// druid scores when the lookup fails (SR-006 pattern), so a missing
		// beast row never blocks the roll.
		scores = ResolveAttackerScores(ctx, s.store, cmd.Attacker, parsed)
		profBonus = int(char.ProficiencyBonus)
		hasTavernBrawler = HasFeat(char.Features, "tavern-brawler")
		charPtr = &char
	}

	// Phase 33 / C-33: cover gate runs BEFORE UseAttack so total cover does
	// not consume an attack resource on a rejected improvised throw.
	coverLevel, err := s.resolveAttackCover(ctx, cmd.Attacker, cmd.Target, cmd.Walls)
	if err != nil {
		return AttackResult{}, err
	}

	updatedTurn, err := UseAttack(cmd.Turn)
	if err != nil {
		return AttackResult{}, fmt.Errorf("using attack resource: %w", err)
	}

	distFt := combatantDistance(cmd.Attacker, cmd.Target)
	// C-35 — consume any DM-set per-attack advantage override; mirrors
	// Service.Attack so improvised swings honor the dashboard override.
	dmAdv, dmDisadv := s.consumeDMAdvOverride(ctx, cmd.Attacker, cmd.DMAdvantage, cmd.DMDisadvantage)
	input := buildAttackInput(
		cmd.Attacker, cmd.Target, weapon, scores, profBonus, distFt,
		cmd.HostileNearAttacker, cmd.AttackerSize,
		dmAdv, dmDisadv, nil,
	)
	// C-35 — auto-populate attacker context (size, hostile-within-5ft).
	s.populateAttackContext(ctx, &input, cmd.Attacker)
	input.IsImprovised = true
	input.ImprovisedThrown = cmd.ImprovisedThrown
	input.HasTavernBrawler = hasTavernBrawler
	input.Cover = coverLevel

	// Obscurement from encounter zones
	attackerObs, targetObs, err := s.resolveObscurement(ctx, cmd.Attacker.EncounterID, cmd.Attacker, cmd.Target, cmd.AttackerVision, cmd.TargetVision)
	if err != nil {
		return AttackResult{}, err
	}
	input.AttackerObscurement = attackerObs
	input.TargetObscurement = targetObs

	// Wire FES so Rage damage / Pack Tactics still apply to improvised hits.
	if err := s.populateAttackFES(ctx, &input, cmd, charPtr, weapon, scores); err != nil {
		return AttackResult{}, err
	}

	result, err := s.resolveAndPersistAttack(ctx, input, updatedTurn, cmd.Attacker, roller)
	if err != nil {
		return result, err
	}
	// SR-010 — mirror Service.Attack: improvised hits that produce a
	// once-per-turn FES effect must close that slot too.
	s.markUsedEffects(cmd.Attacker.EncounterID, cmd.Attacker.ID, result.OncePerTurnEffectsFired)
	s.markRageAttacked(ctx, cmd.Attacker)
	s.populatePostHitPrompts(ctx, &result, cmd.Attacker, charPtr)
	// SR-018 — improvised attacks also consume help_advantage targeting cmd.Target.
	s.consumeHelpAdvantage(ctx, cmd.Attacker, cmd.Target)
	return result, nil
}

// OffhandAttack is the service-level method for a two-weapon fighting off-hand attack.
// It uses the bonus action, validates both weapons are light, and resolves the off-hand attack
// with 0 damage modifier (unless the character has the Two-Weapon Fighting fighting style).
func (s *Service) OffhandAttack(ctx context.Context, cmd OffhandAttackCommand, roller *dice.Roller) (AttackResult, error) {
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return AttackResult{}, err
	}

	// C-40: a charmed combatant cannot attack the source of its charm.
	if err := validateCharmedAttack(cmd.Attacker, cmd.Target); err != nil {
		return AttackResult{}, err
	}

	if !cmd.Attacker.CharacterID.Valid {
		return AttackResult{}, fmt.Errorf("off-hand attack requires a character (not NPC)")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Attacker.CharacterID.UUID)
	if err != nil {
		return AttackResult{}, fmt.Errorf("getting character: %w", err)
	}

	// C-C03: off-hand attack requires the Attack action to have been taken this turn.
	maxAttacks := int32(s.resolveAttacksPerAction(ctx, char))
	if cmd.Turn.AttacksRemaining >= maxAttacks {
		return AttackResult{}, fmt.Errorf("off-hand attack requires an attack must be made this turn first")
	}

	parsed, err := ParseAbilityScores(char.AbilityScores)
	if err != nil {
		return AttackResult{}, fmt.Errorf("parsing ability scores: %w", err)
	}
	// SR-022: off-hand swings made while Wild Shaped use the beast's
	// merged STR/DEX/CON. Failed beast lookup degrades to druid scores.
	scores := ResolveAttackerScores(ctx, s.store, cmd.Attacker, parsed)

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
	if IsRangedWeapon(mainWeapon) {
		return AttackResult{}, fmt.Errorf("main hand weapon %q is ranged; off-hand attack requires melee weapons", mainWeapon.Name)
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
	if IsRangedWeapon(offWeapon) {
		return AttackResult{}, fmt.Errorf("off-hand weapon %q is ranged; off-hand attack requires melee weapons", offWeapon.Name)
	}

	// Off-hand attacks use 0 damage modifier unless the character has TWF fighting style
	dmgMod := 0
	if HasFightingStyle(char.Features, "two_weapon_fighting") {
		dmgMod = DamageModifier(scores, offWeapon)
	}

	// Phase 33 / C-33: cover gate runs BEFORE consuming the bonus action so
	// a total-cover off-hand swing doesn't burn the resource.
	coverLevel, err := s.resolveAttackCover(ctx, cmd.Attacker, cmd.Target, cmd.Walls)
	if err != nil {
		return AttackResult{}, err
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return AttackResult{}, fmt.Errorf("using bonus action: %w", err)
	}

	distFt := combatantDistance(cmd.Attacker, cmd.Target)
	// C-35 — consume any DM-set per-attack advantage override on the
	// off-hand swing too. Off-hand attacks rolled independently from the
	// main hand so the override applies to whichever swing fires first.
	dmAdv, dmDisadv := s.consumeDMAdvOverride(ctx, cmd.Attacker, cmd.DMAdvantage, cmd.DMDisadvantage)
	input := buildAttackInput(
		cmd.Attacker, cmd.Target, offWeapon, scores, int(char.ProficiencyBonus), distFt,
		cmd.HostileNearAttacker, cmd.AttackerSize,
		dmAdv, dmDisadv, &dmgMod,
	)
	// C-35 — auto-populate attacker context (size, hostile-within-5ft).
	s.populateAttackContext(ctx, &input, cmd.Attacker)
	input.Cover = coverLevel

	// Obscurement from encounter zones
	attackerObs, targetObs, err := s.resolveObscurement(ctx, cmd.Attacker.EncounterID, cmd.Attacker, cmd.Target, cmd.AttackerVision, cmd.TargetVision)
	if err != nil {
		return AttackResult{}, err
	}
	input.AttackerObscurement = attackerObs
	input.TargetObscurement = targetObs

	result, err := s.resolveAndPersistAttack(ctx, input, updatedTurn, cmd.Attacker, roller)
	if err != nil {
		return result, err
	}
	s.markRageAttacked(ctx, cmd.Attacker)
	s.populatePostHitPrompts(ctx, &result, cmd.Attacker, &char)
	// SR-018 — off-hand swings count as an attack vs the target and so spend
	// any help_advantage scoped to that target.
	s.consumeHelpAdvantage(ctx, cmd.Attacker, cmd.Target)
	return result, nil
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

	parsed, err := ParseAbilityScores(char.AbilityScores)
	if err != nil {
		return refdata.Weapon{}, AbilityScores{}, 0, nil, fmt.Errorf("parsing ability scores: %w", err)
	}
	// SR-022: Wild Shaped druid swings with beast physicals (STR/DEX/CON)
	// while retaining mental scores. ResolveAttackerScores is a no-op when
	// the attacker is not Wild Shaped and silently degrades to druid
	// scores on any beast-lookup failure (SR-006 pattern).
	scores := ResolveAttackerScores(ctx, s.store, cmd.Attacker, parsed)

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

// validateCharmedAttack rejects an attack when the attacker is charmed by the
// target. (C-40) The charmed condition's source_combatant_id identifies the
// charmer; targeting the charmer is forbidden until the condition ends.
func validateCharmedAttack(attacker, target refdata.Combatant) error {
	conds, _ := parseConditions(attacker.Conditions)
	if IsCharmedBy(conds, target.ID.String()) {
		return fmt.Errorf("%s is charmed by %s and cannot attack them", attacker.DisplayName, target.DisplayName)
	}
	return nil
}

// resolveAttackerSize returns the size category label ("Tiny" / "Small" /
// "Medium" / "Large" / ...) used by the heavy-weapon disadvantage path in
// DetectAdvantage (C-35). NPC combatants look up their creature row; PCs
// default to Medium (matching grapple_shove.resolveCombatantSize). An empty
// string is returned when no size is resolvable so the downstream check
// short-circuits.
func (s *Service) resolveAttackerSize(ctx context.Context, attacker refdata.Combatant) string {
	if attacker.CreatureRefID.Valid && attacker.CreatureRefID.String != "" {
		creature, err := s.store.GetCreature(ctx, attacker.CreatureRefID.String)
		if err != nil {
			return ""
		}
		return creature.Size
	}
	// PCs default to Medium. A future race-aware lookup can refine this.
	return "Medium"
}

// detectHostileNear inspects the encounter's combatant list and returns
// true when any *living*, *non-incapacitated* hostile (opposite IsNpc
// faction) is within 5ft (grid Chebyshev distance) of the attacker.
// Triggers the ranged-with-hostile-adjacent disadvantage in
// DetectAdvantage (C-35). Returns false when the attacker has no
// encounter ID or the store lookup fails — we never block the attack on
// a metadata hiccup.
func (s *Service) detectHostileNear(ctx context.Context, attacker refdata.Combatant) bool {
	if attacker.EncounterID == uuid.Nil {
		return false
	}
	all, err := s.store.ListCombatantsByEncounterID(ctx, attacker.EncounterID)
	if err != nil {
		return false
	}
	attackerCol := colToIndex(attacker.PositionCol)
	attackerRow := int(attacker.PositionRow) - 1
	for _, c := range all {
		if c.ID == attacker.ID {
			continue
		}
		if !c.IsAlive {
			continue
		}
		// Faction split mirrors AllHostilesDefeated: PCs and NPCs are
		// mutually hostile. Same-faction combatants (PC ally, NPC ally)
		// do not threaten the ranged shot.
		if c.IsNpc == attacker.IsNpc {
			continue
		}
		// Incapacitated foes (unconscious, paralyzed, stunned, ...) do
		// not threaten the shot per RAW.
		conds, _ := parseConditions(c.Conditions)
		if IsIncapacitated(conds) {
			continue
		}
		cCol := colToIndex(c.PositionCol)
		cRow := int(c.PositionRow) - 1
		if gridDistance(attackerCol, attackerRow, cCol, cRow) <= 1 {
			return true
		}
	}
	return false
}

// populateAttackContext fills the auto-derived advantage context fields on
// an AttackInput: attacker size (heavy-weapon disadvantage) and
// hostile-within-5ft (ranged disadvantage). Discord callers can still
// override the values explicitly by setting them on the AttackCommand —
// non-empty / true values from the command win over the auto-detected
// ones. (C-35)
func (s *Service) populateAttackContext(ctx context.Context, input *AttackInput, attacker refdata.Combatant) {
	if input.AttackerSize == "" {
		input.AttackerSize = s.resolveAttackerSize(ctx, attacker)
	}
	if !input.HostileNearAttacker {
		input.HostileNearAttacker = s.detectHostileNear(ctx, attacker)
	}
}

// applyRecklessMarker writes a transient `reckless` condition onto the
// attacker so DetectAdvantage's target-side branch grants advantage to
// incoming attacks until the marker expires at the start of the attacker's
// next turn (C-38). The marker is idempotent: if a reckless condition with
// the same source already exists, this is a no-op. Errors are silently
// swallowed so a Discord/DB hiccup never rolls back a committed attack.
func (s *Service) applyRecklessMarker(ctx context.Context, attacker refdata.Combatant) {
	if HasCondition(attacker.Conditions, "reckless") {
		return
	}
	marker := CombatCondition{
		Condition:         "reckless",
		DurationRounds:    1,
		SourceCombatantID: attacker.ID.String(),
		ExpiresOn:         "start_of_turn",
	}
	if _, _, err := s.ApplyCondition(ctx, attacker.ID, marker); err != nil {
		log.Printf("applying reckless marker for %s: %v", attacker.ID, err)
	}
}

// consumeHelpAdvantage clears a help_advantage condition from the attacker
// when its TargetCombatantID matches the combatant just attacked. SR-018:
// Help grants advantage on the helped creature's next attack vs the named
// target; once that attack has been resolved (hit or miss) the condition is
// spent and must not carry over to subsequent attacks (same target or not).
// Conditions scoped to a different target are left in place so a fresh
// /attack vs the named target still triggers DetectAdvantage's help branch.
func (s *Service) consumeHelpAdvantage(ctx context.Context, attacker refdata.Combatant, target refdata.Combatant) {
	conds, err := parseConditions(attacker.Conditions)
	if err != nil {
		return
	}
	targetID := target.ID.String()
	filtered := make([]CombatCondition, 0, len(conds))
	consumed := false
	for _, c := range conds {
		if c.Condition == "help_advantage" && c.TargetCombatantID == targetID {
			consumed = true
			continue
		}
		filtered = append(filtered, c)
	}
	if !consumed {
		return
	}
	newConds, err := json.Marshal(filtered)
	if err != nil {
		log.Printf("marshaling conditions after help_advantage consume for %s: %v", attacker.ID, err)
		return
	}
	if _, err := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
		ID:              attacker.ID,
		Conditions:      newConds,
		ExhaustionLevel: attacker.ExhaustionLevel,
	}); err != nil {
		log.Printf("clearing help_advantage on %s: %v", attacker.ID, err)
	}
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
	autoCrit, autoCritReason := CheckAutoCrit(target.Conditions, distFt, weapon)
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
		TargetCombatantID:   target.ID.String(),
	}
}

// resolveObscurement looks up encounter zones and computes effective obscurement
// for both attacker and target based on their positions and vision capabilities.
func (s *Service) resolveObscurement(ctx context.Context, encounterID uuid.UUID, attacker, target refdata.Combatant, attackerVision, targetVision VisionCapabilities) (ObscurementLevel, ObscurementLevel, error) {
	zones, err := s.ListZonesForEncounter(ctx, encounterID)
	if err != nil {
		return NotObscured, NotObscured, fmt.Errorf("listing zones for obscurement: %w", err)
	}
	if len(zones) == 0 {
		return NotObscured, NotObscured, nil
	}

	attackerCol := colToIndex(attacker.PositionCol)
	attackerRow := int(attacker.PositionRow) - 1
	targetCol := colToIndex(target.PositionCol)
	targetRow := int(target.PositionRow) - 1

	attackerObs := CombatantObscurement(attackerCol, attackerRow, zones, attackerVision)
	targetObs := CombatantObscurement(targetCol, targetRow, zones, targetVision)

	return attackerObs, targetObs, nil
}

// ErrTargetFullyCovered is returned by Service.Attack / OffhandAttack when
// total cover (CoverFull) blocks the attacker→target sightline. Per 5e rules
// a creature behind total cover can't be targeted directly by an attack
// (PHB p.196). Phase 33 / C-33 wires this into the attack pipeline.
var ErrTargetFullyCovered = fmt.Errorf("target has total cover and cannot be attacked")

// resolveAttackCover computes attacker→target cover by combining wall geometry
// and creature-granted cover from intervening occupants. Walls may be empty;
// when both walls and occupants give no cover the result is CoverNone.
// Returns ErrTargetFullyCovered on total cover so the caller can short-circuit
// without consuming attack resources.
//
// Mirrors the AoE-save pattern in CalculateAoECover (aoe.go) but uses
// attacker-corner best-of-4 (CalculateCover) rather than single-origin.
func (s *Service) resolveAttackCover(ctx context.Context, attacker, target refdata.Combatant, walls []renderer.WallSegment) (CoverLevel, error) {
	attackerCol := colToIndex(attacker.PositionCol)
	attackerRow := int(attacker.PositionRow) - 1
	targetCol := colToIndex(target.PositionCol)
	targetRow := int(target.PositionRow) - 1

	occupants, err := s.creatureCoverOccupants(ctx, attacker, target)
	if err != nil {
		return CoverNone, err
	}

	cover := CalculateCover(attackerCol, attackerRow, targetCol, targetRow, walls, occupants)
	if cover == CoverFull {
		return CoverFull, ErrTargetFullyCovered
	}
	return cover, nil
}

// creatureCoverOccupants lists living combatants in the attacker's encounter
// that may grant creature cover, excluding the attacker and target themselves.
// Dead / unconscious / invisible-to-grid combatants are filtered out so they
// don't artificially block sightlines. A combatant with no EncounterID (rare
// in tests) yields an empty slice rather than erroring.
func (s *Service) creatureCoverOccupants(ctx context.Context, attacker, target refdata.Combatant) ([]CoverOccupant, error) {
	if attacker.EncounterID == uuid.Nil {
		return nil, nil
	}
	all, err := s.store.ListCombatantsByEncounterID(ctx, attacker.EncounterID)
	if err != nil {
		return nil, fmt.Errorf("listing combatants for cover: %w", err)
	}
	occupants := make([]CoverOccupant, 0, len(all))
	for _, c := range all {
		if c.ID == attacker.ID || c.ID == target.ID {
			continue
		}
		if !c.IsAlive {
			continue
		}
		occupants = append(occupants, CoverOccupant{
			Col: colToIndex(c.PositionCol),
			Row: int(c.PositionRow) - 1,
		})
	}
	return occupants, nil
}

// colToIndex converts column letters (A, B, ..., Z, AA, AB, ...) to a 0-based index.
func colToIndex(col string) int {
	if len(col) == 0 {
		return 0
	}
	col = strings.ToUpper(col)
	result := 0
	for _, ch := range col {
		result = result*26 + int(ch-'A') + 1
	}
	return result - 1
}

// attackAbilityUsed mirrors abilityModForWeapon's selection logic and reports
// the human-readable ability label ("str" or "dex") that was used for the
// attack roll. Used to populate EffectContext.AbilityUsed so FES filters
// like Rage's `ability_used: str` actually evaluate correctly.
func attackAbilityUsed(scores AbilityScores, weapon refdata.Weapon, monkLevel int) string {
	strMod := AbilityModifier(scores.Str)
	dexMod := AbilityModifier(scores.Dex)

	if HasProperty(weapon, "finesse") {
		if dexMod > strMod {
			return "dex"
		}
		return "str"
	}
	if monkLevel > 0 && IsMonkWeapon(weapon) {
		if dexMod > strMod {
			return "dex"
		}
		return "str"
	}
	if IsRangedWeapon(weapon) {
		return "dex"
	}
	return "str"
}

// noAllyWithinFt is the sentinel returned by nearestAllyDistanceFt when no
// living non-attacker ally was found in the encounter. Chosen far above any
// reachable feature filter (Pack Tactics / Sneak Attack: 5ft).
const noAllyWithinFt = 1_000_000

// nearestAllyDistanceFt returns the chebyshev grid distance (in feet) from
// the nearest living ally of `attacker` to the `target`. Allies are
// combatants in the same encounter on the same side (PC vs NPC) other than
// the attacker themselves. Dead combatants are excluded. Returns
// noAllyWithinFt when no eligible ally exists.
//
// Distance uses Chebyshev (max(|dr|,|dc|)) × 5ft to match /move pathfinding
// (internal/pathfinding/pathfinding.go heuristic) and the OA reach check —
// diagonals count as 5ft, not 5*sqrt(2)ft. Z-axis is ignored: the FES filters
// that consume this value ("ally within Xft") are 2D adjacency checks.
func nearestAllyDistanceFt(attacker, target refdata.Combatant, all []refdata.Combatant) int {
	best := noAllyWithinFt
	tCol := colToIndex(target.PositionCol)
	tRow := int(target.PositionRow) - 1
	for _, c := range all {
		if c.ID == attacker.ID || c.ID == target.ID {
			continue
		}
		if !c.IsAlive {
			continue
		}
		if c.IsNpc != attacker.IsNpc {
			continue
		}
		dc := colToIndex(c.PositionCol) - tCol
		dr := (int(c.PositionRow) - 1) - tRow
		if dc < 0 {
			dc = -dc
		}
		if dr < 0 {
			dr = -dr
		}
		d := dc
		if dr > dc {
			d = dr
		}
		d *= 5
		if d < best {
			best = d
		}
	}
	return best
}

// populateAttackFES enriches an AttackInput with the Feature Effect System
// fields the chunk-4 review called out (Features list + EffectContext flags).
// `char` may be nil for NPC attackers — in that case only combatant-derived
// fields (IsRaging, ally distance) are populated and Features stays empty.
func (s *Service) populateAttackFES(ctx context.Context, input *AttackInput, cmd AttackCommand, char *refdata.Character, weapon refdata.Weapon, scores AbilityScores) error {
	input.IsRaging = cmd.Attacker.IsRaging
	input.AbilityUsed = attackAbilityUsed(scores, weapon, input.MonkLevel)
	input.OneHandedMeleeOnly = !IsRangedWeapon(weapon) &&
		!HasProperty(weapon, "two-handed") &&
		!input.TwoHanded &&
		!input.OffHandOccupied

	// SR-010: thread the per-attacker once-per-turn used-effects map so
	// Sneak Attack's OncePerTurn filter actually trips in production. The
	// tracker is keyed on the attacker's combatant ID (not the active
	// turn's CombatantID) so reaction attacks during another creature's
	// turn correctly read the rogue's slate.
	input.UsedThisTurn = s.usedEffectsSnapshot(cmd.Attacker.EncounterID, cmd.Attacker.ID)

	allCombatants, err := s.store.ListCombatantsByEncounterID(ctx, cmd.Attacker.EncounterID)
	if err != nil {
		return fmt.Errorf("listing combatants for ally distance: %w", err)
	}
	input.AllyWithinFt = nearestAllyDistanceFt(cmd.Attacker, cmd.Target, allCombatants)

	if char == nil {
		return nil
	}

	input.WearingArmor = char.EquippedArmor.Valid && char.EquippedArmor.String != ""

	var classes []CharacterClass
	if len(char.Classes) > 0 {
		_ = json.Unmarshal(char.Classes, &classes)
	}
	var feats []CharacterFeature
	if char.Features.Valid && len(char.Features.RawMessage) > 0 {
		_ = json.Unmarshal(char.Features.RawMessage, &feats)
	}
	// SR-006 / Phase 88a: feed equipped + attuned magic items into the FES
	// pipeline so +1 weapons, Cloak of Protection, etc. actually modify the
	// attack roll / damage. collectMagicItemFeatures degrades to nil on bad
	// inventory JSON rather than failing the whole attack.
	input.Features = BuildFeatureDefinitions(classes, feats, collectMagicItemFeatures(*char))

	// SR-058: Sacred Weapon condition → inject CHA mod as modify_attack_roll.
	if HasCondition(cmd.Attacker.Conditions, "sacred_weapon") {
		chaMod := max(AbilityModifier(scores.Cha), 1)
		input.Features = append(input.Features, SacredWeaponFeature(chaMod))
	}

	return nil
}
