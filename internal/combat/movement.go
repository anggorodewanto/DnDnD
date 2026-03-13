package combat

import (
	"fmt"

	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

// MoveMode represents how a prone combatant chooses to move.
type MoveMode string

const (
	// MoveModeNormal is the default movement mode (not prone).
	MoveModeNormal MoveMode = "normal"
	// MoveModeStandAndMove means the combatant stands up (costing half max speed) then moves normally.
	MoveModeStandAndMove MoveMode = "stand_and_move"
	// MoveModeCrawl means the combatant stays prone and crawls (double movement cost).
	MoveModeCrawl MoveMode = "crawl"
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
	Valid               bool
	Reason              string
	CostFt              int
	RemainingFt         int
	Path                []pathfinding.Point
	DestLabel           string // e.g. "D4"
	HasDifficultTerrain bool
	MoveMode            MoveMode
	StandCostFt         int // cost to stand from prone (only for stand_and_move)
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

	// Check occupancy at destination - cannot end on any occupied tile at the same altitude
	for _, occ := range req.Grid.Occupants {
		if occ.Col == req.DestCol && occ.Row == req.DestRow && occ.AltitudeFt == 0 {
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

// ValidateProneMoveStandAndMove validates a stand-and-move action for a prone combatant.
// It deducts the stand cost (half max speed) from remaining movement, then validates the
// path at normal cost with the reduced movement budget.
func ValidateProneMoveStandAndMove(req MoveRequest, maxSpeed int) (*MoveResult, error) {
	standCost := StandFromProneCost(maxSpeed)

	// Check if we have enough movement to stand
	if int(req.Turn.MovementRemainingFt) < standCost {
		destLabel := renderer.ColumnLabel(req.DestCol) + fmt.Sprintf("%d", req.DestRow+1)
		return &MoveResult{
			Valid:    false,
			Reason:   fmt.Sprintf("Not enough movement to stand (%dft needed, %dft remaining)", standCost, req.Turn.MovementRemainingFt),
			DestLabel: destLabel,
			MoveMode: MoveModeStandAndMove,
		}, nil
	}

	// Create a modified request with reduced movement after standing
	standReq := req
	standReq.Turn.MovementRemainingFt -= int32(standCost)
	standReq.IsProne = false // moving at normal cost after standing

	result, err := ValidateMove(standReq)
	if err != nil {
		return nil, err
	}

	result.MoveMode = MoveModeStandAndMove
	result.StandCostFt = standCost
	return result, nil
}

// ValidateProneMoveCrawl validates a crawling move for a prone combatant.
// Movement costs double while prone (already handled by pathfinding when IsProne=true).
func ValidateProneMoveCrawl(req MoveRequest) (*MoveResult, error) {
	req.IsProne = true
	result, err := ValidateMove(req)
	if err != nil {
		return nil, err
	}
	result.MoveMode = MoveModeCrawl
	return result, nil
}

// FormatMoveConfirmation produces the ephemeral confirmation message for a move.
func FormatMoveConfirmation(result *MoveResult) string {
	switch result.MoveMode {
	case MoveModeStandAndMove:
		return fmt.Sprintf("\U0001f3c3 Stand & move to %s — %dft stand + %dft move, %dft remaining after.",
			result.DestLabel, result.StandCostFt, result.CostFt, result.RemainingFt)
	case MoveModeCrawl:
		return fmt.Sprintf("\U0001f41b Crawl to %s — %dft, %dft remaining after.",
			result.DestLabel, result.CostFt, result.RemainingFt)
	default:
		msg := fmt.Sprintf("\U0001f3c3 Move to %s — %dft", result.DestLabel, result.CostFt)
		if result.HasDifficultTerrain {
			msg += " (includes difficult terrain)"
		}
		msg += fmt.Sprintf(", %dft remaining after.", result.RemainingFt)
		return msg
	}
}

// ValidateEndTurnPosition checks if the combatant is sharing a tile with another creature.
// Returns an error message if they cannot end their turn there, empty string if OK.
func ValidateEndTurnPosition(combatant refdata.Combatant, allCombatants []refdata.Combatant) string {
	for _, other := range allCombatants {
		if other.ID == combatant.ID || !other.IsAlive {
			continue
		}
		if other.PositionCol == combatant.PositionCol && other.PositionRow == combatant.PositionRow && other.AltitudeFt == combatant.AltitudeFt {
			return fmt.Sprintf("You can't end your turn in another creature's space — use `/move` to leave %s's tile", other.DisplayName)
		}
	}
	return ""
}
