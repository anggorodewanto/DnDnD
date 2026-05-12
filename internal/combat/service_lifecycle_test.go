package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- B-26b-combat-log-announcement ---

// fakeCombatLog records every PostCombatLog call.
type fakeCombatLog struct {
	mu    sync.Mutex
	posts []combatLogPost
}

type combatLogPost struct {
	encounterID uuid.UUID
	content     string
}

func (f *fakeCombatLog) PostCombatLog(_ context.Context, encounterID uuid.UUID, content string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.posts = append(f.posts, combatLogPost{encounterID: encounterID, content: content})
}

func (f *fakeCombatLog) all() []combatLogPost {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]combatLogPost, len(f.posts))
	copy(out, f.posts)
	return out
}

// fakeLootCreator records every CreateLootPool call.
type fakeLootCreator struct {
	mu    sync.Mutex
	calls []uuid.UUID
	err   error
}

func (f *fakeLootCreator) CreateLootPool(_ context.Context, encounterID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, encounterID)
	return f.err
}

func (f *fakeLootCreator) all() []uuid.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]uuid.UUID, len(f.calls))
	copy(out, f.calls)
	return out
}

// fakeHostilesNotifier records every NotifyHostilesDefeated call.
type fakeHostilesNotifier struct {
	mu    sync.Mutex
	calls []uuid.UUID
}

func (f *fakeHostilesNotifier) NotifyHostilesDefeated(_ context.Context, encounterID uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, encounterID)
}

func (f *fakeHostilesNotifier) all() []uuid.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]uuid.UUID, len(f.calls))
	copy(out, f.calls)
	return out
}

func endCombatStoreWithCombatants(t *testing.T, encID uuid.UUID, combatants []refdata.Combatant) *mockStore {
	t.Helper()
	store := defaultMockStore()
	store.getEncounterFn = func(_ context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:          id,
			Name:        "Goblin Ambush",
			DisplayName: sql.NullString{String: "The Goblin Ambush", Valid: true},
			Status:      "active",
			RoundNumber: 4,
		}, nil
	}
	store.updateEncounterStatusFn = func(_ context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:          arg.ID,
			Name:        "Goblin Ambush",
			DisplayName: sql.NullString{String: "The Goblin Ambush", Valid: true},
			Status:      arg.Status,
			RoundNumber: 4,
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return combatants, nil
	}
	store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}
	return store
}

func TestEndCombat_PostsCombatLogAnnouncement(t *testing.T) {
	encID := uuid.New()
	combatants := []refdata.Combatant{
		{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, DisplayName: "Goblin 1", Conditions: json.RawMessage(`[]`)},
		{ID: uuid.New(), IsNpc: false, IsAlive: true, HpCurrent: 20, DisplayName: "Aragorn", Conditions: json.RawMessage(`[]`)},
	}
	store := endCombatStoreWithCombatants(t, encID, combatants)
	svc := NewService(store)

	logNotifier := &fakeCombatLog{}
	svc.SetCombatLogNotifier(logNotifier)

	_, err := svc.EndCombat(context.Background(), encID)
	require.NoError(t, err)

	posts := logNotifier.all()
	require.NotEmpty(t, posts, "combat-log announcement must fire after EndCombat")
	announcement := posts[0]
	assert.Equal(t, encID, announcement.encounterID)
	assert.Contains(t, announcement.content, "Combat ended")
	assert.Contains(t, announcement.content, "The Goblin Ambush", "announcement should use the display name")
	assert.Contains(t, announcement.content, "4 round")
	assert.Contains(t, announcement.content, "1 casualty")
}

func TestEndCombat_PostsCombatLogAnnouncement_NilNotifierTolerated(t *testing.T) {
	encID := uuid.New()
	combatants := []refdata.Combatant{
		{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, DisplayName: "Goblin 1", Conditions: json.RawMessage(`[]`)},
	}
	store := endCombatStoreWithCombatants(t, encID, combatants)
	svc := NewService(store)
	// Intentionally do NOT set notifier.

	_, err := svc.EndCombat(context.Background(), encID)
	require.NoError(t, err)
}

func TestFormatCombatEndedAnnouncement_UsesNameWhenDisplayNameBlank(t *testing.T) {
	enc := refdata.Encounter{
		ID:          uuid.New(),
		Name:        "Goblin Ambush",
		DisplayName: sql.NullString{Valid: false},
	}
	got := FormatCombatEndedAnnouncement(enc, 3, 2)
	assert.Contains(t, got, "Goblin Ambush")
	assert.Contains(t, got, "3 round")
	assert.Contains(t, got, "2 casualty")
}

// --- B-26b-loot-auto-create ---

func TestEndCombat_AutoCreatesLootPool(t *testing.T) {
	encID := uuid.New()
	combatants := []refdata.Combatant{
		{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, DisplayName: "Goblin 1", Conditions: json.RawMessage(`[]`)},
	}
	store := endCombatStoreWithCombatants(t, encID, combatants)
	svc := NewService(store)

	creator := &fakeLootCreator{}
	svc.SetLootPoolCreator(creator)

	_, err := svc.EndCombat(context.Background(), encID)
	require.NoError(t, err)

	assert.Equal(t, []uuid.UUID{encID}, creator.all(), "EndCombat should fire CreateLootPool exactly once")
}

func TestEndCombat_LootPoolFailure_DoesNotFailEndCombat(t *testing.T) {
	encID := uuid.New()
	combatants := []refdata.Combatant{
		{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, DisplayName: "Goblin 1", Conditions: json.RawMessage(`[]`)},
	}
	store := endCombatStoreWithCombatants(t, encID, combatants)
	svc := NewService(store)
	svc.SetLootPoolCreator(&fakeLootCreator{err: errors.New("already exists")})

	_, err := svc.EndCombat(context.Background(), encID)
	require.NoError(t, err, "a loot-pool failure must not roll back EndCombat")
}

// --- B-26b-ammo-recovery-prompt ---

func TestEndCombat_PostsAmmoRecoverySummary(t *testing.T) {
	encID := uuid.New()
	charID := uuid.New()
	combatantID := uuid.New()

	startingInventory, _ := json.Marshal([]InventoryItem{
		{Name: "Arrows", Quantity: 10, Type: "ammunition"},
	})
	store := endCombatStoreWithCombatants(t, encID, []refdata.Combatant{
		{ID: combatantID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Archer", IsAlive: true, IsNpc: false, Conditions: json.RawMessage(`[]`)},
	})
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{ID: charID, Inventory: pqtype.NullRawMessage{RawMessage: startingInventory, Valid: true}}, nil
	}
	store.updateCharacterInventoryFn = func(_ context.Context, _ uuid.UUID, _ pqtype.NullRawMessage) error {
		return nil
	}

	svc := NewService(store)
	svc.RecordAmmoSpent(encID, combatantID, "Arrows", 7)

	logNotifier := &fakeCombatLog{}
	svc.SetCombatLogNotifier(logNotifier)

	_, err := svc.EndCombat(context.Background(), encID)
	require.NoError(t, err)

	posts := logNotifier.all()
	require.GreaterOrEqual(t, len(posts), 2, "expected combat-end announcement + ammo summary")

	var ammoPost string
	for _, p := range posts {
		if strings.Contains(p.content, "Ammunition recovered") {
			ammoPost = p.content
			break
		}
	}
	require.NotEmpty(t, ammoPost, "ammo recovery summary should be posted to combat-log")
	assert.Contains(t, ammoPost, "Archer")
	assert.Contains(t, ammoPost, "3 Arrows", "7 spent -> half rounded down = 3 recovered")
}

func TestEndCombat_NoAmmoSpent_NoAmmoSummaryPosted(t *testing.T) {
	encID := uuid.New()
	store := endCombatStoreWithCombatants(t, encID, []refdata.Combatant{
		{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`)},
	})
	svc := NewService(store)

	logNotifier := &fakeCombatLog{}
	svc.SetCombatLogNotifier(logNotifier)

	_, err := svc.EndCombat(context.Background(), encID)
	require.NoError(t, err)

	posts := logNotifier.all()
	for _, p := range posts {
		assert.NotContains(t, p.content, "Ammunition recovered", "no ammo spent -> no recovery summary")
	}
}

func TestFormatAmmoRecoverySummary_EmptySnapshotReturnsEmpty(t *testing.T) {
	assert.Equal(t, "", FormatAmmoRecoverySummary(nil, nil))
	assert.Equal(t, "", FormatAmmoRecoverySummary(nil, map[uuid.UUID]map[string]int{}))
}

func TestFormatAmmoRecoverySummary_SkipsCombatantsWithZeroRecovery(t *testing.T) {
	id := uuid.New()
	got := FormatAmmoRecoverySummary(
		[]refdata.Combatant{{ID: id, DisplayName: "Archer"}},
		map[uuid.UUID]map[string]int{id: {"Arrows": 1}}, // 1/2 = 0 recovered
	)
	assert.Equal(t, "", got)
}

// --- B-26b-all-hostiles-defeated-prompt ---

func TestNotifyCardUpdate_FiresHostilesDefeatedPromptOnce(t *testing.T) {
	encID := uuid.New()
	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, Conditions: json.RawMessage(`[]`)},
			{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, Conditions: json.RawMessage(`[]`)},
			{ID: uuid.New(), IsNpc: false, IsAlive: true, HpCurrent: 20, Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	svc := NewService(store)
	hn := &fakeHostilesNotifier{}
	svc.SetHostilesDefeatedNotifier(hn)

	// Simulate damage paths' hook: notifyCardUpdate fires on every HP write.
	// Two NPC writes in the same encounter should result in exactly ONE
	// prompt (dedupe per encounter).
	svc.notifyCardUpdate(context.Background(), refdata.Combatant{ID: uuid.New(), EncounterID: encID, IsNpc: true})
	svc.notifyCardUpdate(context.Background(), refdata.Combatant{ID: uuid.New(), EncounterID: encID, IsNpc: true})

	assert.Equal(t, []uuid.UUID{encID}, hn.all(), "hostiles-defeated prompt must fire exactly once per encounter")
}

func TestNotifyCardUpdate_SkipsHostilesPromptWhenSurvivorsAlive(t *testing.T) {
	encID := uuid.New()
	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, Conditions: json.RawMessage(`[]`)},
			{ID: uuid.New(), IsNpc: true, IsAlive: true, HpCurrent: 4, Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	svc := NewService(store)
	hn := &fakeHostilesNotifier{}
	svc.SetHostilesDefeatedNotifier(hn)

	svc.notifyCardUpdate(context.Background(), refdata.Combatant{ID: uuid.New(), EncounterID: encID, IsNpc: true})

	assert.Empty(t, hn.all(), "prompt must not fire while a hostile is still alive")
}

func TestNotifyCardUpdate_HostilesPromptNilNotifierTolerated(t *testing.T) {
	encID := uuid.New()
	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	svc := NewService(store)
	// Intentionally do NOT set notifier.

	svc.notifyCardUpdate(context.Background(), refdata.Combatant{ID: uuid.New(), EncounterID: encID, IsNpc: true})
}

func TestNotifyCardUpdate_PCMutation_SkipsHostilesCheck(t *testing.T) {
	encID := uuid.New()
	listCalls := 0
	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		listCalls++
		return []refdata.Combatant{
			{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	svc := NewService(store)
	hn := &fakeHostilesNotifier{}
	svc.SetHostilesDefeatedNotifier(hn)

	// A PC mutation cannot drop the last hostile so we must skip the
	// list-combatants probe entirely (perf optimization).
	svc.notifyCardUpdate(context.Background(), refdata.Combatant{ID: uuid.New(), EncounterID: encID, IsNpc: false})

	assert.Equal(t, 0, listCalls, "PC mutation must not trigger the hostiles-defeated probe")
	assert.Empty(t, hn.all(), "PC mutation must not fire the hostiles prompt")
}

func TestEndCombat_ClearsHostilesPromptedState(t *testing.T) {
	encID := uuid.New()
	store := endCombatStoreWithCombatants(t, encID, []refdata.Combatant{
		{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`)},
	})
	svc := NewService(store)
	hn := &fakeHostilesNotifier{}
	svc.SetHostilesDefeatedNotifier(hn)

	// Force the prompt to fire once.
	svc.notifyCardUpdate(context.Background(), refdata.Combatant{ID: uuid.New(), EncounterID: encID, IsNpc: true})
	require.Equal(t, 1, len(hn.all()))

	// End the encounter; this should drop the dedupe state so a fresh
	// encounter that reuses the same id would prompt again.
	_, err := svc.EndCombat(context.Background(), encID)
	require.NoError(t, err)

	svc.notifyCardUpdate(context.Background(), refdata.Combatant{ID: uuid.New(), EncounterID: encID, IsNpc: true})
	assert.Equal(t, 2, len(hn.all()), "EndCombat should clear hostiles-prompted state so re-use of the id can re-prompt")
}
