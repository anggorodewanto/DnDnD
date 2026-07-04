package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

func TestResolveAttack_ReactionACBonusRaisesEffectiveAC(t *testing.T) {
	// d20=12 + STR(2) + prof(2) = 16.
	rollerFn := func(maxN int) int {
		if maxN == 20 {
			return 12
		}
		return 4
	}
	base := AttackInput{
		AttackerName: "Grix", TargetName: "Windreth", TargetAC: 15,
		Weapon: makeLongsword(), Scores: AbilityScores{Str: 14, Dex: 10}, ProfBonus: 2, DistanceFt: 5,
	}
	hit, err := ResolveAttack(base, dice.NewRoller(rollerFn))
	require.NoError(t, err)
	require.True(t, hit.Hit, "16 vs AC 15 should hit")

	rd := base
	rd.ReactionACBonus = 3
	rd.ReactionReason = "Defensive Duelist"
	miss, err := ResolveAttack(rd, dice.NewRoller(rollerFn))
	require.NoError(t, err)
	assert.Equal(t, 18, miss.EffectiveAC, "reaction +3 raises effective AC to 18")
	assert.False(t, miss.Hit, "16 vs AC 18 should now miss")
	assert.Equal(t, "Defensive Duelist", miss.ReactionReason)
}

func finesseWeapon() refdata.Weapon {
	return refdata.Weapon{ID: "rapier", Name: "Rapier", Damage: "1d8", DamageType: "piercing", Properties: []string{"finesse"}}
}

func TestDefensiveDuelistReaction_OfferedWithFeatAndFinesse(t *testing.T) {
	feats, _ := json.Marshal([]CharacterFeature{{Name: "Defensive Duelist", MechanicalEffect: `[{"effect_type":"reaction_add_proficiency_to_ac"}]`}})
	opt, ok := defensiveDuelistReaction(feats, finesseWeapon(), 3)
	require.True(t, ok)
	assert.Equal(t, "defensive-duelist", opt.ID)
	assert.Equal(t, 3, opt.ACBonus)
	assert.Equal(t, "Defensive Duelist", opt.Reason)
	assert.Contains(t, opt.Label, "+3 AC")
}

func TestDefensiveDuelistReaction_NoneWithoutFeat(t *testing.T) {
	feats, _ := json.Marshal([]CharacterFeature{{Name: "Alert"}})
	_, ok := defensiveDuelistReaction(feats, finesseWeapon(), 3)
	assert.False(t, ok)
}

func TestDefensiveDuelistReaction_NoneWithoutFinesse(t *testing.T) {
	feats, _ := json.Marshal([]CharacterFeature{{Name: "Defensive Duelist"}})
	_, ok := defensiveDuelistReaction(feats, makeLongsword(), 3) // longsword: not finesse
	assert.False(t, ok)
}

func availReactionStore(t *testing.T, target refdata.Combatant, char refdata.Character, mainHand refdata.Weapon) *mockStore {
	t.Helper()
	ms := defaultMockStore()
	ms.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) { return target, nil }
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) { return mainHand, nil }
	// No active turn → reaction is free (CanDeclareReaction returns true).
	ms.getActiveTurnByEncounterIDFn = func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, sql.ErrNoRows
	}
	return ms
}

func TestAvailableReactions_DefensiveDuelistWhenReactionFree(t *testing.T) {
	charID := uuid.New()
	feats, _ := json.Marshal([]CharacterFeature{{Name: "Defensive Duelist"}})
	char := refdata.Character{
		ID: charID, ProficiencyBonus: 3,
		Features:         pqtype.NullRawMessage{RawMessage: feats, Valid: true},
		EquippedMainHand: sql.NullString{String: "rapier", Valid: true},
	}
	target := refdata.Combatant{
		ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Windreth", Conditions: json.RawMessage(`[]`),
	}
	svc := NewService(availReactionStore(t, target, char, finesseWeapon()))

	opts, err := svc.AvailableReactions(context.Background(), target, uuid.New())
	require.NoError(t, err)
	require.Len(t, opts, 1)
	assert.Equal(t, "defensive-duelist", opts[0].ID)
	assert.Equal(t, 3, opts[0].ACBonus)
}

func TestAvailableReactions_NoneForNPCTarget(t *testing.T) {
	target := refdata.Combatant{ID: uuid.New(), IsNpc: true, DisplayName: "Grix", Conditions: json.RawMessage(`[]`)}
	svc := NewService(defaultMockStore())
	opts, err := svc.AvailableReactions(context.Background(), target, uuid.New())
	require.NoError(t, err)
	assert.Empty(t, opts, "NPCs have no PC-driven reaction prompt")
}

func TestServiceAttack_ReactionACBonus_FlipsHitToMiss(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeCharacterWithFeats(14, 10, 2, "longsword", nil, []CharacterClass{{Class: "Fighter", Level: 1}})
	char.ID = charID
	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) { return makeLongsword(), nil }
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	svc := NewService(ms)

	encID := uuid.New()
	attackerID := uuid.New()
	attacker := refdata.Combatant{
		ID: attackerID, EncounterID: encID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Grix", PositionCol: "A", PositionRow: 1, IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encID, DisplayName: "Windreth", PositionCol: "B", PositionRow: 1, Ac: 15,
		IsAlive: true, IsNpc: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	turn := refdata.Turn{ID: uuid.New(), EncounterID: encID, CombatantID: attackerID, AttacksRemaining: 1}

	// d20=12 + STR(2) + prof(2) = 16 → would hit AC 15; +3 reaction → AC 18 → miss.
	roller := dice.NewRoller(func(maxN int) int {
		if maxN == 20 {
			return 12
		}
		return 4
	})
	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn, ReactionACBonus: 3, ReactionReason: "Defensive Duelist"}, roller)
	require.NoError(t, err)
	assert.Equal(t, 18, result.EffectiveAC)
	assert.False(t, result.Hit, "the pre-declared +3 AC reaction should turn the hit into a miss")
	assert.Equal(t, "Defensive Duelist", result.ReactionReason)
}

func TestFormatReactionDeclared_ShowsReactionAndBonus(t *testing.T) {
	line := FormatReactionDeclared("Windreth", ReactionOption{ID: "defensive-duelist", ACBonus: 3, Reason: "Defensive Duelist"})
	assert.Contains(t, line, "Windreth")
	assert.Contains(t, line, "Defensive Duelist")
	assert.Contains(t, line, "+3 AC")
}

// Multiple NPCs attacking one PC in a round is common: once the PC spends their
// reaction (e.g. against the first attacker), it must stay spent for the rest of
// the round — even though the PC has no turn row yet (they act later in
// initiative). The used reaction *declaration* (used_on_round) is the source of
// truth, not the PC's turn row.
func TestCanDeclareReaction_FalseWhenReactionSpentThisRound(t *testing.T) {
	encID, combatantID := uuid.New(), uuid.New()
	ms := defaultMockStore()
	ms.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, Conditions: json.RawMessage(`[]`)}, nil
	}
	ms.getActiveTurnByEncounterIDFn = func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{RoundNumber: 2}, nil
	}
	ms.listReactionDeclarationsByCombatantFn = func(_ context.Context, _ refdata.ListReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{{Status: "used", UsedOnRound: sql.NullInt32{Int32: 2, Valid: true}}}, nil
	}
	free, err := NewService(ms).CanDeclareReaction(context.Background(), encID, combatantID)
	require.NoError(t, err)
	assert.False(t, free, "a reaction already spent this round is not free again — even with no turn row")
}

func TestCanDeclareReaction_TrueWhenReactionSpentPriorRound(t *testing.T) {
	encID, combatantID := uuid.New(), uuid.New()
	ms := defaultMockStore()
	ms.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, Conditions: json.RawMessage(`[]`)}, nil
	}
	ms.getActiveTurnByEncounterIDFn = func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{RoundNumber: 3}, nil
	}
	ms.listReactionDeclarationsByCombatantFn = func(_ context.Context, _ refdata.ListReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{{Status: "used", UsedOnRound: sql.NullInt32{Int32: 2, Valid: true}}}, nil
	}
	free, err := NewService(ms).CanDeclareReaction(context.Background(), encID, combatantID)
	require.NoError(t, err)
	assert.True(t, free, "a reaction spent last round refreshes this round")
}

func TestAvailableReactions_EmptyAfterReactionSpentThisRound(t *testing.T) {
	charID := uuid.New()
	feats, _ := json.Marshal([]CharacterFeature{{Name: "Defensive Duelist"}})
	char := refdata.Character{
		ID: charID, ProficiencyBonus: 3,
		Features:         pqtype.NullRawMessage{RawMessage: feats, Valid: true},
		EquippedMainHand: sql.NullString{String: "rapier", Valid: true},
	}
	target := refdata.Combatant{
		ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Windreth", Conditions: json.RawMessage(`[]`),
	}
	ms := availReactionStore(t, target, char, finesseWeapon())
	// Active turn this round, and the PC already spent a reaction this round.
	ms.getActiveTurnByEncounterIDFn = func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{RoundNumber: 1}, nil
	}
	ms.listReactionDeclarationsByCombatantFn = func(_ context.Context, _ refdata.ListReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{{Status: "used", UsedOnRound: sql.NullInt32{Int32: 1, Valid: true}}}, nil
	}
	opts, err := NewService(ms).AvailableReactions(context.Background(), target, uuid.New())
	require.NoError(t, err)
	assert.Empty(t, opts, "a second attacker this round sees no reaction to offer")
}

func TestApplyReactionToRoll_NormalHitBecomesMiss(t *testing.T) {
	// d20=12, +4 mods = 16 vs base AC 15 hit; +3 reaction → AC 18 → miss.
	r := &AttackRollResult{ToHitRoll: 12, ToHitTotal: 16, Hit: true}
	applyReactionToRoll(r, 15, 3)
	assert.False(t, r.Hit, "16 vs boosted AC 18 must miss")
}

func TestApplyReactionToRoll_StillHitsWhenMarginBigEnough(t *testing.T) {
	r := &AttackRollResult{ToHitRoll: 18, ToHitTotal: 22, Hit: true}
	applyReactionToRoll(r, 15, 3)
	assert.True(t, r.Hit, "22 vs boosted AC 18 still hits")
}

func TestApplyReactionToRoll_CritAlwaysHits(t *testing.T) {
	r := &AttackRollResult{ToHitRoll: 20, ToHitTotal: 24, Hit: true, Critical: true}
	applyReactionToRoll(r, 30, 5)
	assert.True(t, r.Hit, "a natural 20 hits regardless of the reaction bonus")
}

func TestApplyReactionToRoll_Nat1AlwaysMisses(t *testing.T) {
	r := &AttackRollResult{ToHitRoll: 1, ToHitTotal: 6, Hit: false}
	applyReactionToRoll(r, 3, 0)
	assert.False(t, r.Hit, "a natural 1 misses regardless")
}
