package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/sqlc-dev/pqtype"
)

// fakeQueries is a test double that implements the methods DBApprovalStore needs.
type fakeQueries struct {
	pendingRows  []refdata.ListPlayerCharactersAwaitingApprovalRow
	pendingErr   error
	detailRow    refdata.GetPlayerCharacterWithCharacterRow
	detailErr    error
	getChar      refdata.Character
	getCharErr   error
	getPC        refdata.PlayerCharacter
	getPCErr     error
	updatePC     refdata.PlayerCharacter
	updateErr    error
	updateParams refdata.UpdatePlayerCharacterStatusParams
	// review-baseline clear capture (approve path).
	clearReviewCalled bool
	clearReviewID     uuid.UUID
	clearReviewErr    error
}

func (f *fakeQueries) ListPlayerCharactersAwaitingApproval(_ context.Context, _ uuid.UUID) ([]refdata.ListPlayerCharactersAwaitingApprovalRow, error) {
	return f.pendingRows, f.pendingErr
}

func (f *fakeQueries) GetPlayerCharacterWithCharacter(_ context.Context, _ uuid.UUID) (refdata.GetPlayerCharacterWithCharacterRow, error) {
	return f.detailRow, f.detailErr
}

func (f *fakeQueries) GetCharacter(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
	return f.getChar, f.getCharErr
}

func (f *fakeQueries) GetPlayerCharacter(_ context.Context, _ uuid.UUID) (refdata.PlayerCharacter, error) {
	return f.getPC, f.getPCErr
}

func (f *fakeQueries) ClearPlayerCharacterReviewBefore(_ context.Context, id uuid.UUID) error {
	f.clearReviewCalled = true
	f.clearReviewID = id
	return f.clearReviewErr
}

func (f *fakeQueries) UpdatePlayerCharacterStatus(_ context.Context, arg refdata.UpdatePlayerCharacterStatusParams) (refdata.PlayerCharacter, error) {
	f.updateParams = arg
	return f.updatePC, f.updateErr
}

func TestDBApprovalStore_GetApprovalDetail_IncludesReviewAndBaseline(t *testing.T) {
	id := uuid.New()
	charID := uuid.New()
	campaignID := uuid.New()
	baseline := json.RawMessage(`{"name":"Thorin","ac":16}`)

	fq := &fakeQueries{
		detailRow: refdata.GetPlayerCharacterWithCharacterRow{
			ID:            id,
			CampaignID:    campaignID,
			CharacterID:   charID,
			CharacterName: "Thorin",
			Status:        "pending",
			ReviewBefore:  pqtype.NullRawMessage{RawMessage: baseline, Valid: true},
		},
		getChar: refdata.Character{
			Name:          "Thorin",
			Race:          "dwarf",
			AbilityScores: json.RawMessage(`{"str":16,"dex":12,"con":15,"int":10,"wis":12,"cha":8}`),
			Ac:            18,
		},
	}

	store := NewDBApprovalStore(fq)
	detail, err := store.GetApprovalDetail(context.Background(), id)
	require.NoError(t, err)
	// "after" projection reflects the current character state.
	require.NotNil(t, detail.Review)
	assert.Equal(t, "Thorin", detail.Review.Name)
	assert.Equal(t, int32(18), detail.Review.AC)
	// "before" baseline is passed through verbatim for the frontend diff.
	assert.JSONEq(t, string(baseline), string(detail.ReviewBefore))
}

func TestDBApprovalStore_GetApprovalDetail_NoBaselineForNewSubmission(t *testing.T) {
	id := uuid.New()
	fq := &fakeQueries{
		detailRow: refdata.GetPlayerCharacterWithCharacterRow{
			ID:            id,
			CharacterID:   uuid.New(),
			CharacterName: "Newbie",
			Status:        "pending",
			ReviewBefore:  pqtype.NullRawMessage{Valid: false},
		},
		getChar: refdata.Character{Name: "Newbie", AbilityScores: json.RawMessage(`{}`)},
	}

	store := NewDBApprovalStore(fq)
	detail, err := store.GetApprovalDetail(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, detail.Review)
	assert.Nil(t, detail.ReviewBefore)
}

func TestDBApprovalStore_GetApprovalDetail_CharacterLookupError(t *testing.T) {
	fq := &fakeQueries{
		detailRow:  refdata.GetPlayerCharacterWithCharacterRow{CharacterID: uuid.New()},
		getCharErr: fmt.Errorf("boom"),
	}
	store := NewDBApprovalStore(fq)
	_, err := store.GetApprovalDetail(context.Background(), uuid.New())
	assert.Error(t, err)
}

func TestDBApprovalStore_ApproveCharacter_ClearsReviewBaseline(t *testing.T) {
	id := uuid.New()
	fq := &fakeQueries{getPC: refdata.PlayerCharacter{ID: id, Status: "pending"}}
	store := NewDBApprovalStore(fq)

	err := store.ApproveCharacter(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "approved", fq.updateParams.Status)
	// Approve resets the baseline so the next edit re-snapshots from the new state.
	assert.True(t, fq.clearReviewCalled)
	assert.Equal(t, id, fq.clearReviewID)
}

func TestDBApprovalStore_RejectCharacter_KeepsReviewBaseline(t *testing.T) {
	id := uuid.New()
	fq := &fakeQueries{getPC: refdata.PlayerCharacter{ID: id, Status: "pending"}}
	store := NewDBApprovalStore(fq)

	err := store.RejectCharacter(context.Background(), id, "no")
	require.NoError(t, err)
	// Reject/request-changes must NOT clear the baseline so the diff survives resubmits.
	assert.False(t, fq.clearReviewCalled)
}

func TestDBApprovalStore_ListPendingApprovals(t *testing.T) {
	id := uuid.New()
	charID := uuid.New()
	campaignID := uuid.New()

	fq := &fakeQueries{
		pendingRows: []refdata.ListPlayerCharactersAwaitingApprovalRow{
			{
				ID:            id,
				CampaignID:    campaignID,
				CharacterID:   charID,
				DiscordUserID: "player1",
				Status:        "pending",
				CreatedVia:    "import",
				CharacterName: "Gandalf",
				Race:          "Human",
				Level:         5,
			},
		},
	}

	store := NewDBApprovalStore(fq)
	entries, err := store.ListPendingApprovals(context.Background(), campaignID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "Gandalf", entries[0].CharacterName)
	assert.Equal(t, "pending", entries[0].Status)
	assert.Equal(t, "player1", entries[0].DiscordUserID)
}

func TestDBApprovalStore_ListPendingApprovals_Error(t *testing.T) {
	fq := &fakeQueries{pendingErr: fmt.Errorf("db error")}
	store := NewDBApprovalStore(fq)
	_, err := store.ListPendingApprovals(context.Background(), uuid.New())
	assert.Error(t, err)
}

// A-08-retire-created-via-schema / A-16-retire-approval-unreachable: the
// approval queue must also surface retire-flagged rows (created_via='retire')
// whose status is 'approved'. Without this the DM never sees the request and
// the retire branch in approval_handler.go is unreachable end-to-end.
func TestDBApprovalStore_ListPendingApprovals_IncludesRetireRequests(t *testing.T) {
	pendingID := uuid.New()
	retireID := uuid.New()
	campaignID := uuid.New()

	fq := &fakeQueries{
		pendingRows: []refdata.ListPlayerCharactersAwaitingApprovalRow{
			{
				ID:            pendingID,
				CampaignID:    campaignID,
				CharacterID:   uuid.New(),
				DiscordUserID: "player-pending",
				Status:        "pending",
				CreatedVia:    "register",
				CharacterName: "Newbie",
			},
			{
				ID:            retireID,
				CampaignID:    campaignID,
				CharacterID:   uuid.New(),
				DiscordUserID: "player-retiring",
				Status:        "approved",
				CreatedVia:    "retire",
				CharacterName: "Gandalf",
			},
		},
	}

	store := NewDBApprovalStore(fq)
	entries, err := store.ListPendingApprovals(context.Background(), campaignID)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	gotIDs := map[uuid.UUID]ApprovalEntry{}
	for _, e := range entries {
		gotIDs[e.ID] = e
	}
	require.Contains(t, gotIDs, retireID, "retire-flagged row must appear in the approval queue")
	assert.Equal(t, "retire", gotIDs[retireID].CreatedVia)
	assert.Equal(t, "approved", gotIDs[retireID].Status)
}

func TestDBApprovalStore_GetApprovalDetail(t *testing.T) {
	id := uuid.New()
	charID := uuid.New()

	fq := &fakeQueries{
		detailRow: refdata.GetPlayerCharacterWithCharacterRow{
			ID:            id,
			CharacterID:   charID,
			DiscordUserID: "player1",
			Status:        "pending",
			CreatedVia:    "import",
			CharacterName: "Gandalf",
			Race:          "Human",
			Level:         5,
			Classes:       json.RawMessage(`[{"class":"wizard","level":5}]`),
			HpMax:         32,
			HpCurrent:     32,
			Ac:            12,
			SpeedFt:       30,
			AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":12,"int":18,"wis":14,"cha":10}`),
			Languages:     []string{"Common", "Elvish"},
			DdbUrl:        sql.NullString{String: "https://ddb.example.com", Valid: true},
		},
	}

	store := NewDBApprovalStore(fq)
	detail, err := store.GetApprovalDetail(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "Gandalf", detail.CharacterName)
	assert.Equal(t, "Human", detail.Race)
	assert.Equal(t, int32(5), detail.Level)
	assert.Contains(t, detail.Classes, "wizard")
	assert.Equal(t, "https://ddb.example.com", detail.DdbURL)
	assert.Contains(t, detail.Languages, "Common")
}

func TestDBApprovalStore_GetApprovalDetail_DerivesDDBAdvisories(t *testing.T) {
	id := uuid.New()
	charDataJSON := json.RawMessage(`{"spells":[{"name":"Cure Wounds","level":1,"source":"class","homebrew":true,"off_list":true}]}`)

	fq := &fakeQueries{
		detailRow: refdata.GetPlayerCharacterWithCharacterRow{
			ID:            id,
			CharacterID:   uuid.New(),
			DiscordUserID: "player1",
			Status:        "pending",
			CreatedVia:    "import",
			CharacterName: "Mira",
			Race:          "Human",
			Level:         3,
			Classes:       json.RawMessage(`[{"class":"Wizard","level":3}]`),
			AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":12,"int":18,"wis":14,"cha":10}`),
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		},
	}

	store := NewDBApprovalStore(fq)
	detail, err := store.GetApprovalDetail(context.Background(), id)
	require.NoError(t, err)
	require.Len(t, detail.Advisories, 1)
	assert.Contains(t, detail.Advisories[0], "Cure Wounds")
	assert.Contains(t, detail.Advisories[0], "off-list")
	assert.Contains(t, detail.Advisories[0], "homebrew")
}

func TestDBApprovalStore_GetApprovalDetail_NotFound(t *testing.T) {
	fq := &fakeQueries{detailErr: sql.ErrNoRows}
	store := NewDBApprovalStore(fq)
	_, err := store.GetApprovalDetail(context.Background(), uuid.New())
	assert.Error(t, err)
}

func TestDBApprovalStore_ApproveCharacter(t *testing.T) {
	id := uuid.New()
	fq := &fakeQueries{
		getPC: refdata.PlayerCharacter{
			ID:     id,
			Status: "pending",
		},
		updatePC: refdata.PlayerCharacter{
			ID:     id,
			Status: "approved",
		},
	}

	store := NewDBApprovalStore(fq)
	err := store.ApproveCharacter(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "approved", fq.updateParams.Status)
}

func TestDBApprovalStore_ApproveCharacter_InvalidTransition(t *testing.T) {
	id := uuid.New()
	fq := &fakeQueries{
		getPC: refdata.PlayerCharacter{
			ID:     id,
			Status: "approved", // already approved
		},
	}

	store := NewDBApprovalStore(fq)
	err := store.ApproveCharacter(context.Background(), id)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status transition")
}

func TestDBApprovalStore_RequestChanges(t *testing.T) {
	id := uuid.New()
	fq := &fakeQueries{
		getPC: refdata.PlayerCharacter{
			ID:     id,
			Status: "pending",
		},
		updatePC: refdata.PlayerCharacter{
			ID:     id,
			Status: "changes_requested",
		},
	}

	store := NewDBApprovalStore(fq)
	err := store.RequestChanges(context.Background(), id, "Fix HP")
	require.NoError(t, err)
	assert.Equal(t, "changes_requested", fq.updateParams.Status)
	assert.Equal(t, "Fix HP", fq.updateParams.DmFeedback.String)
}

func TestDBApprovalStore_RejectCharacter(t *testing.T) {
	id := uuid.New()
	fq := &fakeQueries{
		getPC: refdata.PlayerCharacter{
			ID:     id,
			Status: "pending",
		},
		updatePC: refdata.PlayerCharacter{
			ID:     id,
			Status: "rejected",
		},
	}

	store := NewDBApprovalStore(fq)
	err := store.RejectCharacter(context.Background(), id, "Not allowed")
	require.NoError(t, err)
	assert.Equal(t, "rejected", fq.updateParams.Status)
	assert.Equal(t, "Not allowed", fq.updateParams.DmFeedback.String)
}
