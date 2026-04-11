package combat

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestUUIDToInt64_Deterministic(t *testing.T) {
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	h1 := UUIDToInt64(id)
	h2 := UUIDToInt64(id)
	assert.Equal(t, h1, h2, "same UUID should produce same hash")
}

func TestUUIDToInt64_ZeroUUID(t *testing.T) {
	id := uuid.UUID{}
	assert.Equal(t, int64(0), UUIDToInt64(id))
}

func TestUUIDToInt64_DifferentUUIDs(t *testing.T) {
	id1 := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	id2 := uuid.MustParse("660e8400-e29b-41d4-a716-446655440000")
	assert.NotEqual(t, UUIDToInt64(id1), UUIDToInt64(id2), "different UUIDs should produce different hashes")
}

// --- Phase 105: per-encounter lock scoping ---
//
// Turn locks are keyed on turn_id, so two active encounters (which always
// have distinct current_turn_id values) receive independent advisory-lock
// keys. This test proves the scoping property in isolation: rapid commands
// in encounter A and encounter B produce distinct lock keys and therefore
// cannot block each other at the Postgres advisory-lock layer.
func TestPhase105_TurnLocksAreScopedPerEncounter(t *testing.T) {
	encounterATurn := uuid.New()
	encounterBTurn := uuid.New()

	keyA := UUIDToInt64(encounterATurn)
	keyB := UUIDToInt64(encounterBTurn)

	assert.NotEqual(t, keyA, keyB,
		"distinct turn_ids from different encounters must produce distinct advisory-lock keys so commands in simultaneous encounters do not block each other")

	// Same turn_id always yields the same key (stable per-turn serialization
	// within a single encounter).
	assert.Equal(t, keyA, UUIDToInt64(encounterATurn))
	assert.Equal(t, keyB, UUIDToInt64(encounterBTurn))
}
