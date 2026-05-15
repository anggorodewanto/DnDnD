package dashboard

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

func TestDBApprovalStore_ResubmitToPending_FromChangesRequested(t *testing.T) {
	id := uuid.New()
	fq := &fakeQueries{
		getPC:    refdata.PlayerCharacter{ID: id, Status: "changes_requested"},
		updatePC: refdata.PlayerCharacter{ID: id, Status: "pending"},
	}
	store := NewDBApprovalStore(fq)
	err := store.ResubmitToPending(t.Context(), id)
	require.NoError(t, err)
	assert.Equal(t, "pending", fq.updateParams.Status)
}

func TestDBApprovalStore_ResubmitToPending_FromRejected(t *testing.T) {
	id := uuid.New()
	fq := &fakeQueries{
		getPC:    refdata.PlayerCharacter{ID: id, Status: "rejected"},
		updatePC: refdata.PlayerCharacter{ID: id, Status: "pending"},
	}
	store := NewDBApprovalStore(fq)
	err := store.ResubmitToPending(t.Context(), id)
	require.NoError(t, err)
	assert.Equal(t, "pending", fq.updateParams.Status)
}
