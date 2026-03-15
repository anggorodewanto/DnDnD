package combat

import (
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/refdata"
)

// CheckUnusedResources returns a list of formatted strings describing
// resources that haven't been used yet this turn. Only includes
// "significant" resources (attacks, bonus action) not reaction/free interact.
func CheckUnusedResources(turn refdata.Turn) []string {
	var unused []string

	if !turn.ActionUsed && turn.AttacksRemaining == 0 {
		unused = append(unused, "\U0001f4a5 Action")
	}

	if turn.AttacksRemaining > 0 {
		if turn.AttacksRemaining == 1 {
			unused = append(unused, "\u2694\ufe0f 1 attack")
		} else {
			unused = append(unused, fmt.Sprintf("\u2694\ufe0f %d attacks", turn.AttacksRemaining))
		}
	}

	if !turn.BonusActionUsed {
		unused = append(unused, "\U0001f381 Bonus action")
	}

	return unused
}

// FormatUnusedResourcesWarning formats the confirmation prompt shown when
// a player has unused resources. Returns empty string if no unused resources.
func FormatUnusedResourcesWarning(unused []string) string {
	if len(unused) == 0 {
		return ""
	}
	return fmt.Sprintf("\u26a0\ufe0f You still have: %s. End turn?", strings.Join(unused, " | "))
}
