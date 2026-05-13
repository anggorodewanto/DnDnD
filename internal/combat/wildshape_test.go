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
	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

func TestWildShapeCRLimit(t *testing.T) {
	tests := []struct {
		level  int
		moon   bool
		wantCR float64
	}{
		// Standard druid
		{2, false, 0.25},
		{3, false, 0.25},
		{4, false, 0.5},
		{7, false, 0.5},
		{8, false, 1},
		{10, false, 1},
		{20, false, 1},
		// Circle of the Moon
		{2, true, 1},
		{5, true, 1},
		{6, true, 2},
		{9, true, 3},
		{12, true, 4},
		{15, true, 5},
		{18, true, 6},
	}
	for _, tt := range tests {
		got := WildShapeCRLimit(tt.level, tt.moon)
		assert.InDelta(t, tt.wantCR, got, 0.001,
			"WildShapeCRLimit(%d, moon=%v)", tt.level, tt.moon)
	}
}

func TestSnapshotCombatantState(t *testing.T) {
	c := refdata.Combatant{
		HpMax:     28,
		HpCurrent: 25,
		Ac:        16,
	}
	snap, err := SnapshotCombatantState(c, 30, json.RawMessage(`{"str":10,"dex":14,"con":12,"int":16,"wis":18,"cha":10}`))
	require.NoError(t, err)

	var data WildShapeSnapshot
	err = json.Unmarshal(snap, &data)
	require.NoError(t, err)
	assert.Equal(t, int32(28), data.HpMax)
	assert.Equal(t, int32(25), data.HpCurrent)
	assert.Equal(t, int32(16), data.Ac)
	assert.Equal(t, int32(30), data.SpeedFt)
	assert.Equal(t, 10, data.AbilityScores["str"])
	assert.Equal(t, 18, data.AbilityScores["wis"])
}

func TestApplyBeastFormToCombatant(t *testing.T) {
	c := refdata.Combatant{
		HpMax:     28,
		HpCurrent: 25,
		Ac:        16,
	}
	beast := refdata.Creature{
		ID:            "wolf",
		HpAverage:     11,
		Ac:            13,
		Speed:         json.RawMessage(`{"walk":40}`),
		AbilityScores: json.RawMessage(`{"str":12,"dex":15,"con":12,"int":3,"wis":12,"cha":6}`),
	}

	result := ApplyBeastFormToCombatant(c, beast)
	assert.Equal(t, int32(11), result.HpMax)
	assert.Equal(t, int32(11), result.HpCurrent)
	assert.Equal(t, int32(13), result.Ac)
	assert.True(t, result.IsWildShaped)
	assert.Equal(t, "wolf", result.WildShapeCreatureRef.String)
	assert.True(t, result.WildShapeCreatureRef.Valid)
}

func TestRevertWildShape(t *testing.T) {
	original := WildShapeSnapshot{
		HpMax:         28,
		HpCurrent:     25,
		Ac:            16,
		SpeedFt:       30,
		AbilityScores: map[string]int{"str": 10, "dex": 14, "con": 12},
	}
	snapJSON, _ := json.Marshal(original)

	c := refdata.Combatant{
		IsWildShaped:         true,
		WildShapeCreatureRef: sql.NullString{String: "wolf", Valid: true},
		WildShapeOriginal:    pqtype.NullRawMessage{RawMessage: snapJSON, Valid: true},
		HpMax:                11,
		HpCurrent:            0, // beast HP reached 0
		Ac:                   13,
	}

	reverted, overflow, err := RevertWildShape(c, 5) // 5 overflow damage
	require.NoError(t, err)
	assert.False(t, reverted.IsWildShaped)
	assert.False(t, reverted.WildShapeCreatureRef.Valid)
	assert.False(t, reverted.WildShapeOriginal.Valid)
	assert.Equal(t, int32(28), reverted.HpMax)
	assert.Equal(t, int32(20), reverted.HpCurrent) // 25 - 5 overflow
	assert.Equal(t, int32(16), reverted.Ac)
	assert.Equal(t, int32(5), overflow)
}

func TestRevertWildShape_NoOverflow(t *testing.T) {
	original := WildShapeSnapshot{
		HpMax:         28,
		HpCurrent:     28,
		Ac:            16,
		SpeedFt:       30,
		AbilityScores: map[string]int{"str": 10},
	}
	snapJSON, _ := json.Marshal(original)

	c := refdata.Combatant{
		IsWildShaped:      true,
		WildShapeOriginal: pqtype.NullRawMessage{RawMessage: snapJSON, Valid: true},
		HpCurrent:         3, // voluntary revert with HP left
	}

	reverted, overflow, err := RevertWildShape(c, 0)
	require.NoError(t, err)
	assert.Equal(t, int32(28), reverted.HpCurrent)
	assert.Equal(t, int32(0), overflow)
	assert.False(t, reverted.IsWildShaped)
}

func TestFormatWildShapeActivation(t *testing.T) {
	got := FormatWildShapeActivation("Elara", "Wolf", 1, 11, 13, 40, "Bite (+4, 2d4+2 piercing)")
	assert.Contains(t, got, "Elara Wild Shapes into a Wolf")
	assert.Contains(t, got, "1 use remaining")
	assert.Contains(t, got, "HP: 11")
	assert.Contains(t, got, "AC: 13")
	assert.Contains(t, got, "40ft")
	assert.Contains(t, got, "Attacks: Bite")
}

func TestFormatWildShapeActivation_NoAttacks(t *testing.T) {
	got := FormatWildShapeActivation("Elara", "Wolf", 1, 11, 13, 40, "")
	assert.Contains(t, got, "Wild Shapes into a Wolf")
	assert.NotContains(t, got, "Attacks:")
}

func TestFormatWildShapeRevert(t *testing.T) {
	got := FormatWildShapeRevert("Elara")
	assert.Contains(t, got, "Elara reverts from Wild Shape")
}

func TestFormatWildShapeAutoRevert(t *testing.T) {
	got := FormatWildShapeAutoRevert("Elara", "wolf", 5, 23, 28)
	assert.Contains(t, got, "wolf form drops to 0 HP")
	assert.Contains(t, got, "5 overflow damage")
	assert.Contains(t, got, "23/28 HP")

	// Different beast name
	got = FormatWildShapeAutoRevert("Elara", "brown bear", 3, 20, 28)
	assert.Contains(t, got, "brown bear form drops to 0 HP")
}

func TestRevertWildShape_NotWildShaped(t *testing.T) {
	c := refdata.Combatant{IsWildShaped: false}
	_, _, err := RevertWildShape(c, 0)
	assert.ErrorContains(t, err, "not in Wild Shape")
}

func TestValidateWildShapeActivation(t *testing.T) {
	tests := []struct {
		name       string
		isWild     bool
		beastType  string
		beastCR    string
		druidLevel int
		moon       bool
		beastSpeed []byte
		wantErr    string
	}{
		{"happy path", false, "beast", "1/4", 2, false, []byte(`{"walk":40}`), ""},
		{"already wild shaped", true, "beast", "1/4", 2, false, []byte(`{"walk":40}`), "already in Wild Shape"},
		{"not a beast", false, "monstrosity", "1/4", 2, false, []byte(`{"walk":40}`), "not a beast"},
		{"CR too high", false, "beast", "1", 2, false, []byte(`{"walk":40}`), "CR 1 exceeds limit"},
		{"swim speed too early", false, "beast", "1/4", 2, false, []byte(`{"walk":30,"swim":40}`), "swim speed requires Druid level 4"},
		{"fly speed too early", false, "beast", "1/4", 4, false, []byte(`{"walk":30,"fly":60}`), "fly speed requires Druid level 8"},
		{"swim OK at level 4", false, "beast", "1/4", 4, false, []byte(`{"walk":30,"swim":40}`), ""},
		{"fly OK at level 8", false, "beast", "1/4", 8, false, []byte(`{"walk":30,"fly":60}`), ""},
		{"moon CR 1 at level 2", false, "beast", "1", 2, true, []byte(`{"walk":40}`), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWildShapeActivation(tt.isWild, tt.beastType, tt.beastCR, tt.druidLevel, tt.moon, tt.beastSpeed)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestCreatureHasSwimSpeed(t *testing.T) {
	assert.True(t, CreatureHasSwimSpeed([]byte(`{"walk":30,"swim":40}`)))
	assert.False(t, CreatureHasSwimSpeed([]byte(`{"walk":30}`)))
	assert.False(t, CreatureHasSwimSpeed([]byte(`{"walk":30,"swim":0}`)))
	assert.False(t, CreatureHasSwimSpeed(nil))
}

func TestCreatureHasFlySpeed(t *testing.T) {
	assert.True(t, CreatureHasFlySpeed([]byte(`{"walk":30,"fly":60}`)))
	assert.False(t, CreatureHasFlySpeed([]byte(`{"walk":30}`)))
	assert.False(t, CreatureHasFlySpeed([]byte(`{"walk":30,"fly":0}`)))
	assert.False(t, CreatureHasFlySpeed(nil))
}

func TestCreatureHasSpeed_InvalidJSON(t *testing.T) {
	assert.False(t, CreatureHasSwimSpeed([]byte(`invalid`)))
	assert.False(t, CreatureHasFlySpeed([]byte(`invalid`)))
}

func TestCanWildShapeSpellcast(t *testing.T) {
	assert.False(t, CanWildShapeSpellcast(17))
	assert.True(t, CanWildShapeSpellcast(18))
	assert.True(t, CanWildShapeSpellcast(20))
}

func TestParseCR(t *testing.T) {
	tests := []struct {
		cr   string
		want float64
	}{
		{"0", 0},
		{"1/8", 0.125},
		{"1/4", 0.25},
		{"1/2", 0.5},
		{"1", 1},
		{"2", 2},
		{"3", 3},
		{"10", 10},
		{"1/0", 0}, // zero denominator edge case
		{"abc", 0}, // non-numeric string
	}
	for _, tt := range tests {
		t.Run(tt.cr, func(t *testing.T) {
			got := ParseCR(tt.cr)
			assert.InDelta(t, tt.want, got, 0.001, "ParseCR(%q)", tt.cr)
		})
	}
}

// --- Service-level tests ---

func makeWolfBeast() refdata.Creature {
	return refdata.Creature{
		ID:            "wolf",
		Name:          "Wolf",
		Type:          "beast",
		Ac:            13,
		HpAverage:     11,
		Speed:         json.RawMessage(`{"walk":40}`),
		AbilityScores: json.RawMessage(`{"str":12,"dex":15,"con":12,"int":3,"wis":12,"cha":6}`),
		Cr:            "1/4",
		Attacks:       json.RawMessage(`[{"name":"Bite","to_hit":4,"damage":"2d4+2","damage_type":"piercing"}]`),
	}
}

func makeDruidCharacter(charID uuid.UUID, level int, wildShapeUses int) refdata.Character {
	featureUses := fmt.Sprintf(`{"wild_shape":{"current":%d,"max":2,"recharge":"short"}}`, wildShapeUses)
	return refdata.Character{
		ID:            charID,
		Name:          "Elara",
		Classes:       json.RawMessage(fmt.Sprintf(`[{"class":"Druid","level":%d}]`, level)),
		AbilityScores: json.RawMessage(`{"str":10,"dex":14,"con":12,"int":16,"wis":18,"cha":10}`),
		HpMax:         28,
		HpCurrent:     28,
		Ac:            16,
		SpeedFt:       30,
		FeatureUses: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(featureUses),
			Valid:      true,
		},
	}
}

func TestService_ActivateWildShape_Success(t *testing.T) {
	charID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()
	wolf := makeWolfBeast()

	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return makeDruidCharacter(charID, 4, 2), nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		if id == "wolf" {
			return wolf, nil
		}
		return refdata.Creature{}, sql.ErrNoRows
	}
	store.updateCombatantWildShapeFn = func(ctx context.Context, arg refdata.UpdateCombatantWildShapeParams) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:                   arg.ID,
			DisplayName:          "Elara",
			IsWildShaped:         arg.IsWildShaped,
			WildShapeCreatureRef: arg.WildShapeCreatureRef,
			WildShapeOriginal:    arg.WildShapeOriginal,
			HpMax:                arg.HpMax,
			HpCurrent:            arg.HpCurrent,
			Ac:                   arg.Ac,
			Conditions:           json.RawMessage(`[]`),
		}, nil
	}
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: true}, nil
	}

	svc := NewService(store)
	result, err := svc.ActivateWildShape(context.Background(), WildShapeCommand{
		Combatant: refdata.Combatant{
			ID:          combatantID,
			DisplayName: "Elara",
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			HpMax:       28,
			HpCurrent:   28,
			Ac:          16,
		},
		Turn:      refdata.Turn{ID: turnID},
		BeastName: "wolf",
	})

	require.NoError(t, err)
	assert.True(t, result.Combatant.IsWildShaped)
	assert.Equal(t, int32(11), result.Combatant.HpMax)
	assert.Equal(t, int32(11), result.Combatant.HpCurrent)
	assert.Equal(t, int32(13), result.Combatant.Ac)
	assert.Contains(t, result.CombatLog, "Wild Shapes into a Wolf")
	assert.Equal(t, 1, result.UsesRemaining)
}

func TestService_ActivateWildShape_BonusActionSpent(t *testing.T) {
	store := defaultMockStore()
	svc := NewService(store)
	_, err := svc.ActivateWildShape(context.Background(), WildShapeCommand{
		Combatant: refdata.Combatant{CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true}},
		Turn:      refdata.Turn{BonusActionUsed: true},
		BeastName: "wolf",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bonus action")
}

func TestService_ActivateWildShape_NPC(t *testing.T) {
	store := defaultMockStore()
	svc := NewService(store)
	_, err := svc.ActivateWildShape(context.Background(), WildShapeCommand{
		Combatant: refdata.Combatant{},
		Turn:      refdata.Turn{},
		BeastName: "wolf",
	})
	assert.ErrorContains(t, err, "character")
}

func TestService_ActivateWildShape_NotDruid(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:      charID,
			Classes: json.RawMessage(`[{"class":"Fighter","level":5}]`),
		}, nil
	}
	svc := NewService(store)
	_, err := svc.ActivateWildShape(context.Background(), WildShapeCommand{
		Combatant: refdata.Combatant{CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
		Turn:      refdata.Turn{},
		BeastName: "wolf",
	})
	assert.ErrorContains(t, err, "Druid class")
}

func TestService_ActivateWildShape_NoUsesLeft(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return makeDruidCharacter(charID, 4, 0), nil
	}
	svc := NewService(store)
	_, err := svc.ActivateWildShape(context.Background(), WildShapeCommand{
		Combatant: refdata.Combatant{CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
		Turn:      refdata.Turn{},
		BeastName: "wolf",
	})
	assert.ErrorContains(t, err, "no Wild Shape uses remaining")
}

func TestService_ActivateWildShape_AlreadyWildShaped(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return makeDruidCharacter(charID, 4, 2), nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return makeWolfBeast(), nil
	}
	svc := NewService(store)
	_, err := svc.ActivateWildShape(context.Background(), WildShapeCommand{
		Combatant: refdata.Combatant{
			CharacterID:  uuid.NullUUID{UUID: charID, Valid: true},
			IsWildShaped: true,
		},
		Turn:      refdata.Turn{},
		BeastName: "wolf",
	})
	assert.ErrorContains(t, err, "already in Wild Shape")
}

func TestService_ActivateWildShape_BeastNotFound(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return makeDruidCharacter(charID, 4, 2), nil
	}
	// getCreatureFn defaults to ErrNoRows
	svc := NewService(store)
	_, err := svc.ActivateWildShape(context.Background(), WildShapeCommand{
		Combatant: refdata.Combatant{CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
		Turn:      refdata.Turn{},
		BeastName: "nonexistent",
	})
	assert.Error(t, err)
}

func TestService_ActivateWildShape_CRTooHigh(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return makeDruidCharacter(charID, 2, 2), nil // level 2 druid, CR limit 1/4
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		beast := makeWolfBeast()
		beast.Cr = "1" // CR 1, too high for level 2
		return beast, nil
	}
	svc := NewService(store)
	_, err := svc.ActivateWildShape(context.Background(), WildShapeCommand{
		Combatant: refdata.Combatant{CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
		Turn:      refdata.Turn{},
		BeastName: "wolf",
	})
	assert.ErrorContains(t, err, "CR 1 exceeds limit")
}

// --- Revert service tests ---

func TestService_RevertWildShape_Success(t *testing.T) {
	original := WildShapeSnapshot{
		HpMax: 28, HpCurrent: 28, Ac: 16, SpeedFt: 30,
		AbilityScores: map[string]int{"str": 10, "dex": 14, "con": 12},
	}
	snapJSON, _ := json.Marshal(original)

	store := defaultMockStore()
	store.updateCombatantWildShapeFn = func(ctx context.Context, arg refdata.UpdateCombatantWildShapeParams) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:           arg.ID,
			DisplayName:  "Elara",
			IsWildShaped: arg.IsWildShaped,
			HpMax:        arg.HpMax,
			HpCurrent:    arg.HpCurrent,
			Ac:           arg.Ac,
			Conditions:   json.RawMessage(`[]`),
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: true}, nil
	}

	svc := NewService(store)
	result, err := svc.RevertWildShapeService(context.Background(), RevertWildShapeCommand{
		Combatant: refdata.Combatant{
			ID:                   uuid.New(),
			DisplayName:          "Elara",
			IsWildShaped:         true,
			WildShapeOriginal:    pqtype.NullRawMessage{RawMessage: snapJSON, Valid: true},
			WildShapeCreatureRef: sql.NullString{String: "wolf", Valid: true},
			HpMax:                11,
			HpCurrent:            5,
			Ac:                   13,
		},
		Turn: refdata.Turn{ID: uuid.New()},
	})

	require.NoError(t, err)
	assert.False(t, result.Combatant.IsWildShaped)
	assert.Equal(t, int32(28), result.Combatant.HpMax)
	assert.Equal(t, int32(28), result.Combatant.HpCurrent)
	assert.Contains(t, result.CombatLog, "reverts from Wild Shape")
}

func TestService_RevertWildShape_NotWildShaped(t *testing.T) {
	store := defaultMockStore()
	svc := NewService(store)
	_, err := svc.RevertWildShapeService(context.Background(), RevertWildShapeCommand{
		Combatant: refdata.Combatant{IsWildShaped: false},
		Turn:      refdata.Turn{},
	})
	assert.ErrorContains(t, err, "not in Wild Shape")
}

// --- AutoRevert test ---

func TestAutoRevertWildShape(t *testing.T) {
	original := WildShapeSnapshot{
		HpMax: 28, HpCurrent: 25, Ac: 16, SpeedFt: 30,
		AbilityScores: map[string]int{"str": 10},
	}
	snapJSON, _ := json.Marshal(original)

	c := refdata.Combatant{
		IsWildShaped:      true,
		WildShapeOriginal: pqtype.NullRawMessage{RawMessage: snapJSON, Valid: true},
		HpMax:             11,
		HpCurrent:         0,
		Ac:                13,
	}

	reverted, overflow, err := AutoRevertWildShape(c, 5) // took 16 damage on 11 HP beast = 5 overflow
	require.NoError(t, err)
	assert.False(t, reverted.IsWildShaped)
	assert.Equal(t, int32(28), reverted.HpMax)
	assert.Equal(t, int32(20), reverted.HpCurrent) // 25 - 5
	assert.Equal(t, int32(5), overflow)
}

func TestDruidLevel(t *testing.T) {
	tests := []struct {
		name    string
		classes json.RawMessage
		want    int
	}{
		{"druid 5", json.RawMessage(`[{"class":"Druid","level":5}]`), 5},
		{"no druid", json.RawMessage(`[{"class":"Fighter","level":10}]`), 0},
		{"multiclass", json.RawMessage(`[{"class":"Druid","level":3},{"class":"Cleric","level":2}]`), 3},
		{"empty", json.RawMessage(`[]`), 0},
		{"nil", nil, 0},
		{"invalid json", json.RawMessage(`invalid`), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassLevelFromJSON(tt.classes, "Druid")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestService_ActivateWildShape_GetCreatureError(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return makeDruidCharacter(charID, 4, 2), nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{}, fmt.Errorf("db error")
	}
	svc := NewService(store)
	_, err := svc.ActivateWildShape(context.Background(), WildShapeCommand{
		Combatant: refdata.Combatant{CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
		Turn:      refdata.Turn{},
		BeastName: "wolf",
	})
	assert.Error(t, err)
}

func TestService_ActivateWildShape_UpdateWildShapeError(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return makeDruidCharacter(charID, 4, 2), nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return makeWolfBeast(), nil
	}
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: true}, nil
	}
	store.updateCombatantWildShapeFn = func(ctx context.Context, arg refdata.UpdateCombatantWildShapeParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}
	svc := NewService(store)
	_, err := svc.ActivateWildShape(context.Background(), WildShapeCommand{
		Combatant: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			HpMax:       28, HpCurrent: 28, Ac: 16,
		},
		Turn:      refdata.Turn{ID: uuid.New()},
		BeastName: "wolf",
	})
	assert.Error(t, err)
}

func TestBuildFeatureDefinitions_WildShape(t *testing.T) {
	classes := []CharacterClass{{Class: "Druid", Level: 5}}
	features := []CharacterFeature{
		{Name: "Wild Shape", MechanicalEffect: "wild_shape"},
	}

	defs := BuildFeatureDefinitions(classes, features)
	// wild_shape doesn't produce combat effects (it's an activation, not passive)
	// but it should be recognized and not panic
	assert.Empty(t, defs)
}

func TestIsCircleOfMoon(t *testing.T) {
	// Has Circle of the Moon
	features := pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`[{"name":"Circle of the Moon","mechanical_effect":"circle_of_the_moon"}]`),
		Valid:      true,
	}
	assert.True(t, isCircleOfMoon(features))

	// No Circle of the Moon
	features = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`[{"name":"Wild Shape","mechanical_effect":"wild_shape"}]`),
		Valid:      true,
	}
	assert.False(t, isCircleOfMoon(features))

	// Invalid JSON
	features = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`invalid`),
		Valid:      true,
	}
	assert.False(t, isCircleOfMoon(features))

	// Null features
	assert.False(t, isCircleOfMoon(pqtype.NullRawMessage{}))
}

func TestParseWildShapeUses(t *testing.T) {
	// Normal case
	char := refdata.Character{
		FeatureUses: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"wild_shape":{"current":2,"max":2,"recharge":"long"}}`),
			Valid:      true,
		},
	}
	uses, remaining, err := ParseFeatureUses(char, FeatureKeyWildShape)
	require.NoError(t, err)
	assert.Equal(t, 2, remaining)
	assert.Equal(t, 2, uses["wild_shape"].Current)

	// No feature uses
	char = refdata.Character{}
	uses, remaining, err = ParseFeatureUses(char, FeatureKeyWildShape)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining)
	assert.Empty(t, uses)

	// Invalid JSON
	char = refdata.Character{
		FeatureUses: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`invalid`),
			Valid:      true,
		},
	}
	_, _, err = ParseFeatureUses(char, FeatureKeyWildShape)
	assert.Error(t, err)
}

func TestGetBeastWalkSpeed(t *testing.T) {
	assert.Equal(t, int32(40), getBeastWalkSpeed(json.RawMessage(`{"walk":40}`)))
	assert.Equal(t, int32(0), getBeastWalkSpeed(json.RawMessage(`{}`)))
	assert.Equal(t, int32(0), getBeastWalkSpeed(json.RawMessage(`invalid`)))
}

func TestSnapshotCombatantState_InvalidJSON(t *testing.T) {
	c := refdata.Combatant{HpMax: 28}
	_, err := SnapshotCombatantState(c, 30, json.RawMessage(`invalid`))
	assert.Error(t, err)
}

func TestRevertWildShape_InvalidSnapshot(t *testing.T) {
	c := refdata.Combatant{
		IsWildShaped: true,
		WildShapeOriginal: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`invalid`),
			Valid:      true,
		},
	}
	_, _, err := RevertWildShape(c, 0)
	assert.Error(t, err)
}

func TestRevertWildShape_NoSnapshot(t *testing.T) {
	c := refdata.Combatant{
		IsWildShaped:      true,
		WildShapeOriginal: pqtype.NullRawMessage{},
	}
	_, _, err := RevertWildShape(c, 0)
	assert.ErrorContains(t, err, "no Wild Shape snapshot")
}

// SR-022: Wild Shape must swap STR/DEX/CON to the beast's scores while
// preserving the druid's INT/WIS/CHA. Revert must restore the druid's
// original physical scores. The originals are persisted in the existing
// WildShapeOriginal snapshot — no new column needed.

func TestEffectiveAbilityScores_WildShapedBrownBear(t *testing.T) {
	// Druid pre-transform scores live on the character row.
	druid := character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 16, WIS: 18, CHA: 10}
	// Brown Bear MM stat block.
	bear := character.AbilityScores{STR: 19, DEX: 10, CON: 16, INT: 2, WIS: 13, CHA: 7}

	snap := WildShapeSnapshot{
		HpMax: 28, HpCurrent: 28, Ac: 16, SpeedFt: 30,
		AbilityScores: map[string]int{"str": 8, "dex": 14, "con": 12, "int": 16, "wis": 18, "cha": 10},
	}
	snapJSON, _ := json.Marshal(snap)
	c := refdata.Combatant{
		IsWildShaped:      true,
		WildShapeOriginal: pqtype.NullRawMessage{RawMessage: snapJSON, Valid: true},
	}

	got := EffectiveAbilityScores(c, druid, bear)
	// Physicals swapped to bear values.
	assert.Equal(t, 19, got.STR)
	assert.Equal(t, 10, got.DEX)
	assert.Equal(t, 16, got.CON)
	// Mentals retained from druid.
	assert.Equal(t, 16, got.INT)
	assert.Equal(t, 18, got.WIS)
	assert.Equal(t, 10, got.CHA)
}

func TestEffectiveAbilityScores_NotWildShaped(t *testing.T) {
	druid := character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 16, WIS: 18, CHA: 10}
	// Beast scores ignored when not wild-shaped.
	bear := character.AbilityScores{STR: 19, DEX: 10, CON: 16}

	c := refdata.Combatant{IsWildShaped: false}

	got := EffectiveAbilityScores(c, druid, bear)
	assert.Equal(t, druid, got)
}

func TestEffectiveAbilityScores_RevertRestoresPhysicals(t *testing.T) {
	// Round-trip: activate → physicals are bear's; revert → physicals
	// restored to druid's originals from the snapshot.
	druid := character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 16, WIS: 18, CHA: 10}
	bear := character.AbilityScores{STR: 19, DEX: 10, CON: 16, INT: 2, WIS: 13, CHA: 7}

	snap := WildShapeSnapshot{
		HpMax: 28, HpCurrent: 28, Ac: 16, SpeedFt: 30,
		AbilityScores: map[string]int{"str": 8, "dex": 14, "con": 12, "int": 16, "wis": 18, "cha": 10},
	}
	snapJSON, _ := json.Marshal(snap)
	wild := refdata.Combatant{
		IsWildShaped:      true,
		WildShapeOriginal: pqtype.NullRawMessage{RawMessage: snapJSON, Valid: true},
		HpMax:             34, HpCurrent: 34, Ac: 11,
	}

	// While wild-shaped: physicals = bear.
	mid := EffectiveAbilityScores(wild, druid, bear)
	assert.Equal(t, 19, mid.STR)
	assert.Equal(t, 16, mid.CON)

	// Revert clears the snapshot and IsWildShaped.
	reverted, _, err := RevertWildShape(wild, 0)
	require.NoError(t, err)

	// After revert: physicals restored to druid's originals.
	got := EffectiveAbilityScores(reverted, druid, bear)
	assert.Equal(t, 8, got.STR)
	assert.Equal(t, 14, got.DEX)
	assert.Equal(t, 12, got.CON)
	assert.Equal(t, 16, got.INT)
	assert.Equal(t, 18, got.WIS)
	assert.Equal(t, 10, got.CHA)
}

// SR-022 plumbing tests: LookupBeastScores + ResolveAttackerScores are the
// adapter the runtime call sites (/check, /save, /attack) use to thread
// the merged Wild Shape scores into player rolls.

// stubCreatureLookup is a tiny CreatureScoresLookup mock used by the
// SR-022 plumbing tests.
type stubCreatureLookup struct {
	getFn func(ctx context.Context, id string) (refdata.Creature, error)
}

func (s *stubCreatureLookup) GetCreature(ctx context.Context, id string) (refdata.Creature, error) {
	return s.getFn(ctx, id)
}

func wildShapedCombatantOnBeast(beastID string) refdata.Combatant {
	return refdata.Combatant{
		IsWildShaped:         true,
		WildShapeCreatureRef: sql.NullString{String: beastID, Valid: true},
	}
}

func brownBearCreature() refdata.Creature {
	return refdata.Creature{
		ID:            "brown-bear",
		Name:          "Brown Bear",
		Type:          "beast",
		AbilityScores: json.RawMessage(`{"str":19,"dex":10,"con":16,"int":2,"wis":13,"cha":7}`),
	}
}

func TestLookupBeastScores_ReturnsBeastScoresWhenWildShaped(t *testing.T) {
	lookup := &stubCreatureLookup{
		getFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			assert.Equal(t, "brown-bear", id)
			return brownBearCreature(), nil
		},
	}
	got, ok := LookupBeastScores(context.Background(), lookup, wildShapedCombatantOnBeast("brown-bear"))
	require.True(t, ok)
	assert.Equal(t, 19, got.STR)
	assert.Equal(t, 10, got.DEX)
	assert.Equal(t, 16, got.CON)
}

func TestLookupBeastScores_NotWildShaped(t *testing.T) {
	lookup := &stubCreatureLookup{
		getFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			t.Fatalf("GetCreature should not be called when not Wild Shaped")
			return refdata.Creature{}, nil
		},
	}
	_, ok := LookupBeastScores(context.Background(), lookup, refdata.Combatant{IsWildShaped: false})
	assert.False(t, ok)
}

func TestLookupBeastScores_NilLookupDegrades(t *testing.T) {
	_, ok := LookupBeastScores(context.Background(), nil, wildShapedCombatantOnBeast("brown-bear"))
	assert.False(t, ok, "nil lookup must degrade to ok=false, not panic")
}

func TestLookupBeastScores_MissingBeastRefDegrades(t *testing.T) {
	lookup := &stubCreatureLookup{
		getFn: func(ctx context.Context, _ string) (refdata.Creature, error) {
			t.Fatalf("GetCreature should not be called when ref is empty")
			return refdata.Creature{}, nil
		},
	}
	c := refdata.Combatant{
		IsWildShaped:         true,
		WildShapeCreatureRef: sql.NullString{Valid: false},
	}
	_, ok := LookupBeastScores(context.Background(), lookup, c)
	assert.False(t, ok)
}

func TestLookupBeastScores_GetCreatureErrorDegrades(t *testing.T) {
	lookup := &stubCreatureLookup{
		getFn: func(ctx context.Context, _ string) (refdata.Creature, error) {
			return refdata.Creature{}, fmt.Errorf("not found")
		},
	}
	_, ok := LookupBeastScores(context.Background(), lookup, wildShapedCombatantOnBeast("ghost-beast"))
	assert.False(t, ok)
}

func TestLookupBeastScores_MalformedJSONDegrades(t *testing.T) {
	lookup := &stubCreatureLookup{
		getFn: func(ctx context.Context, _ string) (refdata.Creature, error) {
			return refdata.Creature{AbilityScores: json.RawMessage(`not-json`)}, nil
		},
	}
	_, ok := LookupBeastScores(context.Background(), lookup, wildShapedCombatantOnBeast("broken"))
	assert.False(t, ok)
}

func TestResolveAttackerScores_WildShapedMergesBeastPhysicals(t *testing.T) {
	druid := AbilityScores{Str: 8, Dex: 14, Con: 12, Int: 16, Wis: 18, Cha: 10}
	lookup := &stubCreatureLookup{
		getFn: func(ctx context.Context, _ string) (refdata.Creature, error) {
			return brownBearCreature(), nil
		},
	}
	got := ResolveAttackerScores(context.Background(), lookup, wildShapedCombatantOnBeast("brown-bear"), druid)
	// Beast STR/DEX/CON.
	assert.Equal(t, 19, got.Str)
	assert.Equal(t, 10, got.Dex)
	assert.Equal(t, 16, got.Con)
	// Druid mentals.
	assert.Equal(t, 16, got.Int)
	assert.Equal(t, 18, got.Wis)
	assert.Equal(t, 10, got.Cha)
}

func TestResolveAttackerScores_NotWildShapedReturnsDruid(t *testing.T) {
	druid := AbilityScores{Str: 8, Dex: 14, Con: 12, Int: 16, Wis: 18, Cha: 10}
	got := ResolveAttackerScores(context.Background(), nil, refdata.Combatant{}, druid)
	assert.Equal(t, druid, got)
}

func TestResolveAttackerScores_BeastLookupErrorDegradesToDruid(t *testing.T) {
	druid := AbilityScores{Str: 8, Dex: 14, Con: 12, Int: 16, Wis: 18, Cha: 10}
	lookup := &stubCreatureLookup{
		getFn: func(ctx context.Context, _ string) (refdata.Creature, error) {
			return refdata.Creature{}, fmt.Errorf("db down")
		},
	}
	got := ResolveAttackerScores(context.Background(), lookup, wildShapedCombatantOnBeast("brown-bear"), druid)
	// Silent fallback to druid (SR-006 pattern).
	assert.Equal(t, druid, got)
}

func TestRevertWildShape_OverflowClampsToZero(t *testing.T) {
	original := WildShapeSnapshot{
		HpMax: 28, HpCurrent: 5, Ac: 16, SpeedFt: 30,
		AbilityScores: map[string]int{"str": 10},
	}
	snapJSON, _ := json.Marshal(original)

	c := refdata.Combatant{
		IsWildShaped:      true,
		WildShapeOriginal: pqtype.NullRawMessage{RawMessage: snapJSON, Valid: true},
		HpCurrent:         0,
	}

	reverted, _, err := RevertWildShape(c, 100) // massive overflow
	require.NoError(t, err)
	assert.Equal(t, int32(0), reverted.HpCurrent) // clamped to 0
}

func TestService_ActivateWildShape_UpdateFeatureUsesError(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return makeDruidCharacter(charID, 4, 2), nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return makeWolfBeast(), nil
	}
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}
	svc := NewService(store)
	_, err := svc.ActivateWildShape(context.Background(), WildShapeCommand{
		Combatant: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			HpMax:       28, HpCurrent: 28, Ac: 16,
		},
		Turn:      refdata.Turn{ID: uuid.New()},
		BeastName: "wolf",
	})
	assert.Error(t, err)
}

func TestService_ActivateWildShape_UpdateTurnError(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return makeDruidCharacter(charID, 4, 2), nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return makeWolfBeast(), nil
	}
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}
	svc := NewService(store)
	_, err := svc.ActivateWildShape(context.Background(), WildShapeCommand{
		Combatant: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			HpMax:       28, HpCurrent: 28, Ac: 16,
		},
		Turn:      refdata.Turn{ID: uuid.New()},
		BeastName: "wolf",
	})
	assert.Error(t, err)
}

func TestService_ActivateWildShape_InvalidAbilityScores(t *testing.T) {
	charID := uuid.New()
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		char := makeDruidCharacter(charID, 4, 2)
		char.AbilityScores = json.RawMessage(`invalid`) // will fail snapshot
		return char, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return makeWolfBeast(), nil
	}
	store.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: true}, nil
	}
	svc := NewService(store)
	_, err := svc.ActivateWildShape(context.Background(), WildShapeCommand{
		Combatant: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			HpMax:       28, HpCurrent: 28, Ac: 16,
		},
		Turn:      refdata.Turn{ID: uuid.New()},
		BeastName: "wolf",
	})
	assert.ErrorContains(t, err, "creating snapshot")
}

func TestService_ActivateWildShape_GetCharacterError(t *testing.T) {
	store := defaultMockStore()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}
	svc := NewService(store)
	_, err := svc.ActivateWildShape(context.Background(), WildShapeCommand{
		Combatant: refdata.Combatant{CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true}},
		Turn:      refdata.Turn{},
		BeastName: "wolf",
	})
	assert.Error(t, err)
}

func TestService_RevertWildShape_InvalidSnapshot(t *testing.T) {
	store := defaultMockStore()
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: true}, nil
	}
	svc := NewService(store)
	_, err := svc.RevertWildShapeService(context.Background(), RevertWildShapeCommand{
		Combatant: refdata.Combatant{
			ID:           uuid.New(),
			IsWildShaped: true,
			WildShapeOriginal: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`invalid`),
				Valid:      true,
			},
		},
		Turn: refdata.Turn{ID: uuid.New()},
	})
	assert.ErrorContains(t, err, "parsing wild shape snapshot")
}

func TestService_RevertWildShape_BonusActionSpent(t *testing.T) {
	store := defaultMockStore()
	svc := NewService(store)
	_, err := svc.RevertWildShapeService(context.Background(), RevertWildShapeCommand{
		Combatant: refdata.Combatant{IsWildShaped: true},
		Turn:      refdata.Turn{BonusActionUsed: true},
	})
	assert.ErrorContains(t, err, "bonus action")
}

func TestService_RevertWildShape_UpdateTurnError(t *testing.T) {
	original := WildShapeSnapshot{
		HpMax: 28, HpCurrent: 28, Ac: 16, SpeedFt: 30,
		AbilityScores: map[string]int{"str": 10},
	}
	snapJSON, _ := json.Marshal(original)

	store := defaultMockStore()
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}
	svc := NewService(store)
	_, err := svc.RevertWildShapeService(context.Background(), RevertWildShapeCommand{
		Combatant: refdata.Combatant{
			ID:                uuid.New(),
			IsWildShaped:      true,
			WildShapeOriginal: pqtype.NullRawMessage{RawMessage: snapJSON, Valid: true},
		},
		Turn: refdata.Turn{ID: uuid.New()},
	})
	assert.Error(t, err)
}

func TestService_RevertWildShape_UpdateError(t *testing.T) {
	original := WildShapeSnapshot{
		HpMax: 28, HpCurrent: 28, Ac: 16, SpeedFt: 30,
		AbilityScores: map[string]int{"str": 10},
	}
	snapJSON, _ := json.Marshal(original)

	store := defaultMockStore()
	store.updateCombatantWildShapeFn = func(ctx context.Context, arg refdata.UpdateCombatantWildShapeParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: true}, nil
	}
	svc := NewService(store)
	_, err := svc.RevertWildShapeService(context.Background(), RevertWildShapeCommand{
		Combatant: refdata.Combatant{
			ID:                uuid.New(),
			IsWildShaped:      true,
			WildShapeOriginal: pqtype.NullRawMessage{RawMessage: snapJSON, Valid: true},
		},
		Turn: refdata.Turn{ID: uuid.New()},
	})
	assert.Error(t, err)
}
