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
	HpCurrent       int
	HpMax           int    // HP section renders only when > 0
	PositionCol     string // grid column letter; position renders only when non-empty
	PositionRow     int
	Conditions      []ConditionEntry
	Concentration   string // spell name, empty if none
	TempHP          int
	ExhaustionLevel int

	// Class-specific state
	IsRaging               bool
	RageRoundsRemaining    int
	IsWildShaped           bool
	WildShapeCreature      string
	BardicInspirationDie   string
	BardicInspirationSrc   string
	KiCurrent              int
	KiMax                  int
	HasKi                  bool
	SorceryCurrent         int
	SorceryMax             int
	HasSorcery             bool
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

	if info.HpMax > 0 {
		sections = append(sections, fmt.Sprintf("**HP:** %d/%d", info.HpCurrent, info.HpMax))
	}

	if info.PositionCol != "" {
		sections = append(sections, fmt.Sprintf("**Position:** %s%d", info.PositionCol, info.PositionRow))
	}

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
		sections = append(sections, formatRageStatus(info.RageRoundsRemaining))
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

// rageCapWarningRounds is how close the 2024 Rage 10-minute hard cap has to be
// before /status bothers to print it.
const rageCapWarningRounds = 10

// formatRageStatus renders the Rage line. Under 2024 rules a Rage's real clock
// is the turn-by-turn sustain check — attack, force a save, spend a Bonus
// Action, or take damage — not the 100-round (10 minute) hard cap, so quoting
// the cap up front ("100 rounds remaining") buries the thing the player can
// actually act on. The cap is surfaced only once it is near enough to bind.
func formatRageStatus(roundsRemaining int) string {
	if roundsRemaining > 0 && roundsRemaining <= rageCapWarningRounds {
		return fmt.Sprintf("**Rage:** Active (%d rounds remaining)", roundsRemaining)
	}
	return "**Rage:** Active — sustain it each turn (attack, force a save, or `/bonus rage`)"
}

// quotedList formats a slice of strings as a comma-separated list of quoted values.
func quotedList(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(quoted, ", ")
}
