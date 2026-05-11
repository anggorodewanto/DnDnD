package discord

import (
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// --- D-47: Wild Shape ---

func TestBonusHandler_WildShape_RoutesActivate(t *testing.T) {
	h, _, svc, _ := setupBonusHandler()
	svc.wsActivateResult = combat.WildShapeResult{CombatLog: "🐺 Wild Shape!"}
	h.Handle(makeBonusInteraction("wild-shape", "wolf"))
	if len(svc.wsActivateCalls) != 1 {
		t.Fatalf("expected 1 wild-shape call, got %d", len(svc.wsActivateCalls))
	}
	if svc.wsActivateCalls[0].BeastName != "wolf" {
		t.Errorf("expected beast=wolf, got %q", svc.wsActivateCalls[0].BeastName)
	}
}

func TestBonusHandler_WildShape_MissingBeast(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("wild-shape", ""))
	if len(svc.wsActivateCalls) != 0 {
		t.Error("expected no call without beast name")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Wild Shape requires") {
		t.Errorf("expected usage hint, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_RevertWildShape(t *testing.T) {
	h, _, svc, _ := setupBonusHandler()
	svc.wsRevertResult = combat.RevertWildShapeResult{CombatLog: "🧝 Reverted"}
	h.Handle(makeBonusInteraction("revert-wild-shape", ""))
	if len(svc.wsRevertCalls) != 1 {
		t.Fatalf("expected 1 revert call, got %d", len(svc.wsRevertCalls))
	}
}

// --- D-48b: Flurry of Blows ---

func TestBonusHandler_FlurryOfBlows(t *testing.T) {
	h, _, svc, _ := setupBonusHandler()
	svc.flurryResult = combat.FlurryOfBlowsResult{CombatLog: "👊👊 Flurry"}
	h.Handle(makeBonusInteraction("flurry", "OS"))
	if len(svc.flurryCalls) != 1 {
		t.Fatalf("expected 1 flurry call, got %d", len(svc.flurryCalls))
	}
	if svc.flurryCalls[0].Target.ShortID != "OS" {
		t.Errorf("expected target OS, got %s", svc.flurryCalls[0].Target.ShortID)
	}
}

func TestBonusHandler_FlurryOfBlows_AliasFlurryOfBlows(t *testing.T) {
	h, _, svc, _ := setupBonusHandler()
	svc.flurryResult = combat.FlurryOfBlowsResult{CombatLog: "👊👊 Flurry"}
	h.Handle(makeBonusInteraction("flurry-of-blows", "OS"))
	if len(svc.flurryCalls) != 1 {
		t.Fatalf("expected 1 flurry call via alias, got %d", len(svc.flurryCalls))
	}
}

func TestBonusHandler_FlurryOfBlows_MissingTarget(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("flurry", ""))
	if len(svc.flurryCalls) != 0 {
		t.Error("expected no flurry call without target")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Missing target") {
		t.Errorf("expected missing-target hint, got %q", sess.lastResponse.Data.Content)
	}
}

// --- D-54-cunning-action + D-57-cunning-action-hide ---

func TestBonusHandler_CunningAction_Dash(t *testing.T) {
	h, _, svc, _ := setupBonusHandler()
	svc.cunningResult = combat.CunningActionResult{CombatLog: "⚡ Cunning Dash"}
	h.Handle(makeBonusInteraction("cunning-action", "dash"))
	if len(svc.cunningCalls) != 1 {
		t.Fatalf("expected 1 cunning call, got %d", len(svc.cunningCalls))
	}
	if svc.cunningCalls[0].Action != "dash" {
		t.Errorf("expected action=dash, got %q", svc.cunningCalls[0].Action)
	}
}

func TestBonusHandler_CunningAction_Disengage(t *testing.T) {
	h, _, svc, _ := setupBonusHandler()
	svc.cunningResult = combat.CunningActionResult{CombatLog: "⚡ Cunning Disengage"}
	h.Handle(makeBonusInteraction("cunning-action", "disengage"))
	if len(svc.cunningCalls) != 1 {
		t.Fatalf("expected 1 cunning call, got %d", len(svc.cunningCalls))
	}
	if svc.cunningCalls[0].Action != "disengage" {
		t.Errorf("expected action=disengage, got %q", svc.cunningCalls[0].Action)
	}
}

func TestBonusHandler_CunningAction_Hide(t *testing.T) {
	h, _, svc, _ := setupBonusHandler()
	svc.cunningResult = combat.CunningActionResult{CombatLog: "⚡ Cunning Hide"}
	h.Handle(makeBonusInteraction("cunning-action", "hide"))
	if len(svc.cunningCalls) != 1 {
		t.Fatalf("expected 1 cunning call, got %d", len(svc.cunningCalls))
	}
	if svc.cunningCalls[0].Action != "hide" {
		t.Errorf("expected action=hide, got %q", svc.cunningCalls[0].Action)
	}
}

func TestBonusHandler_CunningAction_BadMode(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	h.Handle(makeBonusInteraction("cunning-action", "fly"))
	if len(svc.cunningCalls) != 0 {
		t.Error("expected no cunning call for bad mode")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Cunning Action requires") {
		t.Errorf("expected mode hint, got %q", sess.lastResponse.Data.Content)
	}
}

// --- D-56: Drag / Release ---

func TestBonusHandler_Drag_NoGrappledTargets(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	svc.dragCheckResult = combat.DragCheckResult{HasTargets: false}
	h.Handle(makeBonusInteraction("drag", ""))
	if svc.dragCheckCalls != 1 {
		t.Fatalf("expected 1 drag-check call, got %d", svc.dragCheckCalls)
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "not grappling anyone") {
		t.Errorf("expected no-targets reply, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_Drag_PromptsWithTargets(t *testing.T) {
	h, sess, svc, prov := setupBonusHandler()
	svc.dragCheckResult = combat.DragCheckResult{HasTargets: true, GrappledTargets: []refdata.Combatant{prov.target}}
	h.Handle(makeBonusInteraction("drag", ""))
	if svc.dragCheckCalls != 1 {
		t.Fatalf("expected 1 drag-check call, got %d", svc.dragCheckCalls)
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Orc") {
		t.Errorf("expected drag prompt naming Orc, got %q", sess.lastResponse.Data.Content)
	}
}

func TestBonusHandler_ReleaseDrag(t *testing.T) {
	h, _, svc, prov := setupBonusHandler()
	svc.dragCheckResult = combat.DragCheckResult{HasTargets: true, GrappledTargets: []refdata.Combatant{prov.target}}
	svc.releaseResult = combat.ReleaseDragResult{CombatLogs: []string{"🤼 Aria releases Orc"}}
	h.Handle(makeBonusInteraction("release-drag", ""))
	if len(svc.releaseCalls) != 1 {
		t.Fatalf("expected 1 release call, got %d", len(svc.releaseCalls))
	}
	if len(svc.releaseCalls[0].Targets) != 1 || svc.releaseCalls[0].Targets[0].DisplayName != "Orc" {
		t.Errorf("expected release targets to include Orc, got %+v", svc.releaseCalls[0].Targets)
	}
}

func TestBonusHandler_ReleaseDrag_NothingToRelease(t *testing.T) {
	h, sess, svc, _ := setupBonusHandler()
	svc.dragCheckResult = combat.DragCheckResult{HasTargets: false}
	h.Handle(makeBonusInteraction("release-drag", ""))
	if len(svc.releaseCalls) != 0 {
		t.Error("expected no release call when nothing grappled")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "nothing to release") {
		t.Errorf("expected nothing-to-release reply, got %q", sess.lastResponse.Data.Content)
	}
}
