package rest

import (
	"fmt"
	"sort"
	"strings"
)

// FormatShortRestResult formats a short rest result for Discord display.
func FormatShortRestResult(charName string, result ShortRestResult) string {
	var b strings.Builder

	fmt.Fprintf(&b, "**%s** — Short Rest Complete\n", charName)

	if result.HPHealed > 0 {
		fmt.Fprintf(&b, "> Healed **%d HP** (%d → %d/%d)\n", result.HPHealed, result.HPBefore, result.HPAfter, result.HPMax)
		for _, roll := range result.HitDieRolls {
			fmt.Fprintf(&b, "> • %s: rolled %d + %d CON = %d HP\n", roll.DieType, roll.Rolled, roll.CONMod, roll.Healed)
		}
	} else {
		fmt.Fprintf(&b, "> No hit dice spent. HP: %d/%d\n", result.HPAfter, result.HPMax)
	}

	if len(result.FeaturesRecharged) > 0 {
		sort.Strings(result.FeaturesRecharged)
		fmt.Fprintf(&b, "> Features recharged: %s\n", strings.Join(result.FeaturesRecharged, ", "))
	}

	if result.PactSlotsRestored {
		fmt.Fprintf(&b, "> Pact magic slots restored to %d\n", result.PactSlotsCurrent)
	}

	return b.String()
}

// FormatLongRestResult formats a long rest result for Discord display.
func FormatLongRestResult(charName string, result LongRestResult) string {
	var b strings.Builder

	fmt.Fprintf(&b, "**%s** — Long Rest Complete\n", charName)
	fmt.Fprintf(&b, "> HP fully restored: **%d/%d HP**\n", result.HPAfter, result.HPMax)

	if result.HitDiceRestored > 0 {
		fmt.Fprintf(&b, "> Hit dice restored: +%d\n", result.HitDiceRestored)
	}

	if len(result.SpellSlots) > 0 {
		b.WriteString("> All spell slots restored\n")
	}

	if result.PactSlotsRestored {
		b.WriteString("> Pact magic slots restored\n")
	}

	if len(result.FeaturesRecharged) > 0 {
		sort.Strings(result.FeaturesRecharged)
		fmt.Fprintf(&b, "> Features recharged: %s\n", strings.Join(result.FeaturesRecharged, ", "))
	}

	if result.DeathSavesReset {
		b.WriteString("> Death save tallies reset\n")
	}

	if result.PreparedCasterReminder {
		b.WriteString("> You can change your prepared spells with `/prepare`.\n")
	}

	return b.String()
}
