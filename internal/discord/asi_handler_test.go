package discord

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

func TestBuildASIPromptComponents_ReturnsThreeButtons(t *testing.T) {
	charID := uuid.New()
	components := BuildASIPromptComponents(charID)

	if len(components) != 1 {
		t.Fatalf("expected 1 action row, got %d", len(components))
	}

	row, ok := components[0].(*discordgo.ActionsRow)
	if !ok {
		t.Fatal("expected ActionsRow")
	}

	if len(row.Components) != 3 {
		t.Fatalf("expected 3 buttons, got %d", len(row.Components))
	}

	btn0 := row.Components[0].(discordgo.Button)
	btn1 := row.Components[1].(discordgo.Button)
	btn2 := row.Components[2].(discordgo.Button)

	// Check custom IDs contain character ID
	wantPrefix := "asi_choice:" + charID.String()
	if btn0.CustomID != wantPrefix+":plus2" {
		t.Errorf("btn0 CustomID = %q, want %q", btn0.CustomID, wantPrefix+":plus2")
	}
	if btn1.CustomID != wantPrefix+":plus1plus1" {
		t.Errorf("btn1 CustomID = %q, want %q", btn1.CustomID, wantPrefix+":plus1plus1")
	}
	if btn2.CustomID != wantPrefix+":feat" {
		t.Errorf("btn2 CustomID = %q, want %q", btn2.CustomID, wantPrefix+":feat")
	}
}

func TestBuildAbilitySelectMenu_Plus2(t *testing.T) {
	charID := uuid.New()
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 20, INT: 10, WIS: 12, CHA: 8}
	components := BuildAbilitySelectMenu(charID, "plus2", scores)

	if len(components) != 1 {
		t.Fatalf("expected 1 action row, got %d", len(components))
	}

	row, ok := components[0].(*discordgo.ActionsRow)
	if !ok {
		t.Fatal("expected ActionsRow")
	}

	menu, ok := row.Components[0].(discordgo.SelectMenu)
	if !ok {
		t.Fatal("expected SelectMenu")
	}

	// CON is 20, should be excluded
	if len(menu.Options) != 5 {
		t.Fatalf("expected 5 options (CON excluded at 20), got %d", len(menu.Options))
	}

	// Check that CustomID encodes the type
	if menu.CustomID != "asi_select:"+charID.String()+":plus2" {
		t.Errorf("CustomID = %q", menu.CustomID)
	}

	// Verify option labels show current -> new values
	found := false
	for _, opt := range menu.Options {
		if opt.Value == "str" {
			found = true
			if opt.Label != "STR (16 -> 18)" {
				t.Errorf("STR label = %q, want %q", opt.Label, "STR (16 -> 18)")
			}
		}
	}
	if !found {
		t.Error("expected STR option")
	}
}

func TestBuildAbilitySelectMenu_Plus1Plus1(t *testing.T) {
	charID := uuid.New()
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}
	components := BuildAbilitySelectMenu(charID, "plus1plus1", scores)

	row := components[0].(*discordgo.ActionsRow)
	menu := row.Components[0].(discordgo.SelectMenu)

	// All 6 abilities should be available (none at 20)
	if len(menu.Options) != 6 {
		t.Fatalf("expected 6 options, got %d", len(menu.Options))
	}
	if menu.MaxValues != 2 {
		t.Errorf("MaxValues = %d, want 2", menu.MaxValues)
	}

	// STR should show +1
	for _, opt := range menu.Options {
		if opt.Value == "str" {
			if opt.Label != "STR (16 -> 17)" {
				t.Errorf("STR label = %q, want %q", opt.Label, "STR (16 -> 17)")
			}
		}
	}
}

func TestParseASIChoiceCustomID(t *testing.T) {
	charID := uuid.New()
	customID := "asi_choice:" + charID.String() + ":plus2"
	parsedCharID, asiType, err := ParseASIChoiceCustomID(customID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsedCharID != charID {
		t.Errorf("charID = %s, want %s", parsedCharID, charID)
	}
	if asiType != "plus2" {
		t.Errorf("asiType = %q, want %q", asiType, "plus2")
	}
}

func TestParseASIChoiceCustomID_Invalid(t *testing.T) {
	_, _, err := ParseASIChoiceCustomID("bad:data")
	if err == nil {
		t.Error("expected error for invalid custom ID")
	}
}

func TestParseASISelectCustomID(t *testing.T) {
	charID := uuid.New()
	customID := "asi_select:" + charID.String() + ":plus2"
	parsedCharID, asiType, err := ParseASISelectCustomID(customID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsedCharID != charID {
		t.Errorf("charID = %s, want %s", parsedCharID, charID)
	}
	if asiType != "plus2" {
		t.Errorf("asiType = %q, want %q", asiType, "plus2")
	}
}

func TestBuildDMApprovalComponents(t *testing.T) {
	charID := uuid.New()
	components := BuildDMApprovalComponents(charID)

	if len(components) != 1 {
		t.Fatalf("expected 1 action row, got %d", len(components))
	}

	row := components[0].(*discordgo.ActionsRow)
	if len(row.Components) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(row.Components))
	}

	approve := row.Components[0].(discordgo.Button)
	deny := row.Components[1].(discordgo.Button)

	wantApprove := "asi_approve:" + charID.String()
	wantDeny := "asi_deny:" + charID.String()

	if approve.CustomID != wantApprove {
		t.Errorf("approve CustomID = %q, want %q", approve.CustomID, wantApprove)
	}
	if deny.CustomID != wantDeny {
		t.Errorf("deny CustomID = %q, want %q", deny.CustomID, wantDeny)
	}
	if approve.Style != discordgo.SuccessButton {
		t.Errorf("approve style = %v, want SuccessButton", approve.Style)
	}
	if deny.Style != discordgo.DangerButton {
		t.Errorf("deny style = %v, want DangerButton", deny.Style)
	}
}

func TestFormatDMQueueASIMessage(t *testing.T) {
	msg := FormatDMQueueASIMessage("Aria", "Fighter 8", "+2 STR (16 -> 18)")

	if msg == "" {
		t.Fatal("expected non-empty message")
	}
	if !strings.Contains(msg, "Aria") {
		t.Error("expected character name")
	}
	if !strings.Contains(msg, "Fighter 8") {
		t.Error("expected class info")
	}
	if !strings.Contains(msg, "+2 STR") {
		t.Error("expected choice description")
	}
}


func TestParseDMApprovalCustomID(t *testing.T) {
	charID := uuid.New()

	parsedID, err := ParseDMApprovalCustomID("asi_approve:" + charID.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsedID != charID {
		t.Errorf("charID = %s, want %s", parsedID, charID)
	}

	parsedID, err = ParseDMApprovalCustomID("asi_deny:" + charID.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsedID != charID {
		t.Errorf("charID = %s, want %s", parsedID, charID)
	}
}

func TestParseASISelectCustomID_Invalid(t *testing.T) {
	_, _, err := ParseASISelectCustomID("bad")
	if err == nil {
		t.Error("expected error for invalid custom ID")
	}

	_, _, err = ParseASISelectCustomID("asi_select:bad-uuid:plus2")
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

func TestParseASIChoiceCustomID_BadUUID(t *testing.T) {
	_, _, err := ParseASIChoiceCustomID("asi_choice:bad-uuid:plus2")
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

func TestParseDMApprovalCustomID_Invalid(t *testing.T) {
	_, err := ParseDMApprovalCustomID("bad")
	if err == nil {
		t.Error("expected error")
	}

	_, err = ParseDMApprovalCustomID("wrong_prefix:" + uuid.New().String())
	if err == nil {
		t.Error("expected error for wrong prefix")
	}

	_, err = ParseDMApprovalCustomID("asi_approve:bad-uuid")
	if err == nil {
		t.Error("expected error for bad UUID")
	}
}

func TestASIHandler_HandleASISelect_NoAbilities(t *testing.T) {
	charID := uuid.New()
	svc := &mockASIService{}

	var respondedContent string
	session := &MockSession{
		InteractionRespondFunc: func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			if resp.Data != nil {
				respondedContent = resp.Data.Content
			}
			return nil
		},
	}

	handler := NewASIHandler(session, svc, nil)

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_select:" + charID.String() + ":plus2",
			Values:   []string{},
		},
		Member: &discordgo.Member{User: &discordgo.User{ID: "user123"}},
	}

	handler.HandleASISelect(interaction)

	if !strings.Contains(respondedContent, "No abilities") {
		t.Errorf("expected 'No abilities' message, got: %s", respondedContent)
	}
}

func TestASIHandler_HandleASISelect_CharNotFound(t *testing.T) {
	charID := uuid.New()
	svc := &mockASIService{
		charErr: fmt.Errorf("not found"),
	}

	var respondedContent string
	session := &MockSession{
		InteractionRespondFunc: func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			if resp.Data != nil {
				respondedContent = resp.Data.Content
			}
			return nil
		},
	}

	handler := NewASIHandler(session, svc, nil)

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_select:" + charID.String() + ":plus2",
			Values:   []string{"str"},
		},
		Member: &discordgo.Member{User: &discordgo.User{ID: "user123"}},
	}

	handler.HandleASISelect(interaction)

	if !strings.Contains(respondedContent, "Could not load") {
		t.Errorf("expected error message, got: %s", respondedContent)
	}
}

func TestASIHandler_HandleASISelect_NoDMQueueFunc(t *testing.T) {
	charID := uuid.New()
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}

	svc := &mockASIService{
		character: &ASICharacterData{
			ID:            charID,
			Name:          "Aria",
			DiscordUserID: "user123",
			Scores:        scores,
			ClassInfo:     "Fighter 8",
		},
	}

	session := &MockSession{
		InteractionRespondFunc: func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			return nil
		},
	}

	// No DM queue func
	handler := NewASIHandler(session, svc, nil)

	interaction := &discordgo.Interaction{
		GuildID: "guild1",
		Type:    discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_select:" + charID.String() + ":plus2",
			Values:   []string{"str"},
		},
		Member: &discordgo.Member{User: &discordgo.User{ID: "user123"}},
	}

	// Should not panic
	handler.HandleASISelect(interaction)
}

func TestASIHandler_HandleDMApprove_ApproveError(t *testing.T) {
	charID := uuid.New()
	svc := &mockASIService{
		approveASIErr: fmt.Errorf("db error"),
	}

	var editedContent string
	session := &MockSession{
		InteractionRespondFunc: func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			return nil
		},
		InteractionResponseEditFunc: func(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit) (*discordgo.Message, error) {
			if newresp.Content != nil {
				editedContent = *newresp.Content
			}
			return &discordgo.Message{}, nil
		},
	}

	handler := NewASIHandler(session, svc, nil)
	handler.storePendingChoice(charID, PendingASIChoice{
		CharID:   charID,
		ASIType:  "plus2",
		PlayerID: "user123",
	})

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_approve:" + charID.String(),
		},
	}

	handler.HandleDMApprove(interaction)

	if !strings.Contains(editedContent, "failed") {
		t.Errorf("expected failure message, got: %s", editedContent)
	}
}

func TestASIHandler_HandleDMDeny_NoPending(t *testing.T) {
	charID := uuid.New()
	svc := &mockASIService{}

	var respondedContent string
	session := &MockSession{
		InteractionRespondFunc: func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			if resp.Data != nil {
				respondedContent = resp.Data.Content
			}
			return nil
		},
	}

	handler := NewASIHandler(session, svc, nil)

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_deny:" + charID.String(),
		},
	}

	handler.HandleDMDeny(interaction)

	if !strings.Contains(respondedContent, "No pending") {
		t.Errorf("expected 'No pending' error, got: %s", respondedContent)
	}
}

func TestRouter_RoutesASISelectToHandler(t *testing.T) {
	mock := newTestMock()
	var responded bool
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = true
		return nil
	}

	charID := uuid.New()
	svc := &mockASIService{
		charErr: fmt.Errorf("not found"),
	}

	bot := &Bot{session: mock}
	router := NewCommandRouter(bot, nil)
	asiHandler := NewASIHandler(mock, svc, nil)
	router.SetASIHandler(asiHandler)

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_select:" + charID.String() + ":plus2",
			Values:   []string{"str"},
		},
		Member: &discordgo.Member{User: &discordgo.User{ID: "user123"}},
	}

	router.Handle(interaction)

	if !responded {
		t.Error("expected ASI select to be routed")
	}
}

func TestRouter_RoutesASIDenyToHandler(t *testing.T) {
	mock := newTestMock()
	var responded bool
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = true
		return nil
	}

	charID := uuid.New()
	svc := &mockASIService{}

	bot := &Bot{session: mock}
	router := NewCommandRouter(bot, nil)
	asiHandler := NewASIHandler(mock, svc, nil)
	router.SetASIHandler(asiHandler)

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_deny:" + charID.String(),
		},
	}

	router.Handle(interaction)

	if !responded {
		t.Error("expected ASI deny to be routed")
	}
}

func TestBuildAbilitySelectMenu_Plus2_AllAt20(t *testing.T) {
	charID := uuid.New()
	scores := character.AbilityScores{STR: 20, DEX: 20, CON: 20, INT: 20, WIS: 20, CHA: 20}
	components := BuildAbilitySelectMenu(charID, "plus2", scores)

	if len(components) != 1 {
		t.Fatalf("expected 1 action row, got %d", len(components))
	}
	row := components[0].(*discordgo.ActionsRow)
	menu := row.Components[0].(discordgo.SelectMenu)

	if len(menu.Options) != 0 {
		t.Errorf("expected 0 options when all at 20, got %d", len(menu.Options))
	}
}

// --- ASIHandler integration tests ---

type mockASIService struct {
	approveASICalled bool
	approveASIErr    error
	approveCharID    uuid.UUID
	approveChoice    ASIChoiceData

	denyASICalled bool
	denyCharID    uuid.UUID
	denyReason    string

	character *ASICharacterData
	charErr   error
}

func (m *mockASIService) ApproveASI(ctx context.Context, charID uuid.UUID, choice ASIChoiceData) error {
	m.approveASICalled = true
	m.approveCharID = charID
	m.approveChoice = choice
	return m.approveASIErr
}

func (m *mockASIService) DenyASI(ctx context.Context, charID uuid.UUID, reason string) error {
	m.denyASICalled = true
	m.denyCharID = charID
	m.denyReason = reason
	return nil
}

func (m *mockASIService) GetCharacterForASI(ctx context.Context, charID uuid.UUID) (*ASICharacterData, error) {
	if m.charErr != nil {
		return nil, m.charErr
	}
	return m.character, nil
}

func TestASIHandler_HandleASIChoiceButton_Plus2(t *testing.T) {
	charID := uuid.New()
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}

	svc := &mockASIService{
		character: &ASICharacterData{
			ID:            charID,
			Name:          "Aria",
			DiscordUserID: "user123",
			Scores:        scores,
			ClassInfo:     "Fighter 8",
		},
	}

	var respondedData *discordgo.InteractionResponseData
	session := &MockSession{
		InteractionRespondFunc: func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			respondedData = resp.Data
			return nil
		},
	}

	handler := NewASIHandler(session, svc, nil)

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_choice:" + charID.String() + ":plus2",
		},
		Member: &discordgo.Member{User: &discordgo.User{ID: "user123"}},
	}

	handler.HandleASIChoice(interaction)

	if respondedData == nil {
		t.Fatal("expected interaction response")
	}

	// Should respond with a select menu for ability scores
	if len(respondedData.Components) == 0 {
		t.Fatal("expected components in response")
	}
}

func TestASIHandler_HandleASISelect_Plus2_PostsToDMQueue(t *testing.T) {
	charID := uuid.New()
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}

	svc := &mockASIService{
		character: &ASICharacterData{
			ID:            charID,
			Name:          "Aria",
			DiscordUserID: "user123",
			Scores:        scores,
			ClassInfo:     "Fighter 8",
		},
	}

	var sentChannelID string
	var sentMessage *discordgo.MessageSend
	session := &MockSession{
		InteractionRespondFunc: func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			return nil
		},
		ChannelMessageSendComplexFunc: func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
			sentChannelID = channelID
			sentMessage = data
			return &discordgo.Message{ID: "msg1"}, nil
		},
	}

	dmQueueFunc := func(guildID string) string { return "dm-queue-channel" }
	handler := NewASIHandler(session, svc, dmQueueFunc)

	interaction := &discordgo.Interaction{
		GuildID: "guild1",
		Type:    discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_select:" + charID.String() + ":plus2",
			Values:   []string{"str"},
		},
		Member: &discordgo.Member{User: &discordgo.User{ID: "user123"}},
	}

	handler.HandleASISelect(interaction)

	if sentChannelID != "dm-queue-channel" {
		t.Errorf("sent to channel %q, want %q", sentChannelID, "dm-queue-channel")
	}
	if sentMessage == nil {
		t.Fatal("expected message sent to DM queue")
	}
	// Should have approve/deny buttons
	if len(sentMessage.Components) == 0 {
		t.Error("expected components in DM queue message")
	}
}

func TestASIHandler_HandleDMApprove(t *testing.T) {
	charID := uuid.New()
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}

	svc := &mockASIService{
		character: &ASICharacterData{
			ID:            charID,
			Name:          "Aria",
			DiscordUserID: "user123",
			Scores:        scores,
			ClassInfo:     "Fighter 8",
		},
	}

	var respondedContent string
	session := &MockSession{
		InteractionRespondFunc: func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			return nil
		},
		InteractionResponseEditFunc: func(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit) (*discordgo.Message, error) {
			if newresp.Content != nil {
				respondedContent = *newresp.Content
			}
			return &discordgo.Message{}, nil
		},
		UserChannelCreateFunc: func(recipientID string) (*discordgo.Channel, error) {
			return &discordgo.Channel{ID: "dm-channel"}, nil
		},
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			return &discordgo.Message{}, nil
		},
	}

	handler := NewASIHandler(session, svc, nil)
	// Store a pending choice
	handler.storePendingChoice(charID, PendingASIChoice{
		CharID:      charID,
		ASIType:     "plus2",
		Abilities:   []string{"str"},
		PlayerID:    "user123",
		Description: "+2 STR (16 -> 18)",
	})

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_approve:" + charID.String(),
		},
	}

	handler.HandleDMApprove(interaction)

	if !svc.approveASICalled {
		t.Error("expected ApproveASI to be called")
	}
	if svc.approveCharID != charID {
		t.Errorf("approve charID = %s, want %s", svc.approveCharID, charID)
	}
	if respondedContent == "" {
		t.Error("expected response content update")
	}
}

func TestRouter_RoutesASIChoiceToHandler(t *testing.T) {
	mock := newTestMock()
	var respondedCustomID string
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		// The handler will respond - we just want to verify routing happened
		respondedCustomID = "routed"
		return nil
	}

	charID := uuid.New()
	svc := &mockASIService{
		character: &ASICharacterData{
			ID:            charID,
			Name:          "Aria",
			DiscordUserID: "user123",
			Scores:        character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8},
			ClassInfo:     "Fighter 8",
		},
	}

	bot := &Bot{session: mock}
	router := NewCommandRouter(bot, nil)
	asiHandler := NewASIHandler(mock, svc, nil)
	router.SetASIHandler(asiHandler)

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_choice:" + charID.String() + ":plus2",
		},
		Member: &discordgo.Member{User: &discordgo.User{ID: "user123"}},
	}

	router.Handle(interaction)

	if respondedCustomID != "routed" {
		t.Error("expected ASI choice to be routed to ASI handler")
	}
}

func TestRouter_RoutesASIApproveToHandler(t *testing.T) {
	mock := newTestMock()
	var responded bool
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = true
		return nil
	}

	charID := uuid.New()
	svc := &mockASIService{}

	bot := &Bot{session: mock}
	router := NewCommandRouter(bot, nil)
	asiHandler := NewASIHandler(mock, svc, nil)
	router.SetASIHandler(asiHandler)

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_approve:" + charID.String(),
		},
	}

	router.Handle(interaction)

	if !responded {
		t.Error("expected ASI approve to be routed to ASI handler")
	}
}

func TestASIHandler_HandleASIChoiceButton_Feat(t *testing.T) {
	charID := uuid.New()
	svc := &mockASIService{
		character: &ASICharacterData{
			ID:            charID,
			Name:          "Aria",
			DiscordUserID: "user123",
			Scores:        character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8},
			ClassInfo:     "Fighter 8",
		},
	}

	var respondedContent string
	session := &MockSession{
		InteractionRespondFunc: func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			if resp.Data != nil {
				respondedContent = resp.Data.Content
			}
			return nil
		},
	}

	handler := NewASIHandler(session, svc, nil)

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_choice:" + charID.String() + ":feat",
		},
		Member: &discordgo.Member{User: &discordgo.User{ID: "user123"}},
	}

	handler.HandleASIChoice(interaction)

	if respondedContent == "" {
		t.Fatal("expected response")
	}
	if !strings.Contains(respondedContent, "Feat selection") {
		t.Errorf("expected feat placeholder message, got: %s", respondedContent)
	}
}

func TestASIHandler_HandleASISelect_Plus1Plus1(t *testing.T) {
	charID := uuid.New()
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}

	svc := &mockASIService{
		character: &ASICharacterData{
			ID:            charID,
			Name:          "Aria",
			DiscordUserID: "user123",
			Scores:        scores,
			ClassInfo:     "Fighter 8",
		},
	}

	var sentMessage *discordgo.MessageSend
	session := &MockSession{
		InteractionRespondFunc: func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			return nil
		},
		ChannelMessageSendComplexFunc: func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
			sentMessage = data
			return &discordgo.Message{ID: "msg1"}, nil
		},
	}

	dmQueueFunc := func(guildID string) string { return "dm-queue-channel" }
	handler := NewASIHandler(session, svc, dmQueueFunc)

	interaction := &discordgo.Interaction{
		GuildID: "guild1",
		Type:    discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_select:" + charID.String() + ":plus1plus1",
			Values:   []string{"str", "dex"},
		},
		Member: &discordgo.Member{User: &discordgo.User{ID: "user123"}},
	}

	handler.HandleASISelect(interaction)

	if sentMessage == nil {
		t.Fatal("expected message to DM queue")
	}
	// Should mention both abilities
	if !strings.Contains(sentMessage.Content, "STR") || !strings.Contains(sentMessage.Content, "DEX") {
		t.Errorf("expected both abilities in message, got: %s", sentMessage.Content)
	}
}

func TestASIHandler_HandleASIChoice_CharNotFound(t *testing.T) {
	charID := uuid.New()
	svc := &mockASIService{
		charErr: fmt.Errorf("not found"),
	}

	var respondedContent string
	session := &MockSession{
		InteractionRespondFunc: func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			if resp.Data != nil {
				respondedContent = resp.Data.Content
			}
			return nil
		},
	}

	handler := NewASIHandler(session, svc, nil)

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_choice:" + charID.String() + ":plus2",
		},
		Member: &discordgo.Member{User: &discordgo.User{ID: "user123"}},
	}

	handler.HandleASIChoice(interaction)

	if !strings.Contains(respondedContent, "Could not load") {
		t.Errorf("expected error message, got: %s", respondedContent)
	}
}

func TestASIHandler_HandleDMApprove_NoPending(t *testing.T) {
	charID := uuid.New()
	svc := &mockASIService{}

	var respondedContent string
	session := &MockSession{
		InteractionRespondFunc: func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			if resp.Data != nil {
				respondedContent = resp.Data.Content
			}
			return nil
		},
	}

	handler := NewASIHandler(session, svc, nil)

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_approve:" + charID.String(),
		},
	}

	handler.HandleDMApprove(interaction)

	if !strings.Contains(respondedContent, "No pending") {
		t.Errorf("expected 'No pending' error, got: %s", respondedContent)
	}
}

func TestBuildASIDescription_Plus2(t *testing.T) {
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}
	desc := buildASIDescription("plus2", []string{"str"}, scores)
	if desc != "+2 STR (16 -> 18)" {
		t.Errorf("description = %q, want %q", desc, "+2 STR (16 -> 18)")
	}
}

func TestBuildASIDescription_Plus1Plus1(t *testing.T) {
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}
	desc := buildASIDescription("plus1plus1", []string{"str", "dex"}, scores)
	if desc != "+1 STR (16 -> 17), +1 DEX (14 -> 15)" {
		t.Errorf("description = %q", desc)
	}
}

func TestBuildASIDescription_Unknown(t *testing.T) {
	scores := character.AbilityScores{}
	desc := buildASIDescription("unknown", nil, scores)
	if desc != "unknown choice" {
		t.Errorf("description = %q, want %q", desc, "unknown choice")
	}
}

// med-36 / Phase 89: feat select-menu replaces the "not yet available"
// stub. The handler resolves the feat list via FeatLister and posts a
// Discord SelectMenu populated with eligible feats.
type stubFeatLister struct {
	feats []FeatOption
	err   error
}

func (s *stubFeatLister) ListEligibleFeats(_ context.Context, _ uuid.UUID) ([]FeatOption, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.feats, nil
}

func TestASIHandler_HandleASIChoiceButton_Feat_WithLister_PostsSelectMenu(t *testing.T) {
	charID := uuid.New()
	svc := &mockASIService{
		character: &ASICharacterData{
			ID:            charID,
			Name:          "Aria",
			DiscordUserID: "user123",
			Scores:        character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8},
			ClassInfo:     "Fighter 8",
		},
	}

	var respondedComponents []discordgo.MessageComponent
	session := &MockSession{
		InteractionRespondFunc: func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			if resp.Data != nil {
				respondedComponents = resp.Data.Components
			}
			return nil
		},
	}

	handler := NewASIHandler(session, svc, nil)
	handler.SetFeatLister(&stubFeatLister{
		feats: []FeatOption{
			{ID: "lucky", Name: "Lucky", Description: "Reroll d20s"},
			{ID: "tough", Name: "Tough", Description: "+2 HP per level"},
		},
	})

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_choice:" + charID.String() + ":feat",
		},
		Member: &discordgo.Member{User: &discordgo.User{ID: "user123"}},
	}

	handler.HandleASIChoice(interaction)

	if len(respondedComponents) == 0 {
		t.Fatal("expected response with select-menu components")
	}
	row, ok := respondedComponents[0].(*discordgo.ActionsRow)
	if !ok {
		t.Fatalf("expected ActionsRow, got %T", respondedComponents[0])
	}
	menu, ok := row.Components[0].(discordgo.SelectMenu)
	if !ok {
		t.Fatalf("expected SelectMenu, got %T", row.Components[0])
	}
	if menu.CustomID != "asi_feat_select:"+charID.String() {
		t.Errorf("CustomID = %q, want %q", menu.CustomID, "asi_feat_select:"+charID.String())
	}
	if len(menu.Options) != 2 {
		t.Errorf("expected 2 feat options, got %d", len(menu.Options))
	}
}

func TestASIHandler_HandleASIChoiceButton_Feat_NoLister_FallsBackToStub(t *testing.T) {
	charID := uuid.New()
	svc := &mockASIService{
		character: &ASICharacterData{ID: charID, Name: "Aria"},
	}

	var respondedContent string
	session := &MockSession{
		InteractionRespondFunc: func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			if resp.Data != nil {
				respondedContent = resp.Data.Content
			}
			return nil
		},
	}

	handler := NewASIHandler(session, svc, nil)
	// no SetFeatLister wired
	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_choice:" + charID.String() + ":feat",
		},
		Member: &discordgo.Member{User: &discordgo.User{ID: "user123"}},
	}
	handler.HandleASIChoice(interaction)
	if !strings.Contains(respondedContent, "not yet available") {
		t.Errorf("expected stub message when no lister wired, got: %s", respondedContent)
	}
}

func TestASIHandler_HandleASIFeatSelect_PostsToDMQueue(t *testing.T) {
	charID := uuid.New()
	svc := &mockASIService{
		character: &ASICharacterData{
			ID:            charID,
			Name:          "Aria",
			DiscordUserID: "user123",
			Scores:        character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8},
			ClassInfo:     "Fighter 8",
		},
	}

	var sentMessage *discordgo.MessageSend
	session := &MockSession{
		InteractionRespondFunc: func(_ *discordgo.Interaction, _ *discordgo.InteractionResponse) error {
			return nil
		},
		ChannelMessageSendComplexFunc: func(_ string, msg *discordgo.MessageSend) (*discordgo.Message, error) {
			sentMessage = msg
			return &discordgo.Message{}, nil
		},
	}

	handler := NewASIHandler(session, svc, func(_ string) string { return "dm-queue-channel" })
	handler.SetFeatLister(&stubFeatLister{
		feats: []FeatOption{{ID: "lucky", Name: "Lucky"}, {ID: "tough", Name: "Tough"}},
	})

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_feat_select:" + charID.String(),
			Values:   []string{"lucky"},
		},
		Member: &discordgo.Member{User: &discordgo.User{ID: "user123"}},
	}

	handler.HandleASIFeatSelect(interaction)

	pending, ok := handler.getPendingChoice(charID)
	if !ok {
		t.Fatal("expected pending choice stored")
	}
	if pending.FeatID != "lucky" {
		t.Errorf("FeatID = %q, want lucky", pending.FeatID)
	}
	if pending.ASIType != "feat" {
		t.Errorf("ASIType = %q, want feat", pending.ASIType)
	}
	if sentMessage == nil {
		t.Fatal("expected DM-queue message posted")
	}
	if !strings.Contains(sentMessage.Content, "Lucky") {
		t.Errorf("DM-queue content missing feat name: %s", sentMessage.Content)
	}
}

func TestASIHandler_HandleDMDeny(t *testing.T) {
	charID := uuid.New()

	svc := &mockASIService{
		character: &ASICharacterData{
			ID:            charID,
			Name:          "Aria",
			DiscordUserID: "user123",
			ClassInfo:     "Fighter 8",
		},
	}

	session := &MockSession{
		InteractionRespondFunc: func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			return nil
		},
		InteractionResponseEditFunc: func(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit) (*discordgo.Message, error) {
			return &discordgo.Message{}, nil
		},
	}

	handler := NewASIHandler(session, svc, nil)
	handler.storePendingChoice(charID, PendingASIChoice{
		CharID:   charID,
		ASIType:  "plus2",
		PlayerID: "user123",
	})

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "asi_deny:" + charID.String(),
		},
	}

	handler.HandleDMDeny(interaction)

	if !svc.denyASICalled {
		t.Error("expected DenyASI to be called")
	}
}
