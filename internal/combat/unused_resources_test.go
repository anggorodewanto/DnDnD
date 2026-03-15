package combat

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/refdata"
)

func TestCheckUnusedResources_AllSpent(t *testing.T) {
	turn := refdata.Turn{
		ActionUsed:          true,
		BonusActionUsed:     true,
		ReactionUsed:        true,
		FreeInteractUsed:    true,
		MovementRemainingFt: 0,
		AttacksRemaining:    0,
	}
	unused := CheckUnusedResources(turn)
	assert.Empty(t, unused)
}

func TestCheckUnusedResources_HasAttacks(t *testing.T) {
	turn := refdata.Turn{
		ActionUsed:       true,
		BonusActionUsed:  true,
		AttacksRemaining: 1,
	}
	unused := CheckUnusedResources(turn)
	assert.Contains(t, unused, "\u2694\ufe0f 1 attack")
}

func TestCheckUnusedResources_HasBonusAction(t *testing.T) {
	turn := refdata.Turn{
		ActionUsed:      true,
		BonusActionUsed: false,
	}
	unused := CheckUnusedResources(turn)
	assert.Contains(t, unused, "\U0001f381 Bonus action")
}

func TestCheckUnusedResources_MultipleUnused(t *testing.T) {
	turn := refdata.Turn{
		ActionUsed:          false,
		BonusActionUsed:     false,
		AttacksRemaining:    2,
		MovementRemainingFt: 30,
	}
	unused := CheckUnusedResources(turn)
	assert.True(t, len(unused) >= 2)
}

func TestFormatUnusedResourcesWarning_Empty(t *testing.T) {
	msg := FormatUnusedResourcesWarning(nil)
	assert.Equal(t, "", msg)
}

func TestFormatUnusedResourcesWarning_HasResources(t *testing.T) {
	unused := []string{"\u2694\ufe0f 1 attack", "\U0001f381 Bonus action"}
	msg := FormatUnusedResourcesWarning(unused)
	assert.Contains(t, msg, "You still have")
	assert.Contains(t, msg, "1 attack")
	assert.Contains(t, msg, "Bonus action")
	assert.Contains(t, msg, "End turn?")
}
