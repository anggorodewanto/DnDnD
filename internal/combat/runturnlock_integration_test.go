package combat_test

// F-4: RunUnderTurnLock integration tests.
//
// These exercise the lock-held-during-write contract that closes the
// chunk3 cross-cutting bug "TurnGate releases the advisory lock before the
// write". The lock-holding tx is exposed to fn via context, and peers MUST
// block at the pg_advisory_xact_lock acquire until fn returns and the tx
// commits/rolls back.

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/combat"
)

// TestRunUnderTurnLock_Success_FnRunsAndCommits proves the happy path:
// fn is invoked exactly once, sees the tx via context, and the commit
// path persists any writes fn made on that tx.
func TestRunUnderTurnLock_Success_FnRunsAndCommits(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	ctx := context.Background()

	var fnCalls int32
	var fnSawTx bool

	info, err := combat.RunUnderTurnLock(ctx, td.DB, td.Queries, td.EncounterID, td.PlayerUID, func(fnCtx context.Context) error {
		atomic.AddInt32(&fnCalls, 1)
		// fn MUST receive the lock-holding tx via context so it can opt
		// into running its writes on the same tx — this is the contract
		// that lets callers persist inside the lock window.
		if tx := combat.TxFromContext(fnCtx); tx != nil {
			fnSawTx = true
			// Sanity: run a trivial query on the tx to prove it's live.
			var n int
			if qErr := tx.QueryRowContext(fnCtx, "SELECT 1").Scan(&n); qErr != nil {
				return qErr
			}
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&fnCalls), "fn must run exactly once on success")
	assert.True(t, fnSawTx, "fn must see the lock-holding tx via context")
	assert.Equal(t, td.TurnID, info.TurnID)
}

// TestRunUnderTurnLock_FnError_RollsBackAndPropagates: when fn errors,
// the tx is rolled back (no held lock leaks) and fn's error propagates
// unwrapped so the caller can branch on its own sentinels.
func TestRunUnderTurnLock_FnError_RollsBackAndPropagates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	ctx := context.Background()

	myErr := errors.New("write failed")
	_, err := combat.RunUnderTurnLock(ctx, td.DB, td.Queries, td.EncounterID, td.PlayerUID, func(_ context.Context) error {
		return myErr
	})
	require.ErrorIs(t, err, myErr)

	// After rollback the lock must be free: a second call must succeed
	// immediately (no timeout) because no other connection holds the lock.
	deadlineCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err = combat.RunUnderTurnLock(deadlineCtx, td.DB, td.Queries, td.EncounterID, td.PlayerUID, func(_ context.Context) error {
		return nil
	})
	require.NoError(t, err, "lock must be released after fn error rolls back")
}

// TestRunUnderTurnLock_NilFn rejects nil fn — guards against an accidental
// call site that wires up the gate but forgets the callback body.
func TestRunUnderTurnLock_NilFn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	ctx := context.Background()

	_, err := combat.RunUnderTurnLock(ctx, td.DB, td.Queries, td.EncounterID, td.PlayerUID, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fn must not be nil")
}

// TestRunUnderTurnLock_ValidationError_FnNotCalled: a wrong-owner caller
// must not see fn invoked — the gate rejects BEFORE the callback runs so
// a hostile player can't slip a write through by raising an error in fn.
func TestRunUnderTurnLock_ValidationError_FnNotCalled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	ctx := context.Background()

	var fnCalls int32
	_, err := combat.RunUnderTurnLock(ctx, td.DB, td.Queries, td.EncounterID, "wrong-user", func(_ context.Context) error {
		atomic.AddInt32(&fnCalls, 1)
		return nil
	})
	require.Error(t, err)
	var notYourTurn *combat.ErrNotYourTurn
	assert.True(t, errors.As(err, &notYourTurn), "expected ErrNotYourTurn, got %T %v", err, err)
	assert.Equal(t, int32(0), atomic.LoadInt32(&fnCalls), "fn must NOT run when validation fails")
}

// TestRunUnderTurnLock_LockHeldUntilFnReturns: a concurrent peer's
// RunUnderTurnLock call MUST block until our fn returns and the tx
// commits. This is the core F-4 guarantee — without it two handlers
// could pass the gate and interleave writes against the same turn.
func TestRunUnderTurnLock_LockHeldUntilFnReturns(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	ctx := context.Background()

	// Coordinate: fn A signals "started", waits for the test to signal
	// "release", then returns. Peer B starts AFTER A signals started; B
	// must block until A releases.
	aStarted := make(chan struct{})
	aRelease := make(chan struct{})
	peerDone := make(chan error, 1)
	var peerStartedAt atomic.Int64

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := combat.RunUnderTurnLock(ctx, td.DB, td.Queries, td.EncounterID, td.PlayerUID, func(_ context.Context) error {
			close(aStarted)
			<-aRelease
			return nil
		})
		if err != nil {
			t.Errorf("handler A returned err: %v", err)
		}
	}()

	<-aStarted

	// Peer B fires (DM, so it always passes validation). It must BLOCK on
	// the advisory lock until A releases — we record the wall-clock
	// duration peer B spent blocking.
	peerStart := time.Now()
	peerStartedAt.Store(peerStart.UnixNano())
	go func() {
		_, err := combat.RunUnderTurnLock(ctx, td.DB, td.Queries, td.EncounterID, td.DMUserID, func(_ context.Context) error {
			return nil
		})
		peerDone <- err
	}()

	// Give peer B time to actually attempt the lock and block on it.
	time.Sleep(300 * time.Millisecond)
	select {
	case err := <-peerDone:
		t.Fatalf("peer B finished before A released the lock; err=%v", err)
	default:
		// good: peer B is blocked
	}

	close(aRelease)
	wg.Wait()

	select {
	case err := <-peerDone:
		require.NoError(t, err)
		elapsed := time.Since(peerStart)
		assert.GreaterOrEqual(t, elapsed, 250*time.Millisecond, "peer B must have blocked at least until A released")
	case <-time.After(3 * time.Second):
		t.Fatal("peer B did not finish after A released the lock")
	}
}

// TestRunUnderTurnLock_Panic_RollsBack: if fn panics the implementation
// must recover, roll back, and NOT leak the lock — a peer call after
// the panic must succeed without blocking.
func TestRunUnderTurnLock_Panic_RollsBack(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	ctx := context.Background()

	_, err := combat.RunUnderTurnLock(ctx, td.DB, td.Queries, td.EncounterID, td.PlayerUID, func(_ context.Context) error {
		panic("boom")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "panicked")

	// Lock must be free — a follow-up call completes quickly.
	deadlineCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err = combat.RunUnderTurnLock(deadlineCtx, td.DB, td.Queries, td.EncounterID, td.PlayerUID, func(_ context.Context) error {
		return nil
	})
	require.NoError(t, err)
}

// TestContextWithTx_RoundTrip is a pure unit test: ContextWithTx +
// TxFromContext must round-trip the value with no DB needed.
func TestContextWithTx_RoundTrip(t *testing.T) {
	ctx := context.Background()

	assert.Nil(t, combat.TxFromContext(ctx), "empty context returns nil")
	assert.Nil(t, combat.TxFromContext(nil), "nil context returns nil safely")

	// Round-trip a non-nil tx pointer. We can't construct a real *sql.Tx
	// without a DB; pass a sentinel pointer to prove ctx carries it.
	var tx sql.Tx
	enriched := combat.ContextWithTx(ctx, &tx)
	got := combat.TxFromContext(enriched)
	assert.Same(t, &tx, got, "TxFromContext must return the same *sql.Tx stashed by ContextWithTx")
}

// TestRunUnderTurnLock_FnSeesNonEmptyContext sanity-checks that fn's
// context still propagates cancellation, deadlines, and stashed values
// from the caller — only the *sql.Tx is added.
func TestRunUnderTurnLock_FnSeesNonEmptyContext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "marker")

	var fnSawMarker bool
	_, err := combat.RunUnderTurnLock(ctx, td.DB, td.Queries, td.EncounterID, td.PlayerUID, func(fnCtx context.Context) error {
		if v, _ := fnCtx.Value(key{}).(string); v == "marker" {
			fnSawMarker = true
		}
		return nil
	})
	require.NoError(t, err)
	assert.True(t, fnSawMarker, "fn must inherit the caller's context values")
}

// Compile-time guard: the issue we're closing is "fn errors must NOT
// commit." We use a sentinel pointer write inside fn (via the tx) and
// then assert AFTER the call that the write is NOT visible — proves the
// tx was actually rolled back, not committed-then-mutated.
func TestRunUnderTurnLock_FnError_TxWritesNotPersisted(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	ctx := context.Background()

	// Read the combatant HP before the call so we have a baseline.
	combatant, err := td.Queries.GetCombatant(ctx, td.CombatantID)
	require.NoError(t, err)
	originalHP := combatant.HpCurrent

	mutationErr := errors.New("forced rollback")
	_, err = combat.RunUnderTurnLock(ctx, td.DB, td.Queries, td.EncounterID, td.PlayerUID, func(fnCtx context.Context) error {
		tx := combat.TxFromContext(fnCtx)
		require.NotNil(t, tx, "fn must see the tx via context")
		// Mutate the combatant on the tx. If RunUnderTurnLock erroneously
		// commits, the new HP would be visible after the call returns.
		if _, execErr := tx.ExecContext(fnCtx, "UPDATE combatants SET hp_current = $1 WHERE id = $2", 1, td.CombatantID); execErr != nil {
			return execErr
		}
		return mutationErr
	})
	require.ErrorIs(t, err, mutationErr)

	// Re-read: HP must be unchanged because the tx rolled back.
	after, err := td.Queries.GetCombatant(ctx, td.CombatantID)
	require.NoError(t, err)
	assert.Equal(t, originalHP, after.HpCurrent, "fn-error path must roll back tx writes")
}

// Sentinel guard: confirm that uuid.UUID zero value is NOT treated as a
// valid encounter (validation returns NoActiveTurn or similar). This
// prevents a regression where a default-init handler invokes RunUnderTurnLock
// with uuid.Nil and accidentally locks "the empty turn".
func TestRunUnderTurnLock_ZeroEncounterID_FailsValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	ctx := context.Background()

	var fnCalls int32
	_, err := combat.RunUnderTurnLock(ctx, td.DB, td.Queries, uuid.Nil, td.PlayerUID, func(_ context.Context) error {
		atomic.AddInt32(&fnCalls, 1)
		return nil
	})
	require.Error(t, err)
	assert.Equal(t, int32(0), atomic.LoadInt32(&fnCalls))
}
