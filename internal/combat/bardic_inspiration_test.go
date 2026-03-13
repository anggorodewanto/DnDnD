package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// --- TDD Cycle 1: BardicInspirationDie scaling ---

func TestBardicInspirationDie(t *testing.T) {
	tests := []struct {
		level    int
		expected string
	}{
		{1, "d6"},
		{4, "d6"},
		{5, "d8"},
		{9, "d8"},
		{10, "d10"},
		{14, "d10"},
		{15, "d12"},
		{20, "d12"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, BardicInspirationDie(tt.level), "level %d", tt.level)
	}
}

// --- TDD Cycle 2: BardicInspirationMaxUses (CHA mod, min 1) ---

func TestBardicInspirationMaxUses(t *testing.T) {
	tests := []struct {
		chaScore int
		expected int
	}{
		{20, 5}, // +5
		{14, 2}, // +2
		{10, 1}, // +0 → minimum 1
		{8, 1},  // -1 → minimum 1
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, BardicInspirationMaxUses(tt.chaScore), "CHA %d", tt.chaScore)
	}
}

// --- TDD Cycle 3: FontOfInspiration recharge type ---

func TestBardicInspirationRechargeType(t *testing.T) {
	assert.Equal(t, "long", BardicInspirationRechargeType(1))
	assert.Equal(t, "long", BardicInspirationRechargeType(4))
	assert.Equal(t, "short", BardicInspirationRechargeType(5))
	assert.Equal(t, "short", BardicInspirationRechargeType(20))
}

// --- TDD Cycle 4: ValidateBardicInspiration ---

func TestValidateBardicInspiration(t *testing.T) {
	t.Run("not a bard", func(t *testing.T) {
		err := ValidateBardicInspiration(0, false, 3)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Bard class")
	})

	t.Run("already has inspiration", func(t *testing.T) {
		err := ValidateBardicInspiration(3, true, 3)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already has Bardic Inspiration")
	})

	t.Run("no uses remaining", func(t *testing.T) {
		err := ValidateBardicInspiration(3, false, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no Bardic Inspiration uses remaining")
	})

	t.Run("valid", func(t *testing.T) {
		err := ValidateBardicInspiration(3, false, 2)
		assert.NoError(t, err)
	})
}

// --- TDD Cycle 5: FormatBardicInspirationGrant ---

func TestFormatBardicInspirationGrant(t *testing.T) {
	result := FormatBardicInspirationGrant("Thorn", "Aria", "d6")
	assert.Contains(t, result, "Thorn")
	assert.Contains(t, result, "Aria")
	assert.Contains(t, result, "d6")
}

// --- TDD Cycle 6: FormatBardicInspirationUse ---

func TestFormatBardicInspirationUse(t *testing.T) {
	result := FormatBardicInspirationUse("Aria", 4, "d6", 18)
	assert.Contains(t, result, "Aria")
	assert.Contains(t, result, "+4")
	assert.Contains(t, result, "d6")
	assert.Contains(t, result, "18")
}

// --- TDD Cycle 7: FormatBardicInspirationExpired ---

func TestFormatBardicInspirationExpired(t *testing.T) {
	result := FormatBardicInspirationExpired("Thorn")
	assert.Contains(t, result, "Thorn")
	assert.Contains(t, result, "expired")
}

// --- TDD Cycle 8: bardLevelFromJSON ---

func TestBardLevelFromJSON(t *testing.T) {
	assert.Equal(t, 5, bardLevelFromJSON([]byte(`[{"class":"Bard","level":5}]`)))
	assert.Equal(t, 0, bardLevelFromJSON([]byte(`[{"class":"Fighter","level":10}]`)))
	assert.Equal(t, 0, bardLevelFromJSON(nil))
	assert.Equal(t, 0, bardLevelFromJSON([]byte(`invalid`)))
}

// --- TDD Cycle 9: HasBardClass ---

func TestHasBardClass(t *testing.T) {
	assert.True(t, HasBardClass([]byte(`[{"class":"Bard","level":3}]`)))
	assert.False(t, HasBardClass([]byte(`[{"class":"Fighter","level":10}]`)))
	assert.False(t, HasBardClass(nil))
}

// --- TDD Cycle 10: IsBardicInspirationExpired ---

func TestIsBardicInspirationExpired(t *testing.T) {
	now := time.Now()

	t.Run("not expired (just granted)", func(t *testing.T) {
		assert.False(t, IsBardicInspirationExpired(now, now))
	})

	t.Run("not expired (9 minutes)", func(t *testing.T) {
		assert.False(t, IsBardicInspirationExpired(now, now.Add(9*time.Minute)))
	})

	t.Run("expired (10 minutes)", func(t *testing.T) {
		assert.True(t, IsBardicInspirationExpired(now, now.Add(10*time.Minute)))
	})

	t.Run("expired (11 minutes)", func(t *testing.T) {
		assert.True(t, IsBardicInspirationExpired(now, now.Add(11*time.Minute)))
	})
}

// --- TDD Cycle 11: ApplyBardicInspirationToCombatant and ClearBardicInspirationFromCombatant ---

func TestApplyAndClearBardicInspiration(t *testing.T) {
	c := refdata.Combatant{DisplayName: "Aria"}
	now := time.Now()

	// Apply
	c = ApplyBardicInspirationToCombatant(c, "d6", "Thorn", now)
	assert.True(t, CombatantHasBardicInspiration(c))
	assert.Equal(t, "d6", c.BardicInspirationDie.String)
	assert.Equal(t, "Thorn", c.BardicInspirationSource.String)
	assert.True(t, c.BardicInspirationGrantedAt.Valid)

	// Clear
	c = ClearBardicInspirationFromCombatant(c)
	assert.False(t, CombatantHasBardicInspiration(c))
	assert.False(t, c.BardicInspirationDie.Valid)
	assert.False(t, c.BardicInspirationSource.Valid)
	assert.False(t, c.BardicInspirationGrantedAt.Valid)
}

// --- TDD Cycle 12: FormatBardicInspirationStatus ---

func TestFormatBardicInspirationStatus(t *testing.T) {
	result := FormatBardicInspirationStatus("d8")
	assert.Contains(t, result, "d8")
	assert.Contains(t, result, "Bardic Inspiration")
}

// --- TDD Cycle 13: Turn status includes Bardic Inspiration ---

func TestBuildResourceListWithBardicInspiration(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	}
	combatant := refdata.Combatant{
		BardicInspirationDie: sql.NullString{String: "d6", Valid: true},
	}

	parts := BuildResourceListWithInspiration(turn, combatant)
	found := false
	for _, p := range parts {
		if p == "\U0001f3b5 Bardic Inspiration (d6)" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected bardic inspiration in resource list, got: %v", parts)
}

func TestBuildResourceListWithoutBardicInspiration(t *testing.T) {
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	}
	combatant := refdata.Combatant{}

	parts := BuildResourceListWithInspiration(turn, combatant)
	for _, p := range parts {
		assert.NotContains(t, p, "Bardic Inspiration")
	}
}

// --- TDD Cycle 14: Service.GrantBardicInspiration happy path ---

func bardTestCharacter(bardLevel int, chaScore int, usesRemaining int) refdata.Character {
	classes, _ := json.Marshal([]CharacterClass{{Class: "Bard", Level: bardLevel}})
	scores, _ := json.Marshal(AbilityScores{Cha: chaScore, Str: 10, Dex: 10, Con: 10, Int: 10, Wis: 10})
	featureUses, _ := json.Marshal(map[string]int{"bardic-inspiration": usesRemaining})
	return refdata.Character{
		ID:            uuid.New(),
		Classes:       classes,
		AbilityScores: scores,
		FeatureUses:   pqtype.NullRawMessage{RawMessage: featureUses, Valid: true},
	}
}

func TestGrantBardicInspiration_HappyPath(t *testing.T) {
	charID := uuid.New()
	bardID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	char := bardTestCharacter(5, 16, 3) // level 5 bard, CHA 16, 3 uses
	char.ID = charID

	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return char, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, BonusActionUsed: true}, nil
	}
	store.updateCombatantBardicInspirationFn = func(ctx context.Context, arg refdata.UpdateCombatantBardicInspirationParams) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:                   targetID,
			DisplayName:          "Aria",
			BardicInspirationDie: arg.BardicInspirationDie,
			BardicInspirationSource: arg.BardicInspirationSource,
			BardicInspirationGrantedAt: arg.BardicInspirationGrantedAt,
		}, nil
	}

	svc := NewService(store)
	result, err := svc.GrantBardicInspiration(context.Background(), BardicInspirationCommand{
		Bard: refdata.Combatant{
			ID:          bardID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Thorn",
		},
		Target: refdata.Combatant{
			ID:          targetID,
			DisplayName: "Aria",
		},
		Turn: refdata.Turn{ID: turnID},
	})
	require.NoError(t, err)
	assert.Equal(t, "d8", result.Die)
	assert.Equal(t, 2, result.UsesLeft)
	assert.Contains(t, result.CombatLog, "Thorn")
	assert.Contains(t, result.CombatLog, "Aria")
	assert.Contains(t, result.CombatLog, "d8")
	assert.Contains(t, result.Notification, "d8")
	assert.Contains(t, result.Notification, "Thorn")
}

// --- TDD Cycle 15: Service.GrantBardicInspiration validation errors ---

func TestGrantBardicInspiration_BonusActionSpent(t *testing.T) {
	store := defaultMockStore()
	svc := NewService(store)

	_, err := svc.GrantBardicInspiration(context.Background(), BardicInspirationCommand{
		Turn: refdata.Turn{BonusActionUsed: true},
	})
	assert.ErrorIs(t, err, ErrResourceSpent)
}

func TestGrantBardicInspiration_NotPC(t *testing.T) {
	store := defaultMockStore()
	svc := NewService(store)

	_, err := svc.GrantBardicInspiration(context.Background(), BardicInspirationCommand{
		Bard: refdata.Combatant{},
		Turn: refdata.Turn{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not NPC")
}

func TestGrantBardicInspiration_SelfTarget(t *testing.T) {
	store := defaultMockStore()
	svc := NewService(store)
	id := uuid.New()

	_, err := svc.GrantBardicInspiration(context.Background(), BardicInspirationCommand{
		Bard:   refdata.Combatant{ID: id, CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true}},
		Target: refdata.Combatant{ID: id},
		Turn:   refdata.Turn{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "yourself")
}

func TestGrantBardicInspiration_NotBard(t *testing.T) {
	charID := uuid.New()
	char := refdata.Character{
		ID:      charID,
		Classes: json.RawMessage(`[{"class":"Fighter","level":5}]`),
	}
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	svc := NewService(store)

	_, err := svc.GrantBardicInspiration(context.Background(), BardicInspirationCommand{
		Bard:   refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
		Target: refdata.Combatant{ID: uuid.New()},
		Turn:   refdata.Turn{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Bard class")
}

func TestGrantBardicInspiration_TargetAlreadyInspired(t *testing.T) {
	charID := uuid.New()
	char := bardTestCharacter(3, 16, 3)
	char.ID = charID

	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	svc := NewService(store)

	_, err := svc.GrantBardicInspiration(context.Background(), BardicInspirationCommand{
		Bard: refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
		Target: refdata.Combatant{
			ID:                   uuid.New(),
			BardicInspirationDie: sql.NullString{String: "d6", Valid: true},
		},
		Turn: refdata.Turn{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already has Bardic Inspiration")
}

func TestGrantBardicInspiration_NoUsesRemaining(t *testing.T) {
	charID := uuid.New()
	char := bardTestCharacter(3, 16, 0) // 0 uses remaining
	char.ID = charID

	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	svc := NewService(store)

	_, err := svc.GrantBardicInspiration(context.Background(), BardicInspirationCommand{
		Bard:   refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
		Target: refdata.Combatant{ID: uuid.New()},
		Turn:   refdata.Turn{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no Bardic Inspiration uses remaining")
}

// --- TDD Cycle 16: FormatBardicInspirationNotification ---

// --- TDD Cycle 17: parseBardicInspirationUses with no feature_uses ---

func TestParseBardicInspirationUses_NoFeatureUses(t *testing.T) {
	char := refdata.Character{}
	featureUses, remaining, err := parseBardicInspirationUses(char)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining)
	assert.NotNil(t, featureUses)
}

func TestParseBardicInspirationUses_InvalidJSON(t *testing.T) {
	char := refdata.Character{
		FeatureUses: pqtype.NullRawMessage{RawMessage: json.RawMessage(`invalid`), Valid: true},
	}
	_, _, err := parseBardicInspirationUses(char)
	assert.Error(t, err)
}

// --- TDD Cycle 18: GrantBardicInspiration error paths in store calls ---

func TestGrantBardicInspiration_GetCharacterError(t *testing.T) {
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}
	svc := NewService(store)

	_, err := svc.GrantBardicInspiration(context.Background(), BardicInspirationCommand{
		Bard:   refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true}},
		Target: refdata.Combatant{ID: uuid.New()},
		Turn:   refdata.Turn{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

func TestGrantBardicInspiration_UpdateFeatureUsesError(t *testing.T) {
	charID := uuid.New()
	char := bardTestCharacter(3, 16, 3)
	char.ID = charID

	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}
	svc := NewService(store)

	_, err := svc.GrantBardicInspiration(context.Background(), BardicInspirationCommand{
		Bard:   refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
		Target: refdata.Combatant{ID: uuid.New()},
		Turn:   refdata.Turn{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating feature_uses")
}

func TestGrantBardicInspiration_UpdateTurnActionsError(t *testing.T) {
	charID := uuid.New()
	char := bardTestCharacter(3, 16, 3)
	char.ID = charID

	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return char, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}
	svc := NewService(store)

	_, err := svc.GrantBardicInspiration(context.Background(), BardicInspirationCommand{
		Bard:   refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
		Target: refdata.Combatant{ID: uuid.New()},
		Turn:   refdata.Turn{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

func TestGrantBardicInspiration_PersistInspirationError(t *testing.T) {
	charID := uuid.New()
	char := bardTestCharacter(3, 16, 3)
	char.ID = charID

	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return char, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{BonusActionUsed: true}, nil
	}
	store.updateCombatantBardicInspirationFn = func(ctx context.Context, arg refdata.UpdateCombatantBardicInspirationParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}
	svc := NewService(store)

	_, err := svc.GrantBardicInspiration(context.Background(), BardicInspirationCommand{
		Bard:   refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
		Target: refdata.Combatant{ID: uuid.New()},
		Turn:   refdata.Turn{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating combatant bardic inspiration")
}

func TestFormatBardicInspirationNotification(t *testing.T) {
	result := FormatBardicInspirationNotification("d8", "Thorn")
	assert.Contains(t, result, "d8")
	assert.Contains(t, result, "Thorn")
	assert.Contains(t, result, "attack roll")
	assert.Contains(t, result, "ability check")
	assert.Contains(t, result, "saving throw")
}

// --- TDD Cycle 19: UseBardicInspiration happy path ---

func TestUseBardicInspiration_HappyPath(t *testing.T) {
	targetID := uuid.New()
	store := defaultMockStore()
	store.updateCombatantBardicInspirationFn = func(ctx context.Context, arg refdata.UpdateCombatantBardicInspirationParams) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          targetID,
			DisplayName: "Aria",
			// Cleared
			BardicInspirationDie:       arg.BardicInspirationDie,
			BardicInspirationSource:    arg.BardicInspirationSource,
			BardicInspirationGrantedAt: arg.BardicInspirationGrantedAt,
		}, nil
	}

	svc := NewService(store)
	roller := dice.NewRoller(func(max int) int { return 4 }) // always rolls 4

	result, err := svc.UseBardicInspiration(context.Background(), UseBardicInspirationCommand{
		Combatant: refdata.Combatant{
			ID:                      targetID,
			DisplayName:             "Aria",
			BardicInspirationDie:    sql.NullString{String: "d8", Valid: true},
			BardicInspirationSource: sql.NullString{String: "Thorn", Valid: true},
			BardicInspirationGrantedAt: sql.NullTime{Time: time.Now(), Valid: true},
		},
		OriginalTotal: 14,
	}, roller)

	require.NoError(t, err)
	assert.Equal(t, 4, result.DieResult)
	assert.Equal(t, 18, result.NewTotal)
	assert.Equal(t, "d8", result.Die)
	assert.Contains(t, result.CombatLog, "Aria")
	assert.Contains(t, result.CombatLog, "+4")
	assert.Contains(t, result.CombatLog, "d8")
	assert.Contains(t, result.CombatLog, "18")
	// Combatant should have inspiration cleared
	assert.False(t, CombatantHasBardicInspiration(result.Combatant))
}

// --- TDD Cycle 20: UseBardicInspiration no inspiration ---

func TestUseBardicInspiration_NoInspiration(t *testing.T) {
	store := defaultMockStore()
	svc := NewService(store)
	roller := dice.NewRoller(nil)

	_, err := svc.UseBardicInspiration(context.Background(), UseBardicInspirationCommand{
		Combatant:     refdata.Combatant{DisplayName: "Aria"},
		OriginalTotal: 14,
	}, roller)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not have Bardic Inspiration")
}

// --- TDD Cycle 21: UseBardicInspiration persist error ---

func TestUseBardicInspiration_PersistError(t *testing.T) {
	store := defaultMockStore()
	store.updateCombatantBardicInspirationFn = func(ctx context.Context, arg refdata.UpdateCombatantBardicInspirationParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}
	svc := NewService(store)
	roller := dice.NewRoller(func(max int) int { return 3 })

	_, err := svc.UseBardicInspiration(context.Background(), UseBardicInspirationCommand{
		Combatant: refdata.Combatant{
			ID:                      uuid.New(),
			DisplayName:             "Aria",
			BardicInspirationDie:    sql.NullString{String: "d6", Valid: true},
			BardicInspirationSource: sql.NullString{String: "Thorn", Valid: true},
			BardicInspirationGrantedAt: sql.NullTime{Time: time.Now(), Valid: true},
		},
		OriginalTotal: 10,
	}, roller)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "clearing bardic inspiration")
}

// --- TDD Cycle 22: GrantBardicInspiration accepts time.Time parameter ---

func TestGrantBardicInspiration_AcceptsNowParameter(t *testing.T) {
	charID := uuid.New()
	bardID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	char := bardTestCharacter(5, 16, 3)
	char.ID = charID

	fixedTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	var capturedGrantedAt time.Time

	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return char, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, BonusActionUsed: true}, nil
	}
	store.updateCombatantBardicInspirationFn = func(ctx context.Context, arg refdata.UpdateCombatantBardicInspirationParams) (refdata.Combatant, error) {
		capturedGrantedAt = arg.BardicInspirationGrantedAt.Time
		return refdata.Combatant{
			ID:                         targetID,
			DisplayName:                "Aria",
			BardicInspirationDie:       arg.BardicInspirationDie,
			BardicInspirationSource:    arg.BardicInspirationSource,
			BardicInspirationGrantedAt: arg.BardicInspirationGrantedAt,
		}, nil
	}

	svc := NewService(store)
	result, err := svc.GrantBardicInspiration(context.Background(), BardicInspirationCommand{
		Bard: refdata.Combatant{
			ID:          bardID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Thorn",
		},
		Target: refdata.Combatant{
			ID:          targetID,
			DisplayName: "Aria",
		},
		Turn: refdata.Turn{ID: turnID},
		Now:  fixedTime,
	})
	require.NoError(t, err)
	assert.Equal(t, "d8", result.Die)
	assert.Equal(t, fixedTime, capturedGrantedAt)
}
