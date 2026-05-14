package discord

import (
	"context"
	"fmt"
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
	// SR-047: now shows interactive Drag / Release & Move buttons instead
	// of a decorative prefix with doubled cost.
	components := sess.lastResponse.Data.Components
	if len(components) == 0 {
		t.Fatal("expected action row with drag choice buttons")
	}
	row := components[0].(discordgo.ActionsRow)
	if len(row.Components) < 2 {
		t.Fatalf("expected at least 2 buttons, got %d", len(row.Components))
	}
	btn0 := row.Components[0].(discordgo.Button)
	if btn0.Label != "Drag" {
		t.Errorf("first button label = %q, want %q", btn0.Label, "Drag")
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

// SR-047: When drag targets exist, /move shows a two-button choice
// (Drag / Release & Move) instead of a decorative prefix.
func TestMoveHandler_Drag_ShowsDragChoiceButtons(t *testing.T) {
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

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "grappling: Goblin") {
		t.Errorf("expected drag prompt naming Goblin, got %q", content)
	}
	// Must have buttons: Drag and Release & Move (not just Confirm/Cancel)
	components := sess.lastResponse.Data.Components
	if len(components) == 0 {
		t.Fatal("expected action row with buttons")
	}
	row, ok := components[0].(discordgo.ActionsRow)
	if !ok {
		t.Fatal("expected ActionsRow")
	}
	if len(row.Components) < 2 {
		t.Fatalf("expected at least 2 buttons, got %d", len(row.Components))
	}
	btn0 := row.Components[0].(discordgo.Button)
	btn1 := row.Components[1].(discordgo.Button)
	if btn0.Label != "Drag" {
		t.Errorf("first button label = %q, want %q", btn0.Label, "Drag")
	}
	if btn1.Label != "Release & Move" {
		t.Errorf("second button label = %q, want %q", btn1.Label, "Release & Move")
	}
	if !strings.HasPrefix(btn0.CustomID, "drag_choice:drag:") {
		t.Errorf("Drag button CustomID = %q, want prefix %q", btn0.CustomID, "drag_choice:drag:")
	}
	if !strings.HasPrefix(btn1.CustomID, "drag_choice:release:") {
		t.Errorf("Release button CustomID = %q, want prefix %q", btn1.CustomID, "drag_choice:release:")
	}
}

// stubMoveDragReleaser implements MoveDragReleaser for tests.
type stubMoveDragReleaser struct {
	calls  int
	result combat.ReleaseDragResult
	err    error
}

func (s *stubMoveDragReleaser) ReleaseDrag(_ context.Context, _ refdata.Combatant, _ []refdata.Combatant) (combat.ReleaseDragResult, error) {
	s.calls++
	return s.result, s.err
}

// SR-047: Clicking "Release & Move" ends the grapple and shows normal-cost confirmation.
func TestMoveHandler_DragChoice_Release_EndsGrappleAndShowsNormalCost(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupMoveHandler(sess)
	stub := &stubMoveDragLookup{
		result: combat.DragCheckResult{
			HasTargets: true,
			GrappledTargets: []refdata.Combatant{
				{ID: uuid.New(), DisplayName: "Goblin"},
			},
		},
	}
	handler.SetDragLookup(stub)

	releaser := &stubMoveDragReleaser{
		result: combat.ReleaseDragResult{CombatLogs: []string{"🤼 Released Goblin"}},
	}
	handler.SetDragReleaser(releaser)

	// Simulate clicking the "Release & Move" button
	// Custom ID format: drag_choice:release:<turnID>:<combatantID>:<destCol>:<destRow>:<baseCostFt>
	customID := fmt.Sprintf("drag_choice:release:%s:%s:%d:%d:%d",
		turnID.String(), combatantID.String(), 3, 0, 15)
	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data:    discordgo.MessageComponentInteractionData{CustomID: customID},
	}

	handler.HandleDragChoice(interaction, "release", turnID, combatantID, 3, 0, 15)

	if releaser.calls != 1 {
		t.Fatalf("expected 1 ReleaseDrag call, got %d", releaser.calls)
	}
	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	// Normal cost (15ft), not doubled
	if !strings.Contains(content, "15ft") {
		t.Errorf("expected normal cost (15ft) after release, got %q", content)
	}
	// Should show Confirm/Cancel buttons for the move
	components := sess.lastResponse.Data.Components
	if len(components) == 0 {
		t.Fatal("expected action row with Confirm/Cancel buttons")
	}
	row := components[0].(discordgo.ActionsRow)
	btn0 := row.Components[0].(discordgo.Button)
	if btn0.Label != "Confirm" {
		t.Errorf("first button label = %q, want %q", btn0.Label, "Confirm")
	}
}

// SR-047: Clicking "Drag" shows doubled-cost confirmation.
func TestMoveHandler_DragChoice_Drag_ShowsDoubledCost(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupMoveHandler(sess)

	// Simulate clicking the "Drag" button
	// Custom ID format: drag_choice:drag:<turnID>:<combatantID>:<destCol>:<destRow>:<baseCostFt>
	handler.HandleDragChoice(nil, "drag", turnID, combatantID, 3, 0, 15)

	// HandleDragChoice for "drag" should show the move confirmation with doubled cost
	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	// Doubled cost: 15 * 2 = 30ft
	if !strings.Contains(content, "30ft") {
		t.Errorf("expected doubled cost (30ft) for drag, got %q", content)
	}
	// Should show Confirm/Cancel buttons
	components := sess.lastResponse.Data.Components
	if len(components) == 0 {
		t.Fatal("expected action row with Confirm/Cancel buttons")
	}
	row := components[0].(discordgo.ActionsRow)
	btn0 := row.Components[0].(discordgo.Button)
	if btn0.Label != "Confirm" {
		t.Errorf("first button label = %q, want %q", btn0.Label, "Confirm")
	}
}
