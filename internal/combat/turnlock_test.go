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
