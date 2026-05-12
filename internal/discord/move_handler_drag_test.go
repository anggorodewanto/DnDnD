package discord

import (
	"context"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// stubMoveDragLookup is the smallest possible MoveDragLookup. The test
// controls the canned drag-check result and records how many times it was
// invoked so we can assert /move did consult the lookup.
type stubMoveDragLookup struct {
	calls  int
	result combat.DragCheckResult
	err    error
}

func (s *stubMoveDragLookup) CheckDragTargets(_ context.Context, _ uuid.UUID, _ refdata.Combatant) (combat.DragCheckResult, error) {
	s.calls++
	return s.result, s.err
}

func TestMoveHandler_Drag_AppendsPromptAndDoublesCost(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
	stub := &stubMoveDragLookup{
		result: combat.DragCheckResult{
			HasTargets: true,
			GrappledTargets: []refdata.Combatant{
				{ID: uuid.New(), DisplayName: "Goblin"},
			},
		},
	}
	handler.SetDragLookup(stub)

	handler.Handle(makeMoveInteraction("D1"))

	if stub.calls != 1 {
		t.Fatalf("expected 1 drag check call, got %d", stub.calls)
	}
	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "grappling: Goblin") {
		t.Errorf("expected drag prompt naming Goblin, got %q", content)
	}
	// Default cost was 15ft; with drag-cost x2 we expect 30ft.
	if !strings.Contains(content, "30ft") {
		t.Errorf("expected doubled cost (30ft) in confirmation, got %q", content)
	}
}

func TestMoveHandler_Drag_NoTargets_NoPrompt(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
	stub := &stubMoveDragLookup{
		result: combat.DragCheckResult{HasTargets: false},
	}
	handler.SetDragLookup(stub)

	handler.Handle(makeMoveInteraction("D1"))

	if stub.calls != 1 {
		t.Fatalf("expected 1 drag check call, got %d", stub.calls)
	}
	content := sess.lastResponse.Data.Content
	if strings.Contains(content, "grappling:") {
		t.Errorf("expected no drag prompt when nothing grappled, got %q", content)
	}
	// Default cost is 15ft and should not be doubled.
	if !strings.Contains(content, "15ft") {
		t.Errorf("expected un-doubled cost (15ft), got %q", content)
	}
}

// D-56-followup-drag-tile-sync: when a grappling combatant moves across a
// multi-tile path, the grappled target's position must be persisted along
// the path (always within 5ft of the dragger) — not just at the endpoint.
// The handler invokes UpdateCombatantPosition for the dragged target
// alongside the dragger's own position update.
func TestMoveHandler_Drag_MultiTilePath_SyncsTargetEachStep(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupMoveHandler(sess)
	targetID := uuid.New()

	// Stub drag lookup announces a single grappled target.
	stub := &stubMoveDragLookup{
		result: combat.DragCheckResult{
			HasTargets: true,
			GrappledTargets: []refdata.Combatant{
				{ID: targetID, DisplayName: "Goblin", PositionCol: "A", PositionRow: 1, IsAlive: true, IsNpc: true},
			},
		},
	}
	handler.SetDragLookup(stub)

	// Capture every UpdateCombatantPosition call so we can verify that the
	// dragged target was synced along the dragger's path.
	type positionUpdate struct {
		id  uuid.UUID
		col string
		row int32
	}
	var updates []positionUpdate
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{}, nil
		},
		getCombatant: func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
			if id == combatantID {
				return refdata.Combatant{ID: combatantID, PositionCol: "A", PositionRow: 1}, nil
			}
			return refdata.Combatant{ID: targetID, PositionCol: "A", PositionRow: 1}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return nil, nil
		},
		updateCombatantPos: func(_ context.Context, id uuid.UUID, col string, row, _ int32) (refdata.Combatant, error) {
			updates = append(updates, positionUpdate{id: id, col: col, row: row})
			return refdata.Combatant{}, nil
		},
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}

	// Dragger moves from A1 to D1 (3 tiles east). Cost is doubled (30ft).
	handler.HandleMoveConfirm(interaction, turnID, combatantID, 3, 0, 30)

	// Dragger must land on D1.
	var draggerUpdate *positionUpdate
	for i := range updates {
		if updates[i].id == combatantID {
			draggerUpdate = &updates[i]
		}
	}
	if draggerUpdate == nil {
		t.Fatalf("expected dragger position update, got %+v", updates)
	}
	if draggerUpdate.col != "D" || draggerUpdate.row != 1 {
		t.Errorf("dragger landed on %s%d, want D1", draggerUpdate.col, draggerUpdate.row)
	}

	// The grappled target must have at least one position update so its
	// tile stays within 5ft of the dragger after the path completes. The
	// final target position must be adjacent (Chebyshev <= 1) to the
	// dragger's endpoint.
	var targetUpdate *positionUpdate
	for i := range updates {
		if updates[i].id == targetID {
			targetUpdate = &updates[i]
		}
	}
	if targetUpdate == nil {
		t.Fatalf("expected drag target position update, got %+v", updates)
	}
	// Dragger ends at D1 (col=3). Target must be within 1 col of that
	// (Chebyshev), so column C, D, or E and row 0..2 are all acceptable.
	colDiff := int(targetUpdate.col[0]) - int('D')
	if colDiff < 0 {
		colDiff = -colDiff
	}
	rowDiff := int(targetUpdate.row) - 1
	if rowDiff < 0 {
		rowDiff = -rowDiff
	}
	if colDiff > 1 || rowDiff > 1 {
		t.Errorf("drag target final position %s%d is not adjacent to dragger D1 (Chebyshev > 1)",
			targetUpdate.col, targetUpdate.row)
	}
}
