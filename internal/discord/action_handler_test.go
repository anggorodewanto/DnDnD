package discord

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// --- Fakes for ActionHandler ---

type fakeActionEncounterResolver struct {
	encounterID uuid.UUID
	err         error
}

func (f *fakeActionEncounterResolver) ActiveEncounterForUser(_ context.Context, _, _ string) (uuid.UUID, error) {
	return f.encounterID, f.err
}

type fakeActionCombatService struct {
	encounters           map[uuid.UUID]refdata.Encounter
	combatants           map[uuid.UUID]refdata.Combatant
	byEncounter          map[uuid.UUID][]refdata.Combatant
	freeformCalledWith   combat.FreeformActionCommand
	freeformResult       combat.FreeformActionResult
	freeformErr          error
	cancelCalledWith     combat.CancelFreeformActionCommand
	cancelResult         combat.CancelFreeformActionResult
	cancelErr            error
	cancelExplCalledWith uuid.UUID
	cancelExplResult     combat.CancelFreeformActionResult
	cancelExplErr        error
	readyCalledWith      combat.ReadyActionCommand
	readyResult          combat.ReadyActionResult
	readyErr             error

	// D-47..D-57 dispatch recordings + canned results.
	surgeCalls           []combat.ActionSurgeCommand
	surgeResult          combat.ActionSurgeResult
	surgeErr             error
	dashCalls            []combat.DashCommand
	dashResult           combat.DashResult
	dashErr              error
	disengageCalls       []combat.DisengageCommand
	disengageResult      combat.DisengageResult
	dodgeCalls           []combat.DodgeCommand
	dodgeResult          combat.DodgeResult
	helpCalls            []combat.HelpCommand
	helpResult           combat.HelpResult
	hideCalls            []combat.HideCommand
	hideResult           combat.HideResult
	standCalls           []combat.StandCommand
	standResult          combat.StandResult
	dropProneCalls       []combat.DropProneCommand
	dropProneResult      combat.DropProneResult
	escapeCalls          []combat.EscapeCommand
	escapeResult         combat.EscapeResult
	turnUndeadCalls      []combat.TurnUndeadCommand
	turnUndeadResult     combat.TurnUndeadResult
	preserveLifeCalls    []combat.PreserveLifeCommand
	preserveLifeResult   combat.PreserveLifeResult
	sacredWeaponCalls    []combat.SacredWeaponCommand
	sacredWeaponResult   combat.SacredWeaponResult
	vowCalls             []combat.VowOfEnmityCommand
	vowResult            combat.VowOfEnmityResult
	cdDMQueueCalls       []combat.ChannelDivinityDMQueueCommand
	cdDMQueueResult      combat.DMQueueResult
	layCalls             []combat.LayOnHandsCommand
	layResult            combat.LayOnHandsResult
}

func (f *fakeActionCombatService) GetEncounter(_ context.Context, id uuid.UUID) (refdata.Encounter, error) {
	enc, ok := f.encounters[id]
	if !ok {
		return refdata.Encounter{}, errors.New("encounter not found")
	}
	return enc, nil
}

func (f *fakeActionCombatService) GetCombatant(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
	c, ok := f.combatants[id]
	if !ok {
		return refdata.Combatant{}, errors.New("combatant not found")
	}
	return c, nil
}

func (f *fakeActionCombatService) ListCombatantsByEncounterID(_ context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
	return f.byEncounter[encounterID], nil
}

func (f *fakeActionCombatService) FreeformAction(_ context.Context, cmd combat.FreeformActionCommand) (combat.FreeformActionResult, error) {
	f.freeformCalledWith = cmd
	return f.freeformResult, f.freeformErr
}

func (f *fakeActionCombatService) CancelFreeformAction(_ context.Context, cmd combat.CancelFreeformActionCommand) (combat.CancelFreeformActionResult, error) {
	f.cancelCalledWith = cmd
	return f.cancelResult, f.cancelErr
}

func (f *fakeActionCombatService) CancelExplorationFreeformAction(_ context.Context, combatantID uuid.UUID) (combat.CancelFreeformActionResult, error) {
	f.cancelExplCalledWith = combatantID
	return f.cancelExplResult, f.cancelExplErr
}

func (f *fakeActionCombatService) ReadyAction(_ context.Context, cmd combat.ReadyActionCommand) (combat.ReadyActionResult, error) {
	f.readyCalledWith = cmd
	return f.readyResult, f.readyErr
}

func (f *fakeActionCombatService) ActionSurge(_ context.Context, cmd combat.ActionSurgeCommand) (combat.ActionSurgeResult, error) {
	f.surgeCalls = append(f.surgeCalls, cmd)
	return f.surgeResult, f.surgeErr
}

func (f *fakeActionCombatService) Dash(_ context.Context, cmd combat.DashCommand) (combat.DashResult, error) {
	f.dashCalls = append(f.dashCalls, cmd)
	return f.dashResult, f.dashErr
}

func (f *fakeActionCombatService) Disengage(_ context.Context, cmd combat.DisengageCommand) (combat.DisengageResult, error) {
	f.disengageCalls = append(f.disengageCalls, cmd)
	return f.disengageResult, nil
}

func (f *fakeActionCombatService) Dodge(_ context.Context, cmd combat.DodgeCommand) (combat.DodgeResult, error) {
	f.dodgeCalls = append(f.dodgeCalls, cmd)
	return f.dodgeResult, nil
}

func (f *fakeActionCombatService) Help(_ context.Context, cmd combat.HelpCommand) (combat.HelpResult, error) {
	f.helpCalls = append(f.helpCalls, cmd)
	return f.helpResult, nil
}

func (f *fakeActionCombatService) Hide(_ context.Context, cmd combat.HideCommand, _ *dice.Roller) (combat.HideResult, error) {
	f.hideCalls = append(f.hideCalls, cmd)
	return f.hideResult, nil
}

func (f *fakeActionCombatService) Stand(_ context.Context, cmd combat.StandCommand) (combat.StandResult, error) {
	f.standCalls = append(f.standCalls, cmd)
	return f.standResult, nil
}

func (f *fakeActionCombatService) DropProne(_ context.Context, cmd combat.DropProneCommand) (combat.DropProneResult, error) {
	f.dropProneCalls = append(f.dropProneCalls, cmd)
	return f.dropProneResult, nil
}

func (f *fakeActionCombatService) Escape(_ context.Context, cmd combat.EscapeCommand, _ *dice.Roller) (combat.EscapeResult, error) {
	f.escapeCalls = append(f.escapeCalls, cmd)
	return f.escapeResult, nil
}

func (f *fakeActionCombatService) TurnUndead(_ context.Context, cmd combat.TurnUndeadCommand, _ *dice.Roller) (combat.TurnUndeadResult, error) {
	f.turnUndeadCalls = append(f.turnUndeadCalls, cmd)
	return f.turnUndeadResult, nil
}

func (f *fakeActionCombatService) PreserveLife(_ context.Context, cmd combat.PreserveLifeCommand) (combat.PreserveLifeResult, error) {
	f.preserveLifeCalls = append(f.preserveLifeCalls, cmd)
	return f.preserveLifeResult, nil
}

func (f *fakeActionCombatService) SacredWeapon(_ context.Context, cmd combat.SacredWeaponCommand) (combat.SacredWeaponResult, error) {
	f.sacredWeaponCalls = append(f.sacredWeaponCalls, cmd)
	return f.sacredWeaponResult, nil
}

func (f *fakeActionCombatService) VowOfEnmity(_ context.Context, cmd combat.VowOfEnmityCommand) (combat.VowOfEnmityResult, error) {
	f.vowCalls = append(f.vowCalls, cmd)
	return f.vowResult, nil
}

func (f *fakeActionCombatService) ChannelDivinityDMQueue(_ context.Context, cmd combat.ChannelDivinityDMQueueCommand) (combat.DMQueueResult, error) {
	f.cdDMQueueCalls = append(f.cdDMQueueCalls, cmd)
	return f.cdDMQueueResult, nil
}

func (f *fakeActionCombatService) LayOnHands(_ context.Context, cmd combat.LayOnHandsCommand) (combat.LayOnHandsResult, error) {
	f.layCalls = append(f.layCalls, cmd)
	return f.layResult, nil
}

type fakeActionTurnProvider struct {
	turns  map[uuid.UUID]refdata.Turn
	getErr error
}

func (f *fakeActionTurnProvider) GetTurn(_ context.Context, id uuid.UUID) (refdata.Turn, error) {
	if f.getErr != nil {
		return refdata.Turn{}, f.getErr
	}
	turn, ok := f.turns[id]
	if !ok {
		return refdata.Turn{}, errors.New("turn not found")
	}
	return turn, nil
}

type fakeActionCampaignProvider struct {
	campaign refdata.Campaign
	err      error
}

func (f *fakeActionCampaignProvider) GetCampaignByGuildID(_ context.Context, _ string) (refdata.Campaign, error) {
	return f.campaign, f.err
}

type fakeActionCharacterLookup struct {
	char refdata.Character
	err  error
}

func (f *fakeActionCharacterLookup) GetCharacterByCampaignAndDiscord(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
	return f.char, f.err
}

type fakeActionPendingStore struct {
	created    refdata.CreatePendingActionParams
	createErr  error
	createResp refdata.PendingAction
}

func (f *fakeActionPendingStore) CreatePendingAction(_ context.Context, arg refdata.CreatePendingActionParams) (refdata.PendingAction, error) {
	f.created = arg
	if f.createErr != nil {
		return refdata.PendingAction{}, f.createErr
	}
	resp := f.createResp
	if resp.ID == uuid.Nil {
		resp = refdata.PendingAction{
			ID:            uuid.New(),
			EncounterID:   arg.EncounterID,
			CombatantID:   arg.CombatantID,
			ActionText:    arg.ActionText,
			Status:        "pending",
			DmQueueItemID: arg.DmQueueItemID,
		}
	}
	return resp, nil
}

// makeActionInteraction builds an /action interaction with an action option value
// and (optionally) args. When action is empty, no action option is appended so
// the handler's missing-option branch fires.
func makeActionInteraction(guildID, userID, action, args string) *discordgo.Interaction {
	var opts []*discordgo.ApplicationCommandInteractionDataOption
	if action != "" {
		opts = append(opts, &discordgo.ApplicationCommandInteractionDataOption{
			Name:  "action",
			Type:  discordgo.ApplicationCommandOptionString,
			Value: action,
		})
	}
	if args != "" {
		opts = append(opts, &discordgo.ApplicationCommandInteractionDataOption{
			Name:  "args",
			Type:  discordgo.ApplicationCommandOptionString,
			Value: args,
		})
	}
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: guildID,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: userID},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "action",
			Options: opts,
		},
	}
}

func runActionHandler(t *testing.T, h *ActionHandler, i *discordgo.Interaction) string {
	t.Helper()
	var content string
	mock := h.session.(*MockSession)
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		content = resp.Data.Content
		return nil
	}
	h.Handle(i)
	return content
}

func TestActionHandler_MissingOption(t *testing.T) {
	sess := &MockSession{}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{},
		&fakeActionCombatService{},
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{},
		&fakeActionCharacterLookup{},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "", ""))

	if resp != "Please provide an action (e.g. `/action flip the table`)." {
		t.Errorf("unexpected response: %q", resp)
	}
}

// --- Combat path ---

func TestActionHandler_CombatMode_CallsFreeformAction(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Thorn",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
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
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}

	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
		freeformResult: combat.FreeformActionResult{
			CombatLog:      "🎭 Thorn: \"flip the table\" — sent to DM queue",
			DMQueueMessage: `🎭 **Action** — Thorn: "flip the table"`,
			DMQueueItemID:  "some-item",
		},
	}
	turnProv := &fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}}

	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		turnProv,
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "flip the table", ""))

	if svc.freeformCalledWith.ActionText != "flip the table" {
		t.Errorf("ActionText = %q want %q", svc.freeformCalledWith.ActionText, "flip the table")
	}
	if svc.freeformCalledWith.Combatant.ID != combatantID {
		t.Errorf("Combatant.ID mismatch: %v", svc.freeformCalledWith.Combatant.ID)
	}
	if svc.freeformCalledWith.Turn.ID != turnID {
		t.Errorf("Turn.ID mismatch: %v", svc.freeformCalledWith.Turn.ID)
	}
	if svc.freeformCalledWith.GuildID != "g1" {
		t.Errorf("GuildID = %q want g1", svc.freeformCalledWith.GuildID)
	}
	if svc.freeformCalledWith.CampaignID != campID.String() {
		t.Errorf("CampaignID = %q want %q", svc.freeformCalledWith.CampaignID, campID.String())
	}
	if resp == "" {
		t.Errorf("expected non-empty response")
	}
}

func TestActionHandler_CombatMode_CombinesActionAndArgs(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Thorn",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}

	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
	}
	turnProv := &fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}}

	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		turnProv,
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)

	_ = runActionHandler(t, h, makeActionInteraction("g1", "u1", "grapple", "the bandit"))

	if svc.freeformCalledWith.ActionText != "grapple the bandit" {
		t.Errorf("ActionText = %q want %q", svc.freeformCalledWith.ActionText, "grapple the bandit")
	}
}

func TestActionHandler_CombatMode_RejectsNonOwner(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	otherCharID := uuid.New()
	invokerCharID := uuid.New()
	campID := uuid.New()

	// The combatant on this turn is OWNED by otherCharID; invoker has invokerCharID.
	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Thorn",
		CharacterID: uuid.NullUUID{UUID: otherCharID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}

	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
	}
	turnProv := &fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}}

	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		turnProv,
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: invokerCharID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "flip the table", ""))

	if resp != "It's not your turn." {
		t.Errorf("unexpected response: %q", resp)
	}
	if svc.freeformCalledWith.ActionText != "" {
		t.Errorf("FreeformAction should not have been called, got %q", svc.freeformCalledWith.ActionText)
	}
}

// --- Exploration path ---

func TestActionHandler_ExplorationMode_PostsToNotifier(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Aria",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	encounter := refdata.Encounter{
		ID:         encounterID,
		CampaignID: campID,
		Mode:       "exploration",
	}

	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
	}
	store := &fakeActionPendingStore{}
	notifier := &cancelRecordingNotifier{nextItemID: "11111111-2222-3333-4444-555555555555"}

	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		store,
	)
	h.SetNotifier(notifier)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "search the bookshelf", ""))

	if svc.freeformCalledWith.ActionText != "" {
		t.Errorf("FreeformAction (combat path) must not be called in exploration mode")
	}
	if len(notifier.posted) != 1 {
		t.Fatalf("expected 1 notifier post, got %d", len(notifier.posted))
	}
	ev := notifier.posted[0]
	if ev.Kind != dmqueue.KindFreeformAction {
		t.Errorf("Kind = %v want KindFreeformAction", ev.Kind)
	}
	if ev.PlayerName != "Aria" {
		t.Errorf("PlayerName = %q want Aria", ev.PlayerName)
	}
	if ev.Summary != `"search the bookshelf"` {
		t.Errorf("Summary = %q want %q", ev.Summary, `"search the bookshelf"`)
	}
	if ev.GuildID != "g1" {
		t.Errorf("GuildID = %q want g1", ev.GuildID)
	}
	if ev.CampaignID != campID.String() {
		t.Errorf("CampaignID = %q want %q", ev.CampaignID, campID.String())
	}
	if resp == "" {
		t.Errorf("expected non-empty response")
	}
}

func TestActionHandler_ExplorationMode_PersistsPendingAction(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()
	itemIDStr := "11111111-2222-3333-4444-555555555555"

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Aria",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	encounter := refdata.Encounter{
		ID:         encounterID,
		CampaignID: campID,
		Mode:       "exploration",
	}

	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
	}
	store := &fakeActionPendingStore{}
	notifier := &cancelRecordingNotifier{nextItemID: itemIDStr}

	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		store,
	)
	h.SetNotifier(notifier)

	_ = runActionHandler(t, h, makeActionInteraction("g1", "u1", "search the bookshelf", ""))

	if store.created.EncounterID != encounterID {
		t.Errorf("pending_action EncounterID mismatch: %v vs %v", store.created.EncounterID, encounterID)
	}
	if store.created.CombatantID != combatantID {
		t.Errorf("pending_action CombatantID mismatch: %v vs %v", store.created.CombatantID, combatantID)
	}
	if store.created.ActionText != "search the bookshelf" {
		t.Errorf("pending_action ActionText = %q", store.created.ActionText)
	}
	if !store.created.DmQueueItemID.Valid {
		t.Errorf("expected DmQueueItemID to be Valid")
	}
	if store.created.DmQueueItemID.UUID.String() != itemIDStr {
		t.Errorf("DmQueueItemID = %q want %q", store.created.DmQueueItemID.UUID.String(), itemIDStr)
	}
}

// --- Combat cancel path ---

func TestActionHandler_CombatMode_Cancel_Success(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Thorn",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}

	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
		cancelResult: combat.CancelFreeformActionResult{
			PendingAction: refdata.PendingAction{
				ActionText: "flip the table",
				Status:     "cancelled",
			},
		},
	}
	turnProv := &fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}}

	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		turnProv,
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "cancel", ""))

	if svc.cancelCalledWith.Combatant.ID != combatantID {
		t.Errorf("CancelFreeformAction not called with combatant %v (got %v)", combatantID, svc.cancelCalledWith.Combatant.ID)
	}
	if svc.cancelCalledWith.Turn.ID != turnID {
		t.Errorf("CancelFreeformAction Turn.ID = %v want %v", svc.cancelCalledWith.Turn.ID, turnID)
	}
	wantContains := "Pending action cancelled"
	if !contains(resp, wantContains) {
		t.Errorf("response %q should contain %q", resp, wantContains)
	}
	if !contains(resp, "flip the table") {
		t.Errorf("response %q should contain the action text", resp)
	}
}

func TestActionHandler_CombatMode_Cancel_NoPending(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Thorn",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}

	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
		cancelErr:   combat.ErrNoPendingAction,
	}
	turnProv := &fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}}

	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		turnProv,
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "cancel", ""))

	if resp != "❌ No pending action to cancel." {
		t.Errorf("response = %q", resp)
	}
}

func TestActionHandler_CombatMode_Cancel_AlreadyResolved(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Thorn",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}

	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
		cancelErr:   combat.ErrActionAlreadyResolved,
	}
	turnProv := &fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}}

	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		turnProv,
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "CANCEL", ""))

	want := "❌ That action has already been resolved — use `/undo` to request a correction instead."
	if resp != want {
		t.Errorf("response = %q want %q", resp, want)
	}
}

// --- Exploration cancel path ---

func TestActionHandler_ExplorationMode_Cancel_Success(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Aria",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	encounter := refdata.Encounter{
		ID:         encounterID,
		CampaignID: campID,
		Mode:       "exploration",
	}

	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
		cancelExplResult: combat.CancelFreeformActionResult{
			PendingAction: refdata.PendingAction{
				ActionText: "search the bookshelf",
				Status:     "cancelled",
			},
		},
	}

	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "cancel", ""))

	if svc.cancelExplCalledWith != combatantID {
		t.Errorf("CancelExplorationFreeformAction called with %v want %v", svc.cancelExplCalledWith, combatantID)
	}
	if !contains(resp, "Pending action cancelled") {
		t.Errorf("response %q missing expected phrase", resp)
	}
	if !contains(resp, "search the bookshelf") {
		t.Errorf("response %q missing action text", resp)
	}
}

func TestActionHandler_ExplorationMode_Cancel_NoPending(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Aria",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	encounter := refdata.Encounter{ID: encounterID, CampaignID: campID, Mode: "exploration"}

	svc := &fakeActionCombatService{
		encounters:    map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:    map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter:   map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
		cancelExplErr: combat.ErrNoPendingAction,
	}

	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "cancel", ""))

	if resp != "❌ No pending action to cancel." {
		t.Errorf("response = %q", resp)
	}
}

// --- Router wiring ---

func TestSetActionHandler_RoutesCommand(t *testing.T) {
	sess := &MockSession{}
	bot := &Bot{session: sess}
	router := NewCommandRouter(bot, nil)

	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Aria",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	encounter := refdata.Encounter{ID: encounterID, CampaignID: campID, Mode: "exploration"}

	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
	}
	notifier := &cancelRecordingNotifier{nextItemID: "11111111-2222-3333-4444-555555555555"}

	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)
	h.SetNotifier(notifier)
	router.SetActionHandler(h)

	var content string
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		content = resp.Data.Content
		return nil
	}

	router.Handle(makeActionInteraction("g1", "u1", "search bookshelf", ""))

	if len(notifier.posted) != 1 {
		t.Fatalf("expected 1 notifier post via router, got %d", len(notifier.posted))
	}
	if content == "" {
		t.Errorf("expected non-empty response via router")
	}
}

// --- Error-branch coverage ---

func TestActionHandler_NoActiveEncounter(t *testing.T) {
	sess := &MockSession{}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{err: errors.New("nope")},
		&fakeActionCombatService{},
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{},
		&fakeActionCharacterLookup{},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "flip the table", ""))

	if resp != "You are not in an active encounter." {
		t.Errorf("response = %q", resp)
	}
}

func TestActionHandler_GetEncounterFails(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	svc := &fakeActionCombatService{
		encounters: map[uuid.UUID]refdata.Encounter{}, // empty so GetEncounter fails
	}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{},
		&fakeActionCharacterLookup{},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "flip the table", ""))

	if resp != "Failed to load encounter." {
		t.Errorf("response = %q", resp)
	}
}

func TestActionHandler_CombatMode_NoActiveTurn(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	campID := uuid.New()
	encounter := refdata.Encounter{
		ID:         encounterID,
		CampaignID: campID,
		Mode:       "combat",
		// CurrentTurnID left invalid
	}
	svc := &fakeActionCombatService{
		encounters: map[uuid.UUID]refdata.Encounter{encounterID: encounter},
	}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: uuid.New()}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "flip", ""))
	if resp != "No active turn." {
		t.Errorf("response = %q", resp)
	}
}

func TestActionHandler_CombatMode_TurnLookupFails(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	campID := uuid.New()
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}
	svc := &fakeActionCombatService{
		encounters: map[uuid.UUID]refdata.Encounter{encounterID: encounter},
	}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{getErr: errors.New("turn gone")},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: uuid.New()}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "flip", ""))
	if resp != "Failed to load turn." {
		t.Errorf("response = %q", resp)
	}
}

func TestActionHandler_CombatMode_CombatantLookupFails(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	campID := uuid.New()
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}

	svc := &fakeActionCombatService{
		encounters: map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants: map[uuid.UUID]refdata.Combatant{}, // missing combatant
	}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: uuid.New()}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "flip", ""))
	if resp != "Failed to load combatant." {
		t.Errorf("response = %q", resp)
	}
}

func TestActionHandler_CombatMode_FreeformActionError(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Thorn",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}

	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
		freeformErr: errors.New("resource spent"),
	}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "flip", ""))
	if !contains(resp, "Failed to post action") {
		t.Errorf("response %q missing 'Failed to post action'", resp)
	}
}

func TestActionHandler_CombatMode_Cancel_OtherError(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Thorn",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}

	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
		cancelErr:   errors.New("db fail"),
	}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "cancel", ""))
	if !contains(resp, "Failed to cancel action") {
		t.Errorf("response %q missing 'Failed to cancel action'", resp)
	}
}

func TestActionHandler_ExplorationMode_NoPCCombatant(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	campID := uuid.New()
	charID := uuid.New()

	encounter := refdata.Encounter{ID: encounterID, CampaignID: campID, Mode: "exploration"}
	// Only an NPC combatant; no PCs.
	npc := refdata.Combatant{
		ID:          uuid.New(),
		EncounterID: encounterID,
		DisplayName: "Bandit",
		IsAlive: true, HpCurrent: 10,
		IsNpc:       true,
	}
	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {npc}},
	}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "search", ""))
	if resp != "Could not find your character in this encounter." {
		t.Errorf("response = %q", resp)
	}
}

func TestActionHandler_ExplorationMode_PendingStoreFails(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Aria",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	encounter := refdata.Encounter{ID: encounterID, CampaignID: campID, Mode: "exploration"}
	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
	}
	store := &fakeActionPendingStore{createErr: errors.New("db down")}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		store,
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "search", ""))
	if !contains(resp, "Failed to record action") {
		t.Errorf("response %q missing 'Failed to record action'", resp)
	}
}

func TestActionHandler_ExplorationMode_Cancel_AlreadyResolved(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Aria",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	encounter := refdata.Encounter{ID: encounterID, CampaignID: campID, Mode: "exploration"}
	svc := &fakeActionCombatService{
		encounters:    map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		byEncounter:   map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
		cancelExplErr: combat.ErrActionAlreadyResolved,
	}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "cancel", ""))
	want := "❌ That action has already been resolved — use `/undo` to request a correction instead."
	if resp != want {
		t.Errorf("response = %q want %q", resp, want)
	}
}

func TestActionHandler_ExplorationMode_Cancel_OtherError(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Aria",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	encounter := refdata.Encounter{ID: encounterID, CampaignID: campID, Mode: "exploration"}
	svc := &fakeActionCombatService{
		encounters:    map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		byEncounter:   map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
		cancelExplErr: errors.New("db boom"),
	}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "cancel", ""))
	if !contains(resp, "Failed to cancel action") {
		t.Errorf("response %q missing expected prefix", resp)
	}
}

func TestActionHandler_ExplorationMode_NoNotifier(t *testing.T) {
	// When notifier is nil, post is a silent no-op but pending_action still persists
	// with Valid=false dm_queue_item_id.
	sess := &MockSession{}
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Aria",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	encounter := refdata.Encounter{ID: encounterID, CampaignID: campID, Mode: "exploration"}
	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
	}
	store := &fakeActionPendingStore{}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		store,
	)
	// no SetNotifier

	_ = runActionHandler(t, h, makeActionInteraction("g1", "u1", "whistle", ""))

	if store.created.DmQueueItemID.Valid {
		t.Errorf("expected DmQueueItemID to be NULL when no notifier wired")
	}
	if store.created.ActionText != "whistle" {
		t.Errorf("action text = %q", store.created.ActionText)
	}
}

func TestActionHandler_ExplorationMode_NotifierPostError(t *testing.T) {
	// A notifier Post error must not fail the whole handler — the pending_action
	// still persists (without a dm_queue_item_id) and the player gets a
	// confirmation so they can cancel and retry.
	sess := &MockSession{}
	encounterID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Aria",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	encounter := refdata.Encounter{ID: encounterID, CampaignID: campID, Mode: "exploration"}
	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
	}
	store := &fakeActionPendingStore{}
	notifier := &cancelRecordingNotifier{postErr: errors.New("discord 5xx")}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		store,
	)
	h.SetNotifier(notifier)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "whistle", ""))

	if resp == "" {
		t.Errorf("expected non-empty response even when notifier fails")
	}
	if store.created.DmQueueItemID.Valid {
		t.Errorf("expected DmQueueItemID to be NULL when notifier post fails")
	}
}

func TestActionHandler_ExplorationMode_MultiplePCs_PicksInvoker(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	combatant1ID := uuid.New()
	combatant2ID := uuid.New()
	char1ID := uuid.New()
	char2ID := uuid.New()
	campID := uuid.New()

	pc1 := refdata.Combatant{
		ID:          combatant1ID,
		EncounterID: encounterID,
		DisplayName: "Aria",
		CharacterID: uuid.NullUUID{UUID: char1ID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	pc2 := refdata.Combatant{
		ID:          combatant2ID,
		EncounterID: encounterID,
		DisplayName: "Bran",
		CharacterID: uuid.NullUUID{UUID: char2ID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	encounter := refdata.Encounter{ID: encounterID, CampaignID: campID, Mode: "exploration"}
	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {pc1, pc2}},
	}
	store := &fakeActionPendingStore{}
	notifier := &cancelRecordingNotifier{nextItemID: "11111111-2222-3333-4444-555555555555"}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: char2ID}}, // invoker is Bran
		store,
	)
	h.SetNotifier(notifier)

	_ = runActionHandler(t, h, makeActionInteraction("g1", "u1", "read the tome", ""))

	if store.created.CombatantID != combatant2ID {
		t.Errorf("expected handler to pick invoker's combatant (%v), got %v", combatant2ID, store.created.CombatantID)
	}
	if len(notifier.posted) != 1 || notifier.posted[0].PlayerName != "Bran" {
		t.Errorf("expected notifier post for Bran, got %+v", notifier.posted)
	}
}

func TestActionHandler_CombatMode_RejectsNPCCombatantTurn(t *testing.T) {
	// When the turn belongs to an NPC (CharacterID NULL), combatantBelongsToUser
	// should return false so "It's not your turn." is surfaced.
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	npc := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Bandit",
		IsAlive: true, HpCurrent: 10,
		IsNpc:       true,
		// CharacterID left invalid
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}
	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: npc},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {npc}},
	}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "flip", ""))
	if resp != "It's not your turn." {
		t.Errorf("response = %q", resp)
	}
}

func TestActionHandler_CombatMode_CampaignLookupFailsRejects(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Thorn",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}
	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
	}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}},
		&fakeActionCampaignProvider{err: errors.New("no campaign")},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "flip", ""))
	if resp != "It's not your turn." {
		t.Errorf("response = %q", resp)
	}
}

func TestActionHandler_CombatMode_CharLookupFailsRejects(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Thorn",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}
	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
	}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{err: errors.New("not registered")},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "flip", ""))
	if resp != "It's not your turn." {
		t.Errorf("response = %q", resp)
	}
}

func TestActionHandler_ExplorationMode_MultiplePCs_CampaignLookupFailsFallsBack(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	combatant1ID := uuid.New()
	combatant2ID := uuid.New()
	char1ID := uuid.New()
	char2ID := uuid.New()
	campID := uuid.New()

	pc1 := refdata.Combatant{
		ID:          combatant1ID,
		EncounterID: encounterID,
		DisplayName: "Aria",
		CharacterID: uuid.NullUUID{UUID: char1ID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	pc2 := refdata.Combatant{
		ID:          combatant2ID,
		EncounterID: encounterID,
		DisplayName: "Bran",
		CharacterID: uuid.NullUUID{UUID: char2ID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	encounter := refdata.Encounter{ID: encounterID, CampaignID: campID, Mode: "exploration"}
	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {pc1, pc2}},
	}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{err: errors.New("no campaign")},
		&fakeActionCharacterLookup{char: refdata.Character{ID: char2ID}},
		&fakeActionPendingStore{},
	)
	h.SetNotifier(&cancelRecordingNotifier{nextItemID: "11111111-2222-3333-4444-555555555555"})

	_ = runActionHandler(t, h, makeActionInteraction("g1", "u1", "probe", ""))

	// Should have fallen back to pcs[0] (Aria, combatant1ID).
	store := h.pendingStore.(*fakeActionPendingStore)
	if store.created.CombatantID != combatant1ID {
		t.Errorf("expected fallback to first PC %v, got %v", combatant1ID, store.created.CombatantID)
	}
}

func TestActionHandler_ExplorationMode_MultiplePCs_CharLookupFailsFallsBack(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	combatant1ID := uuid.New()
	combatant2ID := uuid.New()
	char1ID := uuid.New()
	char2ID := uuid.New()
	campID := uuid.New()

	pc1 := refdata.Combatant{
		ID:          combatant1ID,
		EncounterID: encounterID,
		DisplayName: "Aria",
		CharacterID: uuid.NullUUID{UUID: char1ID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	pc2 := refdata.Combatant{
		ID:          combatant2ID,
		EncounterID: encounterID,
		DisplayName: "Bran",
		CharacterID: uuid.NullUUID{UUID: char2ID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	encounter := refdata.Encounter{ID: encounterID, CampaignID: campID, Mode: "exploration"}
	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {pc1, pc2}},
	}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{err: errors.New("not registered")},
		&fakeActionPendingStore{},
	)
	h.SetNotifier(&cancelRecordingNotifier{nextItemID: "11111111-2222-3333-4444-555555555555"})

	_ = runActionHandler(t, h, makeActionInteraction("g1", "u1", "probe", ""))

	store := h.pendingStore.(*fakeActionPendingStore)
	if store.created.CombatantID != combatant1ID {
		t.Errorf("expected fallback to first PC %v, got %v", combatant1ID, store.created.CombatantID)
	}
}

func TestActionHandler_ExplorationMode_MultiplePCs_NoMatchNotFound(t *testing.T) {
	// Multi-PC exploration where the invoker's character matches NONE of the PC
	// combatants — handler should surface the "not found" error rather than
	// silently posting for the first PC.
	sess := &MockSession{}
	encounterID := uuid.New()
	campID := uuid.New()

	pc1 := refdata.Combatant{
		ID:          uuid.New(),
		EncounterID: encounterID,
		DisplayName: "Aria",
		CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	pc2 := refdata.Combatant{
		ID:          uuid.New(),
		EncounterID: encounterID,
		DisplayName: "Bran",
		CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	encounter := refdata.Encounter{ID: encounterID, CampaignID: campID, Mode: "exploration"}
	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {pc1, pc2}},
	}
	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		&fakeActionTurnProvider{},
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		// Invoker's char ID won't match pc1 or pc2.
		&fakeActionCharacterLookup{char: refdata.Character{ID: uuid.New()}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "probe", ""))
	if resp != "Could not find your character in this encounter." {
		t.Errorf("response = %q", resp)
	}
}

// small util to avoid importing strings in every assertion
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// --- /action ready subcommand ---

func TestActionHandler_Ready_CallsReadyAction(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Thorn",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}

	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
		readyResult: combat.ReadyActionResult{
			CombatLog: "⏳ Thorn readies an action: \"shoot if a goblin opens the door\"",
		},
	}
	turnProv := &fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}}

	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		turnProv,
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "ready", "shoot if a goblin opens the door"))

	if svc.readyCalledWith.Description != "shoot if a goblin opens the door" {
		t.Errorf("Description = %q want %q", svc.readyCalledWith.Description, "shoot if a goblin opens the door")
	}
	if svc.readyCalledWith.Combatant.ID != combatantID {
		t.Errorf("Combatant.ID = %v want %v", svc.readyCalledWith.Combatant.ID, combatantID)
	}
	if svc.readyCalledWith.Turn.ID != turnID {
		t.Errorf("Turn.ID = %v want %v", svc.readyCalledWith.Turn.ID, turnID)
	}
	if !contains(resp, "readies an action") {
		t.Errorf("response %q should mention readies an action", resp)
	}
}

func TestActionHandler_Ready_RequiresDescription(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Thorn",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}

	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
	}
	turnProv := &fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}}

	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		turnProv,
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "ready", ""))

	if svc.readyCalledWith.Description != "" {
		t.Error("ReadyAction must not be called with empty description")
	}
	if !contains(resp, "describe") {
		t.Errorf("expected description prompt, got %q", resp)
	}
}

func TestActionHandler_Ready_RejectsNonOwner(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	otherCharID := uuid.New()
	invokerCharID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Thorn",
		CharacterID: uuid.NullUUID{UUID: otherCharID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}

	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
	}
	turnProv := &fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}}

	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		turnProv,
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: invokerCharID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "ready", "shoot anyone"))

	if svc.readyCalledWith.Description != "" {
		t.Error("ReadyAction must not be called when invoker is not the owner")
	}
	if resp != "It's not your turn." {
		t.Errorf("response = %q", resp)
	}
}

func TestActionHandler_Ready_ServiceError(t *testing.T) {
	sess := &MockSession{}
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Thorn",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsAlive: true, HpCurrent: 10,
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID}
	encounter := refdata.Encounter{
		ID:            encounterID,
		CampaignID:    campID,
		Mode:          "combat",
		CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
	}

	svc := &fakeActionCombatService{
		encounters:  map[uuid.UUID]refdata.Encounter{encounterID: encounter},
		combatants:  map[uuid.UUID]refdata.Combatant{combatantID: combatant},
		byEncounter: map[uuid.UUID][]refdata.Combatant{encounterID: {combatant}},
		readyErr:    errors.New("action already used"),
	}
	turnProv := &fakeActionTurnProvider{turns: map[uuid.UUID]refdata.Turn{turnID: turn}}

	h := NewActionHandler(
		sess,
		&fakeActionEncounterResolver{encounterID: encounterID},
		svc,
		turnProv,
		&fakeActionCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&fakeActionCharacterLookup{char: refdata.Character{ID: charID}},
		&fakeActionPendingStore{},
	)

	resp := runActionHandler(t, h, makeActionInteraction("g1", "u1", "ready", "shoot anyone"))

	if !contains(resp, "action already used") {
		t.Errorf("expected service error in response, got %q", resp)
	}
}
