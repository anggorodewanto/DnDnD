package combat

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

func TestValidateMove_BasicPath(t *testing.T) {
	// 5x5 grid, all open ground, mover at A1 (col=0, row=0), target D1 (col=3, row=0)
	terrain := make([]renderer.TerrainType, 25)
	grid := &pathfinding.Grid{
		Width:   5,
		Height:  5,
		Terrain: terrain,
	}

	turn := refdata.Turn{MovementRemainingFt: 30}
	combatant := refdata.Combatant{PositionCol: "A", PositionRow: 1}

	req := MoveRequest{
		DestCol:      3,
		DestRow:      0,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: pathfinding.SizeMedium,
	}

	result, err := ValidateMove(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid move, got invalid: %s", result.Reason)
	}
	if result.CostFt != 15 {
		t.Errorf("expected cost 15ft, got %d", result.CostFt)
	}
	if result.RemainingFt != 15 {
		t.Errorf("expected 15ft remaining, got %d", result.RemainingFt)
	}
}

func TestValidateMove_NotEnoughMovement(t *testing.T) {
	terrain := make([]renderer.TerrainType, 25)
	grid := &pathfinding.Grid{
		Width:   5,
		Height:  5,
		Terrain: terrain,
	}

	turn := refdata.Turn{MovementRemainingFt: 10}
	combatant := refdata.Combatant{PositionCol: "A", PositionRow: 1}

	req := MoveRequest{
		DestCol:      3,
		DestRow:      0,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: pathfinding.SizeMedium,
	}

	result, err := ValidateMove(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid move")
	}
	if result.Reason == "" {
		t.Error("expected a reason for invalid move")
	}
}

func TestValidateMove_NoPath(t *testing.T) {
	// 3x1 grid, col 1 occupied by enemy of same size blocking the path
	terrain := make([]renderer.TerrainType, 3)
	grid := &pathfinding.Grid{
		Width:   3,
		Height:  1,
		Terrain: terrain,
		Occupants: []pathfinding.Occupant{
			{Col: 1, Row: 0, IsAlly: false, SizeCategory: pathfinding.SizeMedium},
		},
	}

	turn := refdata.Turn{MovementRemainingFt: 30}
	combatant := refdata.Combatant{PositionCol: "A", PositionRow: 1}

	req := MoveRequest{
		DestCol:      2,
		DestRow:      0,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: pathfinding.SizeMedium,
	}

	result, err := ValidateMove(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid move - no path through enemy")
	}
}

func TestValidateMove_DifficultTerrain(t *testing.T) {
	// 5x1 grid, tile at col 1 is difficult terrain
	terrain := make([]renderer.TerrainType, 5)
	terrain[1] = renderer.TerrainDifficultTerrain
	grid := &pathfinding.Grid{
		Width:   5,
		Height:  1,
		Terrain: terrain,
	}

	turn := refdata.Turn{MovementRemainingFt: 30}
	combatant := refdata.Combatant{PositionCol: "A", PositionRow: 1}

	req := MoveRequest{
		DestCol:      2,
		DestRow:      0,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: pathfinding.SizeMedium,
	}

	result, err := ValidateMove(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid move, got: %s", result.Reason)
	}
	// 10ft for difficult terrain + 5ft for open ground = 15ft
	if result.CostFt != 15 {
		t.Errorf("expected cost 15ft (difficult terrain), got %d", result.CostFt)
	}
}

func TestValidateMove_SamePosition(t *testing.T) {
	terrain := make([]renderer.TerrainType, 4)
	grid := &pathfinding.Grid{
		Width:   2,
		Height:  2,
		Terrain: terrain,
	}

	turn := refdata.Turn{MovementRemainingFt: 30}
	combatant := refdata.Combatant{PositionCol: "A", PositionRow: 1}

	req := MoveRequest{
		DestCol:      0,
		DestRow:      0,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: pathfinding.SizeMedium,
	}

	result, err := ValidateMove(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid move - same position")
	}
}

func TestValidateMove_OccupiedByAlly(t *testing.T) {
	// Can path through ally but cannot end on ally tile
	terrain := make([]renderer.TerrainType, 4)
	grid := &pathfinding.Grid{
		Width:   2,
		Height:  2,
		Terrain: terrain,
		Occupants: []pathfinding.Occupant{
			{Col: 1, Row: 0, IsAlly: true, SizeCategory: pathfinding.SizeMedium},
		},
	}

	turn := refdata.Turn{MovementRemainingFt: 30}
	combatant := refdata.Combatant{PositionCol: "A", PositionRow: 1}

	req := MoveRequest{
		DestCol:      1,
		DestRow:      0,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: pathfinding.SizeMedium,
	}

	result, err := ValidateMove(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid move - cannot end on ally tile")
	}
}

func TestValidateMove_OccupiedByEnemy(t *testing.T) {
	// Cannot move through enemy of similar size
	terrain := make([]renderer.TerrainType, 3)
	grid := &pathfinding.Grid{
		Width:   3,
		Height:  1,
		Terrain: terrain,
		Occupants: []pathfinding.Occupant{
			{Col: 1, Row: 0, IsAlly: false, SizeCategory: pathfinding.SizeMedium},
		},
	}

	turn := refdata.Turn{MovementRemainingFt: 30}
	combatant := refdata.Combatant{PositionCol: "A", PositionRow: 1}

	req := MoveRequest{
		DestCol:      2,
		DestRow:      0,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: pathfinding.SizeMedium,
	}

	result, err := ValidateMove(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid move - cannot end on enemy tile")
	}
}

func TestValidateMove_OutOfBounds(t *testing.T) {
	terrain := make([]renderer.TerrainType, 4)
	grid := &pathfinding.Grid{
		Width:   2,
		Height:  2,
		Terrain: terrain,
	}

	turn := refdata.Turn{MovementRemainingFt: 30}
	combatant := refdata.Combatant{PositionCol: "A", PositionRow: 1}

	req := MoveRequest{
		DestCol:      5,
		DestRow:      5,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: pathfinding.SizeMedium,
	}

	result, err := ValidateMove(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid move - out of bounds")
	}
}

func TestValidateMove_ZeroMovement(t *testing.T) {
	terrain := make([]renderer.TerrainType, 4)
	grid := &pathfinding.Grid{
		Width:   2,
		Height:  2,
		Terrain: terrain,
	}

	turn := refdata.Turn{MovementRemainingFt: 0}
	combatant := refdata.Combatant{PositionCol: "A", PositionRow: 1}

	req := MoveRequest{
		DestCol:      1,
		DestRow:      0,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: pathfinding.SizeMedium,
	}

	result, err := ValidateMove(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid move - no movement remaining")
	}
}

func TestValidateMove_InvalidCombatantPosition(t *testing.T) {
	terrain := make([]renderer.TerrainType, 4)
	grid := &pathfinding.Grid{
		Width:   2,
		Height:  2,
		Terrain: terrain,
	}

	turn := refdata.Turn{MovementRemainingFt: 30}
	combatant := refdata.Combatant{PositionCol: "", PositionRow: 0} // invalid position

	req := MoveRequest{
		DestCol:      1,
		DestRow:      0,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: pathfinding.SizeMedium,
	}

	_, err := ValidateMove(req)
	if err == nil {
		t.Fatal("expected error for invalid combatant position")
	}
}

func TestValidateMove_DifficultTerrainFlag(t *testing.T) {
	// Verify HasDifficultTerrain is set when path goes through difficult terrain
	terrain := make([]renderer.TerrainType, 3)
	terrain[1] = renderer.TerrainDifficultTerrain
	grid := &pathfinding.Grid{
		Width:   3,
		Height:  1,
		Terrain: terrain,
	}

	turn := refdata.Turn{MovementRemainingFt: 30}
	combatant := refdata.Combatant{PositionCol: "A", PositionRow: 1}

	req := MoveRequest{
		DestCol:      2,
		DestRow:      0,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: pathfinding.SizeMedium,
	}

	result, err := ValidateMove(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasDifficultTerrain {
		t.Error("expected HasDifficultTerrain to be true")
	}
}

func TestValidateMove_DestLabel(t *testing.T) {
	terrain := make([]renderer.TerrainType, 25)
	grid := &pathfinding.Grid{
		Width:   5,
		Height:  5,
		Terrain: terrain,
	}

	turn := refdata.Turn{MovementRemainingFt: 30}
	combatant := refdata.Combatant{PositionCol: "A", PositionRow: 1}

	req := MoveRequest{
		DestCol:      3,
		DestRow:      0,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: pathfinding.SizeMedium,
	}

	result, err := ValidateMove(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DestLabel != "D1" {
		t.Errorf("expected dest label D1, got %q", result.DestLabel)
	}
}

func TestFormatMoveConfirmation(t *testing.T) {
	result := &MoveResult{
		DestLabel:   "D4",
		CostFt:      20,
		RemainingFt: 10,
	}
	msg := FormatMoveConfirmation(result)
	if msg != "\U0001f3c3 Move to D4 — 20ft, 10ft remaining after." {
		t.Errorf("unexpected message: %s", msg)
	}
}

func TestFormatMoveConfirmation_DifficultTerrain(t *testing.T) {
	result := &MoveResult{
		DestLabel:           "D4",
		CostFt:              20,
		RemainingFt:         10,
		HasDifficultTerrain: true,
	}
	msg := FormatMoveConfirmation(result)
	if msg != "\U0001f3c3 Move to D4 — 20ft (includes difficult terrain), 10ft remaining after." {
		t.Errorf("unexpected message: %s", msg)
	}
}

func TestValidateEndTurnPosition_OK(t *testing.T) {
	combatant := refdata.Combatant{
		ID:          uuid.New(),
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
	}
	others := []refdata.Combatant{
		{ID: uuid.New(), PositionCol: "B", PositionRow: 1, IsAlive: true, DisplayName: "Goblin"},
	}
	msg := ValidateEndTurnPosition(combatant, others)
	if msg != "" {
		t.Errorf("expected empty message, got: %s", msg)
	}
}

func TestValidateEndTurnPosition_Sharing(t *testing.T) {
	combatant := refdata.Combatant{
		ID:          uuid.New(),
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
	}
	others := []refdata.Combatant{
		{ID: uuid.New(), PositionCol: "A", PositionRow: 1, IsAlive: true, DisplayName: "Goblin"},
	}
	msg := ValidateEndTurnPosition(combatant, others)
	if msg == "" {
		t.Fatal("expected error message for shared tile")
	}
	if !strings.Contains(msg, "Goblin") {
		t.Errorf("expected message to mention Goblin, got: %s", msg)
	}
}

func TestValidateEndTurnPosition_DeadCreatureOK(t *testing.T) {
	combatant := refdata.Combatant{
		ID:          uuid.New(),
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
	}
	others := []refdata.Combatant{
		{ID: uuid.New(), PositionCol: "A", PositionRow: 1, IsAlive: false, DisplayName: "Dead Goblin"},
	}
	msg := ValidateEndTurnPosition(combatant, others)
	if msg != "" {
		t.Errorf("expected empty message for dead creature, got: %s", msg)
	}
}

func TestValidateMove_FlyingOccupantDoesNotBlock(t *testing.T) {
	// An enemy at altitude 30ft should not block destination at ground level
	terrain := make([]renderer.TerrainType, 4)
	grid := &pathfinding.Grid{
		Width:   2,
		Height:  2,
		Terrain: terrain,
		Occupants: []pathfinding.Occupant{
			{Col: 1, Row: 0, IsAlly: false, SizeCategory: pathfinding.SizeMedium, AltitudeFt: 30},
		},
	}

	turn := refdata.Turn{MovementRemainingFt: 30}
	combatant := refdata.Combatant{PositionCol: "A", PositionRow: 1}

	req := MoveRequest{
		DestCol:      1,
		DestRow:      0,
		Turn:         turn,
		Combatant:    combatant,
		Grid:         grid,
		SizeCategory: pathfinding.SizeMedium,
	}

	result, err := ValidateMove(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid move - flying occupant should not block ground, got: %s", result.Reason)
	}
}

func TestValidateEndTurnPosition_DifferentAltitudeOK(t *testing.T) {
	combatant := refdata.Combatant{
		ID:          uuid.New(),
		PositionCol: "A",
		PositionRow: 1,
		AltitudeFt:  30,
		IsAlive:     true,
	}
	others := []refdata.Combatant{
		{ID: uuid.New(), PositionCol: "A", PositionRow: 1, AltitudeFt: 0, IsAlive: true, DisplayName: "Goblin"},
	}
	msg := ValidateEndTurnPosition(combatant, others)
	if msg != "" {
		t.Errorf("expected empty message for different altitude, got: %s", msg)
	}
}

func TestValidateEndTurnPosition_SelfIgnored(t *testing.T) {
	id := uuid.New()
	combatant := refdata.Combatant{
		ID:          id,
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
	}
	others := []refdata.Combatant{combatant}
	msg := ValidateEndTurnPosition(combatant, others)
	if msg != "" {
		t.Errorf("expected empty message when only self, got: %s", msg)
	}
}
