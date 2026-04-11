package combat

import (
	"testing"

	"github.com/ab/dndnd/internal/refdata"
)

// TestNewStoreAdapter_SatisfiesStore ensures the shared adapter wrapping
// *refdata.Queries satisfies the combat.Store interface. This is a compile-time
// check: if the assignment compiles, the adapter bridges the positional-arg
// inventory/gold methods onto sqlc's struct-param convention correctly.
func TestNewStoreAdapter_SatisfiesStore(t *testing.T) {
	var q *refdata.Queries
	var s Store = NewStoreAdapter(q)
	_ = s
}
