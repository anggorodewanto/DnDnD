package discord

import (
	"context"
	"strings"
	"testing"

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
