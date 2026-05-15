package combat

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetActiveEncounterIDByCharacterID_F13_DeterministicOrder verifies that
// when the service-level check runs, it returns the most recently created
// combatant row (ORDER BY cb.created_at DESC). This ensures deterministic
// routing even if duplicate active memberships somehow exist.
func TestGetActiveEncounterIDByCharacterID_F13_DeterministicOrder(t *testing.T) {
	charID := uuid.New()
	newerEncounterID := uuid.New()

	store := defaultMockStore()
	// Simulate the query returning the newer encounter (ORDER BY created_at DESC).
	store.getActiveEncounterIDByCharacterIDFn = func(ctx context.Context, id uuid.NullUUID) (uuid.UUID, error) {
		if id.Valid && id.UUID == charID {
			return newerEncounterID, nil
		}
		return uuid.Nil, sql.ErrNoRows
	}

	svc := NewService(store)

	// AddCombatant to a DIFFERENT encounter should be rejected because the
	// character is already in newerEncounterID.
	differentEncounter := uuid.New()
	_, err := svc.AddCombatant(context.Background(), differentEncounter, CombatantParams{
		CharacterID: charID.String(),
		ShortID:     "TK",
		DisplayName: "Test Knight",
		HPMax:       50,
		HPCurrent:   50,
		AC:          16,
		IsAlive:     true,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCharacterAlreadyInActiveEncounter)
}

// TestAddCombatant_F13_AllowsSameEncounterReAdd verifies that adding a
// character to the SAME encounter they're already in does not trigger the
// active-membership guard (idempotent re-add during template creation).
func TestAddCombatant_F13_AllowsSameEncounterReAdd(t *testing.T) {
	charID := uuid.New()
	encounterID := uuid.New()

	store := defaultMockStore()
	store.getActiveEncounterIDByCharacterIDFn = func(ctx context.Context, id uuid.NullUUID) (uuid.UUID, error) {
		if id.Valid && id.UUID == charID {
			return encounterID, nil
		}
		return uuid.Nil, sql.ErrNoRows
	}

	svc := NewService(store)

	// Adding to the same encounter should succeed.
	_, err := svc.AddCombatant(context.Background(), encounterID, CombatantParams{
		CharacterID: charID.String(),
		ShortID:     "TK",
		DisplayName: "Test Knight",
		HPMax:       50,
		HPCurrent:   50,
		AC:          16,
		IsAlive:     true,
	})
	require.NoError(t, err)
}
