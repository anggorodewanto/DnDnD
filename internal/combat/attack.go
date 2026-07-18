package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
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

// AttackModifier returns the total attack modifier: ability mod + proficiency bonus (if proficient).
func AttackModifier(scores AbilityScores, weapon refdata.Weapon, profBonus int, proficient bool, monkLevel ...int) int {
	mod := abilityModForWeapon(scores, weapon, monkLevel...)
	if proficient {
		mod += profBonus
	}
	return mod
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

// NoAmmunitionError signals that an ammunition-requiring attack found no
// usable ammo in the attacker's inventory. The /attack handler detects it
// (errors.As) to route the shot to the DM queue for lenient adjudication
// instead of hard-failing — DMs commonly hand-wave precise ammo counts.
type NoAmmunitionError struct {
	// AmmoName is the conventional ammunition type ("Bolts" / "Arrows").
	AmmoName string
}

func (e NoAmmunitionError) Error() string {
	return fmt.Sprintf("No %s remaining.", strings.ToLower(e.AmmoName))
}

// ammoMatches reports whether an inventory item is ammunition usable for the
// weapon-derived ammoName ("Bolts" / "Arrows"). Matching is deliberately
// generous so it tolerates every way ammo is actually stored:
//   - character-builder seeding: {item_id:"crossbow-bolt", name:"crossbow-bolt", type:"gear"}
//   - hand-curated / imported:   {name:"Bolts"} or {name:"Crossbow Bolts", type:"ammunition"}
//
// It matches any non-weapon, non-armor, non-consumable item whose name or
// item_id contains the singular ammo keyword ("bolt"/"arrow") as a whole
// word, so a "Lightning Bolt Scroll" (a consumable) is never mistaken for
// crossbow ammo.
func ammoMatches(item character.InventoryItem, ammoName, ammoID string) bool {
	switch strings.ToLower(item.Type) {
	case "weapon", "armor", "consumable":
		return false
	}
	// Prefer exact item-id equality against the weapon's ammunition_id FK
	// (ISSUE-017 phase 2): when the weapon links to its ammo item, a precise id
	// match avoids any name-keyword ambiguity. The keyword scan below stays as a
	// legacy fallback for weapon/inventory rows that predate the FK.
	if ammoID != "" && strings.EqualFold(item.ItemID, ammoID) {
		return true
	}
	keyword := strings.TrimSuffix(strings.ToLower(ammoName), "s") // "Bolts" -> "bolt"
	for _, tok := range ammoWords(item.Name + " " + item.ItemID) {
		if tok == keyword || tok == keyword+"s" {
			return true
		}
	}
	return false
}

// ammoWords splits a string into lowercase alphabetic tokens, treating any
// non-letter (space, hyphen, digit) as a separator: "crossbow-bolt 20" ->
// ["crossbow", "bolt"].
func ammoWords(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r < 'a' || r > 'z'
	})
}

// DeductAmmunition decrements the matching ammunition stack by 1. Returns a
// NoAmmunitionError when no matching ammo is found or the stack is empty, so
// the caller can offer DM adjudication rather than a dead-end rejection.
func DeductAmmunition(items []character.InventoryItem, ammoName, ammoID string) ([]character.InventoryItem, error) {
	for i := range items {
		if !ammoMatches(items[i], ammoName, ammoID) {
			continue
		}
		if items[i].Quantity <= 0 {
			return items, NoAmmunitionError{AmmoName: ammoName}
		}
		items[i].Quantity--
		return items, nil
	}
	return items, NoAmmunitionError{AmmoName: ammoName}
}

// RecoverAmmunition adds back half (rounded down) of spent ammunition to the
// matching stack after combat (Phase 37).
func RecoverAmmunition(items []character.InventoryItem, ammoName, ammoID string, spent int) []character.InventoryItem {
	recovered := spent / 2
	if recovered <= 0 {
		return items
	}
	for i := range items {
		if ammoMatches(items[i], ammoName, ammoID) {
			items[i].Quantity += recovered
			return items
		}
	}
	return items
}

// ammunitionItemID returns the canonical ammo item id a weapon consumes, from
// its ammunition_id FK (ISSUE-017 phase 2), or "" for a legacy/homebrew weapon
// row that predates the link.
func ammunitionItemID(weapon refdata.Weapon) string {
	if weapon.AmmunitionID.Valid {
		return weapon.AmmunitionID.String
	}
	return ""
}

// GetAmmunitionName returns the conventional short ammunition name for a weapon
// ("Bolts", "Arrows", "Bullets", "Needles"), used for player-facing messages
// and the ammo-spent tracker key. It reads the weapon's ammunition_id FK and
// derives the name from the canonical item catalog (the last word of the
// display name keeps it keyword-matcher friendly), falling back to the legacy
// "crossbow" substring heuristic when the FK is absent.
func GetAmmunitionName(weapon refdata.Weapon) string {
	if id := ammunitionItemID(weapon); id != "" {
		if e, ok := refdata.ItemCatalogByID()[id]; ok {
			if fields := strings.Fields(e.Name); len(fields) > 0 {
				return fields[len(fields)-1]
			}
		}
	}
	if strings.Contains(strings.ToLower(weapon.Name), "crossbow") {
		return "Bolts"
	}
	return "Arrows"
}

// deductWeaponAmmunition spends one unit of the weapon's ammunition from the
// character's inventory, persists the change, and records the spend in the
// post-combat recovery tracker (C-37) so consumption and half-recovery can
// never drift. It parses/marshals through the full character.InventoryItem so
// the write round-trips every other item's fields (equipped flags, magic props,
// charges, item_id) losslessly. It is a no-op for a non-ammunition weapon or a
// nil character. Returns a NoAmmunitionError when the matching ammo stack is
// empty or absent, so the caller can offer DM adjudication rather than a
// dead-end rejection. Shared by the main Attack path and the Crossbow Expert
// bonus attack (COV-9); attacker supplies the encounter/combatant keys for the
// recovery tracker.
func (s *Service) deductWeaponAmmunition(ctx context.Context, char *refdata.Character, weapon refdata.Weapon, attacker refdata.Combatant) error {
	if char == nil || !HasProperty(weapon, "ammunition") {
		return nil
	}
	ammoName := GetAmmunitionName(weapon)
	ammoID := ammunitionItemID(weapon)
	items, err := character.ParseInventoryItems(char.Inventory.RawMessage, char.Inventory.Valid)
	if err != nil {
		return fmt.Errorf("parsing inventory: %w", err)
	}
	items, err = DeductAmmunition(items, ammoName, ammoID)
	if err != nil {
		return err
	}
	invJSON, err := character.MarshalInventory(items)
	if err != nil {
		return fmt.Errorf("marshaling inventory: %w", err)
	}
	if err := s.store.UpdateCharacterInventory(ctx, char.ID, pqtype.NullRawMessage{RawMessage: invJSON, Valid: true}); err != nil {
		return fmt.Errorf("updating inventory: %w", err)
	}
	// C-37 — track the spend so end-of-combat half-recovery returns some ammo.
	s.recordAmmoForAttack(attacker.EncounterID, attacker.ID, weapon)
	return nil
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
	OverrideDmgMod      *int             // If set, overrides the normal ability modifier for damage
	TwoHanded           bool             // Use versatile damage die (requires free off-hand)
	OffHandOccupied     bool             // True if off-hand is occupied (blocks two-handed)
	Thrown              bool             // Melee weapon thrown at range
	IsImprovised        bool             // Improvised weapon (no proficiency unless Tavern Brawler)
	ImprovisedThrown    bool             // Improvised weapon thrown (range 20/60)
	HasCrossbowExpert   bool             // Character has Crossbow Expert feat
	HasTavernBrawler    bool             // Character has Tavern Brawler feat
	GWM                 bool             // Great Weapon Master: -5 hit, +10 damage (heavy melee)
	Sharpshooter        bool             // Sharpshooter: -5 hit, +10 damage (ranged) — per-attack toggle
	HasSharpshooter     bool             // Character HAS the Sharpshooter feat (passive: ignore cover + no long-range disadvantage)
	Reckless            bool             // Reckless Attack: advantage on melee STR attacks (Barbarian)
	MonkLevel           int              // Monk level (0 = not a monk)
	AttackerHidden      bool             // Attacker is hidden (not visible)
	TargetHidden        bool             // Target is hidden (not visible)
	AttackerObscurement ObscurementLevel // Effective obscurement for attacker
	TargetObscurement   ObscurementLevel // Effective obscurement for target
	// TargetCombatantID is the ID of the combatant currently being attacked.
	// SR-018: piped into DetectAdvantage so target-scoped attacker conditions
	// (help_advantage) only fire when the named target is the one under attack.
	TargetCombatantID  string
	Features           []FeatureDefinition // Feature Effect System definitions (magic items, etc.)
	IsRaging           bool                // Attacker is currently raging (Phase 46)
	WearingArmor       bool                // Attacker is wearing armor (Defense fighting style)
	OneHandedMeleeOnly bool                // Wielding a one-handed melee weapon with no off-hand weapon (Dueling)
	AllyWithinFt       int                 // Distance to nearest ally relative to target (Pack Tactics, Sneak Attack)
	AbilityUsed        string              // "str", "dex", or "cha" — which ability mod was chosen for this attack
	PactBladeCHA       bool                // COV-7: warlock Pact of the Blade — use CHA (if higher) for a pact weapon's attack + damage
	SavageAttacker     bool                // COV-9: attacker has the Savage Attacker feat — reroll a melee weapon's damage dice once per turn, keep the higher total
	UsedThisTurn       map[string]bool     // Per-turn feature usage tracking (Sneak Attack OncePerTurn)
	WeaponMasteries    []string            // weapon ids whose mastery the attacker knows (2024 Weapon Mastery)
	// ReactionACBonus / ReactionReason carry a pre-roll reaction's AC boost
	// (e.g. the target's Defensive Duelist +PB) into the hit test.
	ReactionACBonus int
	ReactionReason  string
	// ExhaustionLevel is the attacker's current exhaustion. 2024 rules apply a
	// flat -2/level penalty to the attack roll (folded into the to-hit modifier).
	ExhaustionLevel int
	// CunningStrike, when set to a known effect ("trip"/"poison"/…), signals a
	// Rogue-5 Cunning Strike whose Sneak Attack die cost has already been forgone
	// from Features. It is set by populateAttackFES ONLY when the attacker carries
	// the feature, so ResolveAttack can treat a non-empty value as authoritative
	// eligibility.
	CunningStrike string
	// BrutalStrike, when set to a known effect ("forceful"/…), signals an eligible
	// Barbarian-9 Brutal Strike (feature present + using Reckless + STR melee,
	// decided in populateAttackFES, which also injects the +1d10 rider).
	// ResolveAttack treats a non-empty value as authoritative: it forgoes Advantage
	// (AdvantageInput.ForgoAdvantage) and records the effect on the result for
	// Service.Attack to resolve post-hit (applyBrutalStrike).
	BrutalStrike string
	// BonusDice, when non-empty, is a player-declared effect-die expression
	// (e.g. "1d4" Bless, "1d8" Bardic Inspiration) added to the to-hit total.
	// Nat 20 / nat 1 semantics key off the raw die and are unaffected; the
	// D20Roll breakdown stays bonus-free.
	BonusDice string
}

// AttackResult holds the full result of an attack resolution.
type AttackResult struct {
	AttackerName        string
	TargetName          string
	WeaponName          string
	DistanceFt          int
	IsMelee             bool
	IsWeaponAttack      bool // True for weapon attacks (not spell attacks)
	Hit                 bool
	CriticalHit         bool
	AutoCrit            bool
	AutoCritReason      string
	D20Roll             dice.D20Result
	// Effect-dice bonus added to the to-hit roll (Bless / Bardic Inspiration).
	// Zero-valued when none supplied. BonusTotal is folded into the hit
	// comparison; D20Roll stays bonus-free. BonusRolls carries per-die results.
	BonusExpression     string
	BonusTotal          int
	BonusRolls          []dice.GroupResult
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
	// ReactionACBonus / ReactionReason echo a pre-roll reaction that raised the
	// target's AC for this attack (e.g. Defensive Duelist), so the log can note it.
	ReactionACBonus    int
	ReactionReason     string
	AttackerRevealed   bool // True if a hidden attacker was revealed by this attack
	InvisibilityBroken bool // True if standard Invisibility condition was broken by this attack

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

	// TODO 3 — 2024 Great Weapon Master bonus attack.
	//   - WeaponIsHeavy is set at roll time (ResolveAttack) from the weapon's
	//     Heavy property; combined with IsMelee it identifies a heavy-melee swing.
	//   - TargetDroppedToZero is set by Service.Attack after damage resolves
	//     (from ApplyDamageResult.DroppedToZero) — the above-0 → 0 HP transition.
	//   - PromptGWMBonusAttackEligible is set in populatePostHitPrompts when the
	//     attacker holds the GWM feat and either crit or dropped the target to 0
	//     with a heavy melee weapon. The Discord layer reads it to offer a
	//     bonus-action swing with the same weapon (Service.GWMBonusAttack).
	WeaponIsHeavy                bool
	TargetDroppedToZero          bool
	PromptGWMBonusAttackEligible bool

	// DownLogLine is the formatted drop-to-0 line (💀/💔) when this attack
	// dropped the target above-0 → 0. FormatAttackLog renders it at the tail of
	// the narrative so #combat-log shows "defeated" AFTER the hit, not before.
	// Stamped by applyHitDamage (which defers the service's own immediate post);
	// empty when the target survived. See DownLogLine on ApplyDamageResult.
	DownLogLine string

	// SR-010: list of EffectType strings whose conditions included
	// OncePerTurn:true and which actually fired (passed condition filtering)
	// in the on_damage_roll pass. Service.Attack reads this to mark the
	// effect types used so subsequent attacks by the same combatant — same
	// turn or a reaction on another creature's turn — skip them until the
	// combatant's own turn starts again.
	OncePerTurnEffectsFired []string

	// OncePerTurnEffectNames carries the source feature names (e.g. "Sneak
	// Attack") of the once-per-turn effects that fired, paralleling
	// OncePerTurnEffectsFired's type strings. Display-only: FormatAttackLog
	// reads it to surface a player-visible tag without decoding the folded
	// dice. The engine's trigger logic never consults this field — populating
	// it does not change when any effect fires.
	OncePerTurnEffectNames []string

	// SavageAttackerUsed reports that Savage Attacker rerolled this attack's
	// weapon damage (the higher of two rolls was kept). Set only on the melee
	// weapon attack that spent the once-per-turn reroll; drives the combat-log
	// tag, and its once-per-turn key rides OncePerTurnEffectsFired so Service.*
	// marks it used.
	SavageAttackerUsed bool

	// DamageBreakdown decomposes DamageTotal into per-rider contributions
	// (Feature Effect System extra dice + flat damage mods such as GWM's +PB,
	// magic-item riders, Hex, Agonizing Blast). Display-only: DamageTotal
	// already includes every component, so the logs render these as "of which"
	// call-outs, not additional damage. Base weapon damage is the unlisted
	// remainder. Empty when no rider fired.
	DamageBreakdown []DamageComponent

	// 2024 Weapon Mastery: the mastery slug that fired on this attack
	// ("" if none). Only set when the attacker knows the weapon's mastery
	// (masteryActive). Graze fires on a miss; Topple fires on a hit.
	MasteryProperty string
	// MasteryToppleSaveDC is the CON save DC the target must beat to avoid the
	// Prone condition from the Topple mastery (8 + prof + attack ability mod).
	// Zero unless MasteryProperty == "topple".
	MasteryToppleSaveDC int

	// COV-8 Cunning Strike: CunningStrikeChoice names the effect that fired
	// ("trip"/"poison"/…), set in ResolveAttack only when a Rogue opted into a
	// known cunning effect, the attack hit, and Sneak Attack actually dealt damage
	// — a non-empty choice IS the "rider fired" gate downstream. CunningStrikeSaveDC
	// is the save DC (8 + prof + DEX, target-ability-independent). CunningStrikeSaved
	// is set by Service.Attack once the save is rolled (true = made the save, no
	// condition). Mirrors how Topple carries MasteryProperty + MasteryToppleSaveDC.
	CunningStrikeChoice string
	CunningStrikeSaveDC int
	CunningStrikeSaved  bool

	// COV-8 Brutal Strike: BrutalStrikeChoice names the effect that fired
	// ("forceful"/…), set in ResolveAttack from input.BrutalStrike (already
	// eligibility-gated). The +1d10 rides the damage breakdown; the effect (e.g.
	// Forceful Blow's push) is resolved on a hit by Service.Attack (applyBrutalStrike).
	BrutalStrikeChoice string

	// CleaveAttack carries the secondary attack resolution from the 2024
	// Cleave mastery (nil when no cleave occurred). The service layer auto-
	// resolves one extra attack with the same weapon against a second creature
	// within 5ft of the primary target and within the attacker's reach; the
	// extra attack adds no ability modifier to damage (unless it is negative).
	// Surfaced so callers can report "Cleave hits <2nd target> for N".
	CleaveAttack *AttackResult
}

// OffhandAttackCommand holds the service-level inputs for an off-hand attack (bonus action).
type OffhandAttackCommand struct {
	Attacker            refdata.Combatant
	Target              refdata.Combatant
	Turn                refdata.Turn
	Thrown              bool // Throw a light "thrown" off-hand weapon (e.g. dagger) instead of a melee swing
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
	TwoHanded           bool // Use versatile two-handed grip
	IsImprovised        bool // Improvised weapon attack
	ImprovisedThrown    bool // Improvised weapon thrown
	Thrown              bool // Throw a melee weapon with "thrown" property
	GWM                 bool // Great Weapon Master 2014 flag (-5 hit / +10 damage)
	GWM2024             bool // Great Weapon Master 2024: +proficiency bonus damage (heavy melee, 1/turn)
	Sharpshooter        bool // Sharpshooter flag
	Reckless            bool // Reckless Attack flag
	// TacticalMastery, when "push"/"sap"/"slow", is a Fighter-9 Tactical Master
	// request to replace the weapon's own mastery with that property for this
	// attack. Ignored unless the attacker carries the feature and already uses
	// the weapon's own mastery (see tacticalMasteryOverride).
	TacticalMastery string
	// CunningStrike, when set to a known effect ("trip"/"poison"/…), is a Rogue-5
	// Cunning Strike request to forgo a Sneak Attack die and add that effect's rider
	// (Trip → DEX save/Prone, Poison → CON save/Poisoned). Ignored unless the
	// attacker carries the feature (see cunning_strike.go).
	CunningStrike string
	// BrutalStrike, when set to a known effect ("forceful"/…), is a Barbarian-9
	// Brutal Strike request to forgo Advantage on this Strength melee attack for
	// +1d10 damage and a forced-movement effect. Ignored unless the attacker
	// carries the feature and is using Reckless Attack (see brutal_strike.go).
	BrutalStrike string
	// ReactionACBonus is AC the targeted PC gains against THIS attack from a
	// reaction declared in the pre-roll window (e.g. Defensive Duelist +PB).
	// ReactionReason names it for the log. Baked into effectiveAC by ResolveAttack.
	ReactionACBonus int
	ReactionReason  string
	AttackerVision  VisionCapabilities // Vision capabilities of the attacker
	TargetVision    VisionCapabilities // Vision capabilities of the target
	// Walls are encounter-map wall segments used to compute attacker→target
	// cover (Phase 33 / C-33). A nil/empty slice degrades to "no wall cover";
	// creature-granted cover still applies via the encounter's combatant list.
	Walls []renderer.WallSegment
	// BonusDice, when non-empty, is a player-declared effect-die expression
	// (e.g. "1d4" Bless, "1d8" Bardic Inspiration) added to the to-hit roll.
	// Applies to the primary swing only (cleared on Cleave/Monk secondaries).
	BonusDice string
}

// masteryActive reports whether the attacker knows this weapon's mastery
// property — the weapon has a mastery AND its id is in the attacker's known list.
func masteryActive(input AttackInput) bool {
	if input.Weapon.Mastery == "" {
		return false
	}
	return slices.Contains(input.WeaponMasteries, input.Weapon.ID)
}

// onHitMastery returns the mastery slug that fires on a HIT for this attack, or
// "" when none applies. It is the single gate shared by the regular-hit and
// auto-crit paths so they stay in lockstep. Graze is excluded here because it
// fires on a MISS, not a hit; Cleave is melee-only. Everything stays gated
// behind masteryActive so non-mastery hits are unaffected.
func onHitMastery(input AttackInput, isMelee bool) string {
	if !masteryActive(input) {
		return ""
	}
	switch input.Weapon.Mastery {
	case "topple", "vex", "sap", "slow", "push":
		return input.Weapon.Mastery
	case "cleave":
		if isMelee {
			return "cleave"
		}
	}
	return ""
}

// recordOnHitMastery records the on-hit mastery slug on the result, plus the
// Topple CON save DC when Topple fired. Shared by the auto-crit hit path and
// the regular hit path so they cannot drift apart. toppleSaveDC is only read
// when the slug is "topple". A "" slug is a no-op.
func recordOnHitMastery(result *AttackResult, slug string, toppleSaveDC int) {
	if slug == "" {
		return
	}
	result.MasteryProperty = slug
	if slug == "topple" {
		result.MasteryToppleSaveDC = toppleSaveDC
	}
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

	// Sharpshooter (COV-9): a ranged attacker with the feat ignores half and
	// three-quarters cover. Full cover still blocks the shot — that is handled
	// earlier at the service layer (resolveAttackCover → ErrTargetFullyCovered)
	// and adds no AC here, so only Half/ThreeQuarters need zeroing.
	cover := input.Cover
	if input.HasSharpshooter && !isMelee && (cover == CoverHalf || cover == CoverThreeQuarters) {
		cover = CoverNone
	}

	// Pre-roll reaction (e.g. Defensive Duelist) raises the target's AC against
	// this one attack. Declared before the roll, so it folds straight into the
	// single hit test — nothing is resolved retroactively.
	effectiveAC := EffectiveAC(input.TargetAC, cover) + input.ReactionACBonus

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

	atkMod := AttackModifier(input.Scores, input.Weapon, profBonus, true, input.MonkLevel)
	dmgMod := DamageModifier(input.Scores, input.Weapon, input.MonkLevel)
	if input.OverrideDmgMod != nil {
		dmgMod = *input.OverrideDmgMod
	}

	// COV-7 Pact of the Blade: a warlock bonded to a pact weapon may use CHA for
	// its attack and damage rolls. effectiveAbilityMod substitutes CHA when it is
	// the higher choice; shift atkMod by the delta (it also carries profBonus) and
	// replace dmgMod unless an explicit OverrideDmgMod already governs damage.
	if input.PactBladeCHA {
		base := abilityModForWeapon(input.Scores, input.Weapon, input.MonkLevel)
		eff := effectiveAbilityMod(input)
		atkMod += eff - base
		if input.OverrideDmgMod == nil {
			dmgMod = eff
		}
	}

	// GWM / Sharpshooter: -5 to hit, +10 to damage
	gwmSharpshooterBonus := 0
	if input.GWM || input.Sharpshooter {
		atkMod -= 5
		gwmSharpshooterBonus = 10
	}

	// 2024 exhaustion: -2 to hit per level, folded into the to-hit modifier just
	// like the GWM/Sharpshooter -5 above.
	atkMod += ExhaustionD20Penalty(input.ExhaustionLevel)

	result := AttackResult{
		AttackerName:    input.AttackerName,
		TargetName:      input.TargetName,
		WeaponName:      input.Weapon.Name,
		DistanceFt:      input.DistanceFt,
		IsMelee:         isMelee,
		WeaponIsHeavy:   HasProperty(input.Weapon, "heavy"),
		IsWeaponAttack:  true,
		EffectiveAC:     effectiveAC,
		DamageType:      input.Weapon.DamageType,
		Cover:           input.Cover,
		InLongRange:     resolveInLongRange(input),
		GWM:             input.GWM,
		Sharpshooter:    input.Sharpshooter,
		Reckless:        input.Reckless,
		ReactionACBonus: input.ReactionACBonus,
		ReactionReason:  input.ReactionReason,
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
		HasSharpshooter:     input.HasSharpshooter,
		// COV-8 Brutal Strike forgoes all Advantage on this roll.
		ForgoAdvantage: input.BrutalStrike != "",
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
	var fesDamageEffects []ResolvedEffect
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
		fesDamageEffects = dmgResult.AppliedEffects
		// SR-010: surface once-per-turn effect types that actually fired
		// so the service layer can mark them used for this combatant's
		// "turn window" (since their own turn last started). The damage
		// trigger is where Sneak Attack's extra_damage_dice lives.
		for _, re := range dmgResult.AppliedEffects {
			if !re.Effect.Conditions.OncePerTurn {
				continue
			}
			result.OncePerTurnEffectsFired = append(result.OncePerTurnEffectsFired, string(re.Effect.Type))
			// Display-only: keep the feature name so FormatAttackLog can tag
			// e.g. "Sneak Attack" by name rather than hard-coding the dice.
			result.OncePerTurnEffectNames = append(result.OncePerTurnEffectNames, re.FeatureName)
		}
	}

	// COV-8 Brutal Strike: carry the eligibility-gated effect onto the result for
	// all paths (auto-crit / miss / hit); Service.Attack resolves it on a hit
	// (applyBrutalStrike). The +1d10 rider already rode input.Features above.
	result.BrutalStrikeChoice = input.BrutalStrike

	// Auto-crit: skip attack roll, auto-hit and auto-crit
	if input.AutoCrit {
		result.Hit = true
		result.CriticalHit = true
		result.AutoCrit = true
		result.AutoCritReason = input.AutoCritReason
		savage := savageAttackerEligible(input, isMelee)
		dmg, damageDice, dmgRoll := rollWeaponDamageSavage(savage, input.Weapon, dmgMod, true, input.TwoHanded, roller, input.MonkLevel)
		if savage {
			result.SavageAttackerUsed = true
			result.OncePerTurnEffectsFired = append(result.OncePerTurnEffectsFired, savageAttackerUsedEffect)
		}
		extra, comps := buildFESDamageBreakdown(fesDamageEffects, true, roller)
		result.DamageTotal = dmg + gwmSharpshooterBonus + extra
		result.DamageBreakdown = comps
		result.DamageDice = damageDice
		result.DamageRoll = dmgRoll
		// 2024 Weapon Mastery — an auto-crit is a hit, so the on-hit masteries
		// (Topple / Vex / Sap / Slow / Push, plus melee-only Cleave) fire here
		// just as they do on a rolled hit. Graze is miss-only and never reached.
		recordOnHitMastery(&result, onHitMastery(input, isMelee),
			8+profBonus+effectiveAbilityMod(input))
		recordCunningStrike(&result, input, 8+profBonus+AbilityModifier(input.Scores.Dex))
		return result, nil
	}

	// Roll attack
	d20, err := roller.RollD20(atkMod, rollMode)
	if err != nil {
		return AttackResult{}, fmt.Errorf("rolling attack: %w", err)
	}
	result.D20Roll = d20

	// Player-declared effect dice (Bless / Bardic Inspiration) add to the
	// to-hit total. Nat 20 / nat 1 key off the raw die and are unaffected; the
	// D20Roll breakdown stays bonus-free — only the hit comparison folds it in.
	bonusToHit := 0
	if input.BonusDice != "" {
		bonus, berr := roller.Roll(input.BonusDice)
		if berr != nil {
			return AttackResult{}, fmt.Errorf("%w: %v", dice.ErrInvalidBonus, berr)
		}
		result.BonusExpression = input.BonusDice
		result.BonusTotal = bonus.Total
		result.BonusRolls = bonus.Groups
		bonusToHit = bonus.Total
	}

	// Nat 20 always hits and crits; nat 1 always misses
	if d20.CriticalHit {
		result.Hit = true
		result.CriticalHit = true
	} else if d20.CriticalFail {
		result.Hit = false
	} else {
		result.Hit = d20.Total+bonusToHit >= effectiveAC
	}

	if !result.Hit {
		// 2024 Weapon Mastery — Graze: on a miss, deal damage equal to the
		// attack's ability modifier (no dice), of the weapon's damage type.
		// Min 0. Gated entirely behind masteryActive so non-mastery misses are
		// unchanged.
		if masteryActive(input) && input.Weapon.Mastery == "graze" {
			result.DamageTotal = max(0, DamageModifier(input.Scores, input.Weapon, input.MonkLevel))
			result.MasteryProperty = "graze"
		}
		return result, nil
	}

	// Roll damage. Savage Attacker (COV-9): a melee weapon attacker with the feat
	// rerolls the weapon's damage dice once per turn and keeps the higher total.
	savage := savageAttackerEligible(input, isMelee)
	dmg, damageDice, dmgRoll := rollWeaponDamageSavage(savage, input.Weapon, dmgMod, result.CriticalHit, input.TwoHanded, roller, input.MonkLevel)
	if savage {
		result.SavageAttackerUsed = true
		result.OncePerTurnEffectsFired = append(result.OncePerTurnEffectsFired, savageAttackerUsedEffect)
	}
	extra, comps := buildFESDamageBreakdown(fesDamageEffects, result.CriticalHit, roller)
	result.DamageTotal = dmg + gwmSharpshooterBonus + extra
	result.DamageBreakdown = comps
	result.DamageDice = damageDice
	result.DamageRoll = dmgRoll

	// 2024 Weapon Mastery — record the on-hit mastery that fired so the service
	// layer can apply the target-side effect:
	//   - Topple: CON save (DC 8 + proficiency + attack ability mod) or Prone.
	//     profBonus already reflects the improvised adjustment.
	//   - Vex: attacker gains advantage on its next attack vs the SAME target.
	//   - Sap: target has disadvantage on its NEXT attack.
	//   - Slow: target's Speed drops 10 ft until the attacker's next turn.
	//   - Push: a Large-or-smaller target is pushed 10 ft away.
	//   - Cleave (melee-only): one extra attack vs an adjacent creature.
	// All gated behind masteryActive (via onHitMastery) so non-mastery hits are
	// unchanged. Graze is handled separately on the miss path above.
	recordOnHitMastery(&result, onHitMastery(input, isMelee),
		8+profBonus+effectiveAbilityMod(input))
	// COV-8 Cunning Strike: if the rogue opted into a known effect and Sneak Attack
	// dealt damage on this hit, record the effect + its save DC (8 + prof + DEX,
	// target-ability-independent). The save is rolled post-hit in Service.Attack
	// (applyCunningStrike).
	recordCunningStrike(&result, input, 8+profBonus+AbilityModifier(input.Scores.Dex))

	return result, nil
}

// buildFESDamageBreakdown rolls each on-damage Feature Effect System effect
// individually and attributes it to its source feature, so the #combat-log and
// DM action log can call out per-rider contributions (Sneak Attack, Hex, GWM,
// magic items, …). It returns the summed extra-dice total plus the component
// list.
//
// The returned total is identical to the previous blind sum for the same roller
// state and effect order: it consumes effects in the same order the underlying
// ExtraDice were appended (ProcessEffects appends AppliedEffects and ExtraDice
// in one pass), and only EffectExtraDamageDice consume the roller — so seeded
// rolls are unchanged.
//
// Flat EffectModifyDamageRoll modifiers are emitted as components (for the
// call-out) but are NOT added to the returned total: they were already folded
// into the base weapon roll via dmgMod and are never crit-doubled.
func buildFESDamageBreakdown(effects []ResolvedEffect, critical bool, roller *dice.Roller) (int, []DamageComponent) {
	extraDiceTotal := 0
	var components []DamageComponent
	for _, re := range effects {
		e := re.Effect
		switch e.Type {
		case EffectModifyDamageRoll:
			if e.Modifier == 0 {
				continue
			}
			components = append(components, DamageComponent{
				SourceName: re.FeatureName,
				Amount:     e.Modifier,
				DamageType: firstDamageType(e.DamageTypes),
			})
		case EffectExtraDamageDice:
			if e.Dice == "" {
				continue
			}
			r, err := roller.RollDamage(e.Dice, critical)
			if err != nil {
				continue
			}
			extraDiceTotal += r.Total
			components = append(components, DamageComponent{
				SourceName: re.FeatureName,
				Amount:     r.Total,
				DamageType: firstDamageType(e.DamageTypes),
			})
		}
	}
	return extraDiceTotal, components
}

// firstDamageType returns the first declared damage type, or "" (weapon/untyped).
func firstDamageType(types []string) string {
	if len(types) == 0 {
		return ""
	}
	return types[0]
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
		// Unarmed strikes have no dice — the flat 1 is not doubled on crit
		// per Sage Advice (no dice to double).
		total := max(1+dmgMod, 0)
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

// sneakAttackTag returns the player-visible Sneak Attack suffix when the
// attacker's own Sneak Attack fired on this attack, or "" otherwise. It is
// keyed on the fired once-per-turn effect's feature name (never the dice), so
// it generalizes and never mislabels a different once-per-turn effect. The tag
// reveals only the attacker's own feature — no enemy HP/AC.
func sneakAttackTag(result AttackResult) string {
	for _, name := range result.OncePerTurnEffectNames {
		if name == "Sneak Attack" {
			return " \u2014 \u26a1 Sneak Attack!"
		}
	}
	return ""
}

// writeDamageBreakdown appends one "of which" sub-line per damage rider (Hex,
// Agonizing Blast, Great Weapon Master, magic items, …) so players see each
// feature's contribution to the total. Sneak Attack is skipped — it is already
// surfaced by sneakAttackTag on the damage line — so it is never double-printed.
func writeDamageBreakdown(b *strings.Builder, result AttackResult) {
	for _, c := range result.DamageBreakdown {
		if c.SourceName == "Sneak Attack" {
			continue
		}
		if c.DamageType != "" {
			fmt.Fprintf(b, "\n        ↳ includes +%d %s (%s)", c.Amount, c.DamageType, c.SourceName)
		} else {
			fmt.Fprintf(b, "\n        ↳ includes +%d (%s)", c.Amount, c.SourceName)
		}
	}
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
		fmt.Fprintf(&b, "\n    \u2192 Damage: %d %s (doubled dice: %s)%s%s", result.DamageTotal, result.DamageType, result.DamageDice, sneakAttackTag(result), savageAttackerTag(result))
		writeDamageBreakdown(&b, result)
		writeDroppedToZero(&b, result)
		return b.String()
	}

	// Attack roll line. On advantage/disadvantage the D20 result holds both raw
	// dice (Rolls) plus the kept one (Chosen); surface both so the log reveals
	// every die rolled, not only the one that counted. Any effect-dice bonus
	// (Bless / Bardic Inspiration) is added to the leading to-hit total and
	// shown as a trailing fragment; the d20 breakdown stays bonus-free.
	toHitTotal := result.D20Roll.Total + result.BonusTotal
	rollStr := fmt.Sprintf("%d (%d + %d)", toHitTotal, result.D20Roll.Chosen, result.D20Roll.Modifier)
	if len(result.D20Roll.Rolls) == 2 {
		rollStr = fmt.Sprintf("%d (%d / %d \u2192 %d + %d)",
			toHitTotal, result.D20Roll.Rolls[0], result.D20Roll.Rolls[1],
			result.D20Roll.Chosen, result.D20Roll.Modifier)
	}
	if result.BonusExpression != "" {
		rollStr += fmt.Sprintf(" +%d (%s)", result.BonusTotal, result.BonusExpression)
	}
	if result.CriticalHit {
		b.WriteString("\n    \u2192 Roll to hit: \U0001f3af NAT 20 \u2014 CRITICAL HIT!")
		if len(result.D20Roll.Rolls) == 2 {
			fmt.Fprintf(&b, " (%d / %d)", result.D20Roll.Rolls[0], result.D20Roll.Rolls[1])
		}
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
		fmt.Fprintf(&b, "\n    \u2192 Damage: %d %s (%s)%s%s", result.DamageTotal, result.DamageType, diceLabel, sneakAttackTag(result), savageAttackerTag(result))
		writeDamageBreakdown(&b, result)
	}

	// 2024 Weapon Mastery \u2014 Cleave: surface the auto-resolved secondary hit so
	// players see the extra attack against the adjacent creature.
	if result.CleaveAttack != nil && result.CleaveAttack.Hit {
		fmt.Fprintf(&b, "\n    \u2192 \u2694\ufe0f Cleave hits %s for %d %s",
			result.CleaveAttack.TargetName, result.CleaveAttack.DamageTotal, result.CleaveAttack.DamageType)
	} else if result.CleaveAttack != nil {
		fmt.Fprintf(&b, "\n    \u2192 \u2694\ufe0f Cleave misses %s", result.CleaveAttack.TargetName)
	}

	// COV-8 Cunning Strike: surface the save outcome so players see whether the
	// forgone die landed the effect (Trip \u2192 Prone, Poison \u2192 Poisoned, \u2026).
	if rider, ok := cunningStrikeRiders[result.CunningStrikeChoice]; ok {
		if result.CunningStrikeSaved {
			fmt.Fprintf(&b, "\n    \u2192 \U0001f5e1\ufe0f Cunning Strike (%s): %s saves (DC %d)",
				rider.label, result.TargetName, result.CunningStrikeSaveDC)
		} else {
			fmt.Fprintf(&b, "\n    \u2192 \U0001f5e1\ufe0f Cunning Strike (%s): %s fails (DC %d) \u2014 %s",
				rider.label, result.TargetName, result.CunningStrikeSaveDC, rider.onFail)
		}
	}

	// COV-8 Brutal Strike: note the forgone Advantage and the effect that fired.
	// The +1d10 already shows in the damage breakdown as a "Brutal Strike" rider.
	if result.BrutalStrikeChoice == "forceful" {
		if result.Hit {
			fmt.Fprintf(&b, "\n    \u2192 \U0001f4aa Brutal Strike (Forceful Blow): Advantage forgone \u2014 %s pushed up to 15 ft", result.TargetName)
		} else {
			b.WriteString("\n    \u2192 \U0001f4aa Brutal Strike: Advantage forgone")
		}
	}

	if result.InvisibilityBroken {
		fmt.Fprintf(&b, "\n    \u2192 \U0001f441\ufe0f Invisibility ends \u2014 %s is visible again.", result.AttackerName)
	}

	writeDroppedToZero(&b, result)

	return b.String()
}

// writeDroppedToZero appends the target's drop-to-0 line (\ud83d\udc80/\ud83d\udc94) at the tail of
// the attack narrative when the hit dropped the target. Carried on the result
// (DownLogLine) so it rides the same #combat-log message as the hit \u2014 the
// service defers its own immediate post for FormatAttackLog paths, guaranteeing
// the "defeated" line lands AFTER the damage line, not before it.
func writeDroppedToZero(b *strings.Builder, result AttackResult) {
	if result.DownLogLine == "" {
		return
	}
	fmt.Fprintf(b, "\n    \u2192 %s", result.DownLogLine)
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

// applyHitDamage routes a landed attack's primary-hit damage through the shared
// ApplyDamage pipeline (R/I/V, temp HP, death-saves, concentration,
// unconscious-at-0, card refresh) — the same seam Graze/Cleave use. No-op on a
// miss or zero damage, so it never double-applies with the miss-only Graze or
// the different-target Cleave paths. Returns the post-damage target so callers
// that land multiple strikes on one target in a single command (Flurry of
// Blows) can thread the reduced HP into the next strike; on a no-op the target
// is returned unchanged.
// deferDownLog controls the drop-to-0 #combat-log post: attack paths whose
// result is rendered through FormatAttackLog pass true so the "defeated" line
// rides the same message as the hit (stamped onto result.DownLogLine here);
// paths with their own aggregated log (Flurry of Blows) pass false to keep the
// service's immediate post.
func (s *Service) applyHitDamage(ctx context.Context, encounterID uuid.UUID, target refdata.Combatant, result *AttackResult, deferDownLog bool) (refdata.Combatant, ApplyDamageResult, error) {
	if !result.Hit || result.DamageTotal <= 0 {
		return target, ApplyDamageResult{}, nil
	}
	out, err := s.ApplyDamage(ctx, ApplyDamageInput{
		EncounterID:  encounterID,
		Target:       target,
		RawDamage:    result.DamageTotal,
		DamageType:   result.DamageType,
		IsCritical:   result.CriticalHit,
		DeferDownLog: deferDownLog,
	})
	if err != nil {
		return target, ApplyDamageResult{}, fmt.Errorf("applying attack damage: %w", err)
	}
	if deferDownLog {
		result.DownLogLine = out.DownLogLine
	}
	return out.Updated, out, nil
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

	// Ammunition: spend one unit from inventory and track it for post-combat
	// recovery (see deductWeaponAmmunition, shared with the Crossbow Expert
	// bonus attack).
	if err := s.deductWeaponAmmunition(ctx, char, weapon, cmd.Attacker); err != nil {
		return AttackResult{}, err
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
	input.ReactionACBonus = cmd.ReactionACBonus
	input.ReactionReason = cmd.ReactionReason
	input.BonusDice = cmd.BonusDice
	input.Cover = coverLevel

	// 2024 Weapon Mastery: thread the attacker's known masteries (weapon ids)
	// so masteryActive can gate Graze / Topple. Tolerant parse → empty slice on
	// missing/invalid character_data.
	if char != nil && char.CharacterData.Valid {
		input.WeaponMasteries = parseWeaponMasteries(char.CharacterData.RawMessage)
	}

	// 2024 Tactical Master (Fighter 9) — the /attack tactical option replaces the
	// weapon's own mastery with Push/Sap/Slow for this attack. Gated so it only
	// substitutes a mastery the fighter already uses (never fabricates one); the
	// swapped slug flows through onHitMastery + applyMasteryEffects unchanged.
	// Absent the feature (or on a mastery-less/unknown weapon) it silently falls
	// back to the weapon's own mastery — safe because the fallback is always a
	// mastery the fighter can already use, unlike the gwm/reckless power toggles
	// that hard-error on misuse.
	if char != nil {
		if slug := tacticalMasteryOverride(cmd.TacticalMastery, input, char.Features); slug != "" {
			input.Weapon.Mastery = slug
		}
	}

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

	_, dmgOut, err := s.applyHitDamage(ctx, cmd.Attacker.EncounterID, cmd.Target, &result, true)
	if err != nil {
		return result, err
	}
	// TODO 3 — carry the above-0 → 0 HP transition (computed once inside
	// ApplyDamage) onto the result so populatePostHitPrompts can gate the GWM
	// bonus attack. The no-op path returns a zero-value result (false).
	result.TargetDroppedToZero = dmgOut.DroppedToZero

	// 2024 Weapon Mastery — apply the target-side effect that fired:
	//   - Graze: a miss deals the ability-modifier damage to the target HP.
	//   - Topple: a hit forces a CON save or the target falls Prone.
	// Gated by result.MasteryProperty so only an active mastery touches state.
	if err := s.applyMasteryEffects(ctx, cmd.Attacker, cmd.Target, &result, roller); err != nil {
		return result, err
	}

	// COV-8 Cunning Strike: a Rogue who dealt Sneak Attack damage this hit forces
	// the chosen effect's save or condition (Trip → DEX/Prone, Poison → CON/
	// Poisoned). Gated by result.CunningStrikeChoice (set in ResolveAttack only when
	// the feature is present, the hit landed, and Sneak Attack actually dealt damage).
	if err := s.applyCunningStrike(ctx, cmd.Attacker, cmd.Target, &result, roller); err != nil {
		return result, err
	}

	// COV-8 Brutal Strike: on a hit, resolve the chosen effect (Forceful Blow →
	// push 15 ft). The +1d10 and the forgone Advantage were already applied in
	// ResolveAttack; this is the forced-movement half.
	if err := s.applyBrutalStrike(ctx, cmd.Attacker, cmd.Target, &result); err != nil {
		return result, err
	}

	// 2024 Weapon Mastery — Cleave: a melee hit may auto-resolve one extra
	// attack against a second creature within 5ft of the primary target and
	// within the attacker's reach. Gated by result.MasteryProperty == "cleave"
	// and limited to once per the attacker's turn window.
	if err := s.applyCleaveAttack(ctx, cmd.Attacker, cmd.Target, weapon, input, &result, roller); err != nil {
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

	// SR-018 — clear any help_advantage / vex_advantage scoped to this target.
	// The grant is single-shot: the next attack vs the named target spends it.
	// Runs after resolveAndPersistAttack so a range/cover/charmed rejection
	// does not burn the Help / Vex grant.
	s.consumeHelpAdvantage(ctx, cmd.Attacker, cmd.Target)
	// 2024 Weapon Mastery — Sap: a sapped attacker's next attack rolls at
	// disadvantage; spend the grant once this attack has resolved (any target).
	s.consumeSapDisadvantage(ctx, cmd.Attacker)

	// Ammunition spend is tracked for post-combat half-recovery inside
	// deductWeaponAmmunition (above), coupled with the inventory write.

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
		// ISSUE-035: a thrown LIGHT melee weapon still satisfies the two-weapon-
		// fighting prerequisite for an off-hand swing this turn, even though it
		// has now left the hand. Record it so OffhandAttack doesn't reject on the
		// emptied main hand (lets a two-dagger thrower throw the off-hand next).
		if HasProperty(weapon, "light") && !IsRangedWeapon(weapon) {
			s.markUsedEffects(cmd.Attacker.EncounterID, cmd.Attacker.ID, []string{mainHandThrownLightEffect})
		}
	}

	// ISSUE-014: persist the attack to action_log for the DM Console timeline.
	s.recordCombatAction(ctx, cmd.Turn.ID, cmd.Attacker.EncounterID, cmd.Attacker.ID,
		nullableCombatantID(cmd.Target.ID), actionTypeAttack, describeAttack(result))

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
	if _, _, err := s.applyHitDamage(ctx, cmd.Attacker.EncounterID, cmd.Target, &result, true); err != nil {
		return result, err
	}
	// SR-010 — mirror Service.Attack: improvised hits that produce a
	// once-per-turn FES effect must close that slot too.
	s.markUsedEffects(cmd.Attacker.EncounterID, cmd.Attacker.ID, result.OncePerTurnEffectsFired)
	s.markRageAttacked(ctx, cmd.Attacker)
	s.populatePostHitPrompts(ctx, &result, cmd.Attacker, charPtr)
	// SR-018 — improvised attacks also consume help_advantage / vex_advantage
	// targeting cmd.Target, and any sap_disadvantage on the attacker.
	s.consumeHelpAdvantage(ctx, cmd.Attacker, cmd.Target)
	s.consumeSapDisadvantage(ctx, cmd.Attacker)

	// ISSUE-014: persist the improvised attack to action_log for the DM Console.
	s.recordCombatAction(ctx, cmd.Turn.ID, cmd.Attacker.EncounterID, cmd.Attacker.ID,
		nullableCombatantID(cmd.Target.ID), actionTypeAttack, describeAttack(result))

	return result, nil
}

// OffhandAttack is the service-level method for a two-weapon fighting off-hand attack.
// It uses the bonus action, validates both weapons are light, and resolves the off-hand attack
// with 0 damage modifier (unless the character has the Two-Weapon Fighting fighting style).
func (s *Service) OffhandAttack(ctx context.Context, cmd OffhandAttackCommand, roller *dice.Roller) (AttackResult, error) {
	// NOTE: the bonus-action availability check is deliberately NOT done here.
	// The 2024 Nick mastery absorbs the off-hand swing into the Attack action so
	// it costs no bonus action — a Nick attacker who already spent their bonus
	// action (Steady Aim, Cunning Action, Fast Hands) may still swing. The
	// bonus action is validated + spent only on the non-Nick path below, via the
	// conditional UseResource(ResourceBonusAction) after nickFree is known.

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

	// Validate the main hand held a light melee weapon for this two-weapon-
	// fighting swing. ISSUE-035: a LIGHT melee weapon thrown from the main hand
	// THIS turn still satisfies the prerequisite even though it has now left the
	// hand (the throw cleared EquippedMainHand) — otherwise a legitimate two-
	// dagger thrower can't follow a thrown main-hand dagger with an off-hand throw.
	used := s.usedEffectsSnapshot(cmd.Attacker.EncounterID, cmd.Attacker.ID)

	// ISSUE-062: once-per-turn cap on the Light-property (off-hand) extra
	// attack. The first off-hand swing this turn is always allowed; a SECOND is
	// allowed only with the Dual Wielder feat (and never a third). Without the
	// feat, Nick merely frees the bonus action — it does not grant an extra
	// swing. Recorded (offhand_extra / dw_extra) only after the swing resolves,
	// below, so a rejected/out-of-range attack never burns the per-turn slot.
	secondExtra := used[offhandExtraUsedEffect]
	if secondExtra {
		if !HasFeatureByName(char.Features.RawMessage, "Dual Wielder") {
			return AttackResult{}, fmt.Errorf("you've already made your off-hand (Light weapon) extra attack this turn; only the Dual Wielder feat grants a second")
		}
		if used[dwExtraUsedEffect] {
			return AttackResult{}, fmt.Errorf("no off-hand attacks remain this turn")
		}
	}

	if !used[mainHandThrownLightEffect] {
		if !char.EquippedMainHand.Valid || char.EquippedMainHand.String == "" {
			return AttackResult{}, fmt.Errorf("off-hand attack needs a light melee weapon in your main hand (two-weapon fighting requires a weapon in each hand)")
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
	// A thrown off-hand swing requires the weapon to actually have the "thrown"
	// property (e.g. a dagger). Reject early with a clear, player-facing message.
	if cmd.Thrown && !HasProperty(offWeapon, "thrown") {
		return AttackResult{}, fmt.Errorf("off-hand weapon %q cannot be thrown (no thrown property)", offWeapon.Name)
	}

	// Off-hand attacks: no ability modifier unless TWF fighting style,
	// but negative modifiers still apply per RAW (PHB p195).
	dmgMod := 0
	if HasFightingStyle(char.Features, "two_weapon_fighting") {
		dmgMod = DamageModifier(scores, offWeapon)
	} else {
		mod := DamageModifier(scores, offWeapon)
		if mod < 0 {
			dmgMod = mod
		}
	}

	// Phase 33 / C-33: cover gate runs BEFORE consuming the bonus action so
	// a total-cover off-hand swing doesn't burn the resource.
	coverLevel, err := s.resolveAttackCover(ctx, cmd.Attacker, cmd.Target, cmd.Walls)
	if err != nil {
		return AttackResult{}, err
	}

	// 2024 Weapon Mastery — Nick: the off-hand weapon has Nick AND the attacker
	// knows its mastery → the Light-property extra attack is absorbed into the
	// Attack action and does NOT cost the bonus action (once per turn). Thread
	// the attacker's known masteries like the main Attack path does.
	var offMasteries []string
	if char.CharacterData.Valid {
		offMasteries = parseWeaponMasteries(char.CharacterData.RawMessage)
	}
	nickFree := s.nickAbsorbsBonusAction(cmd.Attacker, offWeapon, offMasteries)

	// Spend the bonus action unless Nick makes this off-hand attack free.
	updatedTurn := cmd.Turn
	if !nickFree {
		updatedTurn, err = UseResource(cmd.Turn, ResourceBonusAction)
		if err != nil {
			return AttackResult{}, fmt.Errorf("using bonus action: %w", err)
		}
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
	input.WeaponMasteries = offMasteries
	// Thrown off-hand: expand effective range to the weapon's thrown range and
	// apply long-range disadvantage, mirroring the main-hand thrown path.
	input.Thrown = cmd.Thrown

	// Obscurement from encounter zones
	attackerObs, targetObs, err := s.resolveObscurement(ctx, cmd.Attacker.EncounterID, cmd.Attacker, cmd.Target, cmd.AttackerVision, cmd.TargetVision)
	if err != nil {
		return AttackResult{}, err
	}
	input.AttackerObscurement = attackerObs
	input.TargetObscurement = targetObs

	// Both hands hold a weapon during a two-weapon swing — flag it so the FES
	// OneHandedMeleeOnly check stays false (no spurious Dueling +2 on the off-hand).
	input.OffHandOccupied = true

	// Wire the full Feature Effect System for the off-hand swing, exactly as the
	// main Attack path (the populateAttackFES call in Service.Attack) and
	// GWMBonusAttack do. Before this, OffhandAttack built a bare AttackInput and
	// skipped every FES rider, so a two-weapon swing silently dropped Hex /
	// Hunter's Mark, magic-weapon +X, Sneak Attack, Savage Attacker, etc. The
	// two-weapon "no ability modifier to the off-hand damage" rule is still
	// enforced by the explicit OverrideDmgMod set in buildAttackInput above —
	// populateAttackFES never touches OverrideDmgMod and ResolveAttack keeps it
	// (see the OverrideDmgMod branch) — so the riders ride on top of the correct
	// off-hand base damage. A minimal AttackCommand carries the only fields
	// populateAttackFES reads.
	fesCmd := AttackCommand{Attacker: cmd.Attacker, Target: cmd.Target, Turn: updatedTurn}
	if err := s.populateAttackFES(ctx, &input, fesCmd, &char, offWeapon, scores); err != nil {
		return AttackResult{}, err
	}

	result, err := s.resolveAndPersistAttack(ctx, input, updatedTurn, cmd.Attacker, roller)
	if err != nil {
		return result, err
	}
	if _, _, err := s.applyHitDamage(ctx, cmd.Attacker.EncounterID, cmd.Target, &result, true); err != nil {
		return result, err
	}
	// Persist any once-per-turn rider that fired on the off-hand swing (Sneak
	// Attack, Savage Attacker) so a later attack this same turn — including a
	// reaction on another creature's turn — can't re-fire it. Mirrors the
	// markUsedEffects(result.OncePerTurnEffectsFired) calls in Service.Attack and
	// GWMBonusAttack; the off-hand path previously omitted it.
	s.markUsedEffects(cmd.Attacker.EncounterID, cmd.Attacker.ID, result.OncePerTurnEffectsFired)

	// 2024 Weapon Mastery — apply the on-hit effect that fired on the off-hand
	// swing (Vex/Sap/Topple/Slow/Push), mirroring the main-hand Attack path
	// (see the applyMasteryEffects call in Attack). Without this the off-hand
	// path detected the mastery (result.MasteryProperty was set) but never
	// applied it — e.g. an off-hand Vex hit granted no advantage next turn.
	// Runs before the scoped-advantage consume below, which reads the stale
	// pre-attack condition snapshot and so never clears this fresh grant.
	if err := s.applyMasteryEffects(ctx, cmd.Attacker, cmd.Target, &result, roller); err != nil {
		return result, err
	}

	// 2024 Weapon Mastery — Nick: mark the free off-hand spent for this turn so
	// a SECOND Nick off-hand the same turn costs the bonus action. Recorded
	// after the swing resolves so a rejected attack does not burn the slot.
	if nickFree {
		s.markUsedEffects(cmd.Attacker.EncounterID, cmd.Attacker.ID, []string{nickUsedEffect})
	}
	// ISSUE-062: record the once-per-turn Light-extra cap key. The first off-hand
	// swing marks offhand_extra; a Dual-Wielder-authorised second marks dw_extra
	// (capping total off-hand swings at 2). Marked after the swing resolves so a
	// rejected attack does not consume the slot (same discipline as Nick above).
	if secondExtra {
		s.markUsedEffects(cmd.Attacker.EncounterID, cmd.Attacker.ID, []string{dwExtraUsedEffect})
	} else {
		s.markUsedEffects(cmd.Attacker.EncounterID, cmd.Attacker.ID, []string{offhandExtraUsedEffect})
	}
	s.markRageAttacked(ctx, cmd.Attacker)
	s.populatePostHitPrompts(ctx, &result, cmd.Attacker, &char)
	// SR-018 — off-hand swings count as an attack vs the target and so spend
	// any help_advantage / vex_advantage scoped to that target, plus any
	// sap_disadvantage on the attacker.
	s.consumeHelpAdvantage(ctx, cmd.Attacker, cmd.Target)
	s.consumeSapDisadvantage(ctx, cmd.Attacker)

	// ISSUE-014: persist the off-hand attack to action_log for the DM Console.
	s.recordCombatAction(ctx, cmd.Turn.ID, cmd.Attacker.EncounterID, cmd.Attacker.ID,
		nullableCombatantID(cmd.Target.ID), actionTypeAttack, describeAttack(result))

	// Phase 37 parity: a thrown off-hand weapon leaves the hand. Clear
	// EquippedOffHand so the same dagger can't be re-thrown without re-equipping;
	// inventory retains the item for a future turn. Runs after the swing resolves
	// so a rejected attack does not drop the weapon.
	if cmd.Thrown && char.EquippedOffHand.Valid && char.EquippedOffHand.String != "" {
		if _, equipErr := s.store.UpdateCharacterEquipment(ctx, refdata.UpdateCharacterEquipmentParams{
			ID:               char.ID,
			EquippedMainHand: char.EquippedMainHand,
			EquippedOffHand:  sql.NullString{},
			EquippedArmor:    char.EquippedArmor,
			Ac:               char.Ac,
		}); equipErr != nil {
			return result, fmt.Errorf("clearing thrown off-hand weapon: %w", equipErr)
		}
	}

	return result, nil
}

// GWMBonusAttackCommand holds the inputs for the 2024 Great Weapon Master
// bonus-action attack: one swing with the same Heavy melee weapon at the full
// ability modifier, offered after a crit or a drop-to-0 with that weapon.
type GWMBonusAttackCommand struct {
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
	// cover, mirroring OffhandAttack. A nil/empty slice degrades to "no wall
	// cover"; creature-granted cover still applies via the combatant list.
	Walls []renderer.WallSegment
}

// GWMBonusAttack makes the 2024 Great Weapon Master bonus-action swing. Unlike
// the off-hand (two-weapon-fighting) attack, it uses the attacker's equipped
// MAIN-HAND Heavy weapon at the full ability modifier — the whole point of the
// feat's rider. Eligibility (crit or drop-to-0) is decided at the triggering
// attack (PromptGWMBonusAttackEligible); here we re-validate the durable
// prerequisites (character, feat, Heavy melee main weapon, bonus action free)
// so a stale prompt or a direct call can't produce an illegal swing.
func (s *Service) GWMBonusAttack(ctx context.Context, cmd GWMBonusAttackCommand, roller *dice.Roller) (AttackResult, error) {
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return AttackResult{}, err
	}

	// C-40: a charmed combatant cannot attack the source of its charm.
	if err := validateCharmedAttack(cmd.Attacker, cmd.Target); err != nil {
		return AttackResult{}, err
	}

	if !cmd.Attacker.CharacterID.Valid {
		return AttackResult{}, fmt.Errorf("Great Weapon Master bonus attack requires a character (not NPC)")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Attacker.CharacterID.UUID)
	if err != nil {
		return AttackResult{}, fmt.Errorf("getting character: %w", err)
	}

	if !HasFeatureByName(char.Features.RawMessage, "Great Weapon Master") {
		return AttackResult{}, fmt.Errorf("Great Weapon Master bonus attack requires the feat")
	}

	if !char.EquippedMainHand.Valid || char.EquippedMainHand.String == "" {
		return AttackResult{}, fmt.Errorf("Great Weapon Master bonus attack requires a heavy melee weapon in your main hand")
	}
	weapon, err := s.store.GetWeapon(ctx, char.EquippedMainHand.String)
	if err != nil {
		return AttackResult{}, fmt.Errorf("getting main hand weapon: %w", err)
	}
	if IsRangedWeapon(weapon) || !HasProperty(weapon, "heavy") {
		return AttackResult{}, fmt.Errorf("Great Weapon Master bonus attack requires a heavy melee weapon")
	}

	parsed, err := ParseAbilityScores(char.AbilityScores)
	if err != nil {
		return AttackResult{}, fmt.Errorf("parsing ability scores: %w", err)
	}
	// SR-022: wild-shaped attackers use the beast's merged scores; a failed
	// beast lookup degrades to the character's own scores.
	scores := ResolveAttackerScores(ctx, s.store, cmd.Attacker, parsed)

	// Phase 33 / C-33: cover gate runs BEFORE consuming the bonus action so a
	// total-cover swing doesn't burn the resource.
	coverLevel, err := s.resolveAttackCover(ctx, cmd.Attacker, cmd.Target, cmd.Walls)
	if err != nil {
		return AttackResult{}, err
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return AttackResult{}, fmt.Errorf("using bonus action: %w", err)
	}

	distFt := combatantDistance(cmd.Attacker, cmd.Target)
	dmAdv, dmDisadv := s.consumeDMAdvOverride(ctx, cmd.Attacker, cmd.DMAdvantage, cmd.DMDisadvantage)
	input := buildAttackInput(
		cmd.Attacker, cmd.Target, weapon, scores, int(char.ProficiencyBonus), distFt,
		cmd.HostileNearAttacker, cmd.AttackerSize,
		dmAdv, dmDisadv, nil, // nil dmgMod → full ability modifier (the GWM benefit)
	)
	s.populateAttackContext(ctx, &input, cmd.Attacker)
	input.TwoHanded = true // heavy weapons are swung two-handed
	input.Cover = coverLevel
	if char.CharacterData.Valid {
		input.WeaponMasteries = parseWeaponMasteries(char.CharacterData.RawMessage)
	}

	attackerObs, targetObs, err := s.resolveObscurement(ctx, cmd.Attacker.EncounterID, cmd.Attacker, cmd.Target, cmd.AttackerVision, cmd.TargetVision)
	if err != nil {
		return AttackResult{}, err
	}
	input.AttackerObscurement = attackerObs
	input.TargetObscurement = targetObs

	// Wire FES so class/style riders (Rage, magic items) and the once-per-turn
	// tracker apply to the bonus swing too. A minimal AttackCommand carries the
	// only fields populateAttackFES reads; GWM2024 is intentionally left false so
	// the +PB damage rider stays opt-in on the main /attack and can't double-dip.
	fesCmd := AttackCommand{Attacker: cmd.Attacker, Target: cmd.Target, Turn: updatedTurn}
	if err := s.populateAttackFES(ctx, &input, fesCmd, &char, weapon, scores); err != nil {
		return AttackResult{}, err
	}

	result, err := s.resolveAndPersistAttack(ctx, input, updatedTurn, cmd.Attacker, roller)
	if err != nil {
		return result, err
	}
	if _, _, err := s.applyHitDamage(ctx, cmd.Attacker.EncounterID, cmd.Target, &result, true); err != nil {
		return result, err
	}

	// 2024 Weapon Mastery — apply any on-hit effect that fired (Topple/Cleave etc.),
	// mirroring the main Attack path.
	if err := s.applyMasteryEffects(ctx, cmd.Attacker, cmd.Target, &result, roller); err != nil {
		return result, err
	}
	s.markUsedEffects(cmd.Attacker.EncounterID, cmd.Attacker.ID, result.OncePerTurnEffectsFired)
	s.markRageAttacked(ctx, cmd.Attacker)

	// ISSUE-014: persist to action_log for the DM Console timeline.
	s.recordCombatAction(ctx, cmd.Turn.ID, cmd.Attacker.EncounterID, cmd.Attacker.ID,
		nullableCombatantID(cmd.Target.ID), actionTypeAttack, describeAttack(result))

	return result, nil
}

// nickUsedEffect is the once-per-turn key recorded when the Nick off-hand
// attack is made free (absorbed into the Attack action). Shares the Sneak
// Attack tracker so a second Nick off-hand the same turn costs the bonus action.
const nickUsedEffect = "nick"

// offhandExtraUsedEffect marks that the Light-property extra (off-hand) attack
// was already made this turn. A second off-hand attack is allowed only with the
// Dual Wielder feat (dwExtraUsedEffect below). ISSUE-062.
const offhandExtraUsedEffect = "offhand_extra"

// dwExtraUsedEffect marks that the Dual Wielder bonus-action extra attack was
// made this turn (the legit 2nd off-hand swing). Caps total off-hand attacks at
// 2 (3 total swings with Nick freeing the bonus action). ISSUE-062.
const dwExtraUsedEffect = "dw_extra"

// mainHandThrownLightEffect marks that the attacker threw a LIGHT melee weapon
// from the main hand during the Attack action this turn. A thrown weapon leaves
// the hand (EquippedMainHand is cleared), so the off-hand two-weapon-fighting
// follow-up can no longer see the weapon that justified it. This per-turn marker
// (same lifecycle as Nick — cleared at the combatant's turn start) lets
// OffhandAttack recognise that the TWF prerequisite was met, so a legitimate
// two-dagger thrower can throw the off-hand weapon after throwing the main one.
// ISSUE-035.
const mainHandThrownLightEffect = "mainhand_light_thrown"

// nickAbsorbsBonusAction reports whether the 2024 Nick mastery makes this
// off-hand attack free this turn: the off-hand weapon has Nick, the attacker
// knows that weapon's mastery, and Nick has not already been used this turn.
func (s *Service) nickAbsorbsBonusAction(attacker refdata.Combatant, offWeapon refdata.Weapon, knownMasteries []string) bool {
	if !masteryActive(AttackInput{Weapon: offWeapon, WeaponMasteries: knownMasteries}) {
		return false
	}
	if offWeapon.Mastery != "nick" {
		return false
	}
	used := s.usedEffectsSnapshot(attacker.EncounterID, attacker.ID)
	return !used[nickUsedEffect]
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
	// PCs: look up race size from character record.
	if attacker.CharacterID.Valid {
		char, err := s.store.GetCharacter(ctx, attacker.CharacterID.UUID)
		if err == nil && char.Race != "" {
			race, err := s.store.GetRace(ctx, char.Race)
			if err == nil {
				return race.Size
			}
		}
	}
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

// consumeHelpAdvantage clears single-shot target-scoped attacker advantage
// grants when their TargetCombatantID matches the combatant just attacked:
//   - help_advantage (SR-018: Help action), and
//   - vex_advantage (2024 Weapon Mastery — Vex)
//
// Both grant advantage on the helped/attacking creature's next attack vs the
// named target; once that attack has resolved (hit or miss) the condition is
// spent and must not carry over to subsequent attacks. Conditions scoped to a
// different target are left in place so a fresh /attack vs the named target
// still triggers DetectAdvantage's matching branch.
func (s *Service) consumeHelpAdvantage(ctx context.Context, attacker refdata.Combatant, target refdata.Combatant) {
	conds, err := parseConditions(attacker.Conditions)
	if err != nil {
		return
	}
	targetID := target.ID.String()
	filtered := make([]CombatCondition, 0, len(conds))
	consumed := false
	for _, c := range conds {
		scopedGrant := c.Condition == "help_advantage" || c.Condition == "vex_advantage"
		if scopedGrant && c.TargetCombatantID == targetID {
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
		log.Printf("marshaling conditions after scoped-advantage consume for %s: %v", attacker.ID, err)
		return
	}
	if _, err := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
		ID:              attacker.ID,
		Conditions:      newConds,
		ExhaustionLevel: attacker.ExhaustionLevel,
	}); err != nil {
		log.Printf("clearing scoped advantage on %s: %v", attacker.ID, err)
	}
}

// consumeSapDisadvantage clears a sap_disadvantage condition from the attacker
// after it makes an attack. 2024 Weapon Mastery — Sap imposes disadvantage on
// the sapped creature's NEXT attack; once that attack resolves (hit or miss)
// the grant is spent. Unlike help/vex this is not target-scoped: the next
// attack against any target spends it.
func (s *Service) consumeSapDisadvantage(ctx context.Context, attacker refdata.Combatant) {
	conds, err := parseConditions(attacker.Conditions)
	if err != nil {
		return
	}
	filtered := make([]CombatCondition, 0, len(conds))
	consumed := false
	for _, c := range conds {
		if c.Condition == "sap_disadvantage" {
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
		log.Printf("marshaling conditions after sap_disadvantage consume for %s: %v", attacker.ID, err)
		return
	}
	if _, err := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
		ID:              attacker.ID,
		Conditions:      newConds,
		ExhaustionLevel: attacker.ExhaustionLevel,
	}); err != nil {
		log.Printf("clearing sap_disadvantage on %s: %v", attacker.ID, err)
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
		ExhaustionLevel:     int(attacker.ExhaustionLevel),
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
// the human-readable ability label ("str", "dex", or "cha") that was used for
// the attack roll. Used to populate EffectContext.AbilityUsed so FES filters
// like Rage's `ability_used: str` actually evaluate correctly.
func attackAbilityUsed(scores AbilityScores, weapon refdata.Weapon, monkLevel int, isRaging, pactBladeCHA bool) string {
	// COV-7 Pact of the Blade: CHA replaces the weapon's normal ability when it
	// is the higher choice (see effectiveAbilityMod), so the label must report
	// "cha" — otherwise Rage's melee-STR filter would misfire on a CHA attack.
	if pactBladeCHA && AbilityModifier(scores.Cha) > abilityModForWeapon(scores, weapon, monkLevel) {
		return "cha"
	}
	strMod := AbilityModifier(scores.Str)
	dexMod := AbilityModifier(scores.Dex)

	if HasProperty(weapon, "finesse") {
		if isRaging {
			return "str"
		}
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
		d := max(dr, dc)
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
	// COV-7 Pact of the Blade: a warlock bonded to a pact weapon may use CHA for
	// its attack and damage rolls. Decide eligibility once (guarding the nil char
	// for NPC attackers) so both the value-substitution flag and the AbilityUsed
	// label flow from a single decision. Improvised weapons and unarmed strikes
	// are never pact weapons; the CHA value substitution itself lives in
	// ResolveAttack via effectiveAbilityMod.
	input.PactBladeCHA = char != nil &&
		HasInvocation(char.Features, pactOfTheBladeEffectID) &&
		!input.IsImprovised && weapon.ID != "unarmed-strike"
	input.AbilityUsed = attackAbilityUsed(scores, weapon, input.MonkLevel, input.IsRaging, input.PactBladeCHA)
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

	// COV-8 Cunning Strike (Rogue 5): forgo Sneak Attack dice to add an on-hit
	// rider. For a known effect chosen by a rogue who carries the feature (slug
	// gate), subtract the effect's die cost from the Sneak Attack extra-damage dice
	// here — before the roll — and bake the eligibility into input.CunningStrike so
	// ResolveAttack can precompute the save DC and Service.Attack can resolve the
	// rider post-hit. A non-rogue's /attack cunning (or an unknown choice) is inert:
	// the map lookup short-circuits before hasFeatureEffect, so no dice are touched.
	if rider, ok := cunningStrikeRiders[cmd.CunningStrike]; ok && hasFeatureEffect(char.Features, cunningStrikeEffect) {
		input.CunningStrike = cmd.CunningStrike
		reduceSneakAttackDice(input.Features, rider.diceCost)
	}

	// COV-8 Brutal Strike (Barbarian 9): while using Reckless Attack, forgo the
	// Advantage on a STR melee attack for +1d10 damage plus a forced-movement
	// effect. When eligible, bake the choice into input.BrutalStrike (ResolveAttack
	// forgoes Advantage via AdvantageInput.ForgoAdvantage and records the effect for
	// applyBrutalStrike) and inject the +1d10 EffectExtraDamageDice rider. A
	// non-eligible /attack brutal is inert (see brutalStrikeEligible).
	if brutalStrikeEligible(cmd, char.Features, weapon, input.AbilityUsed) {
		input.BrutalStrike = cmd.BrutalStrike
		input.Features = append(input.Features, BrutalStrikeFeature(weapon.DamageType))
	}

	// SR-058: Sacred Weapon condition → inject CHA mod as modify_attack_roll.
	if HasCondition(cmd.Attacker.Conditions, "sacred_weapon") {
		chaMod := max(AbilityModifier(scores.Cha), 1)
		input.Features = append(input.Features, SacredWeaponFeature(chaMod))
	}

	// 2024 Great Weapon Master (opt-in per attack): when the player toggles the
	// gwm2024 flag AND actually has the feat, inject the +ProficiencyBonus
	// once-per-turn damage rider. Name-based detection dodges the level-up
	// mechanical_effect JSON-array shape that slug matching misses. The heavy
	// melee gating is enforced by the FeatureDefinition's own conditions, so a
	// non-heavy or ranged weapon simply won't trip it.
	if cmd.GWM2024 && HasFeatureByName(char.Features.RawMessage, "Great Weapon Master") {
		input.Features = append(input.Features, GreatWeaponMasterFeature(int(char.ProficiencyBonus)))
	}

	// COV-9 Savage Attacker: a character with the feat rerolls a melee weapon's
	// damage dice once per turn and keeps the higher total. Name-based detection
	// mirrors GWM (dodges the mechanical_effect JSON-array shape slug matching
	// misses). The melee gate and once-per-turn spend live in ResolveAttack
	// (savageAttackerEligible), so every attack path through populateAttackFES —
	// Attack, OffhandAttack, GWMBonusAttack — shares this one flag. Scans the
	// already-parsed `feats` slice (not a fresh json.Unmarshal) to avoid re-parsing
	// char.Features on the attack hot path.
	input.SavageAttacker = featsHaveName(feats, "Savage Attacker")

	// COV-9 Sharpshooter (passive riders): a character with the feat makes ranged
	// attacks that ignore half/three-quarters cover and suffer no long-range
	// disadvantage. These are always on when the feat is present — unlike the
	// -5/+10 power-attack toggle (cmd.Sharpshooter, set in Service.Attack).
	// Name-based detection mirrors Savage Attacker / GWM: level-up feats land in
	// the features JSON by name without a mechanical_effect slug.
	input.HasSharpshooter = featsHaveName(feats, "Sharpshooter")

	// Hex: when the target carries this attacker's source-tagged Hex marker,
	// every hit adds 1d6 necrotic (5e Hex). Gating by the marker means only the
	// caster concentrating on Hex against this target gets the rider.
	if targetHexedBy(cmd.Target.Conditions, cmd.Attacker.ID) {
		input.Features = append(input.Features, HexFeature())
	}

	// Hunter's Mark: when the target carries this attacker's source-tagged mark,
	// every weapon hit adds 1d6 force (2024 Hunter's Mark). Gating by the marker
	// means only the ranger concentrating on Hunter's Mark against this target
	// gets the rider. Mirrors the Hex block above.
	if targetHuntersMarkedBy(cmd.Target.Conditions, cmd.Attacker.ID) {
		input.Features = append(input.Features, HuntersMarkFeature())
	}

	// COV-6 Lifedrinker: a warlock bonded to a pact weapon with this invocation
	// adds its Charisma modifier (min 1) as necrotic damage on every pact-weapon
	// hit (2024). Gated on the same pact-weapon eligibility as the CHA
	// substitution (PactBladeCHA), so it rides only the warlock's pact-weapon
	// attacks. A flat modifier rider, unlike the dice-based Hex/Hunter's Mark.
	if input.PactBladeCHA && HasInvocation(char.Features, lifedrinkerEffectID) {
		input.Features = append(input.Features, LifedrinkerFeature(max(AbilityModifier(scores.Cha), 1)))
	}

	return nil
}
