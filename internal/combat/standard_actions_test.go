package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// --- helpers ---

func makeStdTestSetup() (uuid.UUID, uuid.UUID, uuid.UUID, *mockStore) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	ms := defaultMockStore()
	return encounterID, combatantID, charID, ms
}

func makePCCombatant(id, encounterID uuid.UUID, charID uuid.UUID, name string) refdata.Combatant {
	return refdata.Combatant{
		ID:          id,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		ShortID:     "TE",
		DisplayName: name,
		HpMax:       40,
		HpCurrent:   40,
		Ac:          16,
		PositionCol: "C",
		PositionRow: 3,
		IsNpc:       false,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
}

func makeNPCCombatant(id, encounterID uuid.UUID, name string) refdata.Combatant {
	return refdata.Combatant{
		ID:          id,
		EncounterID: encounterID,
		ShortID:     "NP",
		DisplayName: name,
		HpMax:       30,
		HpCurrent:   30,
		Ac:          13,
		PositionCol: "D",
		PositionRow: 5,
		IsNpc:       true,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
}

func makeNPCCombatantWithCreature(id, encounterID uuid.UUID, name, creatureRef string) refdata.Combatant {
	c := makeNPCCombatant(id, encounterID, name)
	c.CreatureRefID = sql.NullString{String: creatureRef, Valid: true}
	return c
}

func makeBasicTurn() refdata.Turn {
	return refdata.Turn{
		ID:                  uuid.New(),
		ActionUsed:          false,
		BonusActionUsed:     false,
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	}
}

func makeBasicChar(charID uuid.UUID, speed int32) refdata.Character {
	return refdata.Character{
		ID:               charID,
		Name:             "Tester",
		Classes:          json.RawMessage(`[{"class":"Fighter","level":5}]`),
		AbilityScores:    json.RawMessage(`{"str":16,"dex":14,"con":12,"int":10,"wis":13,"cha":8}`),
		Level:            5,
		HpMax:            40,
		HpCurrent:        40,
		SpeedFt:          speed,
		ProficiencyBonus: 3,
	}
}

func makeRogueChar(charID uuid.UUID, rogueLevel int) refdata.Character {
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Rogue", Level: rogueLevel}})
	return refdata.Character{
		ID:               charID,
		Name:             "Shadow",
		Classes:          classesJSON,
		AbilityScores:    json.RawMessage(`{"str":10,"dex":18,"con":12,"int":14,"wis":13,"cha":8}`),
		Level:            int32(rogueLevel),
		HpMax:            35,
		HpCurrent:        35,
		SpeedFt:          30,
		ProficiencyBonus: 3,
	}
}

func makeBasicEncounter(encounterID uuid.UUID, round int32) refdata.Encounter {
	return refdata.Encounter{
		ID:          encounterID,
		Name:        "Test Encounter",
		Status:      "active",
		RoundNumber: round,
	}
}

func setupUpdateTurnActions(ms *mockStore) {
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{
			ID:                  arg.ID,
			ActionUsed:          arg.ActionUsed,
			BonusActionUsed:     arg.BonusActionUsed,
			MovementRemainingFt: arg.MovementRemainingFt,
			HasDisengaged:       arg.HasDisengaged,
			HasStoodThisTurn:    arg.HasStoodThisTurn,
			AttacksRemaining:    arg.AttacksRemaining,
		}, nil
	}
}

func deterministic(n int) int { return n - 1 } // always returns max on 1-indexed die

// profJSON builds a proficiencies JSON for testing.
func profJSON(skills []string, expertise []string, jackOfAllTrades bool) pqtype.NullRawMessage {
	type profData struct {
		Skills          []string `json:"skills"`
		Expertise       []string `json:"expertise"`
		JackOfAllTrades bool     `json:"jack_of_all_trades"`
	}
	data := profData{Skills: skills, Expertise: expertise, JackOfAllTrades: jackOfAllTrades}
	raw, _ := json.Marshal(data)
	return pqtype.NullRawMessage{RawMessage: raw, Valid: true}
}

// =====================
// 1. DASH
// =====================

// TDD Cycle 1: Dash happy path - PC
func TestDash_HappyPath_PC(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	char := makeBasicChar(charID, 30)
	encounter := makeBasicEncounter(encounterID, 1)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := DashCommand{Combatant: combatant, Turn: turn, Encounter: encounter}
	result, err := svc.Dash(context.Background(), cmd)
	require.NoError(t, err)

	assert.True(t, result.Turn.ActionUsed, "action should be consumed")
	assert.Equal(t, turn.MovementRemainingFt+30, result.Turn.MovementRemainingFt)
	assert.Equal(t, int32(30), result.AddedMovement)
	assert.Contains(t, result.CombatLog, "Kael")
	assert.Contains(t, result.CombatLog, "Dash")
}

// TDD Cycle 2: Dash NPC uses default 30ft
func TestDash_HappyPath_NPC(t *testing.T) {
	encounterID, _, _, ms := makeStdTestSetup()
	npc := makeNPCCombatant(uuid.New(), encounterID, "Goblin")
	encounter := makeBasicEncounter(encounterID, 1)
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := DashCommand{Combatant: npc, Turn: turn, Encounter: encounter}
	result, err := svc.Dash(context.Background(), cmd)
	require.NoError(t, err)

	assert.Equal(t, int32(30), result.AddedMovement)
	assert.Equal(t, turn.MovementRemainingFt+30, result.Turn.MovementRemainingFt)
}

// TDD Cycle 3: Dash fails when action already used
func TestDash_ActionAlreadyUsed(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	encounter := makeBasicEncounter(encounterID, 1)

	svc := NewService(ms)
	turn := makeBasicTurn()
	turn.ActionUsed = true

	cmd := DashCommand{Combatant: combatant, Turn: turn, Encounter: encounter}
	_, err := svc.Dash(context.Background(), cmd)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrResourceSpent)
}

// TDD Cycle 4: Dash fails when incapacitated
func TestDash_Incapacitated(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	combatant.Conditions = json.RawMessage(`[{"condition":"stunned","duration_rounds":1,"started_round":1}]`)
	encounter := makeBasicEncounter(encounterID, 1)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := DashCommand{Combatant: combatant, Turn: turn, Encounter: encounter}
	_, err := svc.Dash(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot act")
}

// TDD Cycle 5: Dash with PC speed 35ft
func TestDash_CustomSpeed(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	char := makeBasicChar(charID, 35) // Wood elf or monk
	encounter := makeBasicEncounter(encounterID, 1)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := DashCommand{Combatant: combatant, Turn: turn, Encounter: encounter}
	result, err := svc.Dash(context.Background(), cmd)
	require.NoError(t, err)
	assert.Equal(t, int32(35), result.AddedMovement)
}

// TDD Cycle 6: Dash - GetCharacter error
func TestDash_GetCharacterError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	encounter := makeBasicEncounter(encounterID, 1)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := DashCommand{Combatant: combatant, Turn: turn, Encounter: encounter}
	_, err := svc.Dash(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

// TDD Cycle 7: Dash - UpdateTurnActions error
func TestDash_UpdateTurnActionsError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	char := makeBasicChar(charID, 30)
	encounter := makeBasicEncounter(encounterID, 1)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := DashCommand{Combatant: combatant, Turn: turn, Encounter: encounter}
	_, err := svc.Dash(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// =====================
// 2. DISENGAGE
// =====================

// TDD Cycle 8: Disengage happy path
func TestDisengage_HappyPath(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := DisengageCommand{Combatant: combatant, Turn: turn}
	result, err := svc.Disengage(context.Background(), cmd)
	require.NoError(t, err)

	assert.True(t, result.Turn.ActionUsed)
	assert.True(t, result.Turn.HasDisengaged)
	assert.Contains(t, result.CombatLog, "Kael")
	assert.Contains(t, result.CombatLog, "Disengage")
}

// TDD Cycle 9: Disengage fails when action used
func TestDisengage_ActionAlreadyUsed(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")

	svc := NewService(ms)
	turn := makeBasicTurn()
	turn.ActionUsed = true

	cmd := DisengageCommand{Combatant: combatant, Turn: turn}
	_, err := svc.Disengage(context.Background(), cmd)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrResourceSpent)
}

// TDD Cycle 10: Disengage fails when incapacitated
func TestDisengage_Incapacitated(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	combatant.Conditions = json.RawMessage(`[{"condition":"paralyzed","duration_rounds":1,"started_round":1}]`)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := DisengageCommand{Combatant: combatant, Turn: turn}
	_, err := svc.Disengage(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot act")
}

// =====================
// 3. DODGE
// =====================

// TDD Cycle 11: Dodge happy path
func TestDodge_HappyPath(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := DodgeCommand{Combatant: combatant, Turn: turn, CurrentRound: 2}
	result, err := svc.Dodge(context.Background(), cmd)
	require.NoError(t, err)

	assert.True(t, result.Turn.ActionUsed)
	assert.True(t, HasCondition(result.Combatant.Conditions, "dodge"))
	assert.Contains(t, result.CombatLog, "Kael")
	assert.Contains(t, result.CombatLog, "Dodge")
}

// TDD Cycle 12: Dodge condition has correct duration
func TestDodge_ConditionDetails(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")

	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := DodgeCommand{Combatant: combatant, Turn: turn, CurrentRound: 3}
	result, err := svc.Dodge(context.Background(), cmd)
	require.NoError(t, err)

	cond, found := GetCondition(result.Combatant.Conditions, "dodge")
	require.True(t, found)
	assert.Equal(t, 1, cond.DurationRounds)
	assert.Equal(t, 3, cond.StartedRound)
	assert.Equal(t, "start_of_turn", cond.ExpiresOn)
	assert.Equal(t, combatant.ID.String(), cond.SourceCombatantID)
}

// TDD Cycle 13: Dodge fails when action used
func TestDodge_ActionAlreadyUsed(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")

	svc := NewService(ms)
	turn := makeBasicTurn()
	turn.ActionUsed = true

	cmd := DodgeCommand{Combatant: combatant, Turn: turn, CurrentRound: 1}
	_, err := svc.Dodge(context.Background(), cmd)
	assert.ErrorIs(t, err, ErrResourceSpent)
}

// =====================
// 4. HELP
// =====================

// TDD Cycle 14: Help happy path
func TestHelp_HappyPath(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	helper := makePCCombatant(combatantID, encounterID, charID, "Kael")
	helper.PositionCol = "C"
	helper.PositionRow = 3

	allyID := uuid.New()
	ally := makePCCombatant(allyID, encounterID, uuid.New(), "Aria")
	ally.PositionCol = "C"
	ally.PositionRow = 4

	targetID := uuid.New()
	target := makeNPCCombatant(targetID, encounterID, "Goblin")
	target.PositionCol = "C"
	target.PositionRow = 4 // adjacent to helper

	encounter := makeBasicEncounter(encounterID, 1)
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HelpCommand{
		Helper: helper, Ally: ally, Target: target,
		Turn: turn, Encounter: encounter,
	}
	result, err := svc.Help(context.Background(), cmd)
	require.NoError(t, err)

	assert.True(t, result.Turn.ActionUsed)
	assert.True(t, HasCondition(result.Ally.Conditions, "help_advantage"))
	assert.Contains(t, result.CombatLog, "Kael")
	assert.Contains(t, result.CombatLog, "Help")
	assert.Contains(t, result.CombatLog, "Aria")
	assert.Contains(t, result.CombatLog, "Goblin")
}

// TDD Cycle 15: Help fails when too far from target
func TestHelp_TooFarFromTarget(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	helper := makePCCombatant(combatantID, encounterID, charID, "Kael")
	helper.PositionCol = "A"
	helper.PositionRow = 1

	ally := makePCCombatant(uuid.New(), encounterID, uuid.New(), "Aria")
	target := makeNPCCombatant(uuid.New(), encounterID, "Goblin")
	target.PositionCol = "D"
	target.PositionRow = 5 // far away

	encounter := makeBasicEncounter(encounterID, 1)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HelpCommand{
		Helper: helper, Ally: ally, Target: target,
		Turn: turn, Encounter: encounter,
	}
	_, err := svc.Help(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "within 5ft")
}

// TDD Cycle 16: Help fails when action used
func TestHelp_ActionAlreadyUsed(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	helper := makePCCombatant(combatantID, encounterID, charID, "Kael")
	ally := makePCCombatant(uuid.New(), encounterID, uuid.New(), "Aria")
	target := makeNPCCombatant(uuid.New(), encounterID, "Goblin")
	target.PositionCol = "C"
	target.PositionRow = 4

	encounter := makeBasicEncounter(encounterID, 1)
	svc := NewService(ms)
	turn := makeBasicTurn()
	turn.ActionUsed = true

	cmd := HelpCommand{
		Helper: helper, Ally: ally, Target: target,
		Turn: turn, Encounter: encounter,
	}
	_, err := svc.Help(context.Background(), cmd)
	assert.ErrorIs(t, err, ErrResourceSpent)
}

// =====================
// 5. HIDE
// =====================

// TDD Cycle 17: Hide success
func TestHide_Success(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	// DEX 14 => +2 mod, stealth roll = 20 + 2 = 22 (with deterministic roller rolling max)
	char := makeBasicChar(charID, 30)
	char.AbilityScores = json.RawMessage(`{"str":10,"dex":18,"con":12,"int":14,"wis":13,"cha":8}`) // +4 DEX

	hostile := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Goblin", "goblin")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "goblin",
			AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`), // WIS 8 => -1, PP=9
		}, nil
	}
	setupUpdateTurnActions(ms)

	encounter := makeBasicEncounter(encounterID, 1)
	roller := dice.NewRoller(deterministic) // always rolls max (20)
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HideCommand{Combatant: combatant, Turn: turn, Encounter: encounter, Hostiles: []refdata.Combatant{hostile}}
	result, err := svc.Hide(context.Background(), cmd, roller)
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.False(t, result.Combatant.IsVisible)
	assert.True(t, result.Turn.ActionUsed)
	assert.Contains(t, result.CombatLog, "Hidden from all hostiles")
}

// TDD Cycle 18: Hide failure
func TestHide_Failure(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	char := makeBasicChar(charID, 30)
	char.AbilityScores = json.RawMessage(`{"str":10,"dex":8,"con":12,"int":14,"wis":13,"cha":8}`) // DEX 8 => -1

	hostile := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Guard", "guard")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "guard",
			AbilityScores: json.RawMessage(`{"str":14,"dex":12,"con":12,"int":10,"wis":16,"cha":10}`), // WIS 16 => +3, PP=13
		}, nil
	}
	setupUpdateTurnActions(ms)

	encounter := makeBasicEncounter(encounterID, 1)
	// Always rolls 1 on d20
	roller := dice.NewRoller(func(n int) int { return 0 })
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HideCommand{Combatant: combatant, Turn: turn, Encounter: encounter, Hostiles: []refdata.Combatant{hostile}}
	result, err := svc.Hide(context.Background(), cmd, roller)
	require.NoError(t, err)

	assert.False(t, result.Success)
	assert.True(t, result.Combatant.IsVisible, "should stay visible on failure")
	assert.Contains(t, result.CombatLog, "Failed (spotted by")
}

// TDD Cycle 19: Hide action already used
func TestHide_ActionAlreadyUsed(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	encounter := makeBasicEncounter(encounterID, 1)
	roller := dice.NewRoller(deterministic)

	svc := NewService(ms)
	turn := makeBasicTurn()
	turn.ActionUsed = true

	cmd := HideCommand{Combatant: combatant, Turn: turn, Encounter: encounter}
	_, err := svc.Hide(context.Background(), cmd, roller)
	assert.ErrorIs(t, err, ErrResourceSpent)
}

// TDD Cycle 20: Hide with no hostiles (PP=0)
func TestHide_NoHostiles(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	char := makeBasicChar(charID, 30)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	setupUpdateTurnActions(ms)

	encounter := makeBasicEncounter(encounterID, 1)
	roller := dice.NewRoller(func(n int) int { return 0 }) // rolls 1
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HideCommand{Combatant: combatant, Turn: turn, Encounter: encounter, Hostiles: []refdata.Combatant{}}
	result, err := svc.Hide(context.Background(), cmd, roller)
	require.NoError(t, err)
	assert.True(t, result.Success, "should always succeed with no hostiles (stealth 3 >= PP 0)")
}

// =====================
// 6. STAND
// =====================

// TDD Cycle 21: Stand happy path
func TestStand_HappyPath(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	proneCond, _ := json.Marshal([]CombatCondition{{Condition: "prone"}})
	combatant.Conditions = proneCond
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	turn := makeBasicTurn()
	turn.MovementRemainingFt = 30

	cmd := StandCommand{Combatant: combatant, Turn: turn, MaxSpeed: 30}
	result, err := svc.Stand(context.Background(), cmd)
	require.NoError(t, err)

	assert.Equal(t, 15, result.MovementCost, "should cost half speed")
	assert.Equal(t, int32(15), result.Turn.MovementRemainingFt)
	assert.True(t, result.Turn.HasStoodThisTurn)
	assert.False(t, HasCondition(result.Combatant.Conditions, "prone"))
	assert.Contains(t, result.CombatLog, "stands up")
}

// TDD Cycle 22: Stand fails when not prone
func TestStand_NotProne(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := StandCommand{Combatant: combatant, Turn: turn, MaxSpeed: 30}
	_, err := svc.Stand(context.Background(), cmd)
	assert.ErrorIs(t, err, ErrNotProne)
}

// TDD Cycle 23: Stand fails when not enough movement
func TestStand_NotEnoughMovement(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	proneCond, _ := json.Marshal([]CombatCondition{{Condition: "prone"}})
	combatant.Conditions = proneCond

	svc := NewService(ms)
	turn := makeBasicTurn()
	turn.MovementRemainingFt = 10 // need 15 (half of 30)

	cmd := StandCommand{Combatant: combatant, Turn: turn, MaxSpeed: 30}
	_, err := svc.Stand(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enough movement")
}

// TDD Cycle 24: Stand does NOT cost an action
func TestStand_DoesNotCostAction(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	proneCond, _ := json.Marshal([]CombatCondition{{Condition: "prone"}})
	combatant.Conditions = proneCond
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	turn := makeBasicTurn()
	turn.ActionUsed = true // action already used - should still work

	cmd := StandCommand{Combatant: combatant, Turn: turn, MaxSpeed: 30}
	result, err := svc.Stand(context.Background(), cmd)
	require.NoError(t, err)
	assert.True(t, result.Turn.ActionUsed, "action should remain used (stand doesn't touch it)")
}

// =====================
// 7. DROP PRONE
// =====================

// TDD Cycle 25: Drop Prone happy path
func TestDropProne_HappyPath(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")

	svc := NewService(ms)
	turn := makeBasicTurn()
	encounter := makeBasicEncounter(encounterID, 1)

	cmd := DropProneCommand{Combatant: combatant, Turn: turn, Encounter: encounter, CurrentRound: 1}
	result, err := svc.DropProne(context.Background(), cmd)
	require.NoError(t, err)

	assert.True(t, HasCondition(result.Combatant.Conditions, "prone"))
	assert.Contains(t, result.CombatLog, "drops prone")
}

// TDD Cycle 26: Drop Prone fails when already prone
func TestDropProne_AlreadyProne(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	proneCond, _ := json.Marshal([]CombatCondition{{Condition: "prone"}})
	combatant.Conditions = proneCond

	svc := NewService(ms)
	turn := makeBasicTurn()
	encounter := makeBasicEncounter(encounterID, 1)

	cmd := DropProneCommand{Combatant: combatant, Turn: turn, Encounter: encounter, CurrentRound: 1}
	_, err := svc.DropProne(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already prone")
}

// TDD Cycle 27: Drop Prone does NOT cost an action
func TestDropProne_NoCost(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")

	svc := NewService(ms)
	turn := makeBasicTurn()
	turn.ActionUsed = true
	turn.BonusActionUsed = true
	encounter := makeBasicEncounter(encounterID, 1)

	cmd := DropProneCommand{Combatant: combatant, Turn: turn, Encounter: encounter, CurrentRound: 1}
	_, err := svc.DropProne(context.Background(), cmd)
	require.NoError(t, err, "drop prone should work even with all actions spent")
}

// SR-052: DropProne while airborne triggers fall damage via ApplyCondition
func TestDropProne_Airborne_FallDamage(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()

	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	combatant.AltitudeFt = 30

	var altitudeAfter int32 = -1
	var hpAfter int32 = -1

	ms := defaultMockStore()
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return combatant, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		combatant.Conditions = arg.Conditions
		return combatant, nil
	}
	ms.updateCombatantPositionFn = func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
		altitudeAfter = arg.AltitudeFt
		combatant.AltitudeFt = arg.AltitudeFt
		return combatant, nil
	}
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpAfter = arg.HpCurrent
		combatant.HpCurrent = arg.HpCurrent
		combatant.IsAlive = arg.IsAlive
		return combatant, nil
	}

	svc := NewService(ms)
	svc.SetRoller(dice.NewRoller(func(max int) int { return 3 })) // d6 → 3 each

	turn := makeBasicTurn()
	encounter := makeBasicEncounter(encounterID, 1)
	cmd := DropProneCommand{Combatant: combatant, Turn: turn, Encounter: encounter, CurrentRound: 1}
	result, err := svc.DropProne(context.Background(), cmd)
	require.NoError(t, err)

	assert.True(t, HasCondition(result.Combatant.Conditions, "prone"))
	assert.Equal(t, int32(0), altitudeAfter, "altitude should be reset to 0 after fall")
	assert.Equal(t, int32(31), hpAfter, "30ft fall = 3d6, deterministic 3 each = 9 damage, 40-9=31")
	assert.Contains(t, result.CombatLog, "falls 30ft")
}

// =====================
// 8. ESCAPE
// =====================

// TDD Cycle 28: Escape success (Athletics)
func TestEscape_Success_Athletics(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	escapee := makePCCombatant(combatantID, encounterID, charID, "Kael")
	grappledCond, _ := json.Marshal([]CombatCondition{{Condition: "grappled"}})
	escapee.Conditions = grappledCond
	char := makeBasicChar(charID, 30) // STR 16 => +3

	grapplerID := uuid.New()
	grappler := makeNPCCombatantWithCreature(grapplerID, encounterID, "Ogre", "ogre")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "ogre",
			AbilityScores: json.RawMessage(`{"str":10,"dex":8,"con":12,"int":6,"wis":10,"cha":8}`), // STR 10 => +0
		}, nil
	}
	setupUpdateTurnActions(ms)

	encounter := makeBasicEncounter(encounterID, 1)
	// Deterministic: always roll max (20) for both, but with modifier +3 vs +0, escapee wins tie
	roller := dice.NewRoller(deterministic)
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := EscapeCommand{
		Escapee: escapee, Grappler: grappler, Turn: turn,
		Encounter: encounter, UseAcrobatics: false,
	}
	result, err := svc.Escape(context.Background(), cmd, roller)
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.False(t, HasCondition(result.Escapee.Conditions, "grappled"))
	assert.True(t, result.Turn.ActionUsed)
	assert.Contains(t, result.CombatLog, "escapes")
	assert.Contains(t, result.CombatLog, "Athletics")
}

// TDD Cycle 29: Escape failure
func TestEscape_Failure(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	escapee := makePCCombatant(combatantID, encounterID, charID, "Kael")
	grappledCond, _ := json.Marshal([]CombatCondition{{Condition: "grappled"}})
	escapee.Conditions = grappledCond
	char := makeBasicChar(charID, 30)
	char.AbilityScores = json.RawMessage(`{"str":8,"dex":8,"con":12,"int":10,"wis":13,"cha":8}`) // STR 8 => -1

	grappler := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Ogre", "ogre")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "ogre",
			AbilityScores: json.RawMessage(`{"str":20,"dex":8,"con":16,"int":6,"wis":10,"cha":8}`), // STR 20 => +5
		}, nil
	}
	setupUpdateTurnActions(ms)

	encounter := makeBasicEncounter(encounterID, 1)
	// Same roll for both: escapee has -1 mod, grappler has +5 mod => escapee loses
	roller := dice.NewRoller(func(n int) int { return 4 }) // roll 5 for both => 4 vs 10
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := EscapeCommand{
		Escapee: escapee, Grappler: grappler, Turn: turn,
		Encounter: encounter, UseAcrobatics: false,
	}
	result, err := svc.Escape(context.Background(), cmd, roller)
	require.NoError(t, err)

	assert.False(t, result.Success)
	assert.True(t, HasCondition(result.Escapee.Conditions, "grappled"))
	assert.Contains(t, result.CombatLog, "fails to escape")
}

// TDD Cycle 30: Escape with Acrobatics
func TestEscape_Acrobatics(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	escapee := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	grappledCond, _ := json.Marshal([]CombatCondition{{Condition: "grappled"}})
	escapee.Conditions = grappledCond
	char := makeBasicChar(charID, 30)
	char.AbilityScores = json.RawMessage(`{"str":8,"dex":18,"con":12,"int":14,"wis":13,"cha":8}`) // DEX 18 => +4

	grappler := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Ogre", "ogre")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "ogre",
			AbilityScores: json.RawMessage(`{"str":10,"dex":8,"con":16,"int":6,"wis":10,"cha":8}`), // STR 10 => +0
		}, nil
	}
	setupUpdateTurnActions(ms)

	encounter := makeBasicEncounter(encounterID, 1)
	roller := dice.NewRoller(deterministic)
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := EscapeCommand{
		Escapee: escapee, Grappler: grappler, Turn: turn,
		Encounter: encounter, UseAcrobatics: true,
	}
	result, err := svc.Escape(context.Background(), cmd, roller)
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.Contains(t, result.CombatLog, "Acrobatics")
}

// TDD Cycle 31: Escape fails when not grappled
func TestEscape_NotGrappled(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	escapee := makePCCombatant(combatantID, encounterID, charID, "Kael")
	grappler := makeNPCCombatant(uuid.New(), encounterID, "Ogre")

	encounter := makeBasicEncounter(encounterID, 1)
	roller := dice.NewRoller(deterministic)
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := EscapeCommand{
		Escapee: escapee, Grappler: grappler, Turn: turn,
		Encounter: encounter,
	}
	_, err := svc.Escape(context.Background(), cmd, roller)
	assert.ErrorIs(t, err, ErrNotGrappled)
}

// TDD Cycle 32: Escape fails when action used
func TestEscape_ActionAlreadyUsed(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	escapee := makePCCombatant(combatantID, encounterID, charID, "Kael")
	grappledCond, _ := json.Marshal([]CombatCondition{{Condition: "grappled"}})
	escapee.Conditions = grappledCond
	grappler := makeNPCCombatant(uuid.New(), encounterID, "Ogre")

	encounter := makeBasicEncounter(encounterID, 1)
	roller := dice.NewRoller(deterministic)
	svc := NewService(ms)
	turn := makeBasicTurn()
	turn.ActionUsed = true

	cmd := EscapeCommand{
		Escapee: escapee, Grappler: grappler, Turn: turn,
		Encounter: encounter,
	}
	_, err := svc.Escape(context.Background(), cmd, roller)
	assert.ErrorIs(t, err, ErrResourceSpent)
}

// =====================
// 9. CUNNING ACTION
// =====================

// TDD Cycle 33: Cunning Action Dash
func TestCunningAction_Dash(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	char := makeRogueChar(charID, 2)
	encounter := makeBasicEncounter(encounterID, 1)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := CunningActionCommand{
		Combatant: combatant, Turn: turn, Encounter: encounter, Action: "dash",
	}
	result, err := svc.CunningAction(context.Background(), cmd)
	require.NoError(t, err)

	assert.True(t, result.Turn.BonusActionUsed, "should use bonus action")
	assert.False(t, result.Turn.ActionUsed, "should NOT use action")
	assert.Equal(t, int32(30), result.AddedMovement)
	assert.Equal(t, turn.MovementRemainingFt+30, result.Turn.MovementRemainingFt)
	assert.Contains(t, result.CombatLog, "Cunning Action")
	assert.Contains(t, result.CombatLog, "Dash")
	assert.Contains(t, result.CombatLog, "bonus action")
}

// TDD Cycle 34: Cunning Action Disengage
func TestCunningAction_Disengage(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	char := makeRogueChar(charID, 5)
	encounter := makeBasicEncounter(encounterID, 1)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := CunningActionCommand{
		Combatant: combatant, Turn: turn, Encounter: encounter, Action: "disengage",
	}
	result, err := svc.CunningAction(context.Background(), cmd)
	require.NoError(t, err)

	assert.True(t, result.Turn.BonusActionUsed)
	assert.True(t, result.Turn.HasDisengaged)
	assert.Contains(t, result.CombatLog, "Cunning Action")
	assert.Contains(t, result.CombatLog, "Disengage")
}

// TDD Cycle 35: Cunning Action fails for non-Rogue
func TestCunningAction_NotRogue(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	char := makeBasicChar(charID, 30) // Fighter
	encounter := makeBasicEncounter(encounterID, 1)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := CunningActionCommand{
		Combatant: combatant, Turn: turn, Encounter: encounter, Action: "dash",
	}
	_, err := svc.CunningAction(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Rogue level 2+")
}

// TDD Cycle 36: Cunning Action fails for Rogue level 1
func TestCunningAction_RogueTooLow(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	char := makeRogueChar(charID, 1)
	encounter := makeBasicEncounter(encounterID, 1)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := CunningActionCommand{
		Combatant: combatant, Turn: turn, Encounter: encounter, Action: "dash",
	}
	_, err := svc.CunningAction(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Rogue level 2+")
}

// TDD Cycle 37: Cunning Action fails when bonus action used
func TestCunningAction_BonusActionUsed(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	encounter := makeBasicEncounter(encounterID, 1)

	svc := NewService(ms)
	turn := makeBasicTurn()
	turn.BonusActionUsed = true

	cmd := CunningActionCommand{
		Combatant: combatant, Turn: turn, Encounter: encounter, Action: "dash",
	}
	_, err := svc.CunningAction(context.Background(), cmd)
	assert.ErrorIs(t, err, ErrResourceSpent)
}

// TDD Cycle 38: Cunning Action invalid action type
func TestCunningAction_InvalidAction(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	encounter := makeBasicEncounter(encounterID, 1)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := CunningActionCommand{
		Combatant: combatant, Turn: turn, Encounter: encounter, Action: "attack",
	}
	_, err := svc.CunningAction(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be 'dash', 'disengage', or 'hide'")
}

// TDD Cycle 39: Cunning Action fails for NPC
func TestCunningAction_NPC(t *testing.T) {
	encounterID, _, _, ms := makeStdTestSetup()
	npc := makeNPCCombatant(uuid.New(), encounterID, "NPC Rogue")
	encounter := makeBasicEncounter(encounterID, 1)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := CunningActionCommand{
		Combatant: npc, Turn: turn, Encounter: encounter, Action: "dash",
	}
	_, err := svc.CunningAction(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not NPC")
}

// =====================
// HELPER FUNCTION TESTS
// =====================

// TDD Cycle 40: GridDistanceFt
func TestGridDistanceFt(t *testing.T) {
	tests := []struct {
		col1, col2 string
		row1, row2 int
		expected   int
	}{
		{"A", "A", 1, 1, 0},     // same spot
		{"A", "B", 1, 1, 5},     // 1 square = 5ft
		{"A", "A", 1, 2, 5},     // 1 square vertical
		{"A", "B", 1, 2, 5},     // diagonal = 5ft (Chebyshev)
		{"A", "C", 1, 3, 10},    // 2 diagonal
		{"A", "D", 1, 5, 20},    // max of col diff (3) and row diff (4) = 4 * 5 = 20
	}
	for _, tc := range tests {
		got := GridDistanceFt(tc.col1, tc.row1, tc.col2, tc.row2)
		assert.Equal(t, tc.expected, got, "GridDistanceFt(%s,%d,%s,%d)", tc.col1, tc.row1, tc.col2, tc.row2)
	}
}

// TDD Cycle 41: colToInt
func TestColToInt(t *testing.T) {
	assert.Equal(t, 1, colToInt("A"))
	assert.Equal(t, 2, colToInt("B"))
	assert.Equal(t, 26, colToInt("Z"))
	assert.Equal(t, 27, colToInt("AA"))
}

// TDD Cycle 42: abilityModFromScores
func TestAbilityModFromScores(t *testing.T) {
	scores := AbilityScores{Str: 16, Dex: 14, Con: 12, Int: 10, Wis: 13, Cha: 8}
	assert.Equal(t, 3, abilityModFromScores(scores, "str"))
	assert.Equal(t, 2, abilityModFromScores(scores, "dex"))
	assert.Equal(t, 1, abilityModFromScores(scores, "con"))
	assert.Equal(t, 0, abilityModFromScores(scores, "int"))
	assert.Equal(t, 1, abilityModFromScores(scores, "wis"))
	assert.Equal(t, -1, abilityModFromScores(scores, "cha"))
	assert.Equal(t, 0, abilityModFromScores(scores, "invalid"))
}

// TDD Cycle 43: resolveBaseSpeed for character with 0 speed defaults to 30
func TestResolveBaseSpeed_ZeroSpeedDefaultsTo30(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(uuid.New(), uuid.New(), charID, "Kael")
	char := makeBasicChar(charID, 0) // zero speed

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)
	speed, err := svc.resolveBaseSpeed(context.Background(), combatant)
	require.NoError(t, err)
	assert.Equal(t, int32(30), speed)
}

// =====================
// ERROR PATH TESTS
// =====================

// TDD Cycle 44: Disengage UpdateTurnActions error
func TestDisengage_UpdateTurnActionsError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := DisengageCommand{Combatant: combatant, Turn: turn}
	_, err := svc.Disengage(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// TDD Cycle 45: Dodge UpdateCombatantConditions error
func TestDodge_UpdateConditionsError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := DodgeCommand{Combatant: combatant, Turn: turn, CurrentRound: 1}
	_, err := svc.Dodge(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating combatant conditions")
}

// TDD Cycle 46: Help UpdateCombatantConditions error
func TestHelp_UpdateConditionsError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	helper := makePCCombatant(combatantID, encounterID, charID, "Kael")
	ally := makePCCombatant(uuid.New(), encounterID, uuid.New(), "Aria")
	target := makeNPCCombatant(uuid.New(), encounterID, "Goblin")
	target.PositionCol = "C"
	target.PositionRow = 4
	encounter := makeBasicEncounter(encounterID, 1)

	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HelpCommand{Helper: helper, Ally: ally, Target: target, Turn: turn, Encounter: encounter}
	_, err := svc.Help(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating ally conditions")
}

// TDD Cycle 47: Stand UpdateCombatantConditions error
func TestStand_UpdateConditionsError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	proneCond, _ := json.Marshal([]CombatCondition{{Condition: "prone"}})
	combatant.Conditions = proneCond

	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := StandCommand{Combatant: combatant, Turn: turn, MaxSpeed: 30}
	_, err := svc.Stand(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating combatant conditions")
}

// TDD Cycle 48: DropProne UpdateCombatantConditions error
func TestDropProne_UpdateConditionsError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")

	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	turn := makeBasicTurn()
	encounter := makeBasicEncounter(encounterID, 1)

	cmd := DropProneCommand{Combatant: combatant, Turn: turn, Encounter: encounter, CurrentRound: 1}
	_, err := svc.DropProne(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "applying prone condition")
}

// TDD Cycle 49: Escape incapacitated
func TestEscape_Incapacitated(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	escapee := makePCCombatant(combatantID, encounterID, charID, "Kael")
	conds, _ := json.Marshal([]CombatCondition{{Condition: "stunned"}, {Condition: "grappled"}})
	escapee.Conditions = conds
	grappler := makeNPCCombatant(uuid.New(), encounterID, "Ogre")
	encounter := makeBasicEncounter(encounterID, 1)
	roller := dice.NewRoller(deterministic)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := EscapeCommand{Escapee: escapee, Grappler: grappler, Turn: turn, Encounter: encounter}
	_, err := svc.Escape(context.Background(), cmd, roller)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot act")
}

// TDD Cycle 50: CunningAction GetCharacter error
func TestCunningAction_GetCharacterError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	encounter := makeBasicEncounter(encounterID, 1)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := CunningActionCommand{Combatant: combatant, Turn: turn, Encounter: encounter, Action: "dash"}
	_, err := svc.CunningAction(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

// TDD Cycle 51: CunningAction incapacitated
func TestCunningAction_Incapacitated(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	combatant.Conditions = json.RawMessage(`[{"condition":"stunned"}]`)
	encounter := makeBasicEncounter(encounterID, 1)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := CunningActionCommand{Combatant: combatant, Turn: turn, Encounter: encounter, Action: "dash"}
	_, err := svc.CunningAction(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot act")
}

// TDD Cycle 52: Hide incapacitated
func TestHide_Incapacitated(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	combatant.Conditions = json.RawMessage(`[{"condition":"unconscious"}]`)
	encounter := makeBasicEncounter(encounterID, 1)
	roller := dice.NewRoller(deterministic)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HideCommand{Combatant: combatant, Turn: turn, Encounter: encounter}
	_, err := svc.Hide(context.Background(), cmd, roller)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot act")
}

// TDD Cycle 53: Help incapacitated
func TestHelp_Incapacitated(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	helper := makePCCombatant(combatantID, encounterID, charID, "Kael")
	helper.Conditions = json.RawMessage(`[{"condition":"stunned"}]`)
	ally := makePCCombatant(uuid.New(), encounterID, uuid.New(), "Aria")
	target := makeNPCCombatant(uuid.New(), encounterID, "Goblin")
	target.PositionCol = "C"
	target.PositionRow = 4
	encounter := makeBasicEncounter(encounterID, 1)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HelpCommand{Helper: helper, Ally: ally, Target: target, Turn: turn, Encounter: encounter}
	_, err := svc.Help(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot act")
}

// TDD Cycle 54: Dodge incapacitated
func TestDodge_Incapacitated(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	combatant.Conditions = json.RawMessage(`[{"condition":"incapacitated"}]`)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := DodgeCommand{Combatant: combatant, Turn: turn, CurrentRound: 1}
	_, err := svc.Dodge(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot act")
}

// TDD Cycle 55: Hide GetCharacter error for DEX mod
func TestHide_GetCharacterError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	encounter := makeBasicEncounter(encounterID, 1)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}
	roller := dice.NewRoller(deterministic)

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HideCommand{Combatant: combatant, Turn: turn, Encounter: encounter, Hostiles: []refdata.Combatant{}}
	_, err := svc.Hide(context.Background(), cmd, roller)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting character for ability")
}

// TDD Cycle 56: NPC passivePerception defaults to 10 when no creature ref
func TestHide_NPCWithoutCreatureRef(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	char := makeBasicChar(charID, 30)
	char.AbilityScores = json.RawMessage(`{"str":10,"dex":18,"con":12,"int":14,"wis":13,"cha":8}`) // DEX +4

	hostile := makeNPCCombatant(uuid.New(), encounterID, "NPC Guard") // no creature ref

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	setupUpdateTurnActions(ms)

	encounter := makeBasicEncounter(encounterID, 1)
	roller := dice.NewRoller(deterministic) // rolls 20 => 20 + 4 = 24 vs PP 10
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HideCommand{Combatant: combatant, Turn: turn, Encounter: encounter, Hostiles: []refdata.Combatant{hostile}}
	result, err := svc.Hide(context.Background(), cmd, roller)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 10, result.HighestPerception, "NPC without creature ref should have PP=10")
}

// TDD Cycle 57: Dodge UpdateTurnActions error
func TestDodge_UpdateTurnActionsError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")

	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := DodgeCommand{Combatant: combatant, Turn: turn, CurrentRound: 1}
	_, err := svc.Dodge(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// TDD Cycle 58: Stand UpdateTurnActions error
func TestStand_UpdateTurnActionsError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Kael")
	proneCond, _ := json.Marshal([]CombatCondition{{Condition: "prone"}})
	combatant.Conditions = proneCond

	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := StandCommand{Combatant: combatant, Turn: turn, MaxSpeed: 30}
	_, err := svc.Stand(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// TDD Cycle 59: CunningAction UpdateTurnActions error
func TestCunningAction_UpdateTurnActionsError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	char := makeRogueChar(charID, 5)
	encounter := makeBasicEncounter(encounterID, 1)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := CunningActionCommand{Combatant: combatant, Turn: turn, Encounter: encounter, Action: "dash"}
	_, err := svc.CunningAction(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// TDD Cycle 60: Escape UpdateTurnActions error
func TestEscape_UpdateTurnActionsError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	escapee := makePCCombatant(combatantID, encounterID, charID, "Kael")
	grappledCond, _ := json.Marshal([]CombatCondition{{Condition: "grappled"}})
	escapee.Conditions = grappledCond
	char := makeBasicChar(charID, 30)

	grappler := makeNPCCombatant(uuid.New(), encounterID, "Ogre")
	encounter := makeBasicEncounter(encounterID, 1)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	roller := dice.NewRoller(func(n int) int { return 0 }) // roll low to fail
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := EscapeCommand{Escapee: escapee, Grappler: grappler, Turn: turn, Encounter: encounter}
	_, err := svc.Escape(context.Background(), cmd, roller)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// TDD Cycle 61: Hide UpdateTurnActions error
func TestHide_UpdateTurnActionsError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	char := makeBasicChar(charID, 30)
	encounter := makeBasicEncounter(encounterID, 1)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	roller := dice.NewRoller(deterministic)
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HideCommand{Combatant: combatant, Turn: turn, Encounter: encounter, Hostiles: []refdata.Combatant{}}
	_, err := svc.Hide(context.Background(), cmd, roller)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// TDD Cycle 62: Escape success but UpdateCombatantConditions error
func TestEscape_Success_UpdateConditionsError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	escapee := makePCCombatant(combatantID, encounterID, charID, "Kael")
	grappledCond, _ := json.Marshal([]CombatCondition{{Condition: "grappled"}})
	escapee.Conditions = grappledCond
	char := makeBasicChar(charID, 30)

	grappler := makeNPCCombatant(uuid.New(), encounterID, "Ogre")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}

	roller := dice.NewRoller(deterministic) // high roll
	svc := NewService(ms)
	turn := makeBasicTurn()
	encounter := makeBasicEncounter(encounterID, 1)

	cmd := EscapeCommand{Escapee: escapee, Grappler: grappler, Turn: turn, Encounter: encounter}
	_, err := svc.Escape(context.Background(), cmd, roller)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating escapee conditions")
}

// TDD Cycle 63: getAbilityMod - creature GetCreature error
func TestHide_GetCreatureError_PassivePerception(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	char := makeBasicChar(charID, 30)

	hostile := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Mystery", "unknown")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{}, fmt.Errorf("creature not found")
	}
	setupUpdateTurnActions(ms)

	encounter := makeBasicEncounter(encounterID, 1)
	roller := dice.NewRoller(deterministic)
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HideCommand{Combatant: combatant, Turn: turn, Encounter: encounter, Hostiles: []refdata.Combatant{hostile}}
	_, err := svc.Hide(context.Background(), cmd, roller)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting creature for ability")
}

// TDD Cycle 64: getAbilityMod - creature with bad ability scores JSON
func TestGetAbilityMod_CreatureBadJSON(t *testing.T) {
	_, _, _, ms := makeStdTestSetup()
	combatant := makeNPCCombatantWithCreature(uuid.New(), uuid.New(), "Bad", "bad-creature")

	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "bad-creature",
			AbilityScores: json.RawMessage(`invalid`),
		}, nil
	}

	svc := NewService(ms)
	_, err := svc.getAbilityMod(context.Background(), combatant, "str")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing creature ability scores")
}

// TDD Cycle 65: getAbilityMod - character with bad ability scores JSON
func TestGetAbilityMod_CharacterBadJSON(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(uuid.New(), uuid.New(), charID, "BadJSON")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:            charID,
			AbilityScores: json.RawMessage(`invalid`),
		}, nil
	}

	svc := NewService(ms)
	_, err := svc.getAbilityMod(context.Background(), combatant, "str")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing ability scores")
}

// TDD Cycle 66: passivePerception error propagation
func TestPassivePerception_Error(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(uuid.New(), uuid.New(), charID, "Kael")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, err := svc.passivePerception(context.Background(), combatant)
	assert.Error(t, err)
}

// TDD Cycle 67: Help UpdateTurnActions error
func TestHelp_UpdateTurnActionsError(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	helper := makePCCombatant(combatantID, encounterID, charID, "Kael")
	ally := makePCCombatant(uuid.New(), encounterID, uuid.New(), "Aria")
	target := makeNPCCombatant(uuid.New(), encounterID, "Goblin")
	target.PositionCol = "C"
	target.PositionRow = 4
	encounter := makeBasicEncounter(encounterID, 1)

	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HelpCommand{Helper: helper, Ally: ally, Target: target, Turn: turn, Encounter: encounter}
	_, err := svc.Help(context.Background(), cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// =====================
// Phase 57: Stealth & Hiding
// =====================

// TDD Cycle P57-1: passivePerception includes Perception proficiency bonus for characters
func TestPassivePerception_CharacterWithPerceptionProficiency(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(uuid.New(), uuid.New(), charID, "Aria")

	// WIS 16 => +3, proficiency bonus 3, proficient in perception => PP = 10 + 3 + 3 = 16
	char := makeBasicChar(charID, 30)
	char.AbilityScores = json.RawMessage(`{"str":10,"dex":14,"con":12,"int":10,"wis":16,"cha":8}`)
	char.ProficiencyBonus = 3
	char.Proficiencies = profJSON([]string{"perception", "stealth"}, nil, false)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)
	pp, err := svc.passivePerception(context.Background(), combatant)
	require.NoError(t, err)
	assert.Equal(t, 16, pp) // 10 + WIS mod(3) + prof(3)
}

// TDD Cycle P57-2: Hide uses full Stealth skill modifier (proficiency + expertise)
func TestHide_UsesStealthProficiency(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")

	// DEX 14 => +2, proficiency 3, expertise in stealth => +2 + 3*2 = +8
	char := makeBasicChar(charID, 30)
	char.AbilityScores = json.RawMessage(`{"str":10,"dex":14,"con":12,"int":10,"wis":10,"cha":8}`)
	char.ProficiencyBonus = 3
	char.Proficiencies = profJSON([]string{"stealth"}, []string{"stealth"}, false)

	hostile := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Goblin", "goblin")
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "goblin",
			AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":10,"cha":8}`), // WIS 10 => PP=10
		}, nil
	}
	setupUpdateTurnActions(ms)

	encounter := makeBasicEncounter(encounterID, 1)
	// deterministic rolls 20 => total = 20 + 8 = 28
	roller := dice.NewRoller(deterministic)
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HideCommand{Combatant: combatant, Turn: turn, Encounter: encounter, Hostiles: []refdata.Combatant{hostile}}
	result, err := svc.Hide(context.Background(), cmd, roller)
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.Equal(t, 27, result.StealthRoll) // deterministic rolls 19 on d20 + 8 (DEX 2 + expertise 6)
}

// TDD Cycle P57-4: Armor with stealth_disadv causes disadvantage on stealth
func TestHide_ArmorStealthDisadvantage(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Knight")

	char := makeBasicChar(charID, 30)
	char.AbilityScores = json.RawMessage(`{"str":16,"dex":10,"con":14,"int":10,"wis":10,"cha":10}`)
	char.ProficiencyBonus = 3
	char.EquippedArmor = sql.NullString{String: "chain-mail", Valid: true}

	hostile := makeNPCCombatant(uuid.New(), encounterID, "Guard")
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		return refdata.Armor{
			ID:            "chain-mail",
			StealthDisadv: sql.NullBool{Bool: true, Valid: true},
			ArmorType:     "heavy",
		}, nil
	}
	setupUpdateTurnActions(ms)

	encounter := makeBasicEncounter(encounterID, 1)
	// With disadvantage: roller is called twice, picks lower
	rollCall := 0
	roller := dice.NewRoller(func(n int) int {
		rollCall++
		if rollCall == 1 {
			return 15
		}
		return 5
	})
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HideCommand{Combatant: combatant, Turn: turn, Encounter: encounter, Hostiles: []refdata.Combatant{hostile}}
	result, err := svc.Hide(context.Background(), cmd, roller)
	require.NoError(t, err)

	// Disadvantage: takes lower roll (5), plus DEX mod 0 = 5
	assert.Equal(t, 5, result.StealthRoll)
}

// TDD Cycle P57-5: Medium Armor Master negates stealth disadvantage for medium armor
func TestHide_MediumArmorMaster_NegatesDisadvantage(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Ranger")

	char := makeBasicChar(charID, 30)
	char.AbilityScores = json.RawMessage(`{"str":14,"dex":16,"con":12,"int":10,"wis":14,"cha":8}`)
	char.ProficiencyBonus = 3
	char.EquippedArmor = sql.NullString{String: "breastplate", Valid: true}
	featJSON, _ := json.Marshal([]struct {
		Name             string `json:"name"`
		MechanicalEffect string `json:"mechanical_effect"`
	}{
		{Name: "Medium Armor Master", MechanicalEffect: "no_stealth_disadvantage_medium_armor"},
	})
	char.Features = pqtype.NullRawMessage{RawMessage: featJSON, Valid: true}

	hostile := makeNPCCombatant(uuid.New(), encounterID, "Guard")
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getArmorFn = func(ctx context.Context, id string) (refdata.Armor, error) {
		return refdata.Armor{
			ID:            "breastplate",
			StealthDisadv: sql.NullBool{Bool: true, Valid: true},
			ArmorType:     "medium",
		}, nil
	}
	setupUpdateTurnActions(ms)

	encounter := makeBasicEncounter(encounterID, 1)
	// Normal roll (no disadvantage due to feat): rolls once, returns 15
	roller := dice.NewRoller(func(n int) int { return 15 })
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HideCommand{Combatant: combatant, Turn: turn, Encounter: encounter, Hostiles: []refdata.Combatant{hostile}}
	result, err := svc.Hide(context.Background(), cmd, roller)
	require.NoError(t, err)

	// Normal roll: 15 + DEX mod(3) = 18 (no disadvantage applied)
	assert.Equal(t, 18, result.StealthRoll)
}

// TDD Cycle P57-7: CunningAction accepts "hide" for Rogue
func TestCunningAction_Hide(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	char := makeRogueChar(charID, 5)
	char.AbilityScores = json.RawMessage(`{"str":10,"dex":18,"con":12,"int":14,"wis":13,"cha":8}`) // DEX +4
	char.Proficiencies = profJSON([]string{"stealth"}, nil, false)
	encounter := makeBasicEncounter(encounterID, 1)

	hostile := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Goblin", "goblin")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "goblin",
			AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`), // PP=9
		}, nil
	}
	setupUpdateTurnActions(ms)

	roller := dice.NewRoller(deterministic) // rolls 19
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := CunningActionCommand{
		Combatant: combatant,
		Turn:      turn,
		Encounter: encounter,
		Action:    "hide",
		Hostiles:  []refdata.Combatant{hostile},
	}
	result, err := svc.CunningAction(context.Background(), cmd, roller)
	require.NoError(t, err)

	assert.True(t, result.Turn.BonusActionUsed, "should use bonus action")
	assert.False(t, result.Turn.ActionUsed, "should NOT use action")
	assert.NotNil(t, result.HideResult)
	assert.True(t, result.HideResult.Success)
	assert.False(t, result.HideResult.Combatant.IsVisible)
	assert.Contains(t, result.CombatLog, "Cunning Action")
	assert.Contains(t, result.CombatLog, "Hide")
}

// TDD Cycle P57-8: CunningAction hide failure (spotted)
func TestCunningAction_Hide_Failure(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	char := makeRogueChar(charID, 5)
	char.AbilityScores = json.RawMessage(`{"str":10,"dex":8,"con":12,"int":14,"wis":13,"cha":8}`) // DEX -1
	encounter := makeBasicEncounter(encounterID, 1)

	hostile := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Guard", "guard")
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "guard",
			AbilityScores: json.RawMessage(`{"str":14,"dex":12,"con":12,"int":10,"wis":16,"cha":10}`), // PP=13
		}, nil
	}
	setupUpdateTurnActions(ms)

	roller := dice.NewRoller(func(n int) int { return 0 }) // rolls 0
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := CunningActionCommand{
		Combatant: combatant, Turn: turn, Encounter: encounter, Action: "hide",
		Hostiles: []refdata.Combatant{hostile},
	}
	result, err := svc.CunningAction(context.Background(), cmd, roller)
	require.NoError(t, err)

	assert.NotNil(t, result.HideResult)
	assert.False(t, result.HideResult.Success)
	assert.Contains(t, result.CombatLog, "Failed (spotted by")
}

// TDD Cycle P57-9: Creature NPC uses pre-calculated stealth skill for hide
func TestHide_CreatureUsesStealthSkill(t *testing.T) {
	encounterID, _, _, ms := makeStdTestSetup()
	npcID := uuid.New()
	npc := makeNPCCombatantWithCreature(npcID, encounterID, "Shadow Assassin", "shadow-assassin")

	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		if id == "shadow-assassin" {
			return refdata.Creature{
				ID:            "shadow-assassin",
				AbilityScores: json.RawMessage(`{"str":10,"dex":16,"con":12,"int":10,"wis":10,"cha":8}`),
				Skills:        pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"stealth":9}`), Valid: true},
			}, nil
		}
		return refdata.Creature{
			ID:            "guard",
			AbilityScores: json.RawMessage(`{"str":14,"dex":12,"con":12,"int":10,"wis":10,"cha":10}`),
		}, nil
	}
	setupUpdateTurnActions(ms)

	hostile := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Guard", "guard")
	encounter := makeBasicEncounter(encounterID, 1)
	// deterministic: rolls 19. stealth mod = 9, total = 19+9=28. PP of guard = 10+0 = 10. Success.
	roller := dice.NewRoller(deterministic)
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HideCommand{Combatant: npc, Turn: turn, Encounter: encounter, Hostiles: []refdata.Combatant{hostile}}
	result, err := svc.Hide(context.Background(), cmd, roller)
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.Equal(t, 28, result.StealthRoll) // 19 + stealth skill 9
}

// TDD Cycle P57-10: Visible attacker is NOT revealed (no DB call)
func TestServiceAttack_VisibleAttackerNotRevealed(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 14, 2, "longsword")
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeLongsword(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	visibilityCalled := false
	ms.updateCombatantVisibilityFn = func(ctx context.Context, arg refdata.UpdateCombatantVisibilityParams) (refdata.Combatant, error) {
		visibilityCalled = true
		return refdata.Combatant{}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 6
	})

	attacker := refdata.Combatant{
		ID:          attackerID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Aria",
		PositionCol: "A", PositionRow: 1,
		IsAlive: true, IsVisible: true, // visible
		Conditions: json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Goblin", PositionCol: "B", PositionRow: 1,
		Ac: 13, IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}
	turn := refdata.Turn{
		ID: uuid.New(), EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1,
	}

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn}, roller)
	require.NoError(t, err)
	assert.False(t, result.AttackerRevealed)
	assert.False(t, visibilityCalled)
}

// TDD Cycle P57-11: parseProficiencies with invalid JSON returns empty
func TestParseProficiencies_InvalidJSON(t *testing.T) {
	skills, expertise, jat := parseProficiencies(json.RawMessage(`invalid`))
	assert.Nil(t, skills)
	assert.Nil(t, expertise)
	assert.False(t, jat)
}

// TDD Cycle P57-12: parseProficiencies with nil returns empty
func TestParseProficiencies_Nil(t *testing.T) {
	skills, expertise, jat := parseProficiencies(nil)
	assert.Nil(t, skills)
	assert.Nil(t, expertise)
	assert.False(t, jat)
}

// TDD Cycle P57-13: creatureSkillMod returns false for missing skill
func TestCreatureSkillMod_MissingSkill(t *testing.T) {
	data := json.RawMessage(`{"perception":5}`)
	_, ok := creatureSkillMod(data, "stealth")
	assert.False(t, ok)
}

// TDD Cycle P57-14: creatureSkillMod with invalid JSON returns false
func TestCreatureSkillMod_InvalidJSON(t *testing.T) {
	_, ok := creatureSkillMod(json.RawMessage(`invalid`), "perception")
	assert.False(t, ok)
}

// TDD Cycle P57-15: creatureSkillMod with nil returns false
func TestCreatureSkillMod_Nil(t *testing.T) {
	_, ok := creatureSkillMod(nil, "perception")
	assert.False(t, ok)
}

// TDD Cycle P57-16: stealthModAndMode error - GetCharacter fails
func TestStealthModAndMode_GetCharacterError(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(uuid.New(), uuid.New(), charID, "Test")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, _, err := svc.stealthModAndMode(context.Background(), combatant)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting character for ability")
}

// TDD Cycle P57-17: stealthModAndMode error - bad ability scores
func TestStealthModAndMode_BadAbilityScores(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(uuid.New(), uuid.New(), charID, "Test")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{ID: charID, AbilityScores: json.RawMessage(`invalid`)}, nil
	}

	svc := NewService(ms)
	_, _, err := svc.stealthModAndMode(context.Background(), combatant)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing ability scores")
}

// TDD Cycle P57-18: stealthModAndMode creature GetCreature error
func TestStealthModAndMode_CreatureError(t *testing.T) {
	_, _, _, ms := makeStdTestSetup()
	combatant := makeNPCCombatantWithCreature(uuid.New(), uuid.New(), "Test", "test-creature")

	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{}, fmt.Errorf("creature not found")
	}

	svc := NewService(ms)
	_, _, err := svc.stealthModAndMode(context.Background(), combatant)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting creature for ability")
}

// TDD Cycle P57-19: stealthModAndMode creature bad ability scores
func TestStealthModAndMode_CreatureBadScores(t *testing.T) {
	_, _, _, ms := makeStdTestSetup()
	combatant := makeNPCCombatantWithCreature(uuid.New(), uuid.New(), "Test", "test-creature")

	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "test-creature", AbilityScores: json.RawMessage(`invalid`)}, nil
	}

	svc := NewService(ms)
	_, _, err := svc.stealthModAndMode(context.Background(), combatant)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing creature ability scores")
}

// TDD Cycle P57-20: stealthModAndMode creature without stealth skill uses DEX
func TestStealthModAndMode_CreatureFallbackToDex(t *testing.T) {
	_, _, _, ms := makeStdTestSetup()
	combatant := makeNPCCombatantWithCreature(uuid.New(), uuid.New(), "Goblin", "goblin")

	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "goblin",
			AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`), // DEX +2
		}, nil
	}

	svc := NewService(ms)
	mod, rollMode, err := svc.stealthModAndMode(context.Background(), combatant)
	require.NoError(t, err)
	assert.Equal(t, 2, mod) // DEX +2
	assert.Equal(t, dice.Normal, rollMode)
}

// TDD Cycle P57-21: stealthModAndMode NPC without creature ref
func TestStealthModAndMode_NoPCNoCreature(t *testing.T) {
	_, _, _, ms := makeStdTestSetup()
	combatant := makeNPCCombatant(uuid.New(), uuid.New(), "Guard") // no creature ref

	svc := NewService(ms)
	mod, rollMode, err := svc.stealthModAndMode(context.Background(), combatant)
	require.NoError(t, err)
	assert.Equal(t, 0, mod)
	assert.Equal(t, dice.Normal, rollMode)
}

// TDD Cycle P57-22: passivePerception character GetCharacter error
func TestPassivePerception_CharacterGetError(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(uuid.New(), uuid.New(), charID, "Test")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, err := svc.passivePerception(context.Background(), combatant)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting character for perception")
}

// TDD Cycle P57-23: passivePerception character bad ability scores
func TestPassivePerception_CharacterBadScores(t *testing.T) {
	_, _, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(uuid.New(), uuid.New(), charID, "Test")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{ID: charID, AbilityScores: json.RawMessage(`invalid`)}, nil
	}

	svc := NewService(ms)
	_, err := svc.passivePerception(context.Background(), combatant)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing ability scores")
}

// TDD Cycle P57-24: passivePerception creature bad ability scores
func TestPassivePerception_CreatureBadScores(t *testing.T) {
	_, _, _, ms := makeStdTestSetup()
	combatant := makeNPCCombatantWithCreature(uuid.New(), uuid.New(), "Test", "bad")

	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "bad", AbilityScores: json.RawMessage(`invalid`)}, nil
	}

	svc := NewService(ms)
	_, err := svc.passivePerception(context.Background(), combatant)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing creature ability scores")
}

// TDD Cycle P57-25: passivePerception uses creature's pre-calculated perception skill
func TestPassivePerception_CreatureWithSkills(t *testing.T) {
	_, _, _, ms := makeStdTestSetup()
	combatant := makeNPCCombatantWithCreature(uuid.New(), uuid.New(), "Guard", "guard")

	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "guard",
			AbilityScores: json.RawMessage(`{"str":14,"dex":12,"con":12,"int":10,"wis":14,"cha":10}`),
			Skills:        pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"perception":5}`), Valid: true},
		}, nil
	}

	svc := NewService(ms)
	pp, err := svc.passivePerception(context.Background(), combatant)
	require.NoError(t, err)
	assert.Equal(t, 15, pp) // 10 + pre-calculated perception skill (5)
}

// TDD Cycle P57-iter2-1: Hide success must persist visibility to DB
func TestHide_Success_PersistsVisibility(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	char := makeBasicChar(charID, 30)
	char.AbilityScores = json.RawMessage(`{"str":10,"dex":18,"con":12,"int":14,"wis":13,"cha":8}`) // DEX +4

	hostile := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Goblin", "goblin")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "goblin",
			AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`), // PP=9
		}, nil
	}
	setupUpdateTurnActions(ms)

	visibilityCalled := false
	var visibilityParams refdata.UpdateCombatantVisibilityParams
	ms.updateCombatantVisibilityFn = func(ctx context.Context, arg refdata.UpdateCombatantVisibilityParams) (refdata.Combatant, error) {
		visibilityCalled = true
		visibilityParams = arg
		return refdata.Combatant{ID: arg.ID, IsVisible: arg.IsVisible, Conditions: json.RawMessage(`[]`)}, nil
	}

	encounter := makeBasicEncounter(encounterID, 1)
	roller := dice.NewRoller(deterministic) // rolls max (20)
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := HideCommand{Combatant: combatant, Turn: turn, Encounter: encounter, Hostiles: []refdata.Combatant{hostile}}
	result, err := svc.Hide(context.Background(), cmd, roller)
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.True(t, visibilityCalled, "UpdateCombatantVisibility must be called on hide success")
	assert.Equal(t, combatantID, visibilityParams.ID)
	assert.False(t, visibilityParams.IsVisible)
}

// TDD Cycle P57-iter2-2: Hide tie-breaking — tie goes to spotter (failure)
func TestHide_TieGoesToSpotter(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	char := makeBasicChar(charID, 30)
	// DEX 10 => +0 mod. With roller returning 2 (rolls 3 on d20), stealth total = 3.
	// We want stealth == PP. Guard with WIS 6 => -2 mod, PP = 8. Need stealth=8.
	// DEX 16 => +3 mod. Roller returns 4 (rolls 5), stealth = 5+3 = 8. PP: WIS 16 => +3, PP=13. No.
	// Simpler: DEX 12 => +1 mod. Roller returns 9 (rolls 10), stealth = 10+1 = 11.
	// Hostile WIS 12 => +1, PP = 11. Tie at 11.
	char.AbilityScores = json.RawMessage(`{"str":10,"dex":12,"con":12,"int":14,"wis":13,"cha":8}`) // DEX +1

	hostile := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Guard", "guard")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "guard",
			AbilityScores: json.RawMessage(`{"str":14,"dex":12,"con":12,"int":10,"wis":12,"cha":10}`), // WIS 12 => +1, PP=11
		}, nil
	}
	setupUpdateTurnActions(ms)

	// Roller returns 9 => d20 result is 10, stealth total = 10 + 1 = 11 == PP of 11
	roller := dice.NewRoller(func(n int) int { return 9 })
	svc := NewService(ms)
	turn := makeBasicTurn()
	encounter := makeBasicEncounter(encounterID, 1)

	cmd := HideCommand{Combatant: combatant, Turn: turn, Encounter: encounter, Hostiles: []refdata.Combatant{hostile}}
	result, err := svc.Hide(context.Background(), cmd, roller)
	require.NoError(t, err)

	assert.False(t, result.Success, "tie should go to spotter, hide should fail")
	assert.True(t, result.Combatant.IsVisible, "should remain visible on tie")
	assert.Contains(t, result.CombatLog, "Failed (spotted by")
}

// TDD Cycle P57-iter2-3: CunningAction hide persists visibility to DB
func TestCunningAction_Hide_PersistsVisibility(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	char := makeRogueChar(charID, 5)
	char.AbilityScores = json.RawMessage(`{"str":10,"dex":18,"con":12,"int":14,"wis":13,"cha":8}`) // DEX +4
	char.Proficiencies = profJSON([]string{"stealth"}, nil, false)
	encounter := makeBasicEncounter(encounterID, 1)

	hostile := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Goblin", "goblin")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "goblin",
			AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`), // PP=9
		}, nil
	}
	setupUpdateTurnActions(ms)

	visibilityCalled := false
	var visibilityParams refdata.UpdateCombatantVisibilityParams
	ms.updateCombatantVisibilityFn = func(ctx context.Context, arg refdata.UpdateCombatantVisibilityParams) (refdata.Combatant, error) {
		visibilityCalled = true
		visibilityParams = arg
		return refdata.Combatant{ID: arg.ID, IsVisible: arg.IsVisible, Conditions: json.RawMessage(`[]`)}, nil
	}

	roller := dice.NewRoller(deterministic) // rolls max
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := CunningActionCommand{
		Combatant: combatant, Turn: turn, Encounter: encounter,
		Action: "hide", Hostiles: []refdata.Combatant{hostile},
	}
	result, err := svc.CunningAction(context.Background(), cmd, roller)
	require.NoError(t, err)

	assert.True(t, result.HideResult.Success)
	assert.True(t, visibilityCalled, "UpdateCombatantVisibility must be called on cunning action hide success")
	assert.Equal(t, combatantID, visibilityParams.ID)
	assert.False(t, visibilityParams.IsVisible)
}

// TDD Cycle P57-iter2-4: CunningAction hide tie-breaking — tie goes to spotter
func TestCunningAction_Hide_TieGoesToSpotter(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	char := makeRogueChar(charID, 5)
	char.AbilityScores = json.RawMessage(`{"str":10,"dex":12,"con":12,"int":14,"wis":13,"cha":8}`) // DEX +1
	encounter := makeBasicEncounter(encounterID, 1)

	hostile := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Guard", "guard")

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "guard",
			AbilityScores: json.RawMessage(`{"str":14,"dex":12,"con":12,"int":10,"wis":12,"cha":10}`), // WIS +1, PP=11
		}, nil
	}
	setupUpdateTurnActions(ms)

	// Roller returns 9 => d20 = 10, stealth total = 10+1 = 11 == PP 11
	roller := dice.NewRoller(func(n int) int { return 9 })
	svc := NewService(ms)
	turn := makeBasicTurn()

	cmd := CunningActionCommand{
		Combatant: combatant, Turn: turn, Encounter: encounter,
		Action: "hide", Hostiles: []refdata.Combatant{hostile},
	}
	result, err := svc.CunningAction(context.Background(), cmd, roller)
	require.NoError(t, err)

	assert.NotNil(t, result.HideResult)
	assert.False(t, result.HideResult.Success, "tie should go to spotter, hide should fail")
	assert.Contains(t, result.CombatLog, "Failed (spotted by")
}
