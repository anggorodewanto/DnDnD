package status

import (
	"fmt"
	"strings"
)

// ConditionEntry represents a single active condition.
type ConditionEntry struct {
	Name            string
	RemainingRounds int
}

// Info holds all the data needed to render a /status response.
type Info struct {
	CharacterName string
	ShortID       string

	// Combat-specific state
	Conditions      []ConditionEntry
	Concentration   string // spell name, empty if none
	TempHP          int
	ExhaustionLevel int

	// Class-specific state
	IsRaging             bool
	RageRoundsRemaining  int
	IsWildShaped         bool
	WildShapeCreature    string
	BardicInspirationDie string
	BardicInspirationSrc string
	KiCurrent            int
	KiMax                int
	HasKi                bool
	SorceryCurrent       int
	SorceryMax           int
	HasSorcery           bool
	ChannelDivinityCurrent int
	ChannelDivinityMax     int
	HasChannelDivinity     bool
	SmiteSlots             string // formatted "1st: 3/4 | 2nd: 1/2"

	// Reaction declarations and readied actions
	Reactions      []string
	ReadiedActions []string
}

// FormatStatus renders an Info into a Discord-friendly status message.
// Sections with no data are omitted entirely.
func FormatStatus(info Info) string {
	header := fmt.Sprintf("**Status — %s**", info.CharacterName)
	if info.ShortID != "" {
		header = fmt.Sprintf("**Status — %s (%s)**", info.CharacterName, info.ShortID)
	}

	var sections []string

	if len(info.Conditions) > 0 {
		var parts []string
		for _, c := range info.Conditions {
			if c.RemainingRounds > 0 {
				parts = append(parts, fmt.Sprintf("%s (%d rounds remaining)", c.Name, c.RemainingRounds))
			} else {
				parts = append(parts, c.Name)
			}
		}
		sections = append(sections, "**Conditions:** "+strings.Join(parts, ", "))
	}

	if info.Concentration != "" {
		sections = append(sections, "**Concentration:** "+info.Concentration)
	}

	if info.TempHP > 0 {
		sections = append(sections, fmt.Sprintf("**Temp HP:** %d", info.TempHP))
	}

	if info.ExhaustionLevel > 0 {
		sections = append(sections, fmt.Sprintf("**Exhaustion:** Level %d", info.ExhaustionLevel))
	}

	if info.IsRaging {
		if info.RageRoundsRemaining > 0 {
			sections = append(sections, fmt.Sprintf("**Rage:** Active (%d rounds remaining)", info.RageRoundsRemaining))
		} else {
			sections = append(sections, "**Rage:** Active")
		}
	}

	if info.IsWildShaped {
		label := "Active"
		if info.WildShapeCreature != "" {
			label = info.WildShapeCreature
		}
		sections = append(sections, "**Wild Shape:** "+label)
	}

	if info.BardicInspirationDie != "" {
		line := "**Bardic Inspiration:** " + info.BardicInspirationDie
		if info.BardicInspirationSrc != "" {
			line += " (from " + info.BardicInspirationSrc + ")"
		}
		sections = append(sections, line)
	}

	if info.HasKi {
		sections = append(sections, fmt.Sprintf("**Ki Points:** %d/%d", info.KiCurrent, info.KiMax))
	}

	if info.HasSorcery {
		sections = append(sections, fmt.Sprintf("**Sorcery Points:** %d/%d", info.SorceryCurrent, info.SorceryMax))
	}

	if info.HasChannelDivinity {
		sections = append(sections, fmt.Sprintf("**Channel Divinity:** %d/%d", info.ChannelDivinityCurrent, info.ChannelDivinityMax))
	}

	if info.SmiteSlots != "" {
		sections = append(sections, "**Smite Slots:** "+info.SmiteSlots)
	}

	if len(info.Reactions) > 0 {
		sections = append(sections, "**Reaction Declarations:** "+quotedList(info.Reactions))
	}

	if len(info.ReadiedActions) > 0 {
		sections = append(sections, "**Readied Actions:** "+quotedList(info.ReadiedActions))
	}

	if len(sections) == 0 {
		return header + "\n\nNo active effects."
	}

	return header + "\n\n" + strings.Join(sections, "\n")
}

// quotedList formats a slice of strings as a comma-separated list of quoted values.
func quotedList(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(quoted, ", ")
}
