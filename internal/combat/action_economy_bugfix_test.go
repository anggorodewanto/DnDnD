package combat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- Bug 1: a full Action must also consume the Attack action's attacks ---
//
// UseAttack couples one way (spending an attack marks ActionUsed), but
// UseResource(ResourceAction) left AttacksRemaining untouched. Because
// Service.Attack gates only on ResourceAttack (AttacksRemaining > 0) and never
// reads ActionUsed, every full-Action alternative — Dodge, Dash, Disengage,
// Help, Hide, Ready, Stabilize, Escape, a freeform action — handed the actor a
// completely free Attack action afterwards.

func TestUseResource_Action_ZeroesAttacksRemaining(t *testing.T) {
	turn := refdata.Turn{AttacksRemaining: 2}

	updated, err := UseResource(turn, ResourceAction)

	require.NoError(t, err)
	assert.True(t, updated.ActionUsed)
	assert.Equal(t, int32(0), updated.AttacksRemaining,
		"spending the Action must consume the Attack action's attacks too")
}

func TestUseResource_NonActionResources_LeaveAttacksAlone(t *testing.T) {
	for _, resource := range []ResourceType{ResourceBonusAction, ResourceReaction, ResourceFreeInteract} {
		turn := refdata.Turn{AttacksRemaining: 2}

		updated, err := UseResource(turn, resource)

		require.NoError(t, err)
		assert.Equal(t, int32(2), updated.AttacksRemaining,
			"%s must not touch AttacksRemaining", resource)
	}
}

// A failed spend must not silently strip the attacks either.
func TestUseResource_ActionAlreadySpent_DoesNotZeroAttacks(t *testing.T) {
	turn := refdata.Turn{ActionUsed: true, AttacksRemaining: 2}

	updated, err := UseResource(turn, ResourceAction)

	assert.ErrorIs(t, err, ErrResourceSpent)
	assert.Equal(t, int32(2), updated.AttacksRemaining)
}
