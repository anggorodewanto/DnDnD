package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// SR-006 / Phase 88a — Service.Attack must thread equipped magic items into
// the Feature Effect System. A +1 longsword adds +1 to both the attack-roll
// total and the damage total. Before the fix, populateAttackFES dropped
// magic-item FeatureDefinitions on the floor.
func TestServiceAttack_PlusOneLongsword_AddsAttackAndDamageBonus(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	classes := []CharacterClass{{Class: "Fighter", Level: 1}}
	char := makeCharacterWithFeats(16, 10, 2, "longsword", nil, classes)
	char.ID = charID
	inv, err := json.Marshal([]character.InventoryItem{
		{
			ItemID:     "longsword-plus-1",
			Name:       "+1 Longsword",
			Type:       "weapon",
			IsMagic:    true,
			MagicBonus: 1,
			Equipped:   true,
		},
	})
	require.NoError(t, err)
	char.Inventory = pqtype.NullRawMessage{RawMessage: inv, Valid: true}

	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) { return makeLongsword(), nil }
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	svc := NewService(ms)

	// d20 = 10, damage d8 = 5. Without magic: total = 10 + 3(STR) + 2(prof) = 15.
	// With +1: total = 16. Damage: 5 + 3 + 1 = 9.
	roller := dice.NewRoller(func(maxN int) int {
		if maxN == 20 {
			return 10
		}
		return 5
	})

	attacker := refdata.Combatant{
		ID: attackerID, EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Aria", PositionCol: "A", PositionRow: 1,
		IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID: targetID, EncounterID: encounterID,
		DisplayName: "Goblin", PositionCol: "B", PositionRow: 1, Ac: 16,
		IsAlive: true, IsNpc: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn}, roller)
	require.NoError(t, err)
	assert.Equal(t, 16, result.D20Roll.Total, "+1 longsword must add +1 to attack roll total")
	require.True(t, result.Hit, "AC 16 with attack total 16 should hit")
	assert.Equal(t, 9, result.DamageTotal, "+1 longsword must add +1 to damage on top of 1d8(5)+STR(3)")
}

// Negative control: an equipped but NON-magic longsword gets no bonus.
func TestServiceAttack_PlainLongsword_NoMagicBonus(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	classes := []CharacterClass{{Class: "Fighter", Level: 1}}
	char := makeCharacterWithFeats(16, 10, 2, "longsword", nil, classes)
	char.ID = charID
	inv, _ := json.Marshal([]character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Type: "weapon", Equipped: true},
	})
	char.Inventory = pqtype.NullRawMessage{RawMessage: inv, Valid: true}

	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) { return makeLongsword(), nil }
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	svc := NewService(ms)

	roller := dice.NewRoller(func(maxN int) int {
		if maxN == 20 {
			return 10
		}
		return 5
	})

	attacker := refdata.Combatant{
		ID: attackerID, EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Aria", PositionCol: "A", PositionRow: 1,
		IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID: targetID, EncounterID: encounterID,
		DisplayName: "Goblin", PositionCol: "B", PositionRow: 1, Ac: 12,
		IsAlive: true, IsNpc: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn}, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, 8, result.DamageTotal, "plain longsword damage = 1d8(5)+STR(3), no magic bonus")
}
