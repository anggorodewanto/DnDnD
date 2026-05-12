package combat

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingCardUpdater records OnCharacterUpdated calls so SR-007 tests can
// assert the post-mutation fan-out fires exactly once per success path.
type recordingCardUpdater struct {
	calls []uuid.UUID
	err   error
}

func (r *recordingCardUpdater) OnCharacterUpdated(ctx context.Context, characterID uuid.UUID) error {
	r.calls = append(r.calls, characterID)
	return r.err
}

// SR-007: combat.Service.Equip MUST fire the CardUpdater hook so the
// persistent #character-cards message stays in sync when a player /equip's
// gear out of combat (or in combat, after Phase 17 wired the in-combat
// channel via combatant mutations).
func TestEquip_FiresCardUpdater_OnSuccess(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()

	char := makeEquipChar(charID)
	ms.getWeaponFn = longswordFn
	setupEquipMock(ms, char)

	svc := NewService(ms)
	rec := &recordingCardUpdater{}
	svc.SetCardUpdater(rec)

	_, err := svc.Equip(context.Background(), EquipCommand{
		Character: char,
		ItemName:  "longsword",
	})
	require.NoError(t, err)

	require.Len(t, rec.calls, 1, "expected exactly one OnCharacterUpdated call")
	assert.Equal(t, charID, rec.calls[0])
}
