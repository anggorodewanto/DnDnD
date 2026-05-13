package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

// open5eSourcePrefix matches the SourcePrefix in internal/open5e. Creatures
// cached from Open5e store prose payloads for actions/abilities (shape
// [{"name":"X","desc":"Y"}]) rather than DnDnD's structured attack array,
// so the combat turn builder has to branch on this prefix before
// unmarshalling.
const open5eSourcePrefix = "open5e:"

// isOpen5eSource reports whether the given refdata creature Source column
// value identifies a row cached from Open5e.
func isOpen5eSource(src sql.NullString) bool {
	return src.Valid && strings.HasPrefix(src.String, open5eSourcePrefix)
}

// OATrigger records a detected opportunity attack trigger.
type OATrigger struct {
	HostileID   uuid.UUID // combatant who can make the OA
	TargetID    uuid.UUID // mover who triggered the OA
	ExitCol     int       // 0-indexed column where target left hostile's reach
	ExitRow     int       // 0-indexed row where target left hostile's reach
	HostileName string    // display name for messaging
	TargetName  string    // display name for messaging
	ExitLabel   string    // human-readable tile label (e.g. "C2")
}

// CreatureAttackEntry represents a single attack in a creature's Attacks JSONB array.
type CreatureAttackEntry struct {
	Name       string `json:"name"`
	ToHit      int    `json:"to_hit"`
	Damage     string `json:"damage"`
	DamageType string `json:"damage_type"`
	ReachFt    int    `json:"reach_ft"`
	RangeFt    int    `json:"range_ft"`
}

// DetectOpportunityAttacks checks whether a moving creature's path triggers any
// opportunity attacks from hostile combatants.
//
// Parameters:
//   - mover: the combatant that is moving
//   - path: the full movement path (0-indexed points), including the start tile
//   - allCombatants: all combatants in the encounter
//   - moverTurn: the mover's current turn (checked for HasDisengaged)
//   - hostileTurns: map of hostile combatant ID → their turn this round (for reaction_used check)
//   - creatureAttacks: map of creature_ref_id → parsed attacks (for NPC reach lookup); nil is OK
//
// Returns a slice of OATrigger for each hostile whose reach the mover exits.
func DetectOpportunityAttacks(
	mover refdata.Combatant,
	path []pathfinding.Point,
	allCombatants []refdata.Combatant,
	moverTurn refdata.Turn,
	hostileTurns map[uuid.UUID]refdata.Turn,
	creatureAttacks map[string][]CreatureAttackEntry,
) []OATrigger {
	return DetectOpportunityAttacksWithReach(mover, path, allCombatants, moverTurn, hostileTurns, creatureAttacks, nil)
}

// DetectOpportunityAttacksWithReach is the same as DetectOpportunityAttacks but
// accepts a per-hostile-id reach override (in feet). Used by /move (med-24) so
// PC hostiles holding reach weapons (glaive, halberd, lance, pike, whip) get
// 10ft reach instead of the default 5ft. NPC reach still flows through
// creatureAttacks as before; an explicit override in pcReachByID always wins.
func DetectOpportunityAttacksWithReach(
	mover refdata.Combatant,
	path []pathfinding.Point,
	allCombatants []refdata.Combatant,
	moverTurn refdata.Turn,
	hostileTurns map[uuid.UUID]refdata.Turn,
	creatureAttacks map[string][]CreatureAttackEntry,
	pcReachByID map[uuid.UUID]int,
) []OATrigger {
	if moverTurn.HasDisengaged {
		return nil
	}
	if len(path) < 2 {
		return nil
	}

	var triggers []OATrigger

	for _, hostile := range allCombatants {
		if hostile.ID == mover.ID {
			continue
		}
		if !hostile.IsAlive {
			continue
		}
		// Must be a different faction (NPC vs PC)
		if hostile.IsNpc == mover.IsNpc {
			continue
		}
		// Check if hostile already used reaction
		if ht, ok := hostileTurns[hostile.ID]; ok && ht.ReactionUsed {
			continue
		}

		reachFt := resolveHostileReach(hostile, creatureAttacks)
		if override, ok := pcReachByID[hostile.ID]; ok && override > 0 {
			reachFt = override
		}
		reachSquares := reachFt / 5

		// Get hostile position in 0-indexed
		hostileCol, hostileRow := combatantGridPos(hostile)
		if hostileCol < 0 {
			continue
		}

		// Check if path exits hostile's reach
		exitCol, exitRow, exits := findReachExit(path, hostileCol, hostileRow, reachSquares)
		if !exits {
			continue
		}

		exitLabel := renderer.ColumnLabel(exitCol) + strconv.Itoa(exitRow+1)

		triggers = append(triggers, OATrigger{
			HostileID:   hostile.ID,
			TargetID:    mover.ID,
			ExitCol:     exitCol,
			ExitRow:     exitRow,
			HostileName: hostile.DisplayName,
			TargetName:  mover.DisplayName,
			ExitLabel:   exitLabel,
		})
	}

	return triggers
}

// resolveHostileReach determines the melee reach of a hostile combatant in feet.
// For NPCs with creature attacks, uses the max reach_ft found. Default is 5ft.
func resolveHostileReach(hostile refdata.Combatant, creatureAttacks map[string][]CreatureAttackEntry) int {
	// For NPCs, check creature attacks
	if hostile.IsNpc && hostile.CreatureRefID.Valid && creatureAttacks != nil {
		if attacks, ok := creatureAttacks[hostile.CreatureRefID.String]; ok {
			maxReach := 5
			for _, atk := range attacks {
				if atk.ReachFt > maxReach {
					maxReach = atk.ReachFt
				}
			}
			return maxReach
		}
	}
	return 5
}

// combatantGridPos returns the 0-indexed grid position of a combatant.
// Returns (-1, -1) if position cannot be parsed.
func combatantGridPos(c refdata.Combatant) (col, row int) {
	coord := c.PositionCol + strconv.Itoa(int(c.PositionRow))
	col, row, err := renderer.ParseCoordinate(coord)
	if err != nil {
		return -1, -1
	}
	return col, row
}

// findReachExit checks if the path exits a hostile's reach area.
// Returns the exit tile (last tile still in reach) and true if the mover leaves reach.
// The path must start within reach for an OA to trigger.
func findReachExit(path []pathfinding.Point, hostileCol, hostileRow, reachSquares int) (exitCol, exitRow int, exits bool) {
	if len(path) < 2 {
		return 0, 0, false
	}

	// Check if mover starts in reach
	startDist := chebyshevDist(path[0].Col, path[0].Row, hostileCol, hostileRow)
	if startDist > reachSquares {
		return 0, 0, false
	}

	// Walk the path; find the first step that leaves reach
	for i := 1; i < len(path); i++ {
		dist := chebyshevDist(path[i].Col, path[i].Row, hostileCol, hostileRow)
		if dist > reachSquares {
			// Previous tile was the exit tile (last in reach)
			return path[i-1].Col, path[i-1].Row, true
		}
	}

	return 0, 0, false
}

// chebyshevDist returns the Chebyshev distance between two points.
func chebyshevDist(col1, row1, col2, row2 int) int {
	dc := col1 - col2
	dr := row1 - row2
	return max(abs(dc), abs(dr))
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// FormatOAPrompt formats the opportunity attack prompt message for a player.
func FormatOAPrompt(trigger OATrigger) string {
	return fmt.Sprintf("⚔️ %s moved out of your reach (left %s) — use your reaction for an opportunity attack? `/reaction oa %s`",
		trigger.TargetName, trigger.ExitLabel, trigger.TargetName)
}

// ParseCreatureAttacks parses a creature's Attacks JSONB into a slice of CreatureAttackEntry.
func ParseCreatureAttacks(attacksJSON json.RawMessage) ([]CreatureAttackEntry, error) {
	if len(attacksJSON) == 0 {
		return nil, nil
	}
	var attacks []CreatureAttackEntry
	if err := json.Unmarshal(attacksJSON, &attacks); err != nil {
		return nil, fmt.Errorf("parsing creature attacks: %w", err)
	}
	return attacks, nil
}

// RecordPendingOA appends a dm-queue itemID to the per-encounter pending-OA
// tracker so the round-advance forfeiture sweep can cancel it later. Called by
// the move handler after a successful Post to #dm-queue for a DM-controlled
// hostile's opportunity attack (SR-028). Empty item IDs are silently ignored
// (Notifier.Post returns "" when no #dm-queue is configured for the guild).
func (s *Service) RecordPendingOA(encounterID uuid.UUID, itemID string) {
	if itemID == "" {
		return
	}
	s.pendingOAsMu.Lock()
	defer s.pendingOAsMu.Unlock()
	s.pendingOAsByEncounter[encounterID] = append(s.pendingOAsByEncounter[encounterID], itemID)
}

// ForfeitPendingOAs drains the per-encounter pending-OA tracker and cancels
// each remaining dm-queue prompt via the wired DM notifier with a "forfeited
// at end of round" reason. Called from advanceRound so a DM-controlled
// hostile's unanswered OA visibly resolves (strikethrough) instead of
// silently rotting in #dm-queue. Best-effort: a nil notifier or a per-item
// Cancel error is swallowed (the in-memory slice is still drained so a flaky
// Notifier never causes the same item to be retried indefinitely).
func (s *Service) ForfeitPendingOAs(ctx context.Context, encounterID uuid.UUID) {
	s.pendingOAsMu.Lock()
	pending := s.pendingOAsByEncounter[encounterID]
	delete(s.pendingOAsByEncounter, encounterID)
	s.pendingOAsMu.Unlock()
	if len(pending) == 0 {
		return
	}
	if s.dmNotifier == nil {
		return
	}
	for _, itemID := range pending {
		_ = s.dmNotifier.Cancel(ctx, itemID, "forfeited at end of round")
	}
}

// ParseCreatureAttacksWithSource parses a creature's Attacks JSONB into
// a slice of CreatureAttackEntry. For rows whose source begins with
// "open5e:" the attacks column holds Open5e prose (shape [{name, desc}])
// that has no mechanical to_hit/damage fields, so we return no structured
// attacks — callers should surface the prose as abilities instead.
func ParseCreatureAttacksWithSource(attacksJSON json.RawMessage, source sql.NullString) ([]CreatureAttackEntry, error) {
	if isOpen5eSource(source) {
		return nil, nil
	}
	return ParseCreatureAttacks(attacksJSON)
}
