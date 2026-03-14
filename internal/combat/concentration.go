package combat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/refdata"
)

// ConcentrationCheckDC computes the DC for a concentration saving throw
// when a concentrating caster takes damage: max(10, floor(damage / 2)).
func ConcentrationCheckDC(damage int) int {
	half := damage / 2
	if half > 10 {
		return half
	}
	return 10
}

// ConcentrationCheckResult describes whether a concentration save is needed
// after a concentrating caster takes damage.
type ConcentrationCheckResult struct {
	NeedsSave bool
	DC        int
	SpellName string
}

// CheckConcentrationOnDamage determines if a concentration save is required
// when a caster takes damage. No save is needed if not concentrating or damage is 0.
func CheckConcentrationOnDamage(currentConcentration string, damage int) ConcentrationCheckResult {
	if currentConcentration == "" || damage <= 0 {
		return ConcentrationCheckResult{}
	}
	return ConcentrationCheckResult{
		NeedsSave: true,
		DC:        ConcentrationCheckDC(damage),
		SpellName: currentConcentration,
	}
}

// ConcentrationBreakResult describes the outcome of a concentration break check.
type ConcentrationBreakResult struct {
	Broken    bool
	SpellName string
	Reason    string
}

// CheckConcentrationOnIncapacitation checks if an incapacitating condition
// should auto-break concentration (no save allowed).
func CheckConcentrationOnIncapacitation(currentConcentration string, conditions []CombatCondition) ConcentrationBreakResult {
	if currentConcentration == "" {
		return ConcentrationBreakResult{}
	}
	for _, c := range conditions {
		if incapacitatingConditions[c.Condition] {
			return ConcentrationBreakResult{
				Broken:    true,
				SpellName: currentConcentration,
				Reason:    c.Condition,
			}
		}
	}
	return ConcentrationBreakResult{}
}

// CheckConcentrationOnIncapacitationRaw checks incapacitation from raw JSON conditions.
func CheckConcentrationOnIncapacitationRaw(currentConcentration string, conditions json.RawMessage) ConcentrationBreakResult {
	conds, err := parseConditions(conditions)
	if err != nil {
		return ConcentrationBreakResult{}
	}
	return CheckConcentrationOnIncapacitation(currentConcentration, conds)
}

// HasVerbalOrSomaticComponent returns true if the spell has V or S components.
func HasVerbalOrSomaticComponent(spell refdata.Spell) bool {
	for _, c := range spell.Components {
		if c == "V" || c == "S" {
			return true
		}
	}
	return false
}

// ValidateSilenceZone checks whether a spell can be cast in a silence zone.
// Spells with V or S components cannot be cast in silence.
func ValidateSilenceZone(inSilence bool, spell refdata.Spell) error {
	if !inSilence {
		return nil
	}
	if HasVerbalOrSomaticComponent(spell) {
		return fmt.Errorf("You cannot cast %s — you are inside a zone of Silence (requires verbal/somatic components).", spell.Name)
	}
	return nil
}

// CheckConcentrationInSilence checks if being in a silence zone breaks
// concentration on a spell with V or S components.
func CheckConcentrationInSilence(currentConcentration string, inSilence bool, spell refdata.Spell) ConcentrationBreakResult {
	if currentConcentration == "" || !inSilence || !HasVerbalOrSomaticComponent(spell) {
		return ConcentrationBreakResult{}
	}
	return ConcentrationBreakResult{
		Broken:    true,
		SpellName: currentConcentration,
		Reason:    "silence",
	}
}

// ConcentrationCleanupFunc is a callback invoked when concentration breaks
// to clean up active spell effects (zones, conditions applied by the spell, etc.).
type ConcentrationCleanupFunc func(spellName string)

// FullConcentrationBreakResult includes the break result plus a formatted log message.
type FullConcentrationBreakResult struct {
	Broken    bool
	SpellName string
	Reason    string
	Message   string
}

// BreakConcentration breaks concentration on a spell, invokes the cleanup callback,
// and returns the formatted log message.
func BreakConcentration(casterName, spellName, reason string, cleanup ConcentrationCleanupFunc) FullConcentrationBreakResult {
	if cleanup != nil {
		cleanup(spellName)
	}
	msg := FormatConcentrationBreakLog(casterName, spellName, reason)
	return FullConcentrationBreakResult{
		Broken:    true,
		SpellName: spellName,
		Reason:    reason,
		Message:   msg,
	}
}

// FormatConcentrationBreakLog produces the combat log message for concentration loss.
// Uses "drops" for voluntary changes (new spell), "loses" for forced breaks.
func FormatConcentrationBreakLog(casterName, spellName, reason string) string {
	verb := "loses"
	if strings.HasPrefix(reason, "cast new concentration spell") {
		verb = "drops"
	}
	return fmt.Sprintf("🔮 %s %s concentration on %s (%s)", casterName, verb, spellName, reason)
}
