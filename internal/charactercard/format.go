package charactercard

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ab/dndnd/internal/character"
)

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
	b.WriteString(formatSpellSlots(d.SpellSlots))
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

func formatSpellSlots(slots map[string]character.SlotInfo) string {
	if len(slots) == 0 {
		return "—"
	}

	// Sort by level
	keys := make([]string, 0, len(slots))
	for k := range slots {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		s := slots[k]
		parts = append(parts, fmt.Sprintf("%s: %d/%d", slotOrdinal(k), s.Current, s.Max))
	}
	return strings.Join(parts, " | ")
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
