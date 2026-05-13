package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/dice"
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
		tiles := []GridPos{{2, 2}, {4, 4}, {10, 10}}
		affected := FindAffectedCombatants(tiles, combatants)
		require.Len(t, affected, 2) // Goblin A + Goblin B (dead excluded)
		var names []string
		for _, a := range affected {
			names = append(names, a.DisplayName)
		}
		assert.Contains(t, names, "Goblin A")
		assert.Contains(t, names, "Goblin B")
		assert.NotContains(t, names, "Dead Goblin")
	})

	t.Run("excludes combatants not in area", func(t *testing.T) {
		tiles := []GridPos{{0, 0}}
		affected := FindAffectedCombatants(tiles, combatants)
		assert.Empty(t, affected)
	})

	t.Run("excludes dead combatants from affected tiles", func(t *testing.T) {
		// Dead combatants on affected tiles should NOT be returned
		tiles := []GridPos{{2, 2}}
		affected := FindAffectedCombatants(tiles, combatants)
		assert.Len(t, affected, 1) // Only Goblin A (alive), Dead Goblin excluded
		assert.Equal(t, "Goblin A", affected[0].DisplayName)
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
	assert.Contains(t, err.Error(), "no spell slots remaining")
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

// TDD Cycle 12 (Phase 118): CastAoE persists concentration AND fires
// BreakConcentrationFully on the previously-concentrated spell.
func TestCastAoE_PersistsConcentrationAndCleansUpPrevious(t *testing.T) {
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

	store.getCombatantConcentrationFn = func(_ context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
		return refdata.GetCombatantConcentrationRow{
			ConcentrationSpellID:   sql.NullString{String: "bless", Valid: true},
			ConcentrationSpellName: sql.NullString{String: "Bless", Valid: true},
		}, nil
	}

	var (
		setConcArg          refdata.SetCombatantConcentrationParams
		setConcCalled       bool
		zoneCleanupCombatID uuid.UUID
	)
	store.setCombatantConcentrationFn = func(_ context.Context, arg refdata.SetCombatantConcentrationParams) error {
		setConcArg = arg
		setConcCalled = true
		return nil
	}
	store.deleteConcentrationZonesByCombatantFn = func(_ context.Context, id uuid.UUID) (int64, error) {
		zoneCleanupCombatID = id
		return 0, nil
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
	assert.Equal(t, caster.ID, zoneCleanupCombatID)
	require.True(t, setConcCalled)
	assert.Equal(t, "entangle", setConcArg.ConcentrationSpellID.String)
	assert.Equal(t, "Entangle", setConcArg.ConcentrationSpellName.String)
	// Consolidated 💨 line on the result.
	assert.Contains(t, result.ConcentrationCleanup.ConsolidatedMessage, "Bless")
	assert.Contains(t, result.ConcentrationCleanup.ConsolidatedMessage, "💨")
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

// TDD Cycle 2 (iter2): ResolveAoESaves applies full damage on failed save
func TestResolveAoESaves_FullDamageOnFail(t *testing.T) {
	goblinID := uuid.New()

	goblin := refdata.Combatant{
		ID: goblinID, DisplayName: "Goblin", HpMax: 30, HpCurrent: 30,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}

	var capturedHP refdata.UpdateCombatantHPParams
	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == goblinID {
			return goblin, nil
		}
		return refdata.Combatant{}, fmt.Errorf("not found")
	}
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		capturedHP = arg
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, TempHp: arg.TempHp, IsAlive: arg.IsAlive}, nil
	}

	// Use a deterministic roller that always rolls 4 (8d6 = 32)
	roller := dice.NewRoller(func(max int) int { return 4 })
	svc := NewService(store)

	input := AoEDamageInput{
		EncounterID: uuid.New(),
		SpellName:   "Fireball",
		DamageDice:  "8d6",
		DamageType:  "fire",
		SaveEffect:  "half_damage",
		SaveResults: []SaveResult{
			{CombatantID: goblinID, Rolled: 8, Total: 8, Success: false, CoverBonus: 0},
		},
	}

	result, err := svc.ResolveAoESaves(context.Background(), input, roller)
	require.NoError(t, err)
	require.Len(t, result.Targets, 1)
	assert.Equal(t, goblinID, result.Targets[0].CombatantID)
	assert.Equal(t, "Goblin", result.Targets[0].DisplayName)
	assert.False(t, result.Targets[0].SaveSuccess)
	assert.Equal(t, 32, result.Targets[0].DamageDealt) // full damage: 8*4=32
	assert.Equal(t, 30, result.Targets[0].HPBefore)
	assert.Equal(t, 0, result.Targets[0].HPAfter) // 30-32 = clamped to 0
	assert.Equal(t, int32(0), capturedHP.HpCurrent)
	assert.False(t, capturedHP.IsAlive)
}

// TDD Cycle 3 (iter2): ResolveAoESaves applies half damage on successful save
func TestResolveAoESaves_HalfDamageOnSave(t *testing.T) {
	goblinID := uuid.New()

	goblin := refdata.Combatant{
		ID: goblinID, DisplayName: "Goblin", HpMax: 50, HpCurrent: 50,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}

	var capturedHP refdata.UpdateCombatantHPParams
	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == goblinID {
			return goblin, nil
		}
		return refdata.Combatant{}, fmt.Errorf("not found")
	}
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		capturedHP = arg
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
	}

	// Deterministic roller: always rolls 3 (8d6 = 24)
	roller := dice.NewRoller(func(max int) int { return 3 })
	svc := NewService(store)

	input := AoEDamageInput{
		EncounterID: uuid.New(),
		SpellName:   "Fireball",
		DamageDice:  "8d6",
		DamageType:  "fire",
		SaveEffect:  "half_damage",
		SaveResults: []SaveResult{
			{CombatantID: goblinID, Rolled: 15, Total: 15, Success: true},
		},
	}

	result, err := svc.ResolveAoESaves(context.Background(), input, roller)
	require.NoError(t, err)
	require.Len(t, result.Targets, 1)
	assert.True(t, result.Targets[0].SaveSuccess)
	// 24 * 0.5 = 12 (half damage, rounded down)
	assert.Equal(t, 12, result.Targets[0].DamageDealt)
	assert.Equal(t, 50, result.Targets[0].HPBefore)
	assert.Equal(t, 38, result.Targets[0].HPAfter) // 50-12=38
	assert.Equal(t, int32(38), capturedHP.HpCurrent)
	assert.True(t, capturedHP.IsAlive)
}

// TDD Cycle 4 (iter2): ResolveAoESaves applies no damage on save with no_effect
func TestResolveAoESaves_NoEffectOnSave(t *testing.T) {
	goblinID := uuid.New()

	goblin := refdata.Combatant{
		ID: goblinID, DisplayName: "Goblin", HpMax: 30, HpCurrent: 30,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return goblin, nil
	}
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
	}

	roller := dice.NewRoller(func(max int) int { return 5 })
	svc := NewService(store)

	input := AoEDamageInput{
		EncounterID: uuid.New(),
		SpellName:   "Entangle",
		DamageDice:  "2d6",
		DamageType:  "bludgeoning",
		SaveEffect:  "no_effect",
		SaveResults: []SaveResult{
			{CombatantID: goblinID, Rolled: 18, Total: 18, Success: true},
		},
	}

	result, err := svc.ResolveAoESaves(context.Background(), input, roller)
	require.NoError(t, err)
	require.Len(t, result.Targets, 1)
	assert.True(t, result.Targets[0].SaveSuccess)
	assert.Equal(t, 0, result.Targets[0].DamageDealt) // no damage on save
	assert.Equal(t, 30, result.Targets[0].HPAfter)     // unchanged
}

// TDD Cycle 5 (iter2): ResolveAoESaves with multiple targets, mixed saves
func TestResolveAoESaves_MultipleTargetsMixedSaves(t *testing.T) {
	goblinAID := uuid.New()
	goblinBID := uuid.New()
	fighterID := uuid.New()

	goblinA := refdata.Combatant{
		ID: goblinAID, DisplayName: "Goblin A", HpMax: 30, HpCurrent: 30,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}
	goblinB := refdata.Combatant{
		ID: goblinBID, DisplayName: "Goblin B", HpMax: 30, HpCurrent: 30,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}
	fighter := refdata.Combatant{
		ID: fighterID, DisplayName: "Fighter", HpMax: 50, HpCurrent: 50,
		IsAlive: true, Conditions: json.RawMessage(`[]`),
	}

	hpUpdates := map[uuid.UUID]refdata.UpdateCombatantHPParams{}
	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		switch id {
		case goblinAID:
			return goblinA, nil
		case goblinBID:
			return goblinB, nil
		case fighterID:
			return fighter, nil
		}
		return refdata.Combatant{}, fmt.Errorf("not found")
	}
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpUpdates[arg.ID] = arg
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
	}

	// 8d6 always rolling 4 = 32 total damage
	roller := dice.NewRoller(func(max int) int { return 4 })
	svc := NewService(store)

	input := AoEDamageInput{
		EncounterID: uuid.New(),
		SpellName:   "Fireball",
		DamageDice:  "8d6",
		DamageType:  "fire",
		SaveEffect:  "half_damage",
		SaveResults: []SaveResult{
			{CombatantID: goblinAID, Success: false}, // full damage: 32
			{CombatantID: goblinBID, Success: false}, // full damage: 32
			{CombatantID: fighterID, Success: true},  // half damage: 16
		},
	}

	result, err := svc.ResolveAoESaves(context.Background(), input, roller)
	require.NoError(t, err)
	require.Len(t, result.Targets, 3)

	// Goblin A: 30 - 32 = 0 (clamped), dead
	assert.Equal(t, 32, result.Targets[0].DamageDealt)
	assert.Equal(t, 0, result.Targets[0].HPAfter)
	assert.False(t, hpUpdates[goblinAID].IsAlive)

	// Goblin B: 30 - 32 = 0 (clamped), dead
	assert.Equal(t, 32, result.Targets[1].DamageDealt)
	assert.Equal(t, 0, result.Targets[1].HPAfter)

	// Fighter: 50 - 16 = 34, alive
	assert.Equal(t, 16, result.Targets[2].DamageDealt)
	assert.Equal(t, 34, result.Targets[2].HPAfter)
	assert.True(t, hpUpdates[fighterID].IsAlive)

	// Total damage
	assert.Equal(t, 80, result.TotalDamage) // 32+32+16
}

// TDD Cycle 6 (iter2): ResolveAoESaves error - combatant not found
func TestResolveAoESaves_CombatantNotFound(t *testing.T) {
	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("not found")
	}

	roller := dice.NewRoller(func(max int) int { return 3 })
	svc := NewService(store)

	input := AoEDamageInput{
		DamageDice:  "8d6",
		SaveEffect:  "half_damage",
		SaveResults: []SaveResult{{CombatantID: uuid.New(), Success: false}},
	}

	_, err := svc.ResolveAoESaves(context.Background(), input, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting combatant")
}

// TDD Cycle 7 (iter2): ResolveAoESaves error - update HP fails
func TestResolveAoESaves_UpdateHPError(t *testing.T) {
	goblinID := uuid.New()
	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: goblinID, DisplayName: "Goblin", HpCurrent: 30, IsAlive: true, Conditions: json.RawMessage(`[]`)}, nil
	}
	store.updateCombatantHPFn = func(_ context.Context, _ refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}

	roller := dice.NewRoller(func(max int) int { return 3 })
	svc := NewService(store)

	input := AoEDamageInput{
		DamageDice:  "2d6",
		SaveEffect:  "half_damage",
		SaveResults: []SaveResult{{CombatantID: goblinID, Success: false}},
	}

	_, err := svc.ResolveAoESaves(context.Background(), input, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating HP")
}

// TDD Cycle 8 (iter2): ResolveAoESaves error - invalid damage dice
func TestResolveAoESaves_InvalidDamageDice(t *testing.T) {
	store := defaultMockStore()
	roller := dice.NewRoller(func(max int) int { return 3 })
	svc := NewService(store)

	input := AoEDamageInput{
		DamageDice:  "invalid",
		SaveEffect:  "half_damage",
		SaveResults: []SaveResult{{CombatantID: uuid.New(), Success: false}},
	}

	_, err := svc.ResolveAoESaves(context.Background(), input, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rolling damage")
}

// TDD Cycle 9 (iter2): Integration test - Full CastAoE -> ResolveAoESaves (Fireball sphere)
func TestIntegration_Fireball_CastAndResolve(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	caster.PositionCol = "E"
	caster.PositionRow = 5

	goblinA := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Goblin A",
		PositionCol: "H", PositionRow: 8, HpMax: 30, HpCurrent: 30,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}
	goblinB := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Goblin B",
		PositionCol: "I", PositionRow: 8, HpMax: 30, HpCurrent: 30,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}
	fighter := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Fighter",
		PositionCol: "H", PositionRow: 9, HpMax: 60, HpCurrent: 60,
		IsAlive: true, Conditions: json.RawMessage(`[]`),
	}

	fireball := makeFireball()
	fireball.AreaOfEffect = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"shape":"sphere","radius_ft":20}`),
		Valid:      true,
	}
	fireball.SaveEffect = sql.NullString{String: "half_damage", Valid: true}
	fireball.Damage = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"dice":"8d6","type":"fire"}`),
		Valid:      true,
	}

	hpUpdates := map[uuid.UUID]refdata.UpdateCombatantHPParams{}
	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return fireball, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		switch id {
		case caster.ID:
			return caster, nil
		case goblinA.ID:
			return goblinA, nil
		case goblinB.ID:
			return goblinB, nil
		case fighter.ID:
			return fighter, nil
		}
		return refdata.Combatant{}, fmt.Errorf("not found")
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster, goblinA, goblinB, fighter}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpUpdates[arg.ID] = arg
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
	}

	svc := NewService(store)

	// Step 1: Cast Fireball
	castCmd := AoECastCommand{
		SpellID:     "fireball",
		CasterID:    caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "H",
		TargetRow:   8,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	castResult, err := svc.CastAoE(context.Background(), castCmd)
	require.NoError(t, err)
	assert.Equal(t, "Fireball", castResult.SpellName)
	assert.Len(t, castResult.AffectedNames, 3) // Goblin A, Goblin B, Fighter
	assert.Len(t, castResult.PendingSaves, 3)
	assert.Equal(t, 15, castResult.SaveDC)
	assert.Equal(t, "dex", castResult.SaveAbility)

	// Step 2: Resolve saves - Goblin A fails, Goblin B fails, Fighter saves
	roller := dice.NewRoller(func(max int) int { return 4 }) // 8d6 = 32
	resolveInput := AoEDamageInput{
		EncounterID: castCmd.EncounterID,
		SpellName:   castResult.SpellName,
		DamageDice:  "8d6",
		DamageType:  "fire",
		SaveEffect:  "half_damage",
		SaveResults: []SaveResult{
			{CombatantID: goblinA.ID, Success: false, Total: 8},
			{CombatantID: goblinB.ID, Success: false, Total: 10},
			{CombatantID: fighter.ID, Success: true, Total: 18, CoverBonus: 0},
		},
	}

	resolveResult, err := svc.ResolveAoESaves(context.Background(), resolveInput, roller)
	require.NoError(t, err)
	require.Len(t, resolveResult.Targets, 3)

	// Goblin A: 30 HP - 32 dmg = 0 HP (dead)
	assert.Equal(t, "Goblin A", resolveResult.Targets[0].DisplayName)
	assert.Equal(t, 32, resolveResult.Targets[0].DamageDealt)
	assert.Equal(t, 0, resolveResult.Targets[0].HPAfter)
	assert.False(t, hpUpdates[goblinA.ID].IsAlive)

	// Goblin B: 30 HP - 32 dmg = 0 HP (dead)
	assert.Equal(t, "Goblin B", resolveResult.Targets[1].DisplayName)
	assert.Equal(t, 32, resolveResult.Targets[1].DamageDealt)
	assert.Equal(t, 0, resolveResult.Targets[1].HPAfter)

	// Fighter saves: 60 HP - 16 dmg = 44 HP (alive)
	assert.Equal(t, "Fighter", resolveResult.Targets[2].DisplayName)
	assert.True(t, resolveResult.Targets[2].SaveSuccess)
	assert.Equal(t, 16, resolveResult.Targets[2].DamageDealt) // half of 32
	assert.Equal(t, 44, resolveResult.Targets[2].HPAfter)
	assert.True(t, hpUpdates[fighter.ID].IsAlive)
}

// TDD Cycle 10 (iter2): Integration test - Burning Hands (cone) with cover DEX bonus
func TestIntegration_BurningHands_ConeWithCover(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	caster.PositionCol = "E"
	caster.PositionRow = 5

	goblin := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Goblin",
		PositionCol: "F", PositionRow: 5, HpMax: 20, HpCurrent: 20,
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
		return goblin, nil
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
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
	}

	svc := NewService(store)

	// Step 1: Cast Burning Hands
	castCmd := AoECastCommand{
		SpellID:     "burning-hands",
		CasterID:    caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "H",
		TargetRow:   5,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	castResult, err := svc.CastAoE(context.Background(), castCmd)
	require.NoError(t, err)
	assert.Contains(t, castResult.AffectedNames, "Goblin")
	assert.Equal(t, "dex", castResult.SaveAbility)
	// Cone origin is caster for cover calculation
	assert.Equal(t, 4, castResult.OriginCol) // colToIndex("E")=4
	assert.Equal(t, 4, castResult.OriginRow) // 5-1=4

	// Step 2: Resolve with save result
	roller := dice.NewRoller(func(max int) int { return 3 }) // 3d6 = 9
	resolveInput := AoEDamageInput{
		EncounterID: castCmd.EncounterID,
		SpellName:   castResult.SpellName,
		DamageDice:  "3d6",
		DamageType:  "fire",
		SaveEffect:  "half_damage",
		SaveResults: []SaveResult{
			{CombatantID: goblin.ID, Success: true, Total: 16, CoverBonus: 2},
		},
	}

	resolveResult, err := svc.ResolveAoESaves(context.Background(), resolveInput, roller)
	require.NoError(t, err)
	require.Len(t, resolveResult.Targets, 1)
	assert.True(t, resolveResult.Targets[0].SaveSuccess)
	assert.Equal(t, 2, resolveResult.Targets[0].CoverBonus)
	// 9 * 0.5 = 4 (half damage)
	assert.Equal(t, 4, resolveResult.Targets[0].DamageDealt)
	assert.Equal(t, 16, resolveResult.Targets[0].HPAfter) // 20-4=16
}

// TDD Cycle 11 (iter2): Integration test - Lightning Bolt (line) hits creatures in a line
func TestIntegration_LightningBolt_Line(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	caster.PositionCol = "A"
	caster.PositionRow = 5

	// Creatures in a line east from caster
	goblinA := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Goblin A",
		PositionCol: "C", PositionRow: 5, HpMax: 25, HpCurrent: 25,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}
	goblinB := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Goblin B",
		PositionCol: "F", PositionRow: 5, HpMax: 25, HpCurrent: 25,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}
	// Creature off to the side, should NOT be hit
	offside := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Offside",
		PositionCol: "C", PositionRow: 8, HpMax: 25, HpCurrent: 25,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}

	lightningBolt := refdata.Spell{
		ID: "lightning-bolt", Name: "Lightning Bolt", Level: 3,
		CastingTime: "1 action", RangeType: "self",
		SaveAbility: sql.NullString{String: "dex", Valid: true},
		SaveEffect:  sql.NullString{String: "half_damage", Valid: true},
		AreaOfEffect: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"shape":"line","length_ft":100,"width_ft":5}`),
			Valid:      true,
		},
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: false, Valid: true},
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return lightningBolt, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		switch id {
		case caster.ID:
			return caster, nil
		case goblinA.ID:
			return goblinA, nil
		case goblinB.ID:
			return goblinB, nil
		case offside.ID:
			return offside, nil
		}
		return refdata.Combatant{}, fmt.Errorf("not found")
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster, goblinA, goblinB, offside}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
	}

	svc := NewService(store)

	// Cast Lightning Bolt east
	castCmd := AoECastCommand{
		SpellID:     "lightning-bolt",
		CasterID:    caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "T", // direction: east
		TargetRow:   5,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	castResult, err := svc.CastAoE(context.Background(), castCmd)
	require.NoError(t, err)
	// Goblin A (C5) and Goblin B (F5) should be in the line
	assert.Contains(t, castResult.AffectedNames, "Goblin A")
	assert.Contains(t, castResult.AffectedNames, "Goblin B")
	// Offside (C8) should NOT be in the line
	assert.NotContains(t, castResult.AffectedNames, "Offside")

	// Resolve: both fail save
	roller := dice.NewRoller(func(max int) int { return 4 }) // 8d6=32
	resolveInput := AoEDamageInput{
		EncounterID: castCmd.EncounterID,
		SpellName:   castResult.SpellName,
		DamageDice:  "8d6",
		DamageType:  "lightning",
		SaveEffect:  "half_damage",
		SaveResults: []SaveResult{
			{CombatantID: goblinA.ID, Success: false},
			{CombatantID: goblinB.ID, Success: true}, // saves
		},
	}

	resolveResult, err := svc.ResolveAoESaves(context.Background(), resolveInput, roller)
	require.NoError(t, err)
	require.Len(t, resolveResult.Targets, 2)
	assert.Equal(t, 32, resolveResult.Targets[0].DamageDealt) // full
	assert.Equal(t, 16, resolveResult.Targets[1].DamageDealt) // half
}

// TDD Cycle 12 (iter2): Cover DEX bonus is reflected in PendingSaves from CastAoE
func TestIntegration_CoverDEXBonus_InPendingSaves(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	caster.PositionCol = "A"
	caster.PositionRow = 1

	// Goblin at the fireball's target point
	goblin := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Goblin",
		PositionCol: "H", PositionRow: 8, HpMax: 20, HpCurrent: 20,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
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
		return goblin, nil
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

	// Caster at A1 targeting H8 (far away so caster is not in 20ft sphere)
	castCmd := AoECastCommand{
		SpellID:     "fireball",
		CasterID:    caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "H",
		TargetRow:   8,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	castResult, err := svc.CastAoE(context.Background(), castCmd)
	require.NoError(t, err)
	require.Len(t, castResult.PendingSaves, 1) // only goblin
	assert.Equal(t, goblin.ID, castResult.PendingSaves[0].CombatantID)
	assert.Equal(t, 0, castResult.PendingSaves[0].CoverBonus) // no walls = no cover
	assert.Equal(t, "dex", castResult.PendingSaves[0].SaveAbility)
	assert.Equal(t, 15, castResult.PendingSaves[0].DC)
	assert.True(t, castResult.PendingSaves[0].IsNPC)
}

// TDD Cycle 13 (iter2): ResolveAoESaves with special save effect returns 0 damage
func TestResolveAoESaves_SpecialSaveEffect(t *testing.T) {
	goblinID := uuid.New()

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: goblinID, DisplayName: "Goblin", HpMax: 30, HpCurrent: 30,
			IsAlive: true, Conditions: json.RawMessage(`[]`),
		}, nil
	}
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
	}

	roller := dice.NewRoller(func(max int) int { return 5 })
	svc := NewService(store)

	input := AoEDamageInput{
		DamageDice:  "4d6",
		SaveEffect:  "special",
		SaveResults: []SaveResult{
			{CombatantID: goblinID, Success: false},
		},
	}

	result, err := svc.ResolveAoESaves(context.Background(), input, roller)
	require.NoError(t, err)
	require.Len(t, result.Targets, 1)
	// Special returns -1.0 multiplier, which is clamped to 0 damage
	assert.Equal(t, 0, result.Targets[0].DamageDealt)
	assert.Equal(t, 30, result.Targets[0].HPAfter) // unchanged
}

// TDD Cycle 14 (iter2): ResolveAoESaves with empty save results returns empty result
func TestResolveAoESaves_EmptySaveResults(t *testing.T) {
	store := defaultMockStore()
	roller := dice.NewRoller(func(max int) int { return 3 })
	svc := NewService(store)

	input := AoEDamageInput{
		DamageDice:  "8d6",
		SaveEffect:  "half_damage",
		SaveResults: []SaveResult{},
	}

	result, err := svc.ResolveAoESaves(context.Background(), input, roller)
	require.NoError(t, err)
	assert.Empty(t, result.Targets)
	assert.Equal(t, 0, result.TotalDamage)
}

// TDD Cycle 15 (iter2): Half damage rounds down
func TestResolveAoESaves_HalfDamageRoundsDown(t *testing.T) {
	goblinID := uuid.New()

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: goblinID, DisplayName: "Goblin", HpMax: 50, HpCurrent: 50,
			IsAlive: true, Conditions: json.RawMessage(`[]`),
		}, nil
	}
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
	}

	// Roll 7 total (odd number to test rounding): 1d6 always rolling 7 won't work,
	// use 7d6 each rolling 1 = 7 total
	roller := dice.NewRoller(func(max int) int { return 1 })
	svc := NewService(store)

	input := AoEDamageInput{
		DamageDice:  "7d6",
		SaveEffect:  "half_damage",
		SaveResults: []SaveResult{
			{CombatantID: goblinID, Success: true},
		},
	}

	result, err := svc.ResolveAoESaves(context.Background(), input, roller)
	require.NoError(t, err)
	// 7 * 0.5 = 3.5, int() truncates to 3 (rounds down)
	assert.Equal(t, 3, result.Targets[0].DamageDealt)
	assert.Equal(t, 47, result.Targets[0].HPAfter) // 50-3=47
}

// crit-03: ResolveAoESaves must route through ApplyDamage so the target's
// resistance / immunity / vulnerability and temp HP apply before the HP write.

func TestResolveAoESaves_NPCResistanceHalvesAoEDamage(t *testing.T) {
	goblinID := uuid.New()
	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: goblinID, DisplayName: "Fire-Goblin", HpMax: 30, HpCurrent: 30,
			IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
			CreatureRefID: sql.NullString{String: "fire-goblin", Valid: true},
		}, nil
	}
	store.getCreatureFn = func(_ context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "fire-goblin", DamageResistances: []string{"fire"}}, nil
	}
	var capturedHP refdata.UpdateCombatantHPParams
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		capturedHP = arg
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
	}

	// 8d6 each rolling 4 = 32 fire damage; resistance halves to 16.
	roller := dice.NewRoller(func(max int) int { return 4 })
	svc := NewService(store)

	input := AoEDamageInput{
		EncounterID: uuid.New(),
		SpellName:   "Fireball",
		DamageDice:  "8d6",
		DamageType:  "fire",
		SaveEffect:  "half_damage",
		SaveResults: []SaveResult{{CombatantID: goblinID, Success: false}},
	}
	result, err := svc.ResolveAoESaves(context.Background(), input, roller)
	require.NoError(t, err)
	require.Len(t, result.Targets, 1)
	assert.Equal(t, 16, result.Targets[0].DamageDealt, "resistance halves 32 -> 16")
	assert.Equal(t, 14, result.Targets[0].HPAfter)    // 30 - 16
	assert.Equal(t, int32(14), capturedHP.HpCurrent)
	assert.True(t, capturedHP.IsAlive)
}

func TestResolveAoESaves_NPCImmunityZeroesAoEDamage(t *testing.T) {
	skeletonID := uuid.New()
	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: skeletonID, DisplayName: "Skeleton", HpMax: 13, HpCurrent: 13,
			IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
			CreatureRefID: sql.NullString{String: "skeleton", Valid: true},
		}, nil
	}
	store.getCreatureFn = func(_ context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "skeleton", DamageImmunities: []string{"poison"}}, nil
	}
	var capturedHP refdata.UpdateCombatantHPParams
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		capturedHP = arg
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
	}

	// 4d4 each rolling 4 = 16 poison damage; immunity zeroes it.
	roller := dice.NewRoller(func(max int) int { return 4 })
	svc := NewService(store)
	input := AoEDamageInput{
		DamageDice: "4d4", DamageType: "poison", SaveEffect: "half_damage",
		SaveResults: []SaveResult{{CombatantID: skeletonID, Success: false}},
	}
	result, err := svc.ResolveAoESaves(context.Background(), input, roller)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Targets[0].DamageDealt)
	assert.Equal(t, 13, result.Targets[0].HPAfter)
	assert.Equal(t, int32(13), capturedHP.HpCurrent, "immunity must keep HP unchanged")
}

// E-59 TDD: CastAoE persists one pending_saves row per affected combatant.
// Before the fix, PendingSaves were returned on the result but never written
// to the pending_saves table, so the resolution loop was unreachable.
func TestCastAoE_PersistsPendingSavesForEachAffectedCombatant(t *testing.T) {
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

	fireball := makeFireball()
	fireball.AreaOfEffect = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"shape":"sphere","radius_ft":20}`),
		Valid:      true,
	}
	fireball.SaveEffect = sql.NullString{String: "half_damage", Valid: true}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return fireball, nil }
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return refdata.Combatant{}, fmt.Errorf("not found")
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster, goblinA, goblinB}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed, ActionSpellCast: arg.ActionSpellCast}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}
	var pendingCalls []refdata.CreatePendingSaveParams
	store.createPendingSaveFn = func(_ context.Context, arg refdata.CreatePendingSaveParams) (refdata.PendingSafe, error) {
		pendingCalls = append(pendingCalls, arg)
		return refdata.PendingSafe{ID: uuid.New(), EncounterID: arg.EncounterID, CombatantID: arg.CombatantID, Source: arg.Source}, nil
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
	require.Equal(t, 2, len(result.PendingSaves))
	require.Equal(t, len(result.PendingSaves), len(pendingCalls),
		"expected one pending_saves row per PendingSave returned")
	for _, ps := range pendingCalls {
		assert.Equal(t, "dex", ps.Ability)
		assert.Equal(t, int32(15), ps.Dc)
		assert.True(t, strings.HasPrefix(ps.Source, "aoe:"), "source should be aoe-tagged, got %q", ps.Source)
		assert.Contains(t, ps.Source, "fireball")
	}
}

// E-59 TDD: ResolveAoEPendingSaves applies damage once every pending row for
// a given (encounter, spell) has been resolved. Mixed-saves scenario.
func TestResolveAoEPendingSaves_AppliesDamageOnceAllResolved(t *testing.T) {
	encounterID := uuid.New()
	combAID := uuid.New()
	combBID := uuid.New()
	spellID := "fireball"
	source := "aoe:" + spellID

	combA := refdata.Combatant{ID: combAID, DisplayName: "Goblin A", HpMax: 30, HpCurrent: 30, IsAlive: true}
	combB := refdata.Combatant{ID: combBID, DisplayName: "Goblin B", HpMax: 30, HpCurrent: 30, IsAlive: true}

	fireball := makeFireball()
	fireball.SaveEffect = sql.NullString{String: "half_damage", Valid: true}
	fireball.Damage = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"dice":"8d6","damage_type":"fire"}`),
		Valid:      true,
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return fireball, nil }
	store.listPendingSavesByEncounterFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		// One rolled (success=true), one rolled (success=false) — fully resolved
		return []refdata.PendingSafe{
			{ID: uuid.New(), EncounterID: encounterID, CombatantID: combAID, Source: source, Ability: "dex", Dc: 15, Status: "rolled", RollResult: sql.NullInt32{Int32: 18, Valid: true}, Success: sql.NullBool{Bool: true, Valid: true}},
			{ID: uuid.New(), EncounterID: encounterID, CombatantID: combBID, Source: source, Ability: "dex", Dc: 15, Status: "rolled", RollResult: sql.NullInt32{Int32: 8, Valid: true}, Success: sql.NullBool{Bool: false, Valid: true}},
		}, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == combAID {
			return combA, nil
		}
		return combB, nil
	}
	var hpUpdates []refdata.UpdateCombatantHPParams
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpUpdates = append(hpUpdates, arg)
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
	}

	// dice always rolls 4 → 8d6 = 32 fire damage; half_damage means saver
	// gets 16, failer gets 32.
	roller := dice.NewRoller(func(_ int) int { return 4 })
	svc := NewService(store)

	res, err := svc.ResolveAoEPendingSaves(context.Background(), encounterID, spellID, roller)
	require.NoError(t, err)
	require.NotNil(t, res, "expected damage to be applied once all saves resolved")
	require.Equal(t, 2, len(res.Targets))
	// Two HP updates expected, one per affected combatant.
	require.Equal(t, 2, len(hpUpdates))
}

// E-59 TDD: AoE source helpers round-trip the spell ID.
func TestAoEPendingSaveSourceHelpers(t *testing.T) {
	src := AoEPendingSaveSource("fireball")
	require.True(t, IsAoEPendingSaveSource(src))
	require.Equal(t, "fireball", SpellIDFromAoEPendingSaveSource(src))
	require.False(t, IsAoEPendingSaveSource("concentration"))
	require.Equal(t, "", SpellIDFromAoEPendingSaveSource("concentration"))
}

// E-59 TDD: RecordAoEPendingSaveRoll resolves the oldest pending AoE row on
// the combatant and writes (total, total>=Dc).
func TestRecordAoEPendingSaveRoll_ResolvesMatchingRow(t *testing.T) {
	combatantID := uuid.New()
	rowID := uuid.New()
	store := defaultMockStore()
	store.listPendingSavesByCombatantFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{
			{ID: rowID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: AoEPendingSaveSource("fireball"), Status: "pending"},
		}, nil
	}
	var updated refdata.UpdatePendingSaveResultParams
	store.updatePendingSaveResultFn = func(_ context.Context, arg refdata.UpdatePendingSaveResultParams) (refdata.PendingSafe, error) {
		updated = arg
		return refdata.PendingSafe{ID: arg.ID, RollResult: arg.RollResult, Success: arg.Success, Source: AoEPendingSaveSource("fireball"), Status: "rolled"}, nil
	}

	svc := NewService(store)
	spellID, resolved, err := svc.RecordAoEPendingSaveRoll(context.Background(), combatantID, "dex", 18, false)
	require.NoError(t, err)
	assert.True(t, resolved)
	assert.Equal(t, "fireball", spellID)
	assert.Equal(t, rowID, updated.ID)
	assert.True(t, updated.Success.Bool, "18 >= 15 should be a success")
	assert.Equal(t, int32(18), updated.RollResult.Int32)
}

// E-59 TDD: RecordAoEPendingSaveRoll fails the save when autoFail is set
// (e.g. natural 1 / paralyzed) regardless of the total.
func TestRecordAoEPendingSaveRoll_AutoFailMarksFailure(t *testing.T) {
	combatantID := uuid.New()
	store := defaultMockStore()
	store.listPendingSavesByCombatantFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{
			{ID: uuid.New(), CombatantID: combatantID, Ability: "dex", Dc: 10, Source: AoEPendingSaveSource("fireball"), Status: "pending"},
		}, nil
	}
	var updated refdata.UpdatePendingSaveResultParams
	store.updatePendingSaveResultFn = func(_ context.Context, arg refdata.UpdatePendingSaveResultParams) (refdata.PendingSafe, error) {
		updated = arg
		return refdata.PendingSafe{ID: arg.ID, Source: AoEPendingSaveSource("fireball"), Status: "rolled"}, nil
	}

	svc := NewService(store)
	_, resolved, err := svc.RecordAoEPendingSaveRoll(context.Background(), combatantID, "dex", 25, true)
	require.NoError(t, err)
	assert.True(t, resolved)
	assert.False(t, updated.Success.Bool, "autoFail must override the total")
}

// E-59 TDD: RecordAoEPendingSaveRoll is a no-op when no matching AoE row is
// pending (player rolled a proactive /save, or row already rolled).
func TestRecordAoEPendingSaveRoll_NoMatchingRowIsNoop(t *testing.T) {
	combatantID := uuid.New()
	store := defaultMockStore()
	store.listPendingSavesByCombatantFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{
			{ID: uuid.New(), CombatantID: combatantID, Ability: "con", Source: ConcentrationSaveSource, Status: "pending"},
		}, nil
	}
	updateCalls := 0
	store.updatePendingSaveResultFn = func(_ context.Context, arg refdata.UpdatePendingSaveResultParams) (refdata.PendingSafe, error) {
		updateCalls++
		return refdata.PendingSafe{ID: arg.ID}, nil
	}

	svc := NewService(store)
	spellID, resolved, err := svc.RecordAoEPendingSaveRoll(context.Background(), combatantID, "dex", 18, false)
	require.NoError(t, err)
	assert.False(t, resolved)
	assert.Equal(t, "", spellID)
	assert.Equal(t, 0, updateCalls)
}

// E-59 TDD: ResolveAoEPendingSaves is a no-op when one or more rows are still
// pending (DM-rolled enemy save outstanding, etc.).
func TestResolveAoEPendingSaves_NoopWhenPendingRemain(t *testing.T) {
	encounterID := uuid.New()
	spellID := "fireball"
	source := "aoe:" + spellID

	fireball := makeFireball()
	fireball.SaveEffect = sql.NullString{String: "half_damage", Valid: true}
	fireball.Damage = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"dice":"8d6","damage_type":"fire"}`),
		Valid:      true,
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return fireball, nil }
	store.listPendingSavesByEncounterFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{
			{ID: uuid.New(), EncounterID: encounterID, CombatantID: uuid.New(), Source: source, Status: "rolled", RollResult: sql.NullInt32{Int32: 18, Valid: true}, Success: sql.NullBool{Bool: true, Valid: true}},
			{ID: uuid.New(), EncounterID: encounterID, CombatantID: uuid.New(), Source: source, Status: "pending"},
		}, nil
	}
	var hpCalls int
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpCalls++
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
	}

	roller := dice.NewRoller(func(_ int) int { return 4 })
	svc := NewService(store)

	res, err := svc.ResolveAoEPendingSaves(context.Background(), encounterID, spellID, roller)
	require.NoError(t, err)
	assert.Nil(t, res, "expected no damage applied while one save is still pending")
	assert.Equal(t, 0, hpCalls)
}

// ---------------------------------------------------------------------------
// SR-013: cylinder shape + AoE Metamagic plumbing
// ---------------------------------------------------------------------------

// TestGetAffectedTiles_Cylinder verifies cylinder shape selects the same disc
// of tiles as a sphere of the same radius (height_ft is decorative for 2D).
func TestGetAffectedTiles_Cylinder(t *testing.T) {
	t.Run("20ft radius cylinder == 20ft sphere disc", func(t *testing.T) {
		cyl := AreaOfEffect{Shape: "cylinder", RadiusFt: 20}
		sph := AreaOfEffect{Shape: "sphere", RadiusFt: 20}
		cylTiles, err := GetAffectedTiles(cyl, 10, 10, 10, 10)
		require.NoError(t, err)
		sphTiles, err := GetAffectedTiles(sph, 10, 10, 10, 10)
		require.NoError(t, err)
		assert.ElementsMatch(t, sphTiles, cylTiles)
	})

	t.Run("5ft radius Moonbeam disc", func(t *testing.T) {
		// 5ft radius from (10,10) = origin + 4 cardinals; diagonals excluded.
		cyl := AreaOfEffect{Shape: "cylinder", RadiusFt: 5}
		tiles, err := GetAffectedTiles(cyl, 5, 5, 10, 10)
		require.NoError(t, err)
		assert.Contains(t, tiles, GridPos{10, 10})
		assert.Contains(t, tiles, GridPos{9, 10})
		assert.Contains(t, tiles, GridPos{11, 10})
		assert.Contains(t, tiles, GridPos{10, 9})
		assert.Contains(t, tiles, GridPos{10, 11})
		assert.NotContains(t, tiles, GridPos{9, 9})
		assert.NotContains(t, tiles, GridPos{11, 11})
	})
}

// TestCastAoE_Cylinder_Moonbeam exercises the full CastAoE pipeline against a
// cylinder-shaped spell so we know the switch fix wires through.
func TestCastAoE_Cylinder_Moonbeam(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	caster.PositionCol = "E"
	caster.PositionRow = 5

	target := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Goblin",
		PositionCol: "H", PositionRow: 8,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}

	moonbeam := refdata.Spell{
		ID:             "moonbeam",
		Name:           "Moonbeam",
		Level:          2,
		CastingTime:    "1 action",
		RangeType:      "ranged",
		RangeFt:        sql.NullInt32{Int32: 120, Valid: true},
		SaveAbility:    sql.NullString{String: "con", Valid: true},
		SaveEffect:     sql.NullString{String: "half_damage", Valid: true},
		Concentration:  sql.NullBool{Bool: true, Valid: true},
		ResolutionMode: "auto",
		AreaOfEffect: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"shape":"cylinder","radius_ft":5,"height_ft":40}`),
			Valid:      true,
		},
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return moonbeam, nil }
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return refdata.Combatant{}, fmt.Errorf("not found")
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster, target}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)
	cmd := AoECastCommand{
		SpellID:     "moonbeam",
		CasterID:    caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "H",
		TargetRow:   8,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	result, err := svc.CastAoE(context.Background(), cmd)
	require.NoError(t, err)
	// Target sits at the cylinder's origin -> affected.
	assert.Contains(t, result.AffectedNames, "Goblin")
	assert.Equal(t, "Moonbeam", result.SpellName)
}

// sr013SorcererFixture is a focused mock setup for AoE metamagic tests.
type sr013SorcererFixture struct {
	svc      *Service
	store    *mockStore
	caster   refdata.Combatant
	ally     refdata.Combatant
	enemy    refdata.Combatant
}

func newSR013SorcererFixture(t *testing.T, spell refdata.Spell) sr013SorcererFixture {
	t.Helper()
	charID := uuid.New()
	char := makeSorcererCharacter(charID, 5, 5) // CHA 18 → mod 4; 5 SP
	caster := makeSorcererCombatant(charID)
	caster.PositionCol = "E"
	caster.PositionRow = 5

	// Two targets near the AoE origin (H8) so a 20ft sphere covers both.
	ally := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Ally",
		PositionCol: "H", PositionRow: 8,
		IsAlive: true, IsNpc: false, Conditions: json.RawMessage(`[]`),
	}
	enemy := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Goblin",
		PositionCol: "I", PositionRow: 8,
		IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`),
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return spell, nil }
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return refdata.Combatant{}, fmt.Errorf("not found")
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster, ally, enemy}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}
	store.updateCharacterFeatureUsesFn = func(_ context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}

	return sr013SorcererFixture{
		svc:    NewService(store),
		store:  store,
		caster: caster,
		ally:   ally,
		enemy:  enemy,
	}
}

func sr013SorcererFireball() refdata.Spell {
	return refdata.Spell{
		ID:          "fireball",
		Name:        "Fireball",
		Level:       3,
		CastingTime: "1 action",
		RangeType:   "ranged",
		RangeFt:     sql.NullInt32{Int32: 150, Valid: true},
		SaveAbility: sql.NullString{String: "dex", Valid: true},
		SaveEffect:  sql.NullString{String: "half_damage", Valid: true},
		Damage: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"dice":"8d6","type":"fire"}`),
			Valid:      true,
		},
		AreaOfEffect: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"shape":"sphere","radius_ft":20}`),
			Valid:      true,
		},
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: false, Valid: true},
		Duration:       "Instantaneous",
		Components:     []string{"V", "S", "M"},
	}
}

// TestCastAoE_Careful_SparesAlly verifies that CarefulTargetIDs flips the
// matched ally's PendingSave to AutoSuccess and immediately resolves the row
// at the DB layer, while non-careful targets remain pending.
func TestCastAoE_Careful_SparesAlly(t *testing.T) {
	f := newSR013SorcererFixture(t, sr013SorcererFireball())

	var resolvedSuccesses []uuid.UUID
	createdRows := map[uuid.UUID]uuid.UUID{} // combatantID -> rowID
	f.store.createPendingSaveFn = func(_ context.Context, arg refdata.CreatePendingSaveParams) (refdata.PendingSafe, error) {
		row := refdata.PendingSafe{
			ID: uuid.New(), EncounterID: arg.EncounterID, CombatantID: arg.CombatantID,
			Ability: arg.Ability, Dc: arg.Dc, Source: arg.Source, Status: "pending",
		}
		createdRows[arg.CombatantID] = row.ID
		return row, nil
	}
	f.store.updatePendingSaveResultFn = func(_ context.Context, arg refdata.UpdatePendingSaveResultParams) (refdata.PendingSafe, error) {
		if arg.Success.Valid && arg.Success.Bool {
			resolvedSuccesses = append(resolvedSuccesses, arg.ID)
		}
		return refdata.PendingSafe{ID: arg.ID, Status: "rolled", RollResult: arg.RollResult, Success: arg.Success}, nil
	}

	cmd := AoECastCommand{
		SpellID:          "fireball",
		CasterID:         f.caster.ID,
		EncounterID:      uuid.New(),
		TargetCol:        "H",
		TargetRow:        8,
		Turn:             refdata.Turn{ID: uuid.New(), CombatantID: f.caster.ID},
		Metamagic:        []string{"careful"},
		CarefulTargetIDs: []uuid.UUID{f.ally.ID},
	}

	result, err := f.svc.CastAoE(context.Background(), cmd)
	require.NoError(t, err)
	assert.Equal(t, 4, result.CarefulSpellCreatures) // CHA mod 4
	assert.Equal(t, 1, result.MetamagicCost)         // 1 SP for careful
	assert.Equal(t, 4, result.SorceryPointsRemaining)

	// PendingSaves: ally has AutoSuccess, enemy does not.
	var allyPS, enemyPS *PendingSave
	for i, ps := range result.PendingSaves {
		switch ps.CombatantID {
		case f.ally.ID:
			allyPS = &result.PendingSaves[i]
		case f.enemy.ID:
			enemyPS = &result.PendingSaves[i]
		}
	}
	require.NotNil(t, allyPS, "ally save row missing")
	require.NotNil(t, enemyPS, "enemy save row missing")
	assert.True(t, allyPS.AutoSuccess, "Careful ally should auto-succeed")
	assert.False(t, enemyPS.AutoSuccess, "enemy should NOT auto-succeed")

	// DB row: ally's row was resolved as a success.
	allyRowID := createdRows[f.ally.ID]
	assert.Contains(t, resolvedSuccesses, allyRowID, "Careful ally save row should be DB-resolved as success")
}

// TestCastAoE_Heightened_FirstTargetDisadvantage verifies the first affected
// target's PendingSave is flagged Disadvantage; the rest are not.
func TestCastAoE_Heightened_FirstTargetDisadvantage(t *testing.T) {
	f := newSR013SorcererFixture(t, sr013SorcererFireball())

	cmd := AoECastCommand{
		SpellID:     "fireball",
		CasterID:    f.caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "H",
		TargetRow:   8,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: f.caster.ID},
		Metamagic:   []string{"heightened"},
	}

	result, err := f.svc.CastAoE(context.Background(), cmd)
	require.NoError(t, err)
	assert.True(t, result.IsHeightened)
	assert.Equal(t, 3, result.MetamagicCost) // Heightened = 3 SP

	require.GreaterOrEqual(t, len(result.PendingSaves), 2, "need >=2 affected targets")
	disCount := 0
	for _, ps := range result.PendingSaves {
		if ps.Disadvantage {
			disCount++
		}
	}
	assert.Equal(t, 1, disCount, "exactly one target should get Heightened disadvantage")
	assert.True(t, result.PendingSaves[0].Disadvantage, "first save row must be the Heightened one")
}

// TestCastAoE_Twinned_Rejected verifies Twinned Spell is rejected on an AoE
// cast (the spec disallows Twinned for area spells; ValidateMetamagicOptions
// surfaces this via validateTwinnedSpell).
func TestCastAoE_Twinned_Rejected(t *testing.T) {
	f := newSR013SorcererFixture(t, sr013SorcererFireball())

	var slotDeducted bool
	f.store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		slotDeducted = true
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	cmd := AoECastCommand{
		SpellID:     "fireball",
		CasterID:    f.caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "H",
		TargetRow:   8,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: f.caster.ID},
		Metamagic:   []string{"twinned"},
	}

	_, err := f.svc.CastAoE(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "area of effect", "Twinned must reject AoE shapes")
	assert.False(t, slotDeducted, "rejected metamagic must NOT burn a slot")
}

// TestCastAoE_Empowered_SetsFlag verifies Empowered surfaces on AoECastResult.
func TestCastAoE_Empowered_SetsFlag(t *testing.T) {
	f := newSR013SorcererFixture(t, sr013SorcererFireball())

	cmd := AoECastCommand{
		SpellID:     "fireball",
		CasterID:    f.caster.ID,
		EncounterID: uuid.New(),
		TargetCol:   "H",
		TargetRow:   8,
		Turn:        refdata.Turn{ID: uuid.New(), CombatantID: f.caster.ID},
		Metamagic:   []string{"empowered"},
	}

	result, err := f.svc.CastAoE(context.Background(), cmd)
	require.NoError(t, err)
	assert.True(t, result.IsEmpowered)
	assert.Equal(t, 4, result.EmpoweredRerolls) // CHA mod 4
	assert.Equal(t, 1, result.MetamagicCost)
}
