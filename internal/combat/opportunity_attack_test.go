package combat

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

// helper to make a combatant at a given position
func makeCombatant(name string, col string, row int32, isNpc bool) refdata.Combatant {
	return refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: name,
		PositionCol: col,
		PositionRow: row,
		IsNpc:       isNpc,
		IsAlive:     true,
	}
}

// --- TDD Cycle 1: Basic OA trigger detection ---

func TestDetectOpportunityAttacks_MoverExitsHostileReach(t *testing.T) {
	// PC at B2, hostile NPC at B3 (adjacent, 5ft)
	// PC moves from B2 to B2 -> D2 (path: B2, C2, D2)
	// At C2, PC is still within 5ft of NPC at B3 (Chebyshev dist = 1)
	// At D2, PC is 10ft from NPC at B3 (Chebyshev dist = 2)
	// So the exit tile is C2 (last tile in reach before leaving)

	mover := makeCombatant("Fighter", "B", 2, false)
	hostile := makeCombatant("Goblin", "B", 3, true)

	turn := refdata.Turn{HasDisengaged: false}
	hostileTurn := refdata.Turn{ReactionUsed: false}

	path := []pathfinding.Point{
		{Col: 1, Row: 1}, // B2 (0-indexed)
		{Col: 2, Row: 1}, // C2
		{Col: 3, Row: 1}, // D2
	}

	triggers := DetectOpportunityAttacks(mover, path, []refdata.Combatant{mover, hostile}, turn, map[uuid.UUID]refdata.Turn{hostile.ID: hostileTurn}, nil)

	require.Len(t, triggers, 1)
	assert.Equal(t, hostile.ID, triggers[0].HostileID)
	assert.Equal(t, mover.ID, triggers[0].TargetID)
	// Exit tile should be C2 (the last tile still in reach) — 0-indexed col=2, row=1
	assert.Equal(t, 2, triggers[0].ExitCol)
	assert.Equal(t, 1, triggers[0].ExitRow)
}

// --- TDD Cycle 2: Disengage suppresses OA ---

func TestDetectOpportunityAttacks_DisengageSuppresses(t *testing.T) {
	mover := makeCombatant("Rogue", "B", 2, false)
	hostile := makeCombatant("Goblin", "B", 3, true)

	turn := refdata.Turn{HasDisengaged: true}
	hostileTurn := refdata.Turn{ReactionUsed: false}

	path := []pathfinding.Point{
		{Col: 1, Row: 1},
		{Col: 2, Row: 1},
		{Col: 3, Row: 1},
	}

	triggers := DetectOpportunityAttacks(mover, path, []refdata.Combatant{mover, hostile}, turn, map[uuid.UUID]refdata.Turn{hostile.ID: hostileTurn}, nil)
	assert.Empty(t, triggers)
}

// --- TDD Cycle 3: Hostile already used reaction → no OA ---

func TestDetectOpportunityAttacks_HostileReactionUsed(t *testing.T) {
	mover := makeCombatant("Fighter", "B", 2, false)
	hostile := makeCombatant("Goblin", "B", 3, true)

	turn := refdata.Turn{HasDisengaged: false}
	hostileTurn := refdata.Turn{ReactionUsed: true}

	path := []pathfinding.Point{
		{Col: 1, Row: 1},
		{Col: 2, Row: 1},
		{Col: 3, Row: 1},
	}

	triggers := DetectOpportunityAttacks(mover, path, []refdata.Combatant{mover, hostile}, turn, map[uuid.UUID]refdata.Turn{hostile.ID: hostileTurn}, nil)
	assert.Empty(t, triggers)
}

// --- TDD Cycle 4: Mover stays in reach → no OA ---

func TestDetectOpportunityAttacks_MoverStaysInReach(t *testing.T) {
	// PC at B2, hostile at B3, PC moves to C2 (still 5ft from B3)
	mover := makeCombatant("Fighter", "B", 2, false)
	hostile := makeCombatant("Goblin", "B", 3, true)

	turn := refdata.Turn{HasDisengaged: false}
	hostileTurn := refdata.Turn{ReactionUsed: false}

	path := []pathfinding.Point{
		{Col: 1, Row: 1}, // B2
		{Col: 2, Row: 1}, // C2 (still 5ft from B3)
	}

	triggers := DetectOpportunityAttacks(mover, path, []refdata.Combatant{mover, hostile}, turn, map[uuid.UUID]refdata.Turn{hostile.ID: hostileTurn}, nil)
	assert.Empty(t, triggers)
}

// --- TDD Cycle 5: Mover starts outside reach → no OA ---

func TestDetectOpportunityAttacks_MoverStartsOutsideReach(t *testing.T) {
	// PC at D2, hostile at A3 (dist = 3 squares = 15ft), moves further away
	mover := makeCombatant("Fighter", "D", 2, false)
	hostile := makeCombatant("Goblin", "A", 3, true)

	turn := refdata.Turn{HasDisengaged: false}
	hostileTurn := refdata.Turn{ReactionUsed: false}

	path := []pathfinding.Point{
		{Col: 3, Row: 1}, // D2
		{Col: 4, Row: 1}, // E2
		{Col: 5, Row: 1}, // F2
	}

	triggers := DetectOpportunityAttacks(mover, path, []refdata.Combatant{mover, hostile}, turn, map[uuid.UUID]refdata.Turn{hostile.ID: hostileTurn}, nil)
	assert.Empty(t, triggers)
}

// --- TDD Cycle 6: Dead hostile → no OA ---

func TestDetectOpportunityAttacks_DeadHostileNoOA(t *testing.T) {
	mover := makeCombatant("Fighter", "B", 2, false)
	hostile := makeCombatant("Goblin", "B", 3, true)
	hostile.IsAlive = false

	turn := refdata.Turn{HasDisengaged: false}

	path := []pathfinding.Point{
		{Col: 1, Row: 1},
		{Col: 2, Row: 1},
		{Col: 3, Row: 1},
	}

	triggers := DetectOpportunityAttacks(mover, path, []refdata.Combatant{mover, hostile}, turn, map[uuid.UUID]refdata.Turn{}, nil)
	assert.Empty(t, triggers)
}

// --- TDD Cycle 7: Same faction → no OA ---

func TestDetectOpportunityAttacks_SameFactionNoOA(t *testing.T) {
	// Two PCs — allies, should not trigger OA
	mover := makeCombatant("Fighter", "B", 2, false)
	ally := makeCombatant("Cleric", "B", 3, false)

	turn := refdata.Turn{HasDisengaged: false}

	path := []pathfinding.Point{
		{Col: 1, Row: 1},
		{Col: 2, Row: 1},
		{Col: 3, Row: 1},
	}

	triggers := DetectOpportunityAttacks(mover, path, []refdata.Combatant{mover, ally}, turn, map[uuid.UUID]refdata.Turn{}, nil)
	assert.Empty(t, triggers)
}

// --- TDD Cycle 8: Multiple hostiles → multiple OA triggers ---

func TestDetectOpportunityAttacks_MultipleHostiles(t *testing.T) {
	// PC at C3, two hostiles adjacent: B3 and D3
	// PC moves from C3 → C1 (path: C3, C2, C1)
	// Both hostiles lose reach at C2 (dist from B3=1, dist from D3=1) then at C1 (dist=2 from both)
	// Actually: B3 to C2 = chebyshev(1,1, 2,2) = max(1,1) = 1 (in reach)
	// B3 to C1 = chebyshev(1,2, 2,0) = max(1,2) = 2 (out of reach)
	// So exit tile for B3 is C2

	// D3 to C2 = chebyshev(3,2, 2,1) = max(1,1) = 1 (in reach)
	// D3 to C1 = chebyshev(3,2, 2,0) = max(1,2) = 2 (out of reach)
	// So exit tile for D3 is C2

	mover := makeCombatant("Fighter", "C", 3, false)
	hostile1 := makeCombatant("Goblin1", "B", 3, true)
	hostile2 := makeCombatant("Goblin2", "D", 3, true)

	turn := refdata.Turn{HasDisengaged: false}
	hostileTurns := map[uuid.UUID]refdata.Turn{
		hostile1.ID: {ReactionUsed: false},
		hostile2.ID: {ReactionUsed: false},
	}

	path := []pathfinding.Point{
		{Col: 2, Row: 2}, // C3 (0-indexed)
		{Col: 2, Row: 1}, // C2
		{Col: 2, Row: 0}, // C1
	}

	triggers := DetectOpportunityAttacks(mover, path, []refdata.Combatant{mover, hostile1, hostile2}, turn, hostileTurns, nil)
	assert.Len(t, triggers, 2)
}

// --- TDD Cycle 9: Reach weapon (10ft) for NPC ---

func TestDetectOpportunityAttacks_ReachWeapon10ft(t *testing.T) {
	// NPC with 10ft reach at B3
	// PC at B2 moves to B2 → E2 (path: B2, C2, D2, E2)
	// 10ft reach = 2 squares
	// B3(1,2) to B2(1,1) = 1 (in reach)
	// B3(1,2) to C2(2,1) = 1 (in reach)
	// B3(1,2) to D2(3,1) = 2 (in reach)
	// B3(1,2) to E2(4,1) = 3 (out of reach)
	// Exit tile = D2

	mover := makeCombatant("Fighter", "B", 2, false)
	hostile := makeCombatant("Ogre", "B", 3, true)
	hostile.CreatureRefID.Valid = true
	hostile.CreatureRefID.String = "ogre"

	turn := refdata.Turn{HasDisengaged: false}
	hostileTurn := refdata.Turn{ReactionUsed: false}

	creatureAttacks := map[string][]CreatureAttackEntry{
		"ogre": {
			{Name: "Greatclub", ToHit: 6, Damage: "2d8+4", DamageType: "bludgeoning", ReachFt: 10},
		},
	}

	path := []pathfinding.Point{
		{Col: 1, Row: 1}, // B2
		{Col: 2, Row: 1}, // C2
		{Col: 3, Row: 1}, // D2
		{Col: 4, Row: 1}, // E2
	}

	triggers := DetectOpportunityAttacks(mover, path, []refdata.Combatant{mover, hostile}, turn, map[uuid.UUID]refdata.Turn{hostile.ID: hostileTurn}, creatureAttacks)

	require.Len(t, triggers, 1)
	assert.Equal(t, 3, triggers[0].ExitCol) // D2 (0-indexed col=3)
	assert.Equal(t, 1, triggers[0].ExitRow)
}

// --- TDD Cycle 10: Short path (single step) ---

func TestDetectOpportunityAttacks_SingleStepPath(t *testing.T) {
	// Only one point in path → no OA possible
	mover := makeCombatant("Fighter", "B", 2, false)
	hostile := makeCombatant("Goblin", "B", 3, true)

	turn := refdata.Turn{HasDisengaged: false}

	path := []pathfinding.Point{
		{Col: 1, Row: 1}, // B2
	}

	triggers := DetectOpportunityAttacks(mover, path, []refdata.Combatant{mover, hostile}, turn, map[uuid.UUID]refdata.Turn{}, nil)
	assert.Empty(t, triggers)
}

// --- TDD Cycle 11: FormatOAPrompt ---

func TestFormatOAPrompt(t *testing.T) {
	trigger := OATrigger{
		TargetName: "Goblin",
		ExitLabel:  "C2",
	}

	msg := FormatOAPrompt(trigger)
	assert.Contains(t, msg, "Goblin")
	assert.Contains(t, msg, "C2")
	assert.Contains(t, msg, "/reaction oa")
}

// --- TDD Cycle 12: ParseCreatureAttacks ---

func TestParseCreatureAttacks(t *testing.T) {
	raw := []byte(`[{"name":"Bite","to_hit":5,"damage":"1d6+3","damage_type":"piercing","reach_ft":5},{"name":"Greatclub","to_hit":6,"damage":"2d8+4","damage_type":"bludgeoning","reach_ft":10}]`)
	attacks, err := ParseCreatureAttacks(raw)
	require.NoError(t, err)
	require.Len(t, attacks, 2)
	assert.Equal(t, 5, attacks[0].ReachFt)
	assert.Equal(t, 10, attacks[1].ReachFt)
}

func TestParseCreatureAttacks_Empty(t *testing.T) {
	attacks, err := ParseCreatureAttacks(nil)
	assert.NoError(t, err)
	assert.Nil(t, attacks)
}

// --- TDD Cycle 13: ExitLabel is correctly formatted ---

func TestDetectOpportunityAttacks_ExitLabel(t *testing.T) {
	mover := makeCombatant("Fighter", "B", 2, false)
	hostile := makeCombatant("Goblin", "B", 3, true)

	turn := refdata.Turn{HasDisengaged: false}
	hostileTurn := refdata.Turn{ReactionUsed: false}

	path := []pathfinding.Point{
		{Col: 1, Row: 1}, // B2
		{Col: 2, Row: 1}, // C2
		{Col: 3, Row: 1}, // D2
	}

	triggers := DetectOpportunityAttacks(mover, path, []refdata.Combatant{mover, hostile}, turn, map[uuid.UUID]refdata.Turn{hostile.ID: hostileTurn}, nil)

	require.Len(t, triggers, 1)
	assert.Equal(t, "C2", triggers[0].ExitLabel)
}

// --- TDD Cycle 14: NPC mover exits PC's reach → PC gets OA ---

func TestDetectOpportunityAttacks_NPCMoverTriggersFromPC(t *testing.T) {
	mover := makeCombatant("Goblin", "B", 2, true)
	hostile := makeCombatant("Fighter", "B", 3, false)

	turn := refdata.Turn{HasDisengaged: false}
	hostileTurn := refdata.Turn{ReactionUsed: false}

	path := []pathfinding.Point{
		{Col: 1, Row: 1},
		{Col: 2, Row: 1},
		{Col: 3, Row: 1},
	}

	triggers := DetectOpportunityAttacks(mover, path, []refdata.Combatant{mover, hostile}, turn, map[uuid.UUID]refdata.Turn{hostile.ID: hostileTurn}, nil)
	require.Len(t, triggers, 1)
	assert.Equal(t, hostile.ID, triggers[0].HostileID)
}

// --- TDD Cycle 15: Hostile not in turn map → still gets OA (reaction not yet used) ---

func TestDetectOpportunityAttacks_HostileNoTurnEntry(t *testing.T) {
	mover := makeCombatant("Fighter", "B", 2, false)
	hostile := makeCombatant("Goblin", "B", 3, true)

	turn := refdata.Turn{HasDisengaged: false}

	path := []pathfinding.Point{
		{Col: 1, Row: 1},
		{Col: 2, Row: 1},
		{Col: 3, Row: 1},
	}

	// No entry in hostileTurns map — reaction hasn't been used (hasn't had their turn yet)
	triggers := DetectOpportunityAttacks(mover, path, []refdata.Combatant{mover, hostile}, turn, map[uuid.UUID]refdata.Turn{}, nil)
	require.Len(t, triggers, 1)
}

// --- TDD Cycle 16: chebyshevDist ---

func TestChebyshevDist(t *testing.T) {
	assert.Equal(t, 0, chebyshevDist(0, 0, 0, 0))
	assert.Equal(t, 1, chebyshevDist(0, 0, 1, 1))
	assert.Equal(t, 3, chebyshevDist(0, 0, 3, 2))
	assert.Equal(t, 5, chebyshevDist(5, 5, 0, 0))
}

// --- TDD Cycle 17: resolveHostileReach ---

func TestResolveHostileReach_Default5ft(t *testing.T) {
	hostile := makeCombatant("Goblin", "A", 1, true)
	assert.Equal(t, 5, resolveHostileReach(hostile, nil, nil))
}

func TestResolveHostileReach_NPCWithReachWeapon(t *testing.T) {
	hostile := makeCombatant("Ogre", "A", 1, true)
	hostile.CreatureRefID.Valid = true
	hostile.CreatureRefID.String = "ogre"

	attacks := map[string][]CreatureAttackEntry{
		"ogre": {
			{Name: "Fist", ReachFt: 5},
			{Name: "Greatclub", ReachFt: 10},
		},
	}
	assert.Equal(t, 10, resolveHostileReach(hostile, attacks, nil))
}

func TestResolveHostileReach_PCDefault(t *testing.T) {
	hostile := makeCombatant("Fighter", "A", 1, false)
	assert.Equal(t, 5, resolveHostileReach(hostile, nil, nil))
}

// C-H10: PC with a reach weapon (glaive) should get 10ft reach without
// the caller needing to pass a pcReachByID override map.
func TestResolveHostileReach_PCWithReachWeapon(t *testing.T) {
	hostile := makeCombatant("Fighter", "A", 1, false)
	pcWeaponProps := map[uuid.UUID][]string{
		hostile.ID: {"two-handed", "heavy", "reach"},
	}
	assert.Equal(t, 10, resolveHostileReach(hostile, nil, pcWeaponProps))
}

func TestDetectOA_PCWithReachWeapon_NoOverrideMap(t *testing.T) {
	// NPC mover at C2, hostile PC with glaive (reach) at A2.
	// 10ft reach = 2 squares. Mover starts at dist 2 (in reach), moves to dist 3 (out).
	mover := makeCombatant("Goblin", "C", 2, true)
	hostile := makeCombatant("Fighter", "A", 2, false)

	turn := refdata.Turn{HasDisengaged: false}
	hostileTurn := refdata.Turn{ReactionUsed: false}

	path := []pathfinding.Point{
		{Col: 2, Row: 1}, // C2 (dist 2 from A2, in 10ft reach)
		{Col: 3, Row: 1}, // D2 (dist 3, out of reach)
		{Col: 4, Row: 1}, // E2
	}

	pcWeaponProps := map[uuid.UUID][]string{
		hostile.ID: {"two-handed", "heavy", "reach"},
	}
	triggers := DetectOpportunityAttacksWithReach(
		mover, path, []refdata.Combatant{mover, hostile}, turn,
		map[uuid.UUID]refdata.Turn{hostile.ID: hostileTurn}, nil, nil, pcWeaponProps,
	)
	require.Len(t, triggers, 1)
	assert.Equal(t, hostile.ID, triggers[0].HostileID)
}

// --- TDD Cycle 18: findReachExit edge cases ---

func TestFindReachExit_EmptyPath(t *testing.T) {
	_, _, exits := findReachExit(nil, 0, 0, 1)
	assert.False(t, exits)
}

func TestFindReachExit_MoverNeverLeaves(t *testing.T) {
	// Path stays within reach the whole time
	path := []pathfinding.Point{
		{Col: 0, Row: 0},
		{Col: 1, Row: 0},
		{Col: 0, Row: 1},
	}
	_, _, exits := findReachExit(path, 0, 0, 1)
	assert.False(t, exits)
}

func TestFindReachExit_MoverLeavesImmediately(t *testing.T) {
	// Start at (1,1), hostile at (1,1), move to (3,3) — first step exits
	path := []pathfinding.Point{
		{Col: 1, Row: 1},
		{Col: 3, Row: 3},
	}
	exitCol, exitRow, exits := findReachExit(path, 1, 1, 1)
	assert.True(t, exits)
	assert.Equal(t, 1, exitCol)
	assert.Equal(t, 1, exitRow)
}

// --- TDD Cycle 19: combatantGridPos ---

func TestCombatantGridPos_Valid(t *testing.T) {
	c := makeCombatant("Test", "C", 4, false) // C4 → col=2, row=3 (0-indexed)
	col, row := combatantGridPos(c)
	assert.Equal(t, 2, col)
	assert.Equal(t, 3, row)
}

func TestCombatantGridPos_InvalidCol(t *testing.T) {
	c := makeCombatant("Test", "", 0, false)
	col, row := combatantGridPos(c)
	assert.Equal(t, -1, col)
	assert.Equal(t, -1, row)
}

// --- TDD Cycle 20: Path exits and re-enters reach → only first exit triggers ---

func TestDetectOpportunityAttacks_ExitAndReenter(t *testing.T) {
	// PC at B2, hostile at C3
	// Path: B2 → D2 → D3 (exits reach at D2, re-enters at D3 which is within reach of C3)
	// Should trigger OA at exit tile B2 (last in-reach tile before leaving)
	// Wait: B2(1,1), C3(2,2). chebyshev(1,1,2,2)=1, in reach.
	// D2(3,1): chebyshev(3,1,2,2)=1, still in reach.
	// D3(3,2): chebyshev(3,2,2,2)=1, still in reach.
	// That doesn't exit. Let me use a different path:
	// PC at B2, hostile at B3. Path: B2 → D2 → D4
	// B2(1,1) → B3(1,2): dist=1, in reach
	// D2(3,1) → B3(1,2): dist=2, out of reach → exit at B2
	// D4(3,3) → B3(1,2): dist=2, out of reach
	// Only one trigger at B2

	mover := makeCombatant("Fighter", "B", 2, false)
	hostile := makeCombatant("Goblin", "B", 3, true)

	turn := refdata.Turn{HasDisengaged: false}
	hostileTurn := refdata.Turn{ReactionUsed: false}

	path := []pathfinding.Point{
		{Col: 1, Row: 1}, // B2
		{Col: 3, Row: 1}, // D2 (out of reach)
		{Col: 3, Row: 3}, // D4 (still out)
	}

	triggers := DetectOpportunityAttacks(mover, path, []refdata.Combatant{mover, hostile}, turn, map[uuid.UUID]refdata.Turn{hostile.ID: hostileTurn}, nil)
	require.Len(t, triggers, 1)
	// Exit tile is B2 (the last tile in reach)
	assert.Equal(t, 1, triggers[0].ExitCol)
	assert.Equal(t, 1, triggers[0].ExitRow)
}

// --- TDD Cycle 21: NPC reach defaults to 5 when attacks don't specify reach ---

func TestResolveHostileReach_NPCNoReachInAttacks(t *testing.T) {
	hostile := makeCombatant("Skeleton", "A", 1, true)
	hostile.CreatureRefID.Valid = true
	hostile.CreatureRefID.String = "skeleton"

	attacks := map[string][]CreatureAttackEntry{
		"skeleton": {
			{Name: "Shortsword", ReachFt: 0}, // 0 is less than default 5
		},
	}
	// Should still return 5 since 0 < 5
	assert.Equal(t, 5, resolveHostileReach(hostile, attacks, nil))
}

// --- TDD Cycle 22: NPC creature ref not found in attacks map ---

func TestParseCreatureAttacks_InvalidJSON(t *testing.T) {
	_, err := ParseCreatureAttacks([]byte(`not json`))
	assert.Error(t, err)
}

// --- TDD Cycle 23: Hostile with invalid position is skipped ---

func TestDetectOpportunityAttacks_HostileInvalidPosition(t *testing.T) {
	mover := makeCombatant("Fighter", "B", 2, false)
	hostile := makeCombatant("Goblin", "", 0, true) // invalid position

	turn := refdata.Turn{HasDisengaged: false}

	path := []pathfinding.Point{
		{Col: 1, Row: 1},
		{Col: 2, Row: 1},
		{Col: 3, Row: 1},
	}

	triggers := DetectOpportunityAttacks(mover, path, []refdata.Combatant{mover, hostile}, turn, map[uuid.UUID]refdata.Turn{}, nil)
	assert.Empty(t, triggers)
}

// med-24: PC reach-weapon override gives a hostile PC 10ft reach so a
// glaive-wielding fighter triggers OAs from 10ft away.
func TestDetectOpportunityAttacksWithReach_PCReachWeaponOverride(t *testing.T) {
	// Mover at C2 (NPC), hostile PC at A2 with 10ft reach. The mover
	// starts 2 squares (10ft) away — within reach — and walks east to
	// E2, leaving reach.
	mover := makeCombatant("Goblin", "C", 2, true)
	hostile := makeCombatant("PaladinSpear", "A", 2, false)

	turn := refdata.Turn{HasDisengaged: false}
	hostileTurn := refdata.Turn{ReactionUsed: false}

	path := []pathfinding.Point{
		{Col: 2, Row: 1}, // C2 (start, dist 2 from A2 = 10ft, in reach)
		{Col: 3, Row: 1}, // D2 (dist 3 = 15ft, out of reach)
		{Col: 4, Row: 1}, // E2
	}

	pcReach := map[uuid.UUID]int{hostile.ID: 10}
	triggers := DetectOpportunityAttacksWithReach(
		mover, path, []refdata.Combatant{mover, hostile}, turn,
		map[uuid.UUID]refdata.Turn{hostile.ID: hostileTurn}, nil, pcReach, nil,
	)
	if assert.Len(t, triggers, 1) {
		assert.Equal(t, hostile.ID, triggers[0].HostileID)
	}

	// Without the reach override, the same path triggers no OA because
	// 5ft (1 square) reach means the mover started outside reach.
	noReachTriggers := DetectOpportunityAttacksWithReach(
		mover, path, []refdata.Combatant{mover, hostile}, turn,
		map[uuid.UUID]refdata.Turn{hostile.ID: hostileTurn}, nil, nil, nil,
	)
	assert.Empty(t, noReachTriggers, "without reach override the start tile is outside reach")
}

func TestResolveHostileReach_NPCNotInMap(t *testing.T) {
	hostile := makeCombatant("Unknown", "A", 1, true)
	hostile.CreatureRefID.Valid = true
	hostile.CreatureRefID.String = "unknown_creature"

	attacks := map[string][]CreatureAttackEntry{
		"goblin": {{Name: "Scimitar", ReachFt: 5}},
	}
	assert.Equal(t, 5, resolveHostileReach(hostile, attacks, nil))
}

// --- SR-028: pending OA tracker + end-of-round forfeit sweep ---

func TestRecordPendingOA_ForfeitsCancelsThroughNotifier(t *testing.T) {
	encounterID := uuid.New()
	notifier := &fakeDMNotifier{}

	svc := NewService(defaultMockStore())
	svc.SetDMNotifier(notifier)

	svc.RecordPendingOA(encounterID, "oa-item-1")
	svc.RecordPendingOA(encounterID, "oa-item-2")

	svc.ForfeitPendingOAs(context.Background(), encounterID)

	require.Len(t, notifier.cancels, 2, "expected each pending OA to be cancelled")
	assert.Equal(t, "oa-item-1", notifier.cancels[0])
	assert.Equal(t, "oa-item-2", notifier.cancels[1])

	// Second sweep is a no-op (slice already drained).
	svc.ForfeitPendingOAs(context.Background(), encounterID)
	assert.Len(t, notifier.cancels, 2, "second sweep should drain to empty")
}

func TestRecordPendingOA_EmptyItemIDIgnored(t *testing.T) {
	encounterID := uuid.New()
	notifier := &fakeDMNotifier{}

	svc := NewService(defaultMockStore())
	svc.SetDMNotifier(notifier)

	svc.RecordPendingOA(encounterID, "")
	svc.ForfeitPendingOAs(context.Background(), encounterID)

	assert.Empty(t, notifier.cancels, "empty item IDs (no #dm-queue configured) are skipped")
}

func TestForfeitPendingOAs_NoNotifierStillDrains(t *testing.T) {
	encounterID := uuid.New()

	svc := NewService(defaultMockStore())
	// No SetDMNotifier — sweep must still drain so a later wiring of the
	// notifier doesn't re-cancel the same stale items.
	svc.RecordPendingOA(encounterID, "oa-orphan-1")
	svc.ForfeitPendingOAs(context.Background(), encounterID)

	// Re-wire after the sweep and confirm the tracker is empty.
	notifier := &fakeDMNotifier{}
	svc.SetDMNotifier(notifier)
	svc.ForfeitPendingOAs(context.Background(), encounterID)
	assert.Empty(t, notifier.cancels, "post-drain sweep should not re-cancel stale items")
}

func TestForfeitPendingOAs_CancelErrorSwallowed(t *testing.T) {
	encounterID := uuid.New()
	notifier := &fakeDMNotifier{cancelErr: errors.New("discord 500")}

	svc := NewService(defaultMockStore())
	svc.SetDMNotifier(notifier)

	svc.RecordPendingOA(encounterID, "oa-item-1")
	svc.RecordPendingOA(encounterID, "oa-item-2")

	// Must not panic / propagate the error; both cancel attempts still fire.
	svc.ForfeitPendingOAs(context.Background(), encounterID)
	assert.Len(t, notifier.cancels, 2)
}

func TestAdvanceTurn_RoundAdvanceForfeitsPendingOAs(t *testing.T) {
	// When the active turn completes and every combatant has already gone
	// in this round, AdvanceTurn rolls the round forward via advanceRound.
	// advanceRound must drain the pending OA tracker and cancel each
	// remaining prompt so DM-controlled hostiles' unanswered OAs visibly
	// forfeit at end of round (SR-028).
	ctx := context.Background()
	encounterID := uuid.New()
	combatant1ID := uuid.New()
	combatant2ID := uuid.New()
	activeTurnID := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(_ context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:            id,
			Status:        "active",
			RoundNumber:   1,
			CurrentTurnID: uuid.NullUUID{UUID: activeTurnID, Valid: true},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: combatant1ID, InitiativeOrder: 1, DisplayName: "Aria", Conditions: rawEmpty(), IsAlive: true},
			{ID: combatant2ID, InitiativeOrder: 2, DisplayName: "Goblin", Conditions: rawEmpty(), IsAlive: true},
		}, nil
	}
	store.getTurnFn = func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: activeTurnID, CombatantID: combatant2ID, RoundNumber: 1, Status: "active"}, nil
	}
	store.completeTurnFn = func(_ context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: id, Status: "completed"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(_ context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		if arg.RoundNumber == 1 {
			return []refdata.Turn{
				{CombatantID: combatant1ID, Status: "completed"},
				{CombatantID: combatant2ID, Status: "completed"},
			}, nil
		}
		return []refdata.Turn{}, nil
	}
	store.updateEncounterRoundFn = func(_ context.Context, arg refdata.UpdateEncounterRoundParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID, RoundNumber: arg.RoundNumber}, nil
	}
	store.createTurnFn = func(_ context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber, Status: arg.Status}, nil
	}

	notifier := &fakeDMNotifier{}
	svc := NewService(store)
	svc.SetDMNotifier(notifier)

	// Pretend an OA prompt was posted earlier in this round and never
	// resolved. Round advance should now cancel it.
	svc.RecordPendingOA(encounterID, "stale-oa-item")

	_, err := svc.AdvanceTurn(ctx, encounterID)
	require.NoError(t, err)

	require.Len(t, notifier.cancels, 1, "round advance should forfeit the pending OA")
	assert.Equal(t, "stale-oa-item", notifier.cancels[0])
}

// rawEmpty is a local helper that returns an empty conditions JSONB array
// — extracted so the OA round-forfeit test doesn't pull in encoding/json
// just to spell []byte("[]").
func rawEmpty() []byte { return []byte(`[]`) }
