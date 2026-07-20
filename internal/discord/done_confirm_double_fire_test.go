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

// doneConfirmFixture wires a DoneHandler whose encounter row carries a *mutable*
// current-turn pointer: AdvanceTurn repoints it at a brand-new turn, exactly as
// the real service does. That makes a second click on an already-spent
// confirmation button observable — without the captured-turn guard it ends
// whoever is active now instead of being rejected.
type doneConfirmFixture struct {
	handler      *DoneHandler
	sess         *mockMoveSession
	encounterID  uuid.UUID
	turnID       uuid.UUID
	currentTurn  uuid.NullUUID
	advanceCalls int
}

func newDoneConfirmFixture(sess *mockMoveSession) *doneConfirmFixture {
	handler, encounterID, turnID, _, nextCombatantID := setupFullDoneHandler(sess)

	f := &doneConfirmFixture{
		handler:     handler,
		sess:        sess,
		encounterID: encounterID,
		turnID:      turnID,
		currentTurn: uuid.NullUUID{UUID: turnID, Valid: true},
	}

	base := handler.combatService.(*mockMoveService)
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:            encounterID,
				Status:        "active",
				CurrentTurnID: f.currentTurn,
			}, nil
		},
		getCombatant:       base.getCombatant,
		listCombatants:     base.listCombatants,
		updateCombatantPos: base.updateCombatantPos,
	}

	handler.SetTurnAdvancer(&mockDoneTurnAdvancer{
		advanceTurn: func(_ context.Context, _ uuid.UUID) (combat.TurnInfo, error) {
			f.advanceCalls++
			f.currentTurn = uuid.NullUUID{UUID: uuid.New(), Valid: true}
			return combat.TurnInfo{CombatantID: nextCombatantID, RoundNumber: 2}, nil
		},
	})

	return f
}

func makeDoneComponentInteraction(customID string) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member: &discordgo.Member{
			User: &discordgo.User{ID: "user1"},
		},
		Data: discordgo.MessageComponentInteractionData{CustomID: customID},
	}
}

// --- Regression: double-clicking the "End Turn" button must not burn two turns ---

func TestDoneHandler_HandleDoneConfirm_DoubleClickAdvancesOnce(t *testing.T) {
	sess := &mockMoveSession{}
	f := newDoneConfirmFixture(sess)

	customID := "done_confirm:" + f.encounterID.String() + ":" + f.turnID.String()

	f.handler.HandleDoneConfirm(makeDoneComponentInteraction(customID), f.encounterID, f.turnID)
	f.handler.HandleDoneConfirm(makeDoneComponentInteraction(customID), f.encounterID, f.turnID)

	if f.advanceCalls != 1 {
		t.Fatalf("expected AdvanceTurn to be called exactly once, got %d", f.advanceCalls)
	}

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "already ended") {
		t.Errorf("expected stale-turn message on second click, got: %s", content)
	}
}

func TestDoneHandler_HandleDoneConfirm_StaleTurnIDRejected(t *testing.T) {
	sess := &mockMoveSession{}
	f := newDoneConfirmFixture(sess)

	staleTurnID := uuid.New()
	f.handler.HandleDoneConfirm(makeDoneComponentInteraction("done_confirm"), f.encounterID, staleTurnID)

	if f.advanceCalls != 0 {
		t.Fatalf("expected no AdvanceTurn for a stale turn ID, got %d calls", f.advanceCalls)
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "already ended") {
		t.Errorf("expected stale-turn message, got: %s", content)
	}
}

func TestDoneHandler_HandleDoneConfirm_NoCurrentTurnRejected(t *testing.T) {
	sess := &mockMoveSession{}
	f := newDoneConfirmFixture(sess)
	f.currentTurn = uuid.NullUUID{Valid: false}

	f.handler.HandleDoneConfirm(makeDoneComponentInteraction("done_confirm"), f.encounterID, f.turnID)

	if f.advanceCalls != 0 {
		t.Fatalf("expected no AdvanceTurn when the encounter has no active turn, got %d calls", f.advanceCalls)
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "already ended") {
		t.Errorf("expected stale-turn message, got: %s", content)
	}
}

func TestDoneHandler_HandleDoneConfirm_MatchingTurnIDAdvancesOnce(t *testing.T) {
	sess := &mockMoveSession{}
	f := newDoneConfirmFixture(sess)

	f.handler.HandleDoneConfirm(makeDoneComponentInteraction("done_confirm"), f.encounterID, f.turnID)

	if f.advanceCalls != 1 {
		t.Fatalf("expected AdvanceTurn once on the happy path, got %d calls", f.advanceCalls)
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Turn ended") {
		t.Errorf("expected turn ended, got: %s", content)
	}
}

// --- The confirmation button must carry the turn it was minted for ---

func TestDoneHandler_SendConfirmation_CustomIDCarriesTurnID(t *testing.T) {
	sess := &mockMoveSession{}
	handler, encounterID, turnID, combatantID, _ := setupFullDoneHandler(sess)

	// Leave the bonus action unspent so Handle takes the confirmation path.
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:               turnID,
				CombatantID:      combatantID,
				ActionUsed:       true,
				BonusActionUsed:  false,
				AttacksRemaining: 0,
			}, nil
		},
	}

	handler.Handle(makeDoneInteraction())

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	row, ok := sess.lastResponse.Data.Components[0].(discordgo.ActionsRow)
	if !ok {
		t.Fatalf("expected an ActionsRow, got %T", sess.lastResponse.Data.Components[0])
	}
	button, ok := row.Components[0].(discordgo.Button)
	if !ok {
		t.Fatalf("expected a Button, got %T", row.Components[0])
	}

	want := "done_confirm:" + encounterID.String() + ":" + turnID.String()
	if button.CustomID != want {
		t.Errorf("expected custom ID %q, got %q", want, button.CustomID)
	}
}

// --- Router parses the 3-segment form and rejects everything else ---

func TestRouter_DoneConfirmRouting_ThreeSegmentDispatchesTurnID(t *testing.T) {
	mock := newTestMock()
	var respondedContent string
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Data != nil {
			respondedContent = resp.Data.Content
		}
		return nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)

	f := newDoneConfirmFixture(&mockMoveSession{})
	f.handler.session = mock
	router.SetDoneHandler(f.handler)

	router.Handle(makeDoneComponentInteraction(
		"done_confirm:" + f.encounterID.String() + ":" + f.turnID.String()))

	if f.advanceCalls != 1 {
		t.Fatalf("expected AdvanceTurn once via router, got %d calls", f.advanceCalls)
	}
	if !strings.Contains(respondedContent, "Turn ended") {
		t.Errorf("expected turn ended from confirm routing, got: %s", respondedContent)
	}

	// A second click on the same (now stale) button must be refused by the guard.
	router.Handle(makeDoneComponentInteraction(
		"done_confirm:" + f.encounterID.String() + ":" + f.turnID.String()))

	if f.advanceCalls != 1 {
		t.Fatalf("expected the stale re-click to be refused, got %d AdvanceTurn calls", f.advanceCalls)
	}
	if !strings.Contains(respondedContent, "already ended") {
		t.Errorf("expected stale-turn message, got: %s", respondedContent)
	}
}

func TestRouter_DoneConfirmRouting_MalformedTurnSegment(t *testing.T) {
	mock := newTestMock()
	var respondedContent string
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Data != nil {
			respondedContent = resp.Data.Content
		}
		return nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)

	f := newDoneConfirmFixture(&mockMoveSession{})
	f.handler.session = mock
	router.SetDoneHandler(f.handler)

	router.Handle(makeDoneComponentInteraction("done_confirm:" + f.encounterID.String() + ":not-a-uuid"))

	if f.advanceCalls != 0 {
		t.Fatalf("expected no dispatch for a malformed turn segment, got %d AdvanceTurn calls", f.advanceCalls)
	}
	if !strings.Contains(respondedContent, "Invalid turn ID") {
		t.Errorf("expected invalid turn ID message, got: %s", respondedContent)
	}
}

func TestRouter_DoneConfirmRouting_LegacyTwoSegmentIsStale(t *testing.T) {
	mock := newTestMock()
	var respondedContent string
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Data != nil {
			respondedContent = resp.Data.Content
		}
		return nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)

	f := newDoneConfirmFixture(&mockMoveSession{})
	f.handler.session = mock
	router.SetDoneHandler(f.handler)

	// An ephemeral button minted before the redeploy: no turn segment. It must
	// not fall back to the old unguarded advance.
	router.Handle(makeDoneComponentInteraction("done_confirm:" + f.encounterID.String()))

	if f.advanceCalls != 0 {
		t.Fatalf("expected no dispatch for a legacy 2-segment custom ID, got %d AdvanceTurn calls", f.advanceCalls)
	}
	if !strings.Contains(respondedContent, "/done") {
		t.Errorf("expected a re-run /done message, got: %s", respondedContent)
	}
}
