package rest

import (
	"fmt"
	"strings"
)

// InterruptResult describes the outcome of a rest interruption.
type InterruptResult struct {
	// Benefits is "none" or "short" (short rest benefits granted for long rest with >= 1 hour).
	Benefits string
}

// InterruptRest determines what benefits to grant when a rest is interrupted.
// Per 5e rules: short rest interrupted = no benefits; long rest interrupted with
// >= 1 hour elapsed = short rest benefits; otherwise no benefits.
func InterruptRest(restType string, oneHourElapsed bool) InterruptResult {
	if restType == "long" && oneHourElapsed {
		return InterruptResult{Benefits: "short"}
	}
	return InterruptResult{Benefits: "none"}
}

// FormatPartyRestSummary formats a party rest summary for #roll-history.
func FormatPartyRestSummary(restType string, rested, excluded []string) string {
	var b strings.Builder

	typeLabel := "Short"
	if restType == "long" {
		typeLabel = "Long"
	}

	fmt.Fprintf(&b, "🛏️ Party %s Rest — ", typeLabel)
	fmt.Fprintf(&b, "%s rested.", strings.Join(rested, ", "))

	if len(excluded) > 0 {
		fmt.Fprintf(&b, " %s kept watch.", strings.Join(excluded, ", "))
	}

	return b.String()
}

// FormatInterruptNotification formats an interruption notification for a player.
func FormatInterruptNotification(charName, restType, reason string, benefits bool) string {
	var b strings.Builder

	fmt.Fprintf(&b, "⚠️ Your %s rest was interrupted — %s. ", restType, reason)

	if benefits {
		b.WriteString("You receive short rest benefits.")
	} else {
		b.WriteString("No benefits granted.")
	}

	return b.String()
}
