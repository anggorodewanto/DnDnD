package combat

import (
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- TDD Cycle 1: Parse legendary actions from creature abilities ---

func TestParseLegendaryActions_AdultDragon(t *testing.T) {
	abilities := []CreatureAbilityEntry{
		{Name: "Legendary Resistance (3/Day)", Description: "If the dragon fails a saving throw, it can choose to succeed instead."},
		{Name: "Legendary Actions", Description: "The dragon can take 3 legendary actions, choosing from the options below. Only one legendary action option can be used at a time and only at the end of another creature's turn. The dragon regains spent legendary actions at the start of its turn."},
		{Name: "Detect", Description: "The dragon makes a Wisdom (Perception) check."},
		{Name: "Tail Attack", Description: "The dragon makes a tail attack."},
		{Name: "Wing Attack (Costs 2 Actions)", Description: "The dragon beats its wings. Each creature within 10 feet of the dragon must succeed on a DC 19 Dexterity saving throw or take 13 (2d6+6) bludgeoning damage and be knocked prone. The dragon can then fly up to half its flying speed."},
	}

	info := ParseLegendaryInfo(abilities)
	require.NotNil(t, info)
	assert.Equal(t, 3, info.Budget)
	require.Len(t, info.Actions, 3)
	assert.Equal(t, "Detect", info.Actions[0].Name)
	assert.Equal(t, 1, info.Actions[0].Cost)
	assert.Equal(t, "Tail Attack", info.Actions[1].Name)
	assert.Equal(t, 1, info.Actions[1].Cost)
	assert.Equal(t, "Wing Attack", info.Actions[2].Name)
	assert.Equal(t, 2, info.Actions[2].Cost)
}

func TestParseLegendaryActions_NoBudget(t *testing.T) {
	abilities := []CreatureAbilityEntry{
		{Name: "Keen Senses", Description: "The wolf has advantage."},
	}
	info := ParseLegendaryInfo(abilities)
	assert.Nil(t, info)
}

func TestParseLegendaryActions_Lich(t *testing.T) {
	abilities := []CreatureAbilityEntry{
		{Name: "Legendary Resistance (3/Day)", Description: "If the lich fails a saving throw, it can choose to succeed instead."},
		{Name: "Legendary Actions", Description: "The lich can take 3 legendary actions, choosing from the options below."},
		{Name: "Cantrip", Description: "The lich casts a cantrip."},
		{Name: "Paralyzing Touch (Costs 2 Actions)", Description: "The lich uses its Paralyzing Touch."},
		{Name: "Frightening Gaze (Costs 2 Actions)", Description: "The lich fixes its gaze on one creature it can see within 10 feet of it."},
		{Name: "Disrupt Life (Costs 3 Actions)", Description: "Each non-undead creature within 20 feet of the lich must make a DC 18 Constitution saving throw."},
	}

	info := ParseLegendaryInfo(abilities)
	require.NotNil(t, info)
	assert.Equal(t, 3, info.Budget)
	require.Len(t, info.Actions, 4)
	assert.Equal(t, 1, info.Actions[0].Cost)
	assert.Equal(t, 2, info.Actions[1].Cost)
	assert.Equal(t, 2, info.Actions[2].Cost)
	assert.Equal(t, 3, info.Actions[3].Cost)
}

// --- TDD Cycle 2: Legendary Action Budget ---

func TestLegendaryActionBudget_SpendAndReset(t *testing.T) {
	budget := NewLegendaryActionBudget(3)
	assert.Equal(t, 3, budget.Total)
	assert.Equal(t, 3, budget.Remaining)
	assert.True(t, budget.CanAfford(1))
	assert.True(t, budget.CanAfford(3))
	assert.False(t, budget.CanAfford(4))

	b2, err := budget.Spend(1)
	require.NoError(t, err)
	assert.Equal(t, 2, b2.Remaining)

	b3, err := b2.Spend(2)
	require.NoError(t, err)
	assert.Equal(t, 0, b3.Remaining)

	_, err = b3.Spend(1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient legendary actions")

	b4 := b3.Reset()
	assert.Equal(t, 3, b4.Remaining)
}

// --- TDD Cycle 3: Parse lair actions ---

func TestParseLairInfo_DragonLair(t *testing.T) {
	abilities := []CreatureAbilityEntry{
		{Name: "Legendary Actions", Description: "The dragon can take 3 legendary actions."},
		{Name: "Detect", Description: "The dragon makes a Perception check."},
		{Name: "Lair Actions", Description: "On initiative count 20 (losing initiative ties), the dragon takes a lair action to cause one of the following effects:"},
		{Name: "Magma Eruption", Description: "Magma erupts from a point on the ground."},
		{Name: "Tremor", Description: "A tremor shakes the lair."},
	}

	info := ParseLairInfo(abilities)
	require.NotNil(t, info)
	require.Len(t, info.Actions, 2)
	assert.Equal(t, "Magma Eruption", info.Actions[0].Name)
	assert.Equal(t, "Tremor", info.Actions[1].Name)
}

func TestParseLairInfo_NoLair(t *testing.T) {
	abilities := []CreatureAbilityEntry{
		{Name: "Keen Senses", Description: "Advantage on perception."},
	}
	info := ParseLairInfo(abilities)
	assert.Nil(t, info)
}

// --- TDD Cycle 4: Lair Action Tracker ---

func TestLairActionTracker_NoRepeat(t *testing.T) {
	tracker := LairActionTracker{}
	assert.True(t, tracker.CanUse("Magma Eruption"))
	assert.True(t, tracker.CanUse("Tremor"))

	tracker = tracker.Use("Magma Eruption")
	assert.False(t, tracker.CanUse("Magma Eruption"))
	assert.True(t, tracker.CanUse("Tremor"))

	tracker = tracker.Use("Tremor")
	assert.True(t, tracker.CanUse("Magma Eruption"))
	assert.False(t, tracker.CanUse("Tremor"))
}

func TestAvailableLairActions_FiltersLastUsed(t *testing.T) {
	info := &LairInfo{
		Actions: []LairAction{
			{Name: "Magma Eruption", Description: "erupts"},
			{Name: "Tremor", Description: "shakes"},
			{Name: "Gases", Description: "volcanic gas"},
		},
	}

	tracker := LairActionTracker{LastUsedName: "Tremor"}
	available := AvailableLairActions(info, tracker)
	require.Len(t, available, 2)
	assert.Equal(t, "Magma Eruption", available[0].Name)
	assert.Equal(t, "Gases", available[1].Name)
}

func TestAvailableLairActions_NilInfo(t *testing.T) {
	available := AvailableLairActions(nil, LairActionTracker{})
	assert.Nil(t, available)
}

// --- TDD Cycle 5: Combat log formatting ---

func TestFormatLegendaryActionLog(t *testing.T) {
	action := LegendaryAction{Name: "Tail Attack", Description: "tail swipe", Cost: 1}
	log := FormatLegendaryActionLog("Adult Red Dragon", action, 1, 3)
	assert.Contains(t, log, "Adult Red Dragon")
	assert.Contains(t, log, "Tail Attack")
	assert.Contains(t, log, "1/3")
}

func TestFormatLairActionLog(t *testing.T) {
	action := LairAction{Name: "Magma erupts", Description: "kaboom"}
	log := FormatLairActionLog(action)
	assert.Contains(t, log, "Lair Action")
	assert.Contains(t, log, "Initiative 20")
	assert.Contains(t, log, "Magma erupts")
}

// --- TDD Cycle 6: LegendaryActionPlan ---

func TestBuildLegendaryActionPlan(t *testing.T) {
	info := &LegendaryInfo{
		Budget: 3,
		Actions: []LegendaryAction{
			{Name: "Detect", Description: "Perception check.", Cost: 1},
			{Name: "Tail Attack", Description: "Tail attack.", Cost: 1},
			{Name: "Wing Attack", Description: "Wings.", Cost: 2},
		},
	}
	budget := NewLegendaryActionBudget(3)

	plan := BuildLegendaryActionPlan("Adult Red Dragon", info, budget)
	assert.Equal(t, "Adult Red Dragon", plan.CreatureName)
	assert.Equal(t, 3, plan.BudgetTotal)
	assert.Equal(t, 3, plan.BudgetRemaining)
	require.Len(t, plan.AvailableActions, 3)
	for _, a := range plan.AvailableActions {
		assert.True(t, a.Affordable)
	}
}

func TestBuildLegendaryActionPlan_PartialBudget(t *testing.T) {
	info := &LegendaryInfo{
		Budget: 3,
		Actions: []LegendaryAction{
			{Name: "Detect", Description: "Perception check.", Cost: 1},
			{Name: "Wing Attack", Description: "Wings.", Cost: 2},
			{Name: "Disrupt Life", Description: "AoE.", Cost: 3},
		},
	}
	budget := LegendaryActionBudget{Total: 3, Remaining: 1}

	plan := BuildLegendaryActionPlan("Lich", info, budget)
	assert.Equal(t, 1, plan.BudgetRemaining)
	require.Len(t, plan.AvailableActions, 3)
	assert.True(t, plan.AvailableActions[0].Affordable)
	assert.False(t, plan.AvailableActions[1].Affordable)
	assert.False(t, plan.AvailableActions[2].Affordable)
}

// --- TDD Cycle 7: BuildLairActionPlan ---

func TestBuildLairActionPlan(t *testing.T) {
	info := &LairInfo{
		Actions: []LairAction{
			{Name: "Magma Eruption", Description: "erupts"},
			{Name: "Tremor", Description: "shakes"},
		},
	}
	tracker := LairActionTracker{LastUsedName: "Magma Eruption"}

	plan := BuildLairActionPlan(info, tracker)
	require.Len(t, plan.AvailableActions, 1)
	assert.Equal(t, "Tremor", plan.AvailableActions[0].Name)
	require.Len(t, plan.DisabledActions, 1)
	assert.Equal(t, "Magma Eruption", plan.DisabledActions[0].Name)
}

func TestBuildLairActionPlan_NoPreviousUse(t *testing.T) {
	info := &LairInfo{
		Actions: []LairAction{
			{Name: "Magma Eruption", Description: "erupts"},
			{Name: "Tremor", Description: "shakes"},
		},
	}
	tracker := LairActionTracker{}

	plan := BuildLairActionPlan(info, tracker)
	require.Len(t, plan.AvailableActions, 2)
	assert.Len(t, plan.DisabledActions, 0)
}

// --- TDD Cycle 12: HasLegendaryActions / HasLairActions ---

func TestHasLegendaryActions(t *testing.T) {
	abilities := []CreatureAbilityEntry{
		{Name: "Legendary Actions", Description: "The dragon can take 3 legendary actions."},
		{Name: "Detect", Description: "Check."},
	}
	assert.True(t, HasLegendaryActions(abilities))
	assert.False(t, HasLegendaryActions([]CreatureAbilityEntry{{Name: "Keen Senses"}}))
	assert.False(t, HasLegendaryActions(nil))
}

func TestHasLairActions(t *testing.T) {
	abilities := []CreatureAbilityEntry{
		{Name: "Lair Actions", Description: "On initiative count 20..."},
		{Name: "Tremor", Description: "shakes."},
	}
	assert.True(t, HasLairActions(abilities))
	assert.False(t, HasLairActions([]CreatureAbilityEntry{{Name: "Keen Senses"}}))
	assert.False(t, HasLairActions(nil))
}

// --- TDD Cycle 13: Turn Queue ---

func TestBuildTurnQueueEntries(t *testing.T) {
	dragonID := uuid.New()
	pcID := uuid.New()

	combatants := []refdata.Combatant{
		{ID: pcID, DisplayName: "Aragorn", InitiativeRoll: 18, InitiativeOrder: 1, IsNpc: false, IsAlive: true},
		{ID: dragonID, DisplayName: "Adult Red Dragon", InitiativeRoll: 15, InitiativeOrder: 2, IsNpc: true, IsAlive: true, CreatureRefID: sql.NullString{String: "adult-red-dragon", Valid: true}},
	}

	legendaryCreatures := map[uuid.UUID]string{dragonID: "Adult Red Dragon"}
	lairCreatures := map[uuid.UUID]string{dragonID: "Adult Red Dragon"}

	entries := BuildTurnQueueEntries(combatants, legendaryCreatures, lairCreatures)
	require.GreaterOrEqual(t, len(entries), 3)

	assert.Equal(t, TurnQueueLairAction, entries[0].Type)
	assert.Equal(t, int32(20), entries[0].Initiative)

	legendaryFound := false
	for _, e := range entries {
		if e.Type == TurnQueueLegendary {
			legendaryFound = true
			assert.Equal(t, "Adult Red Dragon", e.DisplayName)
		}
	}
	assert.True(t, legendaryFound)
}

func TestBuildTurnQueueEntries_NoLegendaryOrLair(t *testing.T) {
	combatants := []refdata.Combatant{
		{ID: uuid.New(), DisplayName: "Aragorn", InitiativeRoll: 18, IsNpc: false, IsAlive: true},
		{ID: uuid.New(), DisplayName: "Goblin", InitiativeRoll: 12, IsNpc: true, IsAlive: true},
	}

	entries := BuildTurnQueueEntries(combatants, nil, nil)
	require.Len(t, entries, 2)
	assert.Equal(t, TurnQueueCombatant, entries[0].Type)
	assert.Equal(t, TurnQueueCombatant, entries[1].Type)
}

// --- Edge cases ---

func TestParseLegendaryActions_LairActionsSectionStopsLegendaryParsing(t *testing.T) {
	abilities := []CreatureAbilityEntry{
		{Name: "Legendary Actions", Description: "The dragon can take 3 legendary actions."},
		{Name: "Detect", Description: "Check."},
		{Name: "Lair Actions", Description: "On initiative count 20..."},
		{Name: "Magma", Description: "erupts"},
	}

	info := ParseLegendaryInfo(abilities)
	require.NotNil(t, info)
	require.Len(t, info.Actions, 1)
	assert.Equal(t, "Detect", info.Actions[0].Name)
}

func TestParseLegendaryActions_DefaultBudgetWhenNotParseable(t *testing.T) {
	abilities := []CreatureAbilityEntry{
		{Name: "Legendary Actions", Description: "This creature has legendary actions."},
		{Name: "Detect", Description: "Check."},
	}

	info := ParseLegendaryInfo(abilities)
	require.NotNil(t, info)
	assert.Equal(t, 3, info.Budget)
}

func TestParseLairInfo_StopsAtNextSection(t *testing.T) {
	abilities := []CreatureAbilityEntry{
		{Name: "Lair Actions", Description: "On initiative count 20..."},
		{Name: "Magma Eruption", Description: "Magma erupts."},
		{Name: "Tremor", Description: "A tremor shakes."},
		{Name: "Reactions", Description: "The dragon can take reactions."},
		{Name: "Tail Swipe", Description: "When hit, the dragon swipes."},
	}

	info := ParseLairInfo(abilities)
	require.NotNil(t, info)
	require.Len(t, info.Actions, 2)
	assert.Equal(t, "Magma Eruption", info.Actions[0].Name)
	assert.Equal(t, "Tremor", info.Actions[1].Name)
}

func TestLegendaryActionBudget_ZeroCost(t *testing.T) {
	budget := NewLegendaryActionBudget(3)
	b2, err := budget.Spend(0)
	require.NoError(t, err)
	assert.Equal(t, 3, b2.Remaining)
}

// --- TDD Cycle F-C04: Lair action loses initiative ties ---

func TestBuildTurnQueueEntries_LairActionLosesTies(t *testing.T) {
	dragonID := uuid.New()
	pcID := uuid.New()

	combatants := []refdata.Combatant{
		{ID: pcID, DisplayName: "Rogue", InitiativeRoll: 20, InitiativeOrder: 1, IsNpc: false, IsAlive: true},
		{ID: dragonID, DisplayName: "Adult Red Dragon", InitiativeRoll: 15, InitiativeOrder: 2, IsNpc: true, IsAlive: true},
	}

	lairCreatures := map[uuid.UUID]string{dragonID: "Adult Red Dragon"}

	entries := BuildTurnQueueEntries(combatants, nil, lairCreatures)

	// Find the positions of the combatant at init 20 and the lair action at init 20
	var rogueIdx, lairIdx int
	for i, e := range entries {
		if e.Type == TurnQueueCombatant && e.DisplayName == "Rogue" {
			rogueIdx = i
		}
		if e.Type == TurnQueueLairAction {
			lairIdx = i
		}
	}

	// Lair action must lose ties: combatant at init 20 goes before lair action at init 20
	assert.Less(t, rogueIdx, lairIdx, "combatant at initiative 20 should appear before lair action (lair loses ties)")
}

// --- TDD Cycle F-H05: Lair action tracker persistence ---

func TestLairActionTracker_PersistsSurvivesRehydration(t *testing.T) {
	// Simulate: use an action, persist it, then create a new tracker from stored state.
	tracker := LairActionTracker{}
	tracker = tracker.Use("Magma Eruption")

	// Simulate persistence: the stored value is the LastUsedName.
	storedValue := tracker.LastUsedName
	assert.Equal(t, "Magma Eruption", storedValue)

	// Simulate rehydration: a fresh tracker built from stored state.
	rehydrated := LairActionTracker{LastUsedName: storedValue}
	assert.False(t, rehydrated.CanUse("Magma Eruption"), "rehydrated tracker should block repeated action")
	assert.True(t, rehydrated.CanUse("Tremor"), "rehydrated tracker should allow different action")
}
