package combat

import (
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

func toNullRawMessage(raw json.RawMessage) pqtype.NullRawMessage {
	return pqtype.NullRawMessage{RawMessage: raw, Valid: true}
}

func filterStepsByType(steps []TurnStep, typ string) []TurnStep {
	var filtered []TurnStep
	for _, s := range steps {
		if s.Type == typ {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// testTerrainGrid creates a flat, open terrain grid of the given size.
func testTerrainGrid(w, h int) []renderer.TerrainType {
	grid := make([]renderer.TerrainType, w*h)
	for i := range grid {
		grid[i] = renderer.TerrainOpenGround
	}
	return grid
}

// --- TDD Cycle 1: TurnPlan types and BuildTurnPlan for simple creature ---

func TestBuildTurnPlan_SimpleCreature_SingleAttack(t *testing.T) {
	npcID := uuid.New()
	pcID := uuid.New()
	encounterID := uuid.New()

	npc := refdata.Combatant{
		ID:            npcID,
		EncounterID:   encounterID,
		DisplayName:   "Goblin",
		PositionCol:   "A",
		PositionRow:   1,
		IsNpc:         true,
		IsAlive:       true,
		HpCurrent:     10,
		CreatureRefID: sql.NullString{String: "goblin", Valid: true},
	}

	pc := refdata.Combatant{
		ID:          pcID,
		EncounterID: encounterID,
		DisplayName: "Aragorn",
		PositionCol: "A",
		PositionRow: 3,
		IsNpc:       false,
		IsAlive:     true,
		HpCurrent:   45,
		Ac:          16,
	}

	creature := refdata.Creature{
		ID:            "goblin",
		Name:          "Goblin",
		Size:          "Small",
		Speed:         json.RawMessage(`{"walk":30}`),
		Attacks:       json.RawMessage(`[{"name":"Scimitar","to_hit":4,"damage":"1d6+2","damage_type":"slashing","reach_ft":5}]`),
		AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`),
	}

	grid := &pathfinding.Grid{
		Width:   5,
		Height:  5,
		Terrain: testTerrainGrid(5, 5),
	}

	combatants := []refdata.Combatant{npc, pc}

	plan, err := BuildTurnPlan(BuildTurnPlanInput{
		Combatant:  npc,
		Creature:   creature,
		Combatants: combatants,
		Grid:       grid,
		Reactions:  nil,
		SpeedFt:    30,
	})
	require.NoError(t, err)
	assert.Equal(t, npcID, plan.CombatantID)
	assert.Equal(t, "Goblin", plan.DisplayName)

	// Should have a movement step and an attack step
	require.GreaterOrEqual(t, len(plan.Steps), 2)

	// First step should be movement
	assert.Equal(t, StepTypeMovement, plan.Steps[0].Type)
	require.NotNil(t, plan.Steps[0].Movement)
	assert.True(t, plan.Steps[0].Suggested)

	// Second step should be an attack
	assert.Equal(t, StepTypeAttack, plan.Steps[1].Type)
	require.NotNil(t, plan.Steps[1].Attack)
	assert.Equal(t, "Scimitar", plan.Steps[1].Attack.WeaponName)
	assert.Equal(t, 4, plan.Steps[1].Attack.ToHit)
	assert.Equal(t, "1d6+2", plan.Steps[1].Attack.DamageDice)
	assert.Equal(t, pcID, plan.Steps[1].Attack.TargetID)
	assert.Equal(t, "Aragorn", plan.Steps[1].Attack.TargetName)
}

// --- TDD Cycle 2: Multiattack creature ---

func TestBuildTurnPlan_MultiattackCreature(t *testing.T) {
	npcID := uuid.New()
	pcID := uuid.New()
	encounterID := uuid.New()

	npc := refdata.Combatant{
		ID:            npcID,
		EncounterID:   encounterID,
		DisplayName:   "Bandit Captain",
		PositionCol:   "B",
		PositionRow:   2,
		IsNpc:         true,
		IsAlive:       true,
		HpCurrent:     65,
		CreatureRefID: sql.NullString{String: "bandit-captain", Valid: true},
	}

	pc := refdata.Combatant{
		ID:          pcID,
		EncounterID: encounterID,
		DisplayName: "Legolas",
		PositionCol: "B",
		PositionRow: 3,
		IsNpc:       false,
		IsAlive:     true,
		HpCurrent:   38,
		Ac:          15,
	}

	creature := refdata.Creature{
		ID:   "bandit-captain",
		Name: "Bandit Captain",
		Size: "Medium",
		Speed: json.RawMessage(`{"walk":30}`),
		Attacks: json.RawMessage(`[
			{"name":"Scimitar","to_hit":5,"damage":"1d6+3","damage_type":"slashing","reach_ft":5},
			{"name":"Dagger","to_hit":5,"damage":"1d4+3","damage_type":"piercing","reach_ft":5}
		]`),
		Abilities: toNullRawMessage(json.RawMessage(`[
			{"name":"Multiattack","description":"The captain makes three melee attacks: two with its scimitar and one with its dagger."}
		]`)),
		AbilityScores: json.RawMessage(`{"str":15,"dex":16,"con":14,"int":14,"wis":11,"cha":14}`),
	}

	grid := &pathfinding.Grid{
		Width:   5,
		Height:  5,
		Terrain: testTerrainGrid(5, 5),
	}

	plan, err := BuildTurnPlan(BuildTurnPlanInput{
		Combatant:  npc,
		Creature:   creature,
		Combatants: []refdata.Combatant{npc, pc},
		Grid:       grid,
		SpeedFt:    30,
	})
	require.NoError(t, err)

	// Should have 3 attack steps (2 scimitar + 1 dagger) — no movement needed (adjacent)
	attackSteps := filterStepsByType(plan.Steps, StepTypeAttack)
	require.Len(t, attackSteps, 3)

	// First two should be scimitar, last should be dagger
	assert.Equal(t, "Scimitar", attackSteps[0].Attack.WeaponName)
	assert.Equal(t, "Scimitar", attackSteps[1].Attack.WeaponName)
	assert.Equal(t, "Dagger", attackSteps[2].Attack.WeaponName)

	// All should target the PC
	for _, s := range attackSteps {
		assert.Equal(t, pcID, s.Attack.TargetID)
	}
}

// --- TDD Cycle 3: Recharge ability ---

func TestBuildTurnPlan_RechargeAbility(t *testing.T) {
	npcID := uuid.New()
	pcID := uuid.New()
	encounterID := uuid.New()

	npc := refdata.Combatant{
		ID:            npcID,
		EncounterID:   encounterID,
		DisplayName:   "Young Blue Dragon",
		PositionCol:   "C",
		PositionRow:   3,
		IsNpc:         true,
		IsAlive:       true,
		HpCurrent:     150,
		CreatureRefID: sql.NullString{String: "young-blue-dragon", Valid: true},
	}

	pc := refdata.Combatant{
		ID:          pcID,
		EncounterID: encounterID,
		DisplayName: "Gimli",
		PositionCol: "C",
		PositionRow: 4,
		IsNpc:       false,
		IsAlive:     true,
		HpCurrent:   60,
		Ac:          18,
	}

	creature := refdata.Creature{
		ID:   "young-blue-dragon",
		Name: "Young Blue Dragon",
		Size: "Large",
		Speed: json.RawMessage(`{"walk":40}`),
		Attacks: json.RawMessage(`[{"name":"Bite","to_hit":9,"damage":"2d10+5","damage_type":"piercing","reach_ft":10}]`),
		Abilities: toNullRawMessage(json.RawMessage(`[
			{"name":"Lightning Breath (Recharge 5-6)","description":"The dragon exhales lightning in a 60-foot line that is 5 feet wide."}
		]`)),
		AbilityScores: json.RawMessage(`{"str":21,"dex":10,"con":19,"int":14,"wis":13,"cha":17}`),
	}

	grid := &pathfinding.Grid{
		Width:   10,
		Height:  10,
		Terrain: testTerrainGrid(10, 10),
	}

	plan, err := BuildTurnPlan(BuildTurnPlanInput{
		Combatant:  npc,
		Creature:   creature,
		Combatants: []refdata.Combatant{npc, pc},
		Grid:       grid,
		SpeedFt:    40,
	})
	require.NoError(t, err)

	// Should include a recharge ability step
	abilitySteps := filterStepsByType(plan.Steps, StepTypeAbility)
	require.Len(t, abilitySteps, 1)
	assert.Equal(t, "Lightning Breath (Recharge 5-6)", abilitySteps[0].Ability.Name)
	assert.True(t, abilitySteps[0].Ability.IsRecharge)
	assert.Equal(t, 5, abilitySteps[0].Ability.RechargeMin)
}

// --- TDD Cycle 4: No movement when already adjacent ---

func TestBuildTurnPlan_NoMovementWhenAdjacent(t *testing.T) {
	npcID := uuid.New()
	pcID := uuid.New()

	npc := refdata.Combatant{
		ID:          npcID,
		DisplayName: "Wolf",
		PositionCol: "C",
		PositionRow: 3,
		IsNpc:       true,
		IsAlive:     true,
		HpCurrent:   11,
	}
	pc := refdata.Combatant{
		ID:          pcID,
		DisplayName: "Frodo",
		PositionCol: "C",
		PositionRow: 4,
		IsNpc:       false,
		IsAlive:     true,
		HpCurrent:   20,
	}

	creature := refdata.Creature{
		Size:    "Medium",
		Speed:   json.RawMessage(`{"walk":40}`),
		Attacks: json.RawMessage(`[{"name":"Bite","to_hit":4,"damage":"2d4+2","damage_type":"piercing","reach_ft":5}]`),
	}

	grid := &pathfinding.Grid{
		Width:   10,
		Height:  10,
		Terrain: testTerrainGrid(10, 10),
	}

	plan, err := BuildTurnPlan(BuildTurnPlanInput{
		Combatant:  npc,
		Creature:   creature,
		Combatants: []refdata.Combatant{npc, pc},
		Grid:       grid,
		SpeedFt:    40,
	})
	require.NoError(t, err)

	// Should NOT have movement step since adjacent
	movementSteps := filterStepsByType(plan.Steps, StepTypeMovement)
	assert.Len(t, movementSteps, 0)

	// Should have attack
	attackSteps := filterStepsByType(plan.Steps, StepTypeAttack)
	assert.Len(t, attackSteps, 1)
}

// --- TDD Cycle 5: Reactions surfaced ---

func TestBuildTurnPlan_ReactionsIncluded(t *testing.T) {
	npcID := uuid.New()
	pcID := uuid.New()
	encounterID := uuid.New()

	npc := refdata.Combatant{
		ID:          npcID,
		DisplayName: "Orc",
		PositionCol: "D",
		PositionRow: 5,
		IsNpc:       true,
		IsAlive:     true,
		HpCurrent:   15,
	}
	pc := refdata.Combatant{
		ID:          pcID,
		DisplayName: "Gandalf",
		PositionCol: "D",
		PositionRow: 6,
		IsNpc:       false,
		IsAlive:     true,
		HpCurrent:   40,
	}

	creature := refdata.Creature{
		Size:    "Medium",
		Speed:   json.RawMessage(`{"walk":30}`),
		Attacks: json.RawMessage(`[{"name":"Greataxe","to_hit":5,"damage":"1d12+3","damage_type":"slashing","reach_ft":5}]`),
	}

	reactions := []refdata.ReactionDeclaration{
		{
			ID:          uuid.New(),
			EncounterID: encounterID,
			CombatantID: pcID,
			Description: "Shield spell if attacked",
			Status:      "active",
		},
	}

	grid := &pathfinding.Grid{
		Width:   10,
		Height:  10,
		Terrain: testTerrainGrid(10, 10),
	}

	plan, err := BuildTurnPlan(BuildTurnPlanInput{
		Combatant:  npc,
		Creature:   creature,
		Combatants: []refdata.Combatant{npc, pc},
		Grid:       grid,
		Reactions:  reactions,
		SpeedFt:    30,
	})
	require.NoError(t, err)
	require.Len(t, plan.Reactions, 1)
	assert.Equal(t, "Shield spell if attacked", plan.Reactions[0].Description)
}

// --- TDD Cycle 6: parseRechargeMin ---

func TestParseRechargeMin(t *testing.T) {
	assert.Equal(t, 5, parseRechargeMin("Fire Breath (Recharge 5-6)"))
	assert.Equal(t, 6, parseRechargeMin("Acid Spray (Recharge 6)"))
	assert.Equal(t, 6, parseRechargeMin("No recharge here"))
}

// --- TDD Cycle 7: parseMultiattackSequence ---

func TestParseMultiattackSequence(t *testing.T) {
	attacks := []CreatureAttackEntry{
		{Name: "Scimitar", ToHit: 5, Damage: "1d6+3", DamageType: "slashing", ReachFt: 5},
		{Name: "Dagger", ToHit: 5, Damage: "1d4+3", DamageType: "piercing", ReachFt: 5},
	}

	// "two with its scimitar and one with its dagger"
	desc := "The captain makes three melee attacks: two with its scimitar and one with its dagger."
	seq := parseMultiattackSequence(desc, attacks)
	require.Len(t, seq, 3)
	assert.Equal(t, "Scimitar", seq[0].Name)
	assert.Equal(t, "Scimitar", seq[1].Name)
	assert.Equal(t, "Dagger", seq[2].Name)
}

func TestParseMultiattackSequence_Empty(t *testing.T) {
	seq := parseMultiattackSequence("", nil)
	assert.Nil(t, seq)
}

// --- TDD Cycle 8: findNearestHostile ---

func TestFindNearestHostile_PicksClosest(t *testing.T) {
	npc := refdata.Combatant{
		ID:          uuid.New(),
		PositionCol: "A",
		PositionRow: 1,
		IsNpc:       true,
		IsAlive:     true,
	}
	farPC := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "FarPC",
		PositionCol: "A",
		PositionRow: 10,
		IsNpc:       false,
		IsAlive:     true,
		HpCurrent:   20,
	}
	nearPC := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "NearPC",
		PositionCol: "A",
		PositionRow: 3,
		IsNpc:       false,
		IsAlive:     true,
		HpCurrent:   20,
	}

	nearest, dist := findNearestHostile(npc, []refdata.Combatant{npc, farPC, nearPC})
	require.NotNil(t, nearest)
	assert.Equal(t, "NearPC", nearest.DisplayName)
	assert.Equal(t, 10, dist) // 2 tiles * 5ft
}

// --- TDD Cycle 9: ExecuteTurnPlan —- rolling attacks, applying damage ---

func TestExecuteTurnPlan_SingleAttack(t *testing.T) {
	roller := newDeterministicRoller(15, 4, 3) // d20=15, damage dice: 4+3

	attack := AttackStep{
		WeaponName: "Scimitar",
		ToHit:      4,
		DamageDice: "1d6+2",
		DamageType: "slashing",
		ReachFt:    5,
		TargetID:   uuid.New(),
		TargetName: "Aragorn",
	}

	result := RollAttack(attack, 14, roller) // target AC 14
	assert.True(t, result.Hit)               // 15+4=19 >= 14
	assert.Equal(t, 15, result.ToHitRoll)
	assert.Equal(t, 19, result.ToHitTotal)
	assert.False(t, result.Critical)
	assert.Greater(t, result.DamageTotal, 0)
}

func TestRollAttack_Miss(t *testing.T) {
	roller := newDeterministicRoller(3) // d20=3

	attack := AttackStep{
		WeaponName: "Scimitar",
		ToHit:      4,
		DamageDice: "1d6+2",
		DamageType: "slashing",
		TargetID:   uuid.New(),
		TargetName: "Aragorn",
	}

	result := RollAttack(attack, 18, roller) // target AC 18
	assert.False(t, result.Hit)              // 3+4=7 < 18
	assert.Equal(t, 3, result.ToHitRoll)
	assert.Equal(t, 7, result.ToHitTotal)
	assert.Equal(t, 0, result.DamageTotal)
}

func TestRollAttack_CriticalHit(t *testing.T) {
	roller := newDeterministicRoller(20, 3, 5) // d20=20 (crit), damage dice

	attack := AttackStep{
		WeaponName: "Scimitar",
		ToHit:      4,
		DamageDice: "1d6+2",
		DamageType: "slashing",
		TargetID:   uuid.New(),
		TargetName: "Aragorn",
	}

	result := RollAttack(attack, 20, roller) // AC 20
	assert.True(t, result.Hit)
	assert.True(t, result.Critical)
	assert.Greater(t, result.DamageTotal, 0)
}

// newDeterministicRoller creates a dice.Roller that returns values from the given sequence.
func newDeterministicRoller(values ...int) *dice.Roller {
	idx := 0
	return dice.NewRoller(func(max int) int {
		if idx >= len(values) {
			return 1
		}
		v := values[idx]
		idx++
		if v > max {
			v = max
		}
		return v
	})
}

// --- TDD Cycle 10: FormatCombatLog ---

func TestFormatCombatLog_Hit(t *testing.T) {
	plan := TurnPlan{
		DisplayName: "Goblin",
		Steps: []TurnStep{
			{
				Type: StepTypeMovement,
				Movement: &MovementStep{
					TotalCostFt: 15,
				},
			},
			{
				Type: StepTypeAttack,
				Attack: &AttackStep{
					WeaponName: "Scimitar",
					TargetName: "Aragorn",
					DamageType: "slashing",
					RollResult: &AttackRollResult{
						ToHitRoll:   14,
						ToHitTotal:  18,
						Hit:         true,
						DamageTotal: 8,
					},
				},
			},
		},
	}
	log := FormatCombatLog(plan)
	assert.Contains(t, log, "Goblin's Turn")
	assert.Contains(t, log, "Moves 15ft")
	assert.Contains(t, log, "Scimitar vs Aragorn")
	assert.Contains(t, log, "Hit!")
	assert.Contains(t, log, "8 slashing damage")
}

func TestFormatCombatLog_Miss(t *testing.T) {
	plan := TurnPlan{
		DisplayName: "Goblin",
		Steps: []TurnStep{
			{
				Type: StepTypeAttack,
				Attack: &AttackStep{
					WeaponName: "Scimitar",
					TargetName: "Aragorn",
					RollResult: &AttackRollResult{
						ToHitRoll:  5,
						ToHitTotal: 9,
						Hit:        false,
					},
				},
			},
		},
	}
	log := FormatCombatLog(plan)
	assert.Contains(t, log, "Miss")
}

func TestFormatCombatLog_Critical(t *testing.T) {
	plan := TurnPlan{
		DisplayName: "Goblin",
		Steps: []TurnStep{
			{
				Type: StepTypeAttack,
				Attack: &AttackStep{
					WeaponName: "Scimitar",
					TargetName: "Aragorn",
					DamageType: "slashing",
					RollResult: &AttackRollResult{
						ToHitRoll:   20,
						ToHitTotal:  24,
						Hit:         true,
						Critical:    true,
						DamageTotal: 14,
					},
				},
			},
		},
	}
	log := FormatCombatLog(plan)
	assert.Contains(t, log, "CRITICAL HIT!")
	assert.Contains(t, log, "14 slashing damage")
}

func TestFormatCombatLog_BonusAction(t *testing.T) {
	plan := TurnPlan{
		DisplayName: "Goblin",
		Steps: []TurnStep{
			{
				Type: StepTypeBonusAction,
				Ability: &AbilityStep{
					Name:        "Nimble Escape",
					Description: "The goblin can take the Disengage or Hide action as a bonus action on each of its turns.",
				},
			},
		},
	}
	log := FormatCombatLog(plan)
	assert.Contains(t, log, "Goblin's Turn")
	assert.Contains(t, log, "Nimble Escape")
}

func TestFindNearestHostile_SkipsDeadTargets(t *testing.T) {
	npc := refdata.Combatant{
		ID:          uuid.New(),
		PositionCol: "A",
		PositionRow: 1,
		IsNpc:       true,
		IsAlive:     true,
	}
	deadPC := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "DeadPC",
		PositionCol: "A",
		PositionRow: 2,
		IsNpc:       false,
		IsAlive:     false,
		HpCurrent:   0,
	}

	nearest, _ := findNearestHostile(npc, []refdata.Combatant{npc, deadPC})
	assert.Nil(t, nearest)
}

// --- Phase 78c: Bonus Action Parsing ---

func TestParseBonusActions_GoblinNimbleEscape(t *testing.T) {
	abilities := []CreatureAbilityEntry{
		{Name: "Nimble Escape", Description: "The goblin can take the Disengage or Hide action as a bonus action on each of its turns."},
	}
	bonusActions := ParseBonusActions(abilities)
	require.Len(t, bonusActions, 1)
	assert.Equal(t, "Nimble Escape", bonusActions[0].Name)
	assert.Contains(t, bonusActions[0].Description, "bonus action")
}

func TestParseBonusActions_NoBonus(t *testing.T) {
	abilities := []CreatureAbilityEntry{
		{Name: "Keen Senses", Description: "The wolf has advantage on Wisdom (Perception) checks that rely on hearing or smell."},
	}
	bonusActions := ParseBonusActions(abilities)
	assert.Len(t, bonusActions, 0)
}

func TestParseBonusActions_MultipleMixed(t *testing.T) {
	abilities := []CreatureAbilityEntry{
		{Name: "Nimble Escape", Description: "The goblin can take the Disengage or Hide action as a bonus action on each of its turns."},
		{Name: "Keen Senses", Description: "The goblin has advantage on Perception checks."},
		{Name: "Aggressive", Description: "As a bonus action, the orc can move up to its speed toward a hostile creature that it can see."},
	}
	bonusActions := ParseBonusActions(abilities)
	require.Len(t, bonusActions, 2)
	assert.Equal(t, "Nimble Escape", bonusActions[0].Name)
	assert.Equal(t, "Aggressive", bonusActions[1].Name)
}

func TestParseBonusActions_CaseInsensitive(t *testing.T) {
	abilities := []CreatureAbilityEntry{
		{Name: "Shadow Stealth", Description: "While in dim light or darkness, the shadow can take the Hide action as a Bonus Action."},
	}
	bonusActions := ParseBonusActions(abilities)
	require.Len(t, bonusActions, 1)
	assert.Equal(t, "Shadow Stealth", bonusActions[0].Name)
}

func TestParseBonusActions_ExcludesMultiattack(t *testing.T) {
	abilities := []CreatureAbilityEntry{
		{Name: "Multiattack", Description: "The creature makes two attacks."},
		{Name: "Rampage", Description: "When the gnoll reduces a creature to 0 hit points with a melee attack on its turn, the gnoll can take a bonus action to move up to half its speed and make a bite attack."},
	}
	bonusActions := ParseBonusActions(abilities)
	require.Len(t, bonusActions, 1)
	assert.Equal(t, "Rampage", bonusActions[0].Name)
}

func TestParseBonusActions_Nil(t *testing.T) {
	bonusActions := ParseBonusActions(nil)
	assert.Len(t, bonusActions, 0)
}

func TestBuildTurnPlan_BonusAction_GoblinNimbleEscape(t *testing.T) {
	npcID := uuid.New()
	pcID := uuid.New()

	npc := refdata.Combatant{
		ID:          npcID,
		DisplayName: "Goblin",
		PositionCol: "A",
		PositionRow: 1,
		IsNpc:       true,
		IsAlive:     true,
		HpCurrent:   7,
	}
	pc := refdata.Combatant{
		ID:          pcID,
		DisplayName: "Gandalf",
		PositionCol: "A",
		PositionRow: 2,
		IsNpc:       false,
		IsAlive:     true,
		HpCurrent:   40,
	}

	creature := refdata.Creature{
		Size:    "Small",
		Speed:   json.RawMessage(`{"walk":30}`),
		Attacks: json.RawMessage(`[{"name":"Scimitar","to_hit":4,"damage":"1d6+2","damage_type":"slashing","reach_ft":5}]`),
		Abilities: toNullRawMessage(json.RawMessage(`[
			{"name":"Nimble Escape","description":"The goblin can take the Disengage or Hide action as a bonus action on each of its turns."}
		]`)),
	}

	grid := &pathfinding.Grid{
		Width:   5,
		Height:  5,
		Terrain: testTerrainGrid(5, 5),
	}

	plan, err := BuildTurnPlan(BuildTurnPlanInput{
		Combatant:  npc,
		Creature:   creature,
		Combatants: []refdata.Combatant{npc, pc},
		Grid:       grid,
		SpeedFt:    30,
	})
	require.NoError(t, err)

	bonusSteps := filterStepsByType(plan.Steps, StepTypeBonusAction)
	require.Len(t, bonusSteps, 1)
	require.NotNil(t, bonusSteps[0].Ability)
	assert.Equal(t, "Nimble Escape", bonusSteps[0].Ability.Name)
	assert.Contains(t, bonusSteps[0].Ability.Description, "bonus action")
	assert.True(t, bonusSteps[0].Suggested)
}

func TestBuildTurnPlan_NoBonusAction_WhenNonePresent(t *testing.T) {
	npcID := uuid.New()
	pcID := uuid.New()

	npc := refdata.Combatant{
		ID:          npcID,
		DisplayName: "Wolf",
		PositionCol: "C",
		PositionRow: 3,
		IsNpc:       true,
		IsAlive:     true,
		HpCurrent:   11,
	}
	pc := refdata.Combatant{
		ID:          pcID,
		DisplayName: "Frodo",
		PositionCol: "C",
		PositionRow: 4,
		IsNpc:       false,
		IsAlive:     true,
		HpCurrent:   20,
	}

	creature := refdata.Creature{
		Size:    "Medium",
		Speed:   json.RawMessage(`{"walk":40}`),
		Attacks: json.RawMessage(`[{"name":"Bite","to_hit":4,"damage":"2d4+2","damage_type":"piercing","reach_ft":5}]`),
		Abilities: toNullRawMessage(json.RawMessage(`[
			{"name":"Keen Hearing and Smell","description":"The wolf has advantage on Wisdom (Perception) checks that rely on hearing or smell."}
		]`)),
	}

	grid := &pathfinding.Grid{
		Width:   10,
		Height:  10,
		Terrain: testTerrainGrid(10, 10),
	}

	plan, err := BuildTurnPlan(BuildTurnPlanInput{
		Combatant:  npc,
		Creature:   creature,
		Combatants: []refdata.Combatant{npc, pc},
		Grid:       grid,
		SpeedFt:    40,
	})
	require.NoError(t, err)

	bonusSteps := filterStepsByType(plan.Steps, StepTypeBonusAction)
	assert.Len(t, bonusSteps, 0)
}
