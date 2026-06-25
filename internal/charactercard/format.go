package charactercard

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ab/dndnd/internal/character"
)

// appearanceMaxRunes caps the public card's appearance line so cards stay lean
// (a one-line teaser, not a backstory).
const appearanceMaxRunes = 100

// ConditionInfo describes an active condition on a character.
type ConditionInfo struct {
	Name            string `json:"name"`
	RemainingRounds int    `json:"remaining_rounds,omitempty"`
}

// CardData holds all data needed to render a character card.
type CardData struct {
	Name               string
	ShortID            string
	Level              int
	Race               string
	Appearance         string
	Classes            []character.ClassEntry
	HpCurrent          int
	HpMax              int
	TempHP             int
	AC                 int
	SpeedFt            int
	AbilityScores      character.AbilityScores
	EquippedMainHand   string
	EquippedOffHand    string
	SpellSlots         map[string]character.SlotInfo
	PactMagicSlots     *character.PactMagicSlots
	Conditions         []ConditionInfo
	Concentration      string
	Exhaustion         int
	Gold               int
	Languages          []string
	SpellCount         int
	PreparedCount      int
	HomebrewSpellCount int
	Retired            bool
	ASIFeatPending     bool
}

// FormatCard produces the formatted character card string per the spec.
func FormatCard(d CardData) string {
	var b strings.Builder

	// Header line
	classStr := character.FormatClassSummary(d.Classes)
	header := fmt.Sprintf("⚔️ %s (%s) — Level %d %s %s", d.Name, d.ShortID, d.Level, d.Race, classStr)
	if d.Retired {
		header = "🏴 RETIRED — " + header
	}
	b.WriteString(header)
	b.WriteByte('\n')
	if d.ASIFeatPending {
		b.WriteString("⏳ ASI/Feat pending\n")
	}

	// Appearance (one short line, near the top; omitted when blank)
	if appearance := oneLine(d.Appearance, appearanceMaxRunes); appearance != "" {
		fmt.Fprintf(&b, "Appearance: %s\n", appearance)
	}

	// HP line
	hpStr := fmt.Sprintf("HP: %d/%d", d.HpCurrent, d.HpMax)
	if d.TempHP > 0 {
		hpStr += fmt.Sprintf(" (+%d temp)", d.TempHP)
	}
	fmt.Fprintf(&b, "%s | AC: %d | Speed: %dft\n", hpStr, d.AC, d.SpeedFt)

	// Ability scores
	fmt.Fprintf(&b, "STR %d | DEX %d | CON %d | WIS %d | INT %d | CHA %d\n",
		d.AbilityScores.STR, d.AbilityScores.DEX, d.AbilityScores.CON,
		d.AbilityScores.WIS, d.AbilityScores.INT, d.AbilityScores.CHA)

	// Equipped
	b.WriteString("Equipped: ")
	b.WriteString(formatEquipped(d.EquippedMainHand, d.EquippedOffHand))
	b.WriteByte('\n')

	// Spell slots
	b.WriteString("Spell Slots: ")
	b.WriteString(formatSpellSlots(d.SpellSlots, d.PactMagicSlots))
	b.WriteByte('\n')

	// Spell count
	if d.SpellCount > 0 {
		b.WriteString("Spells: ")
		if d.PreparedCount > 0 {
			fmt.Fprintf(&b, "%d prepared / %d known", d.PreparedCount, d.SpellCount)
		} else {
			fmt.Fprintf(&b, "%d known", d.SpellCount)
		}
		if d.HomebrewSpellCount > 0 {
			fmt.Fprintf(&b, " (%d homebrew/off-list)", d.HomebrewSpellCount)
		}
		b.WriteByte('\n')
	}

	// Conditions
	b.WriteString("Conditions: ")
	b.WriteString(formatConditions(d.Conditions))
	b.WriteByte('\n')

	// Concentration
	b.WriteString("Concentration: ")
	if d.Concentration == "" {
		b.WriteString("—")
	} else {
		b.WriteString(d.Concentration)
	}
	b.WriteByte('\n')

	if d.Exhaustion > 0 {
		fmt.Fprintf(&b, "Exhaustion: %d\n", d.Exhaustion)
	}

	fmt.Fprintf(&b, "Gold: %dgp\n", d.Gold)

	// Languages
	b.WriteString("Languages: ")
	b.WriteString(strings.Join(d.Languages, ", "))

	return b.String()
}

// oneLine flattens s to a single line — newlines, tabs, and runs of spaces
// collapse to single spaces, leading/trailing whitespace is trimmed — and
// truncates it to max runes, appending "…" when it had to cut. An all-blank or
// empty input yields "".
func oneLine(s string, max int) string {
	flat := strings.Join(strings.Fields(s), " ")
	runes := []rune(flat)
	if len(runes) <= max {
		return flat
	}
	return string(runes[:max]) + "…"
}

func formatEquipped(main, off string) string {
	if main == "" && off == "" {
		return "—"
	}
	var parts []string
	if main != "" {
		parts = append(parts, fmt.Sprintf("%s (main)", main))
	}
	if off != "" {
		parts = append(parts, fmt.Sprintf("%s (off-hand)", off))
	}
	return strings.Join(parts, " | ")
}

// slotOrdinal converts a slot level number to its ordinal string.
func slotOrdinal(level string) string {
	switch level {
	case "1":
		return "1st"
	case "2":
		return "2nd"
	case "3":
		return "3rd"
	default:
		return level + "th"
	}
}

// formatSpellSlots renders the standard spell slots followed by any pact-magic
// slots. A multiclass caster with both shows both, joined by " | ". A pure
// warlock (empty standard slots, pact slots present) shows only the pact line.
// Non-casters / empty render the canonical "—".
func formatSpellSlots(slots map[string]character.SlotInfo, pact *character.PactMagicSlots) string {
	parts := make([]string, 0, len(slots)+1)

	if len(slots) > 0 {
		// Sort by level numerically (not lexicographically)
		keys := make([]string, 0, len(slots))
		for k := range slots {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			a, _ := strconv.Atoi(keys[i])
			b, _ := strconv.Atoi(keys[j])
			return a < b
		})
		for _, k := range keys {
			s := slots[k]
			parts = append(parts, fmt.Sprintf("%s: %d/%d", slotOrdinal(k), s.Current, s.Max))
		}
	}

	if pactLine := formatPactMagicSlots(pact); pactLine != "" {
		parts = append(parts, pactLine)
	}

	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, " | ")
}

// formatPactMagicSlots renders a warlock's pact-magic slots (e.g.
// "Pact Magic: 2 × Lvl 2") or "" when there are none. Pact slots are always all
// the same level, so a single "count × level" line captures them.
func formatPactMagicSlots(pact *character.PactMagicSlots) string {
	if pact == nil || pact.Max == 0 {
		return ""
	}
	return fmt.Sprintf("Pact Magic: %d × Lvl %d", pact.Current, pact.SlotLevel)
}

func formatConditions(conditions []ConditionInfo) string {
	if len(conditions) == 0 {
		return "—"
	}
	parts := make([]string, 0, len(conditions))
	for _, c := range conditions {
		s := c.Name
		if c.RemainingRounds > 0 {
			s += fmt.Sprintf(" (%d rounds remaining)", c.RemainingRounds)
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}
