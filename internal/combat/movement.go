package combat

import (
	"fmt"

	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

// MoveRequest holds everything needed to validate a movement command.
type MoveRequest struct {
	DestCol      int
	DestRow      int
	Turn         refdata.Turn
	Combatant    refdata.Combatant
	Grid         *pathfinding.Grid
	SizeCategory int
	IsProne      bool
}

// MoveResult holds the result of a move validation.
type MoveResult struct {
	Valid       bool
	Reason      string
	CostFt      int
	RemainingFt int
	Path        []pathfinding.Point
	DestLabel   string // e.g. "D4"
	HasDifficultTerrain bool
}

// ValidateMove checks whether a move is valid and returns the cost and path.
func ValidateMove(req MoveRequest) (*MoveResult, error) {
	// Parse current position from combatant
	startCol, startRow, err := renderer.ParseCoordinate(req.Combatant.PositionCol + fmt.Sprintf("%d", req.Combatant.PositionRow))
	if err != nil {
		return nil, fmt.Errorf("parsing combatant position: %w", err)
	}

	destLabel := renderer.ColumnLabel(req.DestCol) + fmt.Sprintf("%d", req.DestRow+1)

	// Same position check
	if startCol == req.DestCol && startRow == req.DestRow {
		return &MoveResult{
			Valid:     false,
			Reason:    "You're already at " + destLabel,
			DestLabel: destLabel,
		}, nil
	}

	// No movement remaining
	if req.Turn.MovementRemainingFt <= 0 {
		return &MoveResult{
			Valid:     false,
			Reason:    "No movement remaining",
			DestLabel: destLabel,
		}, nil
	}

	// Out of bounds
	if req.DestCol < 0 || req.DestCol >= req.Grid.Width || req.DestRow < 0 || req.DestRow >= req.Grid.Height {
		return &MoveResult{
			Valid:     false,
			Reason:    fmt.Sprintf("%s is out of bounds", destLabel),
			DestLabel: destLabel,
		}, nil
	}

	// Check occupancy at destination - cannot end on any occupied tile
	for _, occ := range req.Grid.Occupants {
		if occ.Col == req.DestCol && occ.Row == req.DestRow {
			if occ.IsAlly {
				return &MoveResult{
					Valid:     false,
					Reason:    fmt.Sprintf("Cannot end movement in an occupied tile (%s)", destLabel),
					DestLabel: destLabel,
				}, nil
			}
			// Enemy: cannot end on enemy tile either
			return &MoveResult{
				Valid:     false,
				Reason:    fmt.Sprintf("Cannot end movement in an occupied tile (%s)", destLabel),
				DestLabel: destLabel,
			}, nil
		}
	}

	// Find path using A*
	pathResult, err := pathfinding.FindPath(pathfinding.PathRequest{
		Start:        pathfinding.Point{Col: startCol, Row: startRow},
		End:          pathfinding.Point{Col: req.DestCol, Row: req.DestRow},
		IsProne:      req.IsProne,
		SizeCategory: req.SizeCategory,
		Grid:         req.Grid,
	})
	if err != nil {
		return nil, fmt.Errorf("pathfinding error: %w", err)
	}

	if !pathResult.Found {
		return &MoveResult{
			Valid:     false,
			Reason:    fmt.Sprintf("No valid path to %s", destLabel),
			DestLabel: destLabel,
		}, nil
	}

	// Check if path cost exceeds remaining movement
	cost := pathResult.TotalCostFt
	remaining := int(req.Turn.MovementRemainingFt) - cost
	if remaining < 0 {
		return &MoveResult{
			Valid:     false,
			Reason:    fmt.Sprintf("Not enough movement: %dft needed, %dft remaining", cost, req.Turn.MovementRemainingFt),
			DestLabel: destLabel,
		}, nil
	}

	// Check for difficult terrain in path
	hasDifficult := false
	for _, p := range pathResult.Path {
		idx := p.Row*req.Grid.Width + p.Col
		if idx >= 0 && idx < len(req.Grid.Terrain) && req.Grid.Terrain[idx] == renderer.TerrainDifficultTerrain {
			hasDifficult = true
			break
		}
	}

	return &MoveResult{
		Valid:               true,
		CostFt:              cost,
		RemainingFt:         remaining,
		Path:                pathResult.Path,
		DestLabel:           destLabel,
		HasDifficultTerrain: hasDifficult,
	}, nil
}

// FormatMoveConfirmation produces the ephemeral confirmation message for a move.
func FormatMoveConfirmation(result *MoveResult) string {
	msg := fmt.Sprintf("\U0001f3c3 Move to %s — %dft", result.DestLabel, result.CostFt)
	if result.HasDifficultTerrain {
		msg += " (includes difficult terrain)"
	}
	msg += fmt.Sprintf(", %dft remaining after.", result.RemainingFt)
	return msg
}

// ValidateEndTurnPosition checks if the combatant is sharing a tile with another creature.
// Returns an error message if they cannot end their turn there, empty string if OK.
func ValidateEndTurnPosition(combatant refdata.Combatant, allCombatants []refdata.Combatant) string {
	for _, other := range allCombatants {
		if other.ID == combatant.ID {
			continue
		}
		if !other.IsAlive {
			continue
		}
		if other.PositionCol == combatant.PositionCol && other.PositionRow == combatant.PositionRow {
			return fmt.Sprintf("You can't end your turn in another creature's space — use `/move` to leave %s's tile", other.DisplayName)
		}
	}
	return ""
}
