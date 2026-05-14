package discord

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

var errStub = errors.New("stub lookup error")

// dispatchActionFixture wires a ready-to-go ActionHandler with the canonical
// "owned by invoker" combat encounter so every subcommand test only has to
// provide a `/action <sub> args:<args>` interaction.
type dispatchActionFixture struct {
	handler     *ActionHandler
	svc         *fakeActionCombatService
	sess        *MockSession
	encounterID uuid.UUID
	turnID      uuid.UUID
	combatantID uuid.UUID
	charID      uuid.UUID
	campID      uuid.UUID
	other       refdata.Combatant
}

func setupDispatchActionFixture(t *testing.T) dispatchActionFixture {
	t.Helper()
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	otherID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	actor := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Aria",
		ShortID:     "AR",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
		IsNpc:       false,
		PositionCol: "B",
		PositionRow: 2,
	}
	target := refdata.Combatant{
		ID:          otherID,
		EncounterID: encounterID,
		DisplayName: "Orc",
		ShortID:     "OS",
		IsAlive:     true,
		IsNpc:       true,
		PositionCol: "B",
		PositionRow: 3,
		HpMax:       20,
		HpCurrent:   5,
	}
	turn := refdata.Turn{
		ID:          turnID,
		EncounterID: encounterID,
		CombatantID: combatantID,
	}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		Status:        "active",
		RoundNumber:   3,
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}
	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: actor, otherID: target},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {actor, target}},
	}
	turnProv := &fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}}
	sess := &MockSession{}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		turnProv,
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)
	h.SetRoller(dice.NewRoller(func(int) int { return 10 }))
	return dispatchActionFixture{
		handler:     h,
		svc:         svc,
		sess:        sess,
		encounterID: encounterID,
		turnID:      turnID,
		combatantID: combatantID,
		charID:      charID,
		campID:      campID,
		other:       target,
	}
}

// dispatchInvoke runs the handler with a recorded response and returns the
// last response body.
func dispatchInvoke(t *testing.T, h *ActionHandler, sess *MockSession, sub, args string) string {
	t.Helper()
	return runActionHandler(t, h, makeActionInteraction("g1", "u1", sub, args))
}

// --- D-53: Action Surge ---

func TestActionHandler_Dispatch_ActionSurge(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.svc.surgeResult = combat.ActionSurgeResult{CombatLog: "⚡ Aria uses Action Surge!"}
	resp := dispatchInvoke(t, fx.handler, fx.sess, "surge", "")
	if len(fx.svc.surgeCalls) != 1 {
		t.Fatalf("expected 1 surge call, got %d", len(fx.svc.surgeCalls))
	}
	if !strings.Contains(resp, "Action Surge") {
		t.Errorf("expected Action Surge log, got %q", resp)
	}
}

// --- D-54: Standard actions ---

func TestActionHandler_Dispatch_Dash(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.svc.dashResult = combat.DashResult{CombatLog: "🏃 Dash!"}
	_ = dispatchInvoke(t, fx.handler, fx.sess, "dash", "")
	if len(fx.svc.dashCalls) != 1 {
		t.Fatalf("expected 1 dash call, got %d", len(fx.svc.dashCalls))
	}
}

func TestActionHandler_Dispatch_Disengage(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.svc.disengageResult = combat.DisengageResult{CombatLog: "🚫 Disengage"}
	_ = dispatchInvoke(t, fx.handler, fx.sess, "disengage", "")
	if len(fx.svc.disengageCalls) != 1 {
		t.Fatalf("expected 1 disengage call, got %d", len(fx.svc.disengageCalls))
	}
}

func TestActionHandler_Dispatch_Dodge_PassesRound(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.svc.dodgeResult = combat.DodgeResult{CombatLog: "🛡️ Dodge"}
	_ = dispatchInvoke(t, fx.handler, fx.sess, "dodge", "")
	if len(fx.svc.dodgeCalls) != 1 {
		t.Fatalf("expected 1 dodge call, got %d", len(fx.svc.dodgeCalls))
	}
	if fx.svc.dodgeCalls[0].CurrentRound != 3 {
		t.Errorf("expected CurrentRound=3, got %d", fx.svc.dodgeCalls[0].CurrentRound)
	}
}

func TestActionHandler_Dispatch_Help_RoutesAllyAndTarget(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.svc.helpResult = combat.HelpResult{CombatLog: "🤝 Help"}
	_ = dispatchInvoke(t, fx.handler, fx.sess, "help", "OS")
	if len(fx.svc.helpCalls) != 1 {
		t.Fatalf("expected 1 help call, got %d", len(fx.svc.helpCalls))
	}
	got := fx.svc.helpCalls[0]
	if got.Ally.ShortID != "OS" {
		t.Errorf("expected ally OS, got %s", got.Ally.ShortID)
	}
	if got.Target.ShortID != "OS" {
		t.Errorf("expected target OS (defaulted from single arg), got %s", got.Target.ShortID)
	}
}

func TestActionHandler_Dispatch_Help_MissingArgs(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	resp := dispatchInvoke(t, fx.handler, fx.sess, "help", "")
	if len(fx.svc.helpCalls) != 0 {
		t.Errorf("expected no help call for missing args, got %d", len(fx.svc.helpCalls))
	}
	if !strings.Contains(resp, "Help requires") {
		t.Errorf("expected usage hint, got %q", resp)
	}
}

func TestActionHandler_Dispatch_Hide_PassesHostiles(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.svc.hideResult = combat.HideResult{CombatLog: "🙈 Hide", Success: true}
	_ = dispatchInvoke(t, fx.handler, fx.sess, "hide", "")
	if len(fx.svc.hideCalls) != 1 {
		t.Fatalf("expected 1 hide call, got %d", len(fx.svc.hideCalls))
	}
	if len(fx.svc.hideCalls[0].Hostiles) != 1 || fx.svc.hideCalls[0].Hostiles[0].ShortID != "OS" {
		t.Errorf("expected single hostile OS, got %+v", fx.svc.hideCalls[0].Hostiles)
	}
}

func TestActionHandler_Dispatch_Hide_NoRoller_Rejects(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.handler.roller = nil
	resp := dispatchInvoke(t, fx.handler, fx.sess, "hide", "")
	if len(fx.svc.hideCalls) != 0 {
		t.Errorf("expected no hide call without roller")
	}
	if !strings.Contains(resp, "Hide is not available") {
		t.Errorf("expected no-roller rejection, got %q", resp)
	}
}

func TestActionHandler_Dispatch_Stand(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.svc.standResult = combat.StandResult{CombatLog: "🧍 Stand"}
	_ = dispatchInvoke(t, fx.handler, fx.sess, "stand", "")
	if len(fx.svc.standCalls) != 1 {
		t.Fatalf("expected 1 stand call, got %d", len(fx.svc.standCalls))
	}
}

// fakeActionSpeedLookup is a tiny stub satisfying ActionSpeedLookup.
type fakeActionSpeedLookup struct {
	speed int
	err   error
}

func (f *fakeActionSpeedLookup) LookupWalkSpeed(_ context.Context, _ refdata.Combatant) (int, error) {
	return f.speed, f.err
}

// D-54-followup: without a wired speed lookup, /action stand defaults to
// MaxSpeed=30. (Existing behaviour locked in to detect regressions.)
func TestActionHandler_Dispatch_Stand_NoLookupFallsBackTo30(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.svc.standResult = combat.StandResult{CombatLog: "🧍 Stand"}
	_ = dispatchInvoke(t, fx.handler, fx.sess, "stand", "")
	if len(fx.svc.standCalls) != 1 {
		t.Fatalf("expected 1 stand call, got %d", len(fx.svc.standCalls))
	}
	if fx.svc.standCalls[0].MaxSpeed != 30 {
		t.Errorf("MaxSpeed=%d, want 30 fallback", fx.svc.standCalls[0].MaxSpeed)
	}
}

// D-54-followup: when a speed lookup returns 25 (Halfling), /action stand
// must pass MaxSpeed=25 to combat.StandCommand so the service derives a
// 13ft stand cost (round-up of 25/2) rather than the hardcoded 15ft.
func TestActionHandler_Dispatch_Stand_UsesLookupSpeed_Halfling(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.handler.SetSpeedLookup(&fakeActionSpeedLookup{speed: 25})
	fx.svc.standResult = combat.StandResult{CombatLog: "🧍 Stand"}
	_ = dispatchInvoke(t, fx.handler, fx.sess, "stand", "")
	if len(fx.svc.standCalls) != 1 {
		t.Fatalf("expected 1 stand call, got %d", len(fx.svc.standCalls))
	}
	if fx.svc.standCalls[0].MaxSpeed != 25 {
		t.Errorf("MaxSpeed=%d, want 25 (Halfling)", fx.svc.standCalls[0].MaxSpeed)
	}
}

// D-54-followup: a lookup error or non-positive speed degrades to 30ft so
// /action stand never breaks because of a flaky lookup.
func TestActionHandler_Dispatch_Stand_LookupErrorFallsBack(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.handler.SetSpeedLookup(&fakeActionSpeedLookup{err: errStub})
	fx.svc.standResult = combat.StandResult{CombatLog: "🧍 Stand"}
	_ = dispatchInvoke(t, fx.handler, fx.sess, "stand", "")
	if len(fx.svc.standCalls) != 1 {
		t.Fatalf("expected 1 stand call, got %d", len(fx.svc.standCalls))
	}
	if fx.svc.standCalls[0].MaxSpeed != 30 {
		t.Errorf("MaxSpeed=%d, want 30 fallback on lookup error", fx.svc.standCalls[0].MaxSpeed)
	}
}

func TestActionHandler_Dispatch_DropProne(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.svc.dropProneResult = combat.DropProneResult{CombatLog: "⬇️ Drop Prone"}
	_ = dispatchInvoke(t, fx.handler, fx.sess, "drop-prone", "")
	if len(fx.svc.dropProneCalls) != 1 {
		t.Fatalf("expected 1 drop-prone call, got %d", len(fx.svc.dropProneCalls))
	}
}

func TestActionHandler_Dispatch_Escape_WiresGrappler(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	// Add a "grappled" condition to the actor sourced from the Orc.
	grappledCond := combat.CombatCondition{
		Condition:         "grappled",
		SourceCombatantID: fx.other.ID.String(),
	}
	condJSON, _ := json.Marshal([]combat.CombatCondition{grappledCond})
	actor := fx.svc.combatants[fx.combatantID]
	actor.Conditions = condJSON
	fx.svc.combatants[fx.combatantID] = actor
	fx.svc.byEncounter[fx.encounterID] = []refdata.Combatant{actor, fx.other}

	fx.svc.escapeResult = combat.EscapeResult{CombatLog: "💪 Escape", Success: true}
	_ = dispatchInvoke(t, fx.handler, fx.sess, "escape", "")
	if len(fx.svc.escapeCalls) != 1 {
		t.Fatalf("expected 1 escape call, got %d", len(fx.svc.escapeCalls))
	}
	if fx.svc.escapeCalls[0].Grappler.ID != fx.other.ID {
		t.Errorf("expected grappler %s, got %s", fx.other.ID, fx.svc.escapeCalls[0].Grappler.ID)
	}
}

func TestActionHandler_Dispatch_Escape_NotGrappled(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	resp := dispatchInvoke(t, fx.handler, fx.sess, "escape", "")
	if len(fx.svc.escapeCalls) != 0 {
		t.Errorf("expected no escape call when not grappled")
	}
	if !strings.Contains(resp, "not grappled") {
		t.Errorf("expected not-grappled message, got %q", resp)
	}
}

// --- D-50: Channel Divinity ---

func TestActionHandler_Dispatch_ChannelDivinity_TurnUndead(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.svc.turnUndeadResult = combat.TurnUndeadResult{CombatLog: "✝️ Turn Undead"}
	_ = dispatchInvoke(t, fx.handler, fx.sess, "channel-divinity", "turn-undead")
	if len(fx.svc.turnUndeadCalls) != 1 {
		t.Fatalf("expected 1 turn-undead call, got %d", len(fx.svc.turnUndeadCalls))
	}
}

func TestActionHandler_Dispatch_ChannelDivinity_SacredWeapon(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.svc.sacredWeaponResult = combat.SacredWeaponResult{CombatLog: "✝️ Sacred Weapon"}
	_ = dispatchInvoke(t, fx.handler, fx.sess, "channel-divinity", "sacred-weapon")
	if len(fx.svc.sacredWeaponCalls) != 1 {
		t.Fatalf("expected 1 sacred-weapon call, got %d", len(fx.svc.sacredWeaponCalls))
	}
}

func TestActionHandler_Dispatch_ChannelDivinity_VowOfEnmity(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.svc.vowResult = combat.VowOfEnmityResult{CombatLog: "✝️ Vow"}
	_ = dispatchInvoke(t, fx.handler, fx.sess, "channel-divinity", "vow-of-enmity OS")
	if len(fx.svc.vowCalls) != 1 {
		t.Fatalf("expected 1 vow call, got %d", len(fx.svc.vowCalls))
	}
	if fx.svc.vowCalls[0].Target.ShortID != "OS" {
		t.Errorf("expected target OS, got %s", fx.svc.vowCalls[0].Target.ShortID)
	}
}

func TestActionHandler_Dispatch_ChannelDivinity_PreserveLife(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.svc.preserveLifeResult = combat.PreserveLifeResult{CombatLog: "✝️ Preserve Life"}
	_ = dispatchInvoke(t, fx.handler, fx.sess, "channel-divinity", "preserve-life OS:3")
	if len(fx.svc.preserveLifeCalls) != 1 {
		t.Fatalf("expected 1 preserve-life call, got %d", len(fx.svc.preserveLifeCalls))
	}
	if got := fx.svc.preserveLifeCalls[0].TargetHealing[fx.other.ID.String()]; got != 3 {
		t.Errorf("expected healing=3 for OS, got %d", got)
	}
}

func TestActionHandler_Dispatch_ChannelDivinity_DMQueueFallback(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.svc.cdDMQueueResult = combat.DMQueueResult{CombatLog: "✝️ DM-queued"}
	_ = dispatchInvoke(t, fx.handler, fx.sess, "channel-divinity", "harness-divinity")
	if len(fx.svc.cdDMQueueCalls) != 1 {
		t.Fatalf("expected 1 dm-queue call, got %d", len(fx.svc.cdDMQueueCalls))
	}
}

func TestActionHandler_Dispatch_ChannelDivinity_MissingOption(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	resp := dispatchInvoke(t, fx.handler, fx.sess, "channel-divinity", "")
	if !strings.Contains(resp, "Channel Divinity requires") {
		t.Errorf("expected usage hint, got %q", resp)
	}
}

// --- D-52: Lay on Hands action surface ---

func TestActionHandler_Dispatch_LayOnHands(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.svc.layResult = combat.LayOnHandsResult{CombatLog: "💛 Lay on Hands"}
	_ = dispatchInvoke(t, fx.handler, fx.sess, "lay-on-hands", "OS 10 poison")
	if len(fx.svc.layCalls) != 1 {
		t.Fatalf("expected 1 lay-on-hands call, got %d", len(fx.svc.layCalls))
	}
	got := fx.svc.layCalls[0]
	if got.HP != 10 {
		t.Errorf("HP=%d want 10", got.HP)
	}
	if !got.CurePoison {
		t.Error("expected CurePoison=true")
	}
}

func TestActionHandler_Dispatch_LayOnHands_BadArgs(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	resp := dispatchInvoke(t, fx.handler, fx.sess, "lay-on-hands", "OS")
	if len(fx.svc.layCalls) != 0 {
		t.Error("expected no lay-on-hands call with bad args")
	}
	if !strings.Contains(resp, "Lay on Hands requires") {
		t.Errorf("expected usage hint, got %q", resp)
	}
}

// --- Unknown subcommand still routes through freeform ---

func TestActionHandler_Dispatch_UnknownFallsThroughToFreeform(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	_ = dispatchInvoke(t, fx.handler, fx.sess, "flip the table", "")
	if fx.svc.freeformCalledWith.ActionText != "flip the table" {
		t.Errorf("expected freeform fallback, got ActionText=%q", fx.svc.freeformCalledWith.ActionText)
	}
}

// --- Combat-log mirroring ---

func TestActionHandler_Dispatch_MirrorsToCombatLog(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	captured := []string{}
	fx.handler.session = &capturingSession{
		mockMoveSession: &mockMoveSession{},
		sendFn: func(channelID, content string) {
			captured = append(captured, channelID+":"+content)
		},
	}
	fx.handler.SetChannelIDProvider(&mockDeathSaveCSP{
		fn: func(_ context.Context, _ uuid.UUID) (map[string]string, error) {
			return map[string]string{"combat-log": "ch-cl"}, nil
		},
	})
	fx.svc.surgeResult = combat.ActionSurgeResult{CombatLog: "⚡ surge!"}
	_ = makeActionInteraction("g1", "u1", "surge", "")
	fx.handler.Handle(makeActionInteraction("g1", "u1", "surge", ""))
	if len(captured) != 1 || !strings.HasPrefix(captured[0], "ch-cl:") {
		t.Errorf("expected combat-log mirror to ch-cl, got %v", captured)
	}
}

// --- Wrong-owner rejection still applies on subcommand dispatch ---

func TestActionHandler_Dispatch_WrongOwner_Rejects(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	// Re-wire the handler to a fake whose invoker character ID differs.
	fx.handler = NewActionHandler(
		fx.sess,
		&fakeActionEncounterResolver{encounterID: fx.encounterID},
		fx.svc,
		&fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{fx.turnID: {
			ID:          fx.turnID,
			EncounterID: fx.encounterID,
			CombatantID: fx.combatantID,
		}}},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: fx.campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: uuid.New()}},
		&fakeActionPendingStore{},
	)
	resp := runActionHandler(t, fx.handler, &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "g1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "u1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "action",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: "action", Type: discordgo.ApplicationCommandOptionString, Value: "surge"},
			},
		},
	})
	if len(fx.svc.surgeCalls) != 0 {
		t.Error("expected no surge call when invoker doesn't own turn")
	}
	if resp != "It's not your turn." {
		t.Errorf("expected wrong-owner rejection, got %q", resp)
	}
}

// --- E-69: Hide gating via ObscurementAllowsHide ---

type fakeActionZoneLookup struct {
	zones []combat.ZoneInfo
}

func (f *fakeActionZoneLookup) ListZonesForEncounter(_ context.Context, _ uuid.UUID) ([]combat.ZoneInfo, error) {
	return f.zones, nil
}

// E-69: when zones are wired and the actor's tile has no obscurement
// (NotObscured), Hide is blocked. ObscurementAllowsHide(NotObscured) == false.
func TestActionHandler_Dispatch_Hide_BlockedWhenNotObscured(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	// One zone exists in the encounter but it does NOT cover the actor's tile
	// (actor is at B2; zone covers F6). Actor obscurement is NotObscured.
	dims, _ := json.Marshal(map[string]int{"side_ft": 5})
	fx.handler.SetZoneLookup(&fakeActionZoneLookup{zones: []combat.ZoneInfo{{
		ID:          uuid.New(),
		EncounterID: fx.encounterID,
		Shape:       "square",
		OriginCol:   "F",
		OriginRow:   6,
		Dimensions:  dims,
		ZoneType:    "heavy_obscurement",
	}}})

	resp := dispatchInvoke(t, fx.handler, fx.sess, "hide", "")
	if len(fx.svc.hideCalls) != 0 {
		t.Errorf("expected no hide call when not obscured, got %d", len(fx.svc.hideCalls))
	}
	if !strings.Contains(strings.ToLower(resp), "obscured") {
		t.Errorf("expected obscurement-required rejection, got %q", resp)
	}
}

// E-69: when zones are wired and the actor stands in a lightly/heavily
// obscured tile, Hide proceeds and the obscurement reason is logged.
func TestActionHandler_Dispatch_Hide_AllowedWhenObscured(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	dims, _ := json.Marshal(map[string]int{"side_ft": 5})
	// Zone covers actor tile (B2 = col 1, row 1).
	fx.handler.SetZoneLookup(&fakeActionZoneLookup{zones: []combat.ZoneInfo{{
		ID:          uuid.New(),
		EncounterID: fx.encounterID,
		Shape:       "square",
		OriginCol:   "B",
		OriginRow:   2,
		Dimensions:  dims,
		ZoneType:    "heavy_obscurement",
	}}})

	fx.svc.hideResult = combat.HideResult{CombatLog: "🙈 Aria attempts to Hide — Stealth: 18", Success: true}
	resp := dispatchInvoke(t, fx.handler, fx.sess, "hide", "")
	if len(fx.svc.hideCalls) != 1 {
		t.Fatalf("expected 1 hide call when obscured, got %d", len(fx.svc.hideCalls))
	}
	if !strings.Contains(strings.ToLower(resp), "obscured") {
		t.Errorf("expected obscurement reason in response, got %q", resp)
	}
}

// --- SR-048: /action grapple alias ---

func TestActionHandler_Dispatch_Grapple(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.svc.grappleResult = combat.GrappleResult{CombatLog: "🤼 Aria grapples Orc — Grappled!", Success: true}
	resp := dispatchInvoke(t, fx.handler, fx.sess, "grapple", "OS")
	if len(fx.svc.grappleCalls) != 1 {
		t.Fatalf("expected 1 grapple call, got %d", len(fx.svc.grappleCalls))
	}
	got := fx.svc.grappleCalls[0]
	if got.Target.ShortID != "OS" {
		t.Errorf("expected target OS, got %s", got.Target.ShortID)
	}
	if got.Grappler.ShortID != "AR" {
		t.Errorf("expected grappler AR, got %s", got.Grappler.ShortID)
	}
	if !strings.Contains(resp, "Grappled") {
		t.Errorf("expected grapple log in response, got %q", resp)
	}
}

func TestActionHandler_Dispatch_Grapple_MissingTarget(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	resp := dispatchInvoke(t, fx.handler, fx.sess, "grapple", "")
	if len(fx.svc.grappleCalls) != 0 {
		t.Errorf("expected no grapple call for missing target, got %d", len(fx.svc.grappleCalls))
	}
	if !strings.Contains(resp, "Grapple requires a target") {
		t.Errorf("expected usage hint, got %q", resp)
	}
}

func TestActionHandler_Dispatch_Grapple_ServiceError(t *testing.T) {
	fx := setupDispatchActionFixture(t)
	fx.svc.grappleErr = errors.New("target is too large to grapple")
	resp := dispatchInvoke(t, fx.handler, fx.sess, "grapple", "OS")
	if !strings.Contains(resp, "Grapple failed") {
		t.Errorf("expected failure message, got %q", resp)
	}
}
