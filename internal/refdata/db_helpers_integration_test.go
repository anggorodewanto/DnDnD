package refdata_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/testutil"
)

// TestIntegration_CountActiveEncountersByCharacterID exercises the hand-written
// (non-sqlc) duplicate-detection query: only encounters in status 'active' that
// the character is a combatant in are counted.
func TestIntegration_CountActiveEncountersByCharacterID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := sharedDB.AcquireDB(t)
	ctx := context.Background()
	q := refdata.New(db)

	camp := testutil.NewTestCampaign(t, q, "count-active")
	char := testutil.NewTestCharacter(t, q, camp.ID, "Counter", 1)
	charID := uuid.NullUUID{UUID: char.ID, Valid: true}

	// No encounters yet → 0.
	n, err := q.CountActiveEncountersByCharacterID(ctx, charID)
	if err != nil {
		t.Fatalf("CountActiveEncountersByCharacterID (none): %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 active encounters, got %d", n)
	}

	// A 'preparing' encounter the character is in must NOT count as active.
	prep := testutil.NewTestEncounter(t, q, camp.ID)
	testutil.NewTestCombatant(t, q, prep.ID, char.ID)
	n, err = q.CountActiveEncountersByCharacterID(ctx, charID)
	if err != nil {
		t.Fatalf("CountActiveEncountersByCharacterID (preparing): %v", err)
	}
	if n != 0 {
		t.Fatalf("preparing encounter must not count as active; got %d", n)
	}

	// Promote a second encounter to 'active' → counts as 1.
	active := testutil.NewTestEncounter(t, q, camp.ID)
	testutil.NewTestCombatant(t, q, active.ID, char.ID)
	if _, err := db.ExecContext(ctx, "UPDATE encounters SET status='active' WHERE id=$1", active.ID); err != nil {
		t.Fatalf("promote encounter to active: %v", err)
	}
	n, err = q.CountActiveEncountersByCharacterID(ctx, charID)
	if err != nil {
		t.Fatalf("CountActiveEncountersByCharacterID (active): %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 active encounter, got %d", n)
	}
}

// TestIntegration_UpdateTwoCharacterInventories exercises the atomic two-character
// inventory swap (the happy path: BeginTx → WithTx → two updates → Commit).
func TestIntegration_UpdateTwoCharacterInventories(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := sharedDB.AcquireDB(t)
	ctx := context.Background()
	q := refdata.New(db)

	camp := testutil.NewTestCampaign(t, q, "two-inv")
	alice := testutil.NewTestCharacter(t, q, camp.ID, "Alice", 1)
	bob := testutil.NewTestCharacter(t, q, camp.ID, "Bob", 1)

	invAlice := pqtype.NullRawMessage{RawMessage: []byte(`{"items":["sword"]}`), Valid: true}
	invBob := pqtype.NullRawMessage{RawMessage: []byte(`{"items":["shield"]}`), Valid: true}

	if err := q.UpdateTwoCharacterInventories(ctx, alice.ID, invAlice, bob.ID, invBob); err != nil {
		t.Fatalf("UpdateTwoCharacterInventories: %v", err)
	}

	assertInventoryItems(t, q, alice.ID, []string{"sword"})
	assertInventoryItems(t, q, bob.ID, []string{"shield"})
}

func assertInventoryItems(t *testing.T, q *refdata.Queries, id uuid.UUID, want []string) {
	t.Helper()
	got, err := q.GetCharacter(context.Background(), id)
	if err != nil {
		t.Fatalf("GetCharacter(%s): %v", id, err)
	}
	if !got.Inventory.Valid {
		t.Fatalf("character %s inventory not set", id)
	}
	var decoded struct {
		Items []string `json:"items"`
	}
	if err := json.Unmarshal(got.Inventory.RawMessage, &decoded); err != nil {
		t.Fatalf("decoding inventory for %s: %v (raw=%s)", id, err, got.Inventory.RawMessage)
	}
	if !reflect.DeepEqual(decoded.Items, want) {
		t.Fatalf("character %s inventory items = %v, want %v", id, decoded.Items, want)
	}
}

// noTxDBTX implements refdata.DBTX but NOT refdata.TxBeginner, so
// UpdateTwoCharacterInventories must fail fast without touching the DB. This
// pure-unit case covers the transaction-unsupported branch with no testcontainer.
type noTxDBTX struct{}

func (noTxDBTX) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, nil
}
func (noTxDBTX) PrepareContext(context.Context, string) (*sql.Stmt, error) { return nil, nil }
func (noTxDBTX) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, nil
}
func (noTxDBTX) QueryRowContext(context.Context, string, ...any) *sql.Row { return nil }

func TestUpdateTwoCharacterInventories_NoTxSupport(t *testing.T) {
	q := refdata.New(noTxDBTX{})
	err := q.UpdateTwoCharacterInventories(context.Background(),
		uuid.New(), pqtype.NullRawMessage{},
		uuid.New(), pqtype.NullRawMessage{})
	if err == nil || !strings.Contains(err.Error(), "does not support transactions") {
		t.Fatalf("expected transaction-unsupported error, got %v", err)
	}
}
