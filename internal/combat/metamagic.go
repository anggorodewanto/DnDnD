package combat

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ab/dndnd/internal/refdata"
)

// ValidateMetamagicOptions validates that each metamagic option's prerequisites
// are met by the spell being cast.
func ValidateMetamagicOptions(metamagics []string, spell refdata.Spell) error {
	for _, m := range metamagics {
		if err := validateSingleMetamagicOption(strings.ToLower(m), spell); err != nil {
			return err
		}
	}
	return nil
}

func validateSingleMetamagicOption(option string, spell refdata.Spell) error {
	switch option {
	case "careful":
		return validateCarefulSpell(spell)
	case "distant":
		return validateDistantSpell(spell)
	case "empowered":
		return validateEmpoweredSpell(spell)
	case "extended":
		return validateExtendedSpell(spell)
	case "heightened":
		return validateHeightenedSpell(spell)
	case "quickened":
		return validateQuickenedSpell(spell)
	case "subtle":
		return nil // no restrictions
	case "twinned":
		return validateTwinnedSpell(spell)
	default:
		return nil
	}
}

// Spell predicate helpers used by multiple metamagic validators.

func hasAreaOfEffect(spell refdata.Spell) bool {
	return spell.AreaOfEffect.Valid && len(spell.AreaOfEffect.RawMessage) > 0
}

func hasSavingThrow(spell refdata.Spell) bool {
	return spell.SaveAbility.Valid && spell.SaveAbility.String != ""
}

func hasDamage(spell refdata.Spell) bool {
	return spell.Damage.Valid && len(spell.Damage.RawMessage) > 0
}

func validateCarefulSpell(spell refdata.Spell) error {
	if !hasAreaOfEffect(spell) {
		return fmt.Errorf("Careful Spell requires a spell with an area of effect")
	}
	if !hasSavingThrow(spell) {
		return fmt.Errorf("Careful Spell requires a spell with a saving throw")
	}
	return nil
}

func validateDistantSpell(spell refdata.Spell) error {
	if spell.RangeType == "touch" {
		return nil
	}
	if spell.RangeType == "ranged" && spell.RangeFt.Valid && spell.RangeFt.Int32 > 0 {
		return nil
	}
	return fmt.Errorf("Distant Spell requires a spell with range > 0 or touch (not self)")
}

func validateEmpoweredSpell(spell refdata.Spell) error {
	if !hasDamage(spell) {
		return fmt.Errorf("Empowered Spell requires a spell that deals damage")
	}
	return nil
}

func validateExtendedSpell(spell refdata.Spell) error {
	if !hasDurationAtLeastOneMinute(spell.Duration) {
		return fmt.Errorf("Extended Spell requires a spell with duration of at least 1 minute")
	}
	return nil
}

func validateHeightenedSpell(spell refdata.Spell) error {
	if !hasSavingThrow(spell) {
		return fmt.Errorf("Heightened Spell requires a spell with a saving throw")
	}
	return nil
}

func validateQuickenedSpell(spell refdata.Spell) error {
	if !strings.Contains(strings.ToLower(spell.CastingTime), "1 action") {
		return fmt.Errorf("Quickened Spell requires a spell with casting time of 1 action")
	}
	return nil
}

func validateTwinnedSpell(spell refdata.Spell) error {
	if spell.RangeType == "self" || spell.RangeType == "self (radius)" {
		return fmt.Errorf("Twinned Spell cannot target a self-range spell")
	}
	if hasAreaOfEffect(spell) {
		return fmt.Errorf("Twinned Spell cannot target a spell with an area of effect")
	}
	return nil
}

// durationRegex matches duration strings like "1 minute", "10 minutes", "1 hour", "8 hours", "7 days"
var durationRegex = regexp.MustCompile(`(?i)(\d+)\s*(minute|hour|day)`)

// hasDurationAtLeastOneMinute returns true if the spell duration is >= 1 minute.
func hasDurationAtLeastOneMinute(duration string) bool {
	matches := durationRegex.FindStringSubmatch(duration)
	if len(matches) < 3 {
		return false
	}
	amount, err := strconv.Atoi(matches[1])
	if err != nil || amount <= 0 {
		return false
	}
	return true
}

// ApplyDistantSpell returns the new range description after applying Distant Spell.
// Touch spells become "30 ft.", ranged spells get doubled range.
func ApplyDistantSpell(spell refdata.Spell) string {
	if spell.RangeType == "touch" {
		return "30 ft."
	}
	if spell.RangeType == "ranged" && spell.RangeFt.Valid && spell.RangeFt.Int32 > 0 {
		return fmt.Sprintf("%d ft.", spell.RangeFt.Int32*2)
	}
	return ""
}

// ApplyExtendedSpell returns the doubled duration string, capped at 24 hours.
// Returns empty string if the duration cannot be parsed or extended.
func ApplyExtendedSpell(duration string) string {
	lower := strings.ToLower(duration)
	prefix := ""
	parseDur := lower
	if strings.HasPrefix(lower, "up to ") {
		prefix = "Up to "
		parseDur = lower[6:]
	}

	matches := durationRegex.FindStringSubmatch(parseDur)
	if len(matches) < 3 {
		return ""
	}

	amount, err := strconv.Atoi(matches[1])
	if err != nil || amount <= 0 {
		return ""
	}

	unit := strings.ToLower(matches[2])
	doubled := amount * 2

	// Convert to hours for cap check
	var totalHours float64
	switch unit {
	case "minute":
		totalHours = float64(doubled) / 60.0
	case "hour":
		totalHours = float64(doubled)
	case "day":
		totalHours = float64(doubled) * 24.0
	}

	// Cap at 24 hours
	if totalHours > 24 {
		return "24 hours"
	}

	// Pluralize
	unitStr := unit
	if doubled != 1 {
		unitStr += "s"
	}

	return fmt.Sprintf("%s%d %s", prefix, doubled, unitStr)
}

// applyMetamagicEffects populates the CastResult fields for each active metamagic option.
func applyMetamagicEffects(result *CastResult, metamagics []string, spell refdata.Spell, chaScore int) {
	for _, m := range metamagics {
		switch strings.ToLower(m) {
		case "careful":
			result.CarefulSpellCreatures = CarefulSpellCreatureCount(chaScore)
		case "distant":
			result.DistantRange = ApplyDistantSpell(spell)
		case "empowered":
			result.IsEmpowered = true
			result.EmpoweredRerolls = EmpoweredRerollCount(chaScore)
		case "extended":
			result.ExtendedDuration = ApplyExtendedSpell(spell.Duration)
		case "heightened":
			result.IsHeightened = true
		case "subtle":
			result.IsSubtle = true
		}
	}
}

// chaModMin1 returns the CHA modifier for the given score, with a minimum of 1.
func chaModMin1(chaScore int) int {
	mod := AbilityModifier(chaScore)
	if mod < 1 {
		return 1
	}
	return mod
}

// CarefulSpellCreatureCount returns the number of creatures that can auto-succeed
// on the save, which equals the caster's CHA modifier (minimum 1).
func CarefulSpellCreatureCount(chaScore int) int {
	return chaModMin1(chaScore)
}

// EmpoweredRerollCount returns the max number of damage dice the caster can reroll,
// which equals the caster's CHA modifier (minimum 1).
func EmpoweredRerollCount(chaScore int) int {
	return chaModMin1(chaScore)
}

// RerollLowestDice replaces the `count` lowest values in `rolls` with fresh
// rolls drawn from `randFn` (which must return an integer in [1, faces]).
// Returns a new slice; the input is not mutated. When `count` is <= 0 the
// function returns a copy of the input unchanged; when `count` exceeds
// `len(rolls)` every die is rerolled.
//
// This is the server-side executor for Empowered Spell metamagic. The spec
// lets the sorcerer pick which dice to reroll, but the canonical "always pick
// the worst" choice is what an empowered cast resolves to whenever the
// interactive prompt is forfeited (or when the AoE damage roll fires after
// every per-target save has resolved). SR-025.
func RerollLowestDice(rolls []int, faces, count int, randFn func(int) int) []int {
	out := make([]int, len(rolls))
	copy(out, rolls)
	if count <= 0 || len(out) == 0 {
		return out
	}
	if count > len(out) {
		count = len(out)
	}
	// Build (value, index) tuples, stable-sort by value ascending, take
	// first `count` indices — those are the dice to reroll. n is small
	// (CHA mod, ≤ 5) so a tiny insertion sort is preferred over
	// sort.SliceStable's allocations.
	type indexed struct {
		val, idx int
	}
	pairs := make([]indexed, len(out))
	for i, v := range out {
		pairs[i] = indexed{val: v, idx: i}
	}
	for i := 1; i < len(pairs); i++ {
		for j := i; j > 0 && pairs[j-1].val > pairs[j].val; j-- {
			pairs[j-1], pairs[j] = pairs[j], pairs[j-1]
		}
	}
	for i := 0; i < count; i++ {
		out[pairs[i].idx] = randFn(faces)
	}
	return out
}
