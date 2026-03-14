package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TDD Cycle 1: ParseAreaOfEffect parses AoE JSON into struct
func TestParseAreaOfEffect(t *testing.T) {
	t.Run("sphere", func(t *testing.T) {
		raw := []byte(`{"shape":"sphere","radius_ft":20}`)
		aoe, err := ParseAreaOfEffect(raw)
		require.NoError(t, err)
		assert.Equal(t, "sphere", aoe.Shape)
		assert.Equal(t, 20, aoe.RadiusFt)
	})

	t.Run("cone", func(t *testing.T) {
		raw := []byte(`{"shape":"cone","length_ft":15}`)
		aoe, err := ParseAreaOfEffect(raw)
		require.NoError(t, err)
		assert.Equal(t, "cone", aoe.Shape)
		assert.Equal(t, 15, aoe.LengthFt)
	})

	t.Run("line", func(t *testing.T) {
		raw := []byte(`{"shape":"line","length_ft":100,"width_ft":5}`)
		aoe, err := ParseAreaOfEffect(raw)
		require.NoError(t, err)
		assert.Equal(t, "line", aoe.Shape)
		assert.Equal(t, 100, aoe.LengthFt)
		assert.Equal(t, 5, aoe.WidthFt)
	})

	t.Run("square", func(t *testing.T) {
		raw := []byte(`{"shape":"square","side_ft":20}`)
		aoe, err := ParseAreaOfEffect(raw)
		require.NoError(t, err)
		assert.Equal(t, "square", aoe.Shape)
		assert.Equal(t, 20, aoe.SideFt)
	})

	t.Run("nil returns error", func(t *testing.T) {
		_, err := ParseAreaOfEffect(nil)
		assert.Error(t, err)
	})

	t.Run("empty returns error", func(t *testing.T) {
		_, err := ParseAreaOfEffect([]byte{})
		assert.Error(t, err)
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		_, err := ParseAreaOfEffect([]byte(`{invalid`))
		assert.Error(t, err)
	})
}

// TDD Cycle 2: SphereAffectedTiles returns tiles within radius
func TestSphereAffectedTiles(t *testing.T) {
	t.Run("5ft radius from origin", func(t *testing.T) {
		// 5ft radius = 1 square radius. Origin at (5,5).
		// Should include origin and adjacent tiles whose centers are within 5ft.
		tiles := SphereAffectedTiles(5, 5, 5)
		assert.Contains(t, tiles, GridPos{5, 5}) // origin
		assert.Contains(t, tiles, GridPos{4, 5}) // left
		assert.Contains(t, tiles, GridPos{6, 5}) // right
		assert.Contains(t, tiles, GridPos{5, 4}) // up
		assert.Contains(t, tiles, GridPos{5, 6}) // down
		// Diagonals at exactly sqrt(2)*5 ~= 7.07ft > 5ft, should NOT be included
		assert.NotContains(t, tiles, GridPos{4, 4})
		assert.NotContains(t, tiles, GridPos{6, 6})
	})

	t.Run("20ft radius fireball", func(t *testing.T) {
		// 20ft radius = 4 squares. Should cover a large area.
		tiles := SphereAffectedTiles(5, 5, 20)
		assert.Contains(t, tiles, GridPos{5, 5}) // origin
		assert.Contains(t, tiles, GridPos{5, 1}) // 4 squares up
		assert.Contains(t, tiles, GridPos{5, 9}) // 4 squares down
		assert.Contains(t, tiles, GridPos{1, 5}) // 4 squares left
		assert.Contains(t, tiles, GridPos{9, 5}) // 4 squares right
		// 5 squares away = 25ft > 20ft
		assert.NotContains(t, tiles, GridPos{5, 0})
		assert.NotContains(t, tiles, GridPos{10, 5})
	})

	t.Run("0ft radius returns only origin", func(t *testing.T) {
		tiles := SphereAffectedTiles(3, 3, 0)
		assert.Equal(t, []GridPos{{3, 3}}, tiles)
	})
}

// TDD Cycle 3: ConeAffectedTiles returns tiles in a 53-degree cone
func TestConeAffectedTiles(t *testing.T) {
	t.Run("15ft cone east", func(t *testing.T) {
		// Caster at (2,5), targeting east toward (5,5). 15ft = 3 squares.
		// 53-degree cone: at distance d, width = d (in standard D&D cone).
		tiles := ConeAffectedTiles(2, 5, 5, 5, 15)
		// Should include tiles along the cone direction
		assert.Contains(t, tiles, GridPos{3, 5}) // 1 square east
		assert.Contains(t, tiles, GridPos{4, 5}) // 2 squares east
		assert.Contains(t, tiles, GridPos{5, 5}) // 3 squares east
		// Width expands: at 2 squares out, width is ~2 squares
		assert.Contains(t, tiles, GridPos{4, 4})
		assert.Contains(t, tiles, GridPos{4, 6})
		// At 3 squares out, width is ~3 squares
		assert.Contains(t, tiles, GridPos{5, 4})
		assert.Contains(t, tiles, GridPos{5, 6})
		// Should NOT include the caster's tile
		assert.NotContains(t, tiles, GridPos{2, 5})
		// Should NOT include tiles behind the caster
		assert.NotContains(t, tiles, GridPos{1, 5})
	})

	t.Run("15ft cone north", func(t *testing.T) {
		// Caster at (5,5), targeting north toward (5,2).
		tiles := ConeAffectedTiles(5, 5, 5, 2, 15)
		assert.Contains(t, tiles, GridPos{5, 4}) // 1 square north
		assert.Contains(t, tiles, GridPos{5, 3}) // 2 squares north
		assert.Contains(t, tiles, GridPos{5, 2}) // 3 squares north
		assert.NotContains(t, tiles, GridPos{5, 5}) // caster excluded
	})

	t.Run("0ft cone returns empty", func(t *testing.T) {
		tiles := ConeAffectedTiles(5, 5, 6, 5, 0)
		assert.Empty(t, tiles)
	})
}

// TDD Cycle 4: LineAffectedTiles returns tiles along a line
func TestLineAffectedTiles(t *testing.T) {
	t.Run("100ft line east with 5ft width", func(t *testing.T) {
		// Lightning bolt: 100ft long, 5ft wide, east from (2,5) toward (22,5)
		tiles := LineAffectedTiles(2, 5, 22, 5, 100, 5)
		// Should include 20 tiles along the line (100ft / 5ft per tile)
		for i := 1; i <= 20; i++ {
			assert.Contains(t, tiles, GridPos{2 + i, 5}, "should contain tile %d", i)
		}
		// Should NOT include caster tile
		assert.NotContains(t, tiles, GridPos{2, 5})
		// Width is only 5ft = 1 tile, so no spread
		assert.NotContains(t, tiles, GridPos{3, 4})
		assert.NotContains(t, tiles, GridPos{3, 6})
	})

	t.Run("30ft line north with 10ft width", func(t *testing.T) {
		// 30ft long, 10ft wide line going north from (5,10)
		tiles := LineAffectedTiles(5, 10, 5, 4, 30, 10)
		// Along center
		assert.Contains(t, tiles, GridPos{5, 9})
		assert.Contains(t, tiles, GridPos{5, 8})
		assert.Contains(t, tiles, GridPos{5, 7})
		assert.Contains(t, tiles, GridPos{5, 6})
		assert.Contains(t, tiles, GridPos{5, 5})
		assert.Contains(t, tiles, GridPos{5, 4})
		// Width 10ft = 2 tiles, so 1 tile on each side
		assert.Contains(t, tiles, GridPos{4, 9})
		assert.Contains(t, tiles, GridPos{6, 9})
		// Should NOT include caster
		assert.NotContains(t, tiles, GridPos{5, 10})
	})

	t.Run("0ft line returns empty", func(t *testing.T) {
		tiles := LineAffectedTiles(5, 5, 6, 5, 0, 5)
		assert.Empty(t, tiles)
	})
}

// TDD Cycle 5: SquareAffectedTiles returns tiles in a square area
func TestSquareAffectedTiles(t *testing.T) {
	t.Run("20ft square", func(t *testing.T) {
		// 20ft = 4 squares per side, origin at corner (5,5)
		tiles := SquareAffectedTiles(5, 5, 20)
		assert.Len(t, tiles, 16) // 4x4
		assert.Contains(t, tiles, GridPos{5, 5})
		assert.Contains(t, tiles, GridPos{8, 8})
		assert.NotContains(t, tiles, GridPos{9, 5})
		assert.NotContains(t, tiles, GridPos{5, 9})
	})

	t.Run("5ft square", func(t *testing.T) {
		tiles := SquareAffectedTiles(0, 0, 5)
		assert.Len(t, tiles, 1)
		assert.Contains(t, tiles, GridPos{0, 0})
	})

	t.Run("0ft square returns empty", func(t *testing.T) {
		tiles := SquareAffectedTiles(5, 5, 0)
		assert.Empty(t, tiles)
	})
}

// TDD Cycle 6: FindAffectedCombatants filters combatants by affected tiles
func TestFindAffectedCombatants(t *testing.T) {
	combatants := []refdata.Combatant{
		{ID: uuid.New(), DisplayName: "Goblin A", PositionCol: "C", PositionRow: 3, IsAlive: true},
		{ID: uuid.New(), DisplayName: "Goblin B", PositionCol: "E", PositionRow: 5, IsAlive: true},
		{ID: uuid.New(), DisplayName: "Fighter", PositionCol: "H", PositionRow: 8, IsAlive: true},
		{ID: uuid.New(), DisplayName: "Dead Goblin", PositionCol: "C", PositionRow: 3, IsAlive: false},
	}

	t.Run("finds combatants in affected tiles", func(t *testing.T) {
		// Tiles covering col C row 3 = col index 2, row index 2 (0-based from 1-based row)
		// colToIndex("C") = 2, row 3 - 1 = 2
		// Also includes Dead Goblin at same position
		tiles := []GridPos{{2, 2}, {4, 4}, {10, 10}}
		affected := FindAffectedCombatants(tiles, combatants)
		require.Len(t, affected, 3) // Goblin A + Goblin B + Dead Goblin
		var names []string
		for _, a := range affected {
			names = append(names, a.DisplayName)
		}
		assert.Contains(t, names, "Goblin A")
		assert.Contains(t, names, "Goblin B")
		assert.Contains(t, names, "Dead Goblin")
	})

	t.Run("excludes combatants not in area", func(t *testing.T) {
		tiles := []GridPos{{0, 0}}
		affected := FindAffectedCombatants(tiles, combatants)
		assert.Empty(t, affected)
	})

	t.Run("includes dead combatants on affected tiles", func(t *testing.T) {
		// Dead combatants are still in the area, just not alive
		tiles := []GridPos{{2, 2}}
		affected := FindAffectedCombatants(tiles, combatants)
		assert.Len(t, affected, 2) // Goblin A + Dead Goblin at same position
	})
}

// TDD Cycle 7: CalculateAoECover computes cover bonuses for DEX saves
func TestCalculateAoECover(t *testing.T) {
	t.Run("dex save gets cover bonus", func(t *testing.T) {
		combatant := refdata.Combatant{
			ID:          uuid.New(),
			DisplayName: "Goblin",
			PositionCol: "D",
			PositionRow: 5,
			IsNpc:       true,
		}
		// Origin at col C (2), row 3 (0-based: 2). Combatant at col D (3), row 5 (0-based: 4).
		// No walls, so no cover.
		ps := CalculateAoECover(2, 2, combatant, "dex", 15, nil)
		assert.Equal(t, combatant.ID, ps.CombatantID)
		assert.Equal(t, "dex", ps.SaveAbility)
		assert.Equal(t, 15, ps.DC)
		assert.Equal(t, 0, ps.CoverBonus) // no walls = no cover
		assert.True(t, ps.IsNPC)
	})

	t.Run("non-dex save gets no cover bonus", func(t *testing.T) {
		combatant := refdata.Combatant{
			ID:          uuid.New(),
			DisplayName: "Fighter",
			PositionCol: "D",
			PositionRow: 5,
		}
		ps := CalculateAoECover(2, 2, combatant, "wis", 14, nil)
		assert.Equal(t, 0, ps.CoverBonus)
	})

	t.Run("same tile gets no cover", func(t *testing.T) {
		combatant := refdata.Combatant{
			ID:          uuid.New(),
			DisplayName: "Goblin",
			PositionCol: "C",
			PositionRow: 3,
		}
		// Origin at (2,2), combatant at (2,2) - same tile
		ps := CalculateAoECover(2, 2, combatant, "dex", 15, nil)
		assert.Equal(t, 0, ps.CoverBonus)
	})
}

// TDD Cycle 8: ApplySaveResult determines damage multiplier based on save outcome
func TestApplySaveResult(t *testing.T) {
	t.Run("half_damage: success = half", func(t *testing.T) {
		mult := ApplySaveResult(true, "half_damage")
		assert.Equal(t, 0.5, mult)
	})

	t.Run("half_damage: failure = full", func(t *testing.T) {
		mult := ApplySaveResult(false, "half_damage")
		assert.Equal(t, 1.0, mult)
	})

	t.Run("no_effect: success = zero", func(t *testing.T) {
		mult := ApplySaveResult(true, "no_effect")
		assert.Equal(t, 0.0, mult)
	})

	t.Run("no_effect: failure = full", func(t *testing.T) {
		mult := ApplySaveResult(false, "no_effect")
		assert.Equal(t, 1.0, mult)
	})

	t.Run("special: always returns -1 for DM resolution", func(t *testing.T) {
		mult := ApplySaveResult(true, "special")
		assert.Equal(t, -1.0, mult)
		mult = ApplySaveResult(false, "special")
		assert.Equal(t, -1.0, mult)
	})

	t.Run("unknown save effect: default to full damage", func(t *testing.T) {
		mult := ApplySaveResult(false, "unknown")
		assert.Equal(t, 1.0, mult)
	})
}

// TDD Cycle 9: GetAffectedTiles dispatches to the correct shape function
func TestGetAffectedTiles(t *testing.T) {
	t.Run("sphere", func(t *testing.T) {
		aoe := AreaOfEffect{Shape: "sphere", RadiusFt: 20}
		tiles, err := GetAffectedTiles(aoe, 5, 5, 5, 5)
		require.NoError(t, err)
		assert.Contains(t, tiles, GridPos{5, 5}) // origin
		assert.True(t, len(tiles) > 1)
	})

	t.Run("cone", func(t *testing.T) {
		aoe := AreaOfEffect{Shape: "cone", LengthFt: 15}
		tiles, err := GetAffectedTiles(aoe, 2, 5, 5, 5)
		require.NoError(t, err)
		assert.Contains(t, tiles, GridPos{3, 5})
		assert.NotContains(t, tiles, GridPos{2, 5}) // caster excluded
	})

	t.Run("line", func(t *testing.T) {
		aoe := AreaOfEffect{Shape: "line", LengthFt: 100, WidthFt: 5}
		tiles, err := GetAffectedTiles(aoe, 2, 5, 22, 5)
		require.NoError(t, err)
		assert.Contains(t, tiles, GridPos{3, 5})
	})

	t.Run("square", func(t *testing.T) {
		aoe := AreaOfEffect{Shape: "square", SideFt: 20}
		tiles, err := GetAffectedTiles(aoe, 5, 5, 5, 5)
		require.NoError(t, err)
		assert.Len(t, tiles, 16)
	})

	t.Run("unknown shape returns error", func(t *testing.T) {
		aoe := AreaOfEffect{Shape: "hexagon"}
		_, err := GetAffectedTiles(aoe, 5, 5, 5, 5)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hexagon")
	})
}

// TDD Cycle 10: FormatAoECastLog formats the AoE combat log
func TestFormatAoECastLog(t *testing.T) {
	t.Run("basic AoE cast", func(t *testing.T) {
		result := AoECastResult{
			CasterName: "Gandalf",
			SpellName:  "Fireball",
			SpellLevel: 3,
			SaveDC:     15,
			SaveAbility: "dex",
			AffectedNames: []string{"Goblin A", "Goblin B", "Fighter"},
			SlotUsed:       3,
			SlotsRemaining: 1,
		}
		log := FormatAoECastLog(result)
		assert.Contains(t, log, "Gandalf")
		assert.Contains(t, log, "Fireball")
		assert.Contains(t, log, "Goblin A")
		assert.Contains(t, log, "Goblin B")
		assert.Contains(t, log, "Fighter")
		assert.Contains(t, log, "DC 15")
		assert.Contains(t, log, "DEX")
		assert.Contains(t, log, "slot")
	})

	t.Run("no affected creatures", func(t *testing.T) {
		result := AoECastResult{
			CasterName:     "Gandalf",
			SpellName:      "Fireball",
			SpellLevel:     3,
			SaveDC:         15,
			SaveAbility:    "dex",
			AffectedNames:  []string{},
			SlotUsed:       3,
			SlotsRemaining: 1,
		}
		log := FormatAoECastLog(result)
		assert.Contains(t, log, "No creatures affected")
	})

	t.Run("concentration shown", func(t *testing.T) {
		result := AoECastResult{
			CasterName:    "Gandalf",
			SpellName:     "Entangle",
			SpellLevel:    1,
			SaveDC:        15,
			SaveAbility:   "str",
			AffectedNames: []string{"Goblin"},
			Concentration: ConcentrationResult{
				NewConcentration: "Entangle",
			},
			SlotUsed:       1,
			SlotsRemaining: 3,
		}
		log := FormatAoECastLog(result)
		assert.Contains(t, log, "Concentrating")
		assert.Contains(t, log, "Entangle")
	})
}

// TDD Cycle 11: CastAoE service method - fireball on multiple targets
func TestCastAoE_Fireball(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	caster.PositionCol = "E"
	caster.PositionRow = 5

	goblinA := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Goblin A",
		PositionCol: "H", PositionRow: 8,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}
	goblinB := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Goblin B",
		PositionCol: "I", PositionRow: 8,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}
	fighter := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Fighter",
		PositionCol: "A", PositionRow: 1,
		IsAlive: true, Conditions: json.RawMessage(`[]`),
	}

	fireball := makeFireball()
	fireball.AreaOfEffect = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"shape":"sphere","radius_ft":20}`),
		Valid:      true,
	}
	fireball.SaveEffect = sql.NullString{String: "half_damage", Valid: true}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return fireball, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return refdata.Combatant{}, fmt.Errorf("not found")
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster, goblinA, goblinB, fighter}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed, ActionSpellCast: arg.ActionSpellCast}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:     "fireball",
		CasterID:    caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "H",
		TargetRow:   8,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	result, err := svc.CastAoE(context.Background(), cmd)
	require.NoError(t, err)
	assert.Equal(t, "Fireball", result.SpellName)
	assert.Equal(t, 15, result.SaveDC) // 8 + prof(3) + INT mod(+4)
	assert.Equal(t, "dex", result.SaveAbility)
	// Goblins at H8 and I8 should be affected (within 20ft of H8)
	assert.Contains(t, result.AffectedNames, "Goblin A")
	assert.Contains(t, result.AffectedNames, "Goblin B")
	// Fighter at A1 is far away, should not be affected
	assert.NotContains(t, result.AffectedNames, "Fighter")
	// Pending saves should be created for affected combatants
	assert.Equal(t, len(result.AffectedNames), len(result.PendingSaves))
	// Slot should be deducted
	assert.Equal(t, 3, result.SlotUsed)
}

// TDD Cycle 12: CastAoE with cone shape (Burning Hands)
func TestCastAoE_BurningHands(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	caster.PositionCol = "E"
	caster.PositionRow = 5

	// Goblin directly in front at F5 (1 square east)
	goblin := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Goblin",
		PositionCol: "F", PositionRow: 5,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}

	burningHands := refdata.Spell{
		ID: "burning-hands", Name: "Burning Hands", Level: 1,
		CastingTime: "1 action", RangeType: "self",
		SaveAbility: sql.NullString{String: "dex", Valid: true},
		SaveEffect:  sql.NullString{String: "half_damage", Valid: true},
		AreaOfEffect: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"shape":"cone","length_ft":15}`),
			Valid:      true,
		},
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: false, Valid: true},
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return burningHands, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return refdata.Combatant{}, fmt.Errorf("not found")
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster, goblin}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:     "burning-hands",
		CasterID:    caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "H", // direction: east
		TargetRow:   5,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	result, err := svc.CastAoE(context.Background(), cmd)
	require.NoError(t, err)
	assert.Equal(t, "Burning Hands", result.SpellName)
	assert.Contains(t, result.AffectedNames, "Goblin")
	// Caster should NOT be affected (cone excludes caster tile)
	assert.NotContains(t, result.AffectedNames, "Gandalf")
}

// TDD Cycle 13: CastAoE validates spell range
func TestCastAoE_OutOfRange(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	caster.PositionCol = "A"
	caster.PositionRow = 1

	fireball := makeFireball()
	fireball.AreaOfEffect = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"shape":"sphere","radius_ft":20}`),
		Valid:      true,
	}
	fireball.RangeFt = sql.NullInt32{Int32: 150, Valid: true}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return fireball, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:     "fireball",
		CasterID:    caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "Z",  // Very far
		TargetRow:   26,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	_, err := svc.CastAoE(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

// TDD Cycle 14: CastAoE rejects non-AoE spell
func TestCastAoE_NoAoEData(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)

	noAoE := makeFireBolt() // no area_of_effect

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return noAoE, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:     "fire-bolt",
		CasterID:    caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "E",
		TargetRow:   5,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	_, err := svc.CastAoE(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "area_of_effect")
}

// Edge case: CastAoE spell not found
func TestCastAoE_SpellNotFound(t *testing.T) {
	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return refdata.Spell{}, fmt.Errorf("not found")
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:  "nonexistent",
		CasterID: uuid.New(),
		Turn:     refdata.Turn{ID: uuid.New()},
	}

	_, err := svc.CastAoE(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "looking up spell")
}

// Edge case: CastAoE action already used
func TestCastAoE_ActionAlreadyUsed(t *testing.T) {
	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return makeFireball(), nil
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:  "fireball",
		CasterID: uuid.New(),
		Turn:     refdata.Turn{ID: uuid.New(), ActionUsed: true},
	}

	_, err := svc.CastAoE(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource already spent")
}

// Edge case: CastAoE NPC caster rejected
func TestCastAoE_NPCCasterRejected(t *testing.T) {
	npcCaster := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "Goblin Shaman",
		PositionCol: "E",
		PositionRow: 5,
		IsNpc:       true,
		// No CharacterID
	}

	fireball := makeFireball()
	fireball.AreaOfEffect = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"shape":"sphere","radius_ft":20}`),
		Valid:      true,
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return fireball, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
		return npcCaster, nil
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:  "fireball",
		CasterID: npcCaster.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: npcCaster.ID},
	}

	_, err := svc.CastAoE(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only player characters")
}

// Edge case: CastAoE no slots remaining
func TestCastAoE_NoSlotsRemaining(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	slotsJSON, _ := json.Marshal(map[string]SlotInfo{
		"1": {Current: 4, Max: 4},
		"2": {Current: 3, Max: 3},
		"3": {Current: 0, Max: 2},
	})
	char.SpellSlots = pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true}
	caster := makeSpellCaster(charID)

	fireball := makeFireball()
	fireball.AreaOfEffect = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"shape":"sphere","radius_ft":20}`),
		Valid:      true,
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return fireball, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:     "fireball",
		CasterID:    caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "E",
		TargetRow:   5,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	_, err := svc.CastAoE(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no 3rd-level spell slots remaining")
}

// Edge case: CastAoE bonus action spell
func TestCastAoE_BonusAction(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	caster.PositionCol = "E"
	caster.PositionRow = 5

	goblin := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Goblin",
		PositionCol: "F", PositionRow: 5,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}

	bonusAoE := refdata.Spell{
		ID: "bonus-aoe", Name: "Bonus AoE", Level: 1,
		CastingTime: "1 bonus action", RangeType: "self",
		SaveAbility: sql.NullString{String: "dex", Valid: true},
		SaveEffect:  sql.NullString{String: "half_damage", Valid: true},
		AreaOfEffect: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"shape":"sphere","radius_ft":10}`),
			Valid:      true,
		},
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: false, Valid: true},
	}

	var savedTurn refdata.UpdateTurnActionsParams
	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return bonusAoE, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster, goblin}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		savedTurn = arg
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:     "bonus-aoe",
		CasterID:    caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "E",
		TargetRow:   5,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	result, err := svc.CastAoE(context.Background(), cmd)
	require.NoError(t, err)
	assert.True(t, result.IsBonusAction)
	assert.True(t, savedTurn.BonusActionUsed)
	assert.True(t, savedTurn.BonusActionSpellCast)
}

// Edge case: CastAoE with concentration
func TestCastAoE_Concentration(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	caster.PositionCol = "E"
	caster.PositionRow = 5

	entangle := refdata.Spell{
		ID: "entangle", Name: "Entangle", Level: 1,
		CastingTime: "1 action", RangeType: "ranged",
		RangeFt:     sql.NullInt32{Int32: 90, Valid: true},
		SaveAbility: sql.NullString{String: "str", Valid: true},
		SaveEffect:  sql.NullString{String: "no_effect", Valid: true},
		AreaOfEffect: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"shape":"square","side_ft":20}`),
			Valid:      true,
		},
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: true, Valid: true},
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return entangle, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:              "entangle",
		CasterID:             caster.ID,
		EncounterID:          uuid.New(),
		TargetCol:            "H",
		TargetRow:            8,
		Turn:                 refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		CurrentConcentration: "Bless",
	}

	result, err := svc.CastAoE(context.Background(), cmd)
	require.NoError(t, err)
	assert.True(t, result.Concentration.DroppedPrevious)
	assert.Equal(t, "Bless", result.Concentration.PreviousSpell)
	assert.Equal(t, "Entangle", result.Concentration.NewConcentration)
}

// Edge case: ConeAffectedTiles with same caster and target position
func TestConeAffectedTiles_SamePosition(t *testing.T) {
	tiles := ConeAffectedTiles(5, 5, 5, 5, 15)
	assert.Empty(t, tiles)
}

// Edge case: LineAffectedTiles with same caster and target position
func TestLineAffectedTiles_SamePosition(t *testing.T) {
	tiles := LineAffectedTiles(5, 5, 5, 5, 100, 5)
	assert.Empty(t, tiles)
}

// Edge case: FormatAoECastLog bonus action and dropped concentration
func TestFormatAoECastLog_BonusActionAndDroppedConcentration(t *testing.T) {
	result := AoECastResult{
		CasterName:    "Gandalf",
		SpellName:     "Boom",
		SpellLevel:    2,
		IsBonusAction: true,
		SaveDC:        0, // no save
		AffectedNames: []string{"Goblin"},
		Concentration: ConcentrationResult{
			DroppedPrevious:  true,
			PreviousSpell:    "Shield",
			NewConcentration: "Boom",
		},
		SlotUsed:       2,
		SlotsRemaining: 1,
	}
	log := FormatAoECastLog(result)
	assert.Contains(t, log, "bonus action")
	assert.Contains(t, log, "Dropped concentration on Shield")
	assert.Contains(t, log, "Concentrating on Boom")
	assert.NotContains(t, log, "DC")
}

// Edge case: CastAoE caster not found
func TestCastAoE_CasterNotFound(t *testing.T) {
	fireball := makeFireball()
	fireball.AreaOfEffect = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"shape":"sphere","radius_ft":20}`),
		Valid:      true,
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return fireball, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("not found")
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:  "fireball",
		CasterID: uuid.New(),
		Turn:     refdata.Turn{ID: uuid.New()},
	}

	_, err := svc.CastAoE(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting caster")
}

// Edge case: CastAoE character not found
func TestCastAoE_CharacterNotFound(t *testing.T) {
	charID := uuid.New()
	caster := makeSpellCaster(charID)

	fireball := makeFireball()
	fireball.AreaOfEffect = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"shape":"sphere","radius_ft":20}`),
		Valid:      true,
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return fireball, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("not found")
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:  "fireball",
		CasterID: caster.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	_, err := svc.CastAoE(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

// Edge case: CastAoE bonus action restriction forward
func TestCastAoE_BonusActionRestriction(t *testing.T) {
	fireball := makeFireball()
	fireball.AreaOfEffect = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"shape":"sphere","radius_ft":20}`),
		Valid:      true,
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return fireball, nil
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:  "fireball",
		CasterID: uuid.New(),
		Turn:     refdata.Turn{ID: uuid.New(), BonusActionSpellCast: true},
	}

	_, err := svc.CastAoE(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bonus action spell")
}

// Edge case: CastAoE unsupported AoE shape
func TestCastAoE_UnsupportedShape(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)

	spell := makeFireball()
	spell.AreaOfEffect = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"shape":"hexagon","radius_ft":20}`),
		Valid:      true,
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return spell, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:     "fireball",
		CasterID:    caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "E",
		TargetRow:   5,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	_, err := svc.CastAoE(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported AoE shape")
}

// Edge case: CastAoE list combatants error
func TestCastAoE_ListCombatantsError(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)

	fireball := makeFireball()
	fireball.AreaOfEffect = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"shape":"sphere","radius_ft":20}`),
		Valid:      true,
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return fireball, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return nil, fmt.Errorf("db error")
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:     "fireball",
		CasterID:    caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "E",
		TargetRow:   5,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	_, err := svc.CastAoE(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing combatants")
}

// Edge case: CastAoE update turn error
func TestCastAoE_UpdateTurnError(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)

	fireball := makeFireball()
	fireball.AreaOfEffect = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"shape":"sphere","radius_ft":20}`),
		Valid:      true,
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return fireball, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, _ refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:     "fireball",
		CasterID:    caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "E",
		TargetRow:   5,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	_, err := svc.CastAoE(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn")
}

// Edge case: CastAoE update spell slots error
func TestCastAoE_UpdateSlotsError(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)

	fireball := makeFireball()
	fireball.AreaOfEffect = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"shape":"sphere","radius_ft":20}`),
		Valid:      true,
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return fireball, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, _ refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:     "fireball",
		CasterID:    caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "E",
		TargetRow:   5,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	_, err := svc.CastAoE(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating spell slots")
}

// Edge case: CastAoE with no save ability (damage without save)
func TestCastAoE_NoSaveAbility(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	caster.PositionCol = "E"
	caster.PositionRow = 5

	goblin := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Goblin",
		PositionCol: "F", PositionRow: 5,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}

	noSaveAoE := refdata.Spell{
		ID: "thunderwave", Name: "Thunderwave", Level: 1,
		CastingTime: "1 action", RangeType: "self",
		// No SaveAbility
		AreaOfEffect: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"shape":"sphere","radius_ft":10}`),
			Valid:      true,
		},
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: false, Valid: true},
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return noSaveAoE, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster, goblin}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:     "thunderwave",
		CasterID:    caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "E",
		TargetRow:   5,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	result, err := svc.CastAoE(context.Background(), cmd)
	require.NoError(t, err)
	assert.Equal(t, 0, result.SaveDC)
	assert.Empty(t, result.PendingSaves) // no saves since no save ability
	assert.NotEmpty(t, result.AffectedNames)
}

// Edge case: FormatAoECastLog cantrip (no slot)
func TestFormatAoECastLog_Cantrip(t *testing.T) {
	result := AoECastResult{
		CasterName:    "Gandalf",
		SpellName:     "Acid Splash",
		SpellLevel:    0,
		AffectedNames: []string{"Goblin A"},
		SaveDC:        15,
		SaveAbility:   "dex",
	}
	log := FormatAoECastLog(result)
	assert.NotContains(t, log, "slot")
	assert.Contains(t, log, "DC 15")
}
