package discord

import (
	"fmt"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandDefinitions_IncludesSetup(t *testing.T) {
	cmds := CommandDefinitions()
	var found bool
	for _, cmd := range cmds {
		if cmd.Name == "setup" {
			found = true
			assert.Equal(t, "Create the full channel structure for this campaign", cmd.Description)
			require.NotNil(t, cmd.DefaultMemberPermissions)
			assert.Equal(t, int64(discordgo.PermissionManageChannels), *cmd.DefaultMemberPermissions)
			break
		}
	}
	assert.True(t, found, "expected /setup command in definitions")
}

func TestChannelStructure_HasExpectedCategories(t *testing.T) {
	structure := ChannelStructure()
	categoryNames := make([]string, 0, len(structure))
	for _, cat := range structure {
		categoryNames = append(categoryNames, cat.Name)
	}
	assert.Equal(t, []string{"SYSTEM", "NARRATION", "COMBAT", "REFERENCE"}, categoryNames)
}

func TestChannelStructure_SystemChannels(t *testing.T) {
	structure := ChannelStructure()
	system := structure[0]
	assert.Equal(t, "SYSTEM", system.Name)
	channelNames := channelNamesFrom(system.Channels)
	assert.Equal(t, []string{"initiative-tracker", "combat-log", "roll-history"}, channelNames)
}

func TestChannelStructure_NarrationChannels(t *testing.T) {
	structure := ChannelStructure()
	narration := structure[1]
	assert.Equal(t, "NARRATION", narration.Name)
	channelNames := channelNamesFrom(narration.Channels)
	assert.Equal(t, []string{"the-story", "in-character", "player-chat"}, channelNames)
}

func TestChannelStructure_CombatChannels(t *testing.T) {
	structure := ChannelStructure()
	combat := structure[2]
	assert.Equal(t, "COMBAT", combat.Name)
	channelNames := channelNamesFrom(combat.Channels)
	assert.Equal(t, []string{"combat-map", "your-turn"}, channelNames)
}

func TestChannelStructure_ReferenceChannels(t *testing.T) {
	structure := ChannelStructure()
	reference := structure[3]
	assert.Equal(t, "REFERENCE", reference.Name)
	channelNames := channelNamesFrom(reference.Channels)
	assert.Equal(t, []string{"character-cards", "dm-queue"}, channelNames)
}

func channelNamesFrom(defs []ChannelDef) []string {
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Name
	}
	return names
}

func TestSetupChannels_CreatesAllChannelsAndCategories(t *testing.T) {
	mock := newTestMock()
	mock.GuildChannelsFunc = func(guildID string) ([]*discordgo.Channel, error) {
		return nil, nil // no existing channels
	}

	var createdNames []string
	channelIDCounter := 0
	mock.GuildChannelCreateComplexFunc = func(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
		channelIDCounter++
		createdNames = append(createdNames, data.Name)
		return &discordgo.Channel{
			ID:   fmt.Sprintf("chan-%d", channelIDCounter),
			Name: data.Name,
			Type: data.Type,
		}, nil
	}

	result, err := SetupChannels(mock, "guild-1", "bot-user-1", "dm-user-1")
	require.NoError(t, err)

	// 4 categories + 10 channels = 14 total creates
	assert.Equal(t, 14, len(createdNames))

	// Verify result contains all channel IDs
	expectedChannels := []string{
		"initiative-tracker", "combat-log", "roll-history",
		"the-story", "in-character", "player-chat",
		"combat-map", "your-turn",
		"character-cards", "dm-queue",
	}
	for _, name := range expectedChannels {
		assert.Contains(t, result, name, "expected channel %s in result map", name)
		assert.NotEmpty(t, result[name], "expected non-empty ID for channel %s", name)
	}
}

func TestSetupChannels_SkipsExistingChannels(t *testing.T) {
	mock := newTestMock()
	mock.GuildChannelsFunc = func(guildID string) ([]*discordgo.Channel, error) {
		return []*discordgo.Channel{
			{ID: "existing-cat", Name: "SYSTEM", Type: discordgo.ChannelTypeGuildCategory},
			{ID: "existing-chan", Name: "initiative-tracker", Type: discordgo.ChannelTypeGuildText, ParentID: "existing-cat"},
		}, nil
	}

	var createdNames []string
	channelIDCounter := 0
	mock.GuildChannelCreateComplexFunc = func(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
		channelIDCounter++
		createdNames = append(createdNames, data.Name)
		return &discordgo.Channel{
			ID:   fmt.Sprintf("new-%d", channelIDCounter),
			Name: data.Name,
			Type: data.Type,
		}, nil
	}

	result, err := SetupChannels(mock, "guild-1", "bot-user-1", "dm-user-1")
	require.NoError(t, err)

	// Should NOT have created SYSTEM category or initiative-tracker (they already exist)
	assert.NotContains(t, createdNames, "SYSTEM")
	assert.NotContains(t, createdNames, "initiative-tracker")

	// Should still have the existing channel ID in the result
	assert.Equal(t, "existing-chan", result["initiative-tracker"])
}

// captureOverwrites runs SetupChannels and returns the permission overwrites
// applied to the named channel.
func captureOverwrites(t *testing.T, channelName string) []*discordgo.PermissionOverwrite {
	t.Helper()
	mock := newTestMock()
	mock.GuildChannelsFunc = func(guildID string) ([]*discordgo.Channel, error) {
		return nil, nil
	}

	var overwrites []*discordgo.PermissionOverwrite
	channelIDCounter := 0
	mock.GuildChannelCreateComplexFunc = func(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
		channelIDCounter++
		if data.Name == channelName {
			overwrites = data.PermissionOverwrites
		}
		return &discordgo.Channel{
			ID:   fmt.Sprintf("chan-%d", channelIDCounter),
			Name: data.Name,
			Type: data.Type,
		}, nil
	}

	_, err := SetupChannels(mock, "guild-1", "bot-user-1", "dm-user-1")
	require.NoError(t, err)
	return overwrites
}

// assertExclusiveWrite checks that the overwrites deny @everyone SendMessages
// and allow the specified user SendMessages.
func assertExclusiveWrite(t *testing.T, overwrites []*discordgo.PermissionOverwrite, allowedUserID, label string) {
	t.Helper()
	require.NotEmpty(t, overwrites, "expected permission overwrites on #%s", label)

	var everyoneDeny, userAllow bool
	for _, ow := range overwrites {
		if ow.ID == "guild-1" && ow.Type == discordgo.PermissionOverwriteTypeRole && ow.Deny&discordgo.PermissionSendMessages != 0 {
			everyoneDeny = true
		}
		if ow.ID == allowedUserID && ow.Type == discordgo.PermissionOverwriteTypeMember && ow.Allow&discordgo.PermissionSendMessages != 0 {
			userAllow = true
		}
	}
	assert.True(t, everyoneDeny, "@everyone should be denied SendMessages in #%s", label)
	assert.True(t, userAllow, "%s should be allowed SendMessages in #%s", allowedUserID, label)
}

func TestSetupChannels_TheStoryIsDMWriteOnly(t *testing.T) {
	overwrites := captureOverwrites(t, "the-story")
	assertExclusiveWrite(t, overwrites, "dm-user-1", "the-story")
}

func TestSetupChannels_CombatMapIsBotWriteOnly(t *testing.T) {
	overwrites := captureOverwrites(t, "combat-map")
	assertExclusiveWrite(t, overwrites, "bot-user-1", "combat-map")
}

func TestSetupChannels_InCharacterIsPlayerAndDMWritable(t *testing.T) {
	overwrites := captureOverwrites(t, "in-character")

	// #in-character should have no @everyone deny (everyone can write by default)
	for _, ow := range overwrites {
		if ow.ID == "guild-1" && ow.Type == discordgo.PermissionOverwriteTypeRole {
			assert.Zero(t, ow.Deny&discordgo.PermissionSendMessages,
				"@everyone should NOT be denied SendMessages in #in-character")
		}
	}
}

func TestSetupChannels_GuildChannelsError(t *testing.T) {
	mock := newTestMock()
	mock.GuildChannelsFunc = func(guildID string) ([]*discordgo.Channel, error) {
		return nil, fmt.Errorf("api error")
	}

	_, err := SetupChannels(mock, "guild-1", "bot-user-1", "dm-user-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching guild channels")
}

func TestSetupChannels_CreateCategoryError(t *testing.T) {
	mock := newTestMock()
	mock.GuildChannelsFunc = func(guildID string) ([]*discordgo.Channel, error) {
		return nil, nil
	}
	mock.GuildChannelCreateComplexFunc = func(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
		if data.Type == discordgo.ChannelTypeGuildCategory {
			return nil, fmt.Errorf("category create failed")
		}
		return &discordgo.Channel{ID: "chan-1", Name: data.Name}, nil
	}

	_, err := SetupChannels(mock, "guild-1", "bot-user-1", "dm-user-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating category")
}

func TestSetupChannels_CreateChannelError(t *testing.T) {
	mock := newTestMock()
	mock.GuildChannelsFunc = func(guildID string) ([]*discordgo.Channel, error) {
		return nil, nil
	}
	callCount := 0
	mock.GuildChannelCreateComplexFunc = func(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
		callCount++
		if data.Type == discordgo.ChannelTypeGuildText {
			return nil, fmt.Errorf("channel create failed")
		}
		return &discordgo.Channel{ID: fmt.Sprintf("cat-%d", callCount), Name: data.Name, Type: data.Type}, nil
	}

	_, err := SetupChannels(mock, "guild-1", "bot-user-1", "dm-user-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating channel")
}

func TestSetupChannels_SkipsExistingChannelsInNewCategory(t *testing.T) {
	// Category exists but some channels are missing
	mock := newTestMock()
	mock.GuildChannelsFunc = func(guildID string) ([]*discordgo.Channel, error) {
		return []*discordgo.Channel{
			{ID: "cat-system", Name: "SYSTEM", Type: discordgo.ChannelTypeGuildCategory},
			{ID: "chan-init", Name: "initiative-tracker", Type: discordgo.ChannelTypeGuildText, ParentID: "cat-system"},
			// combat-log and roll-history are missing
		}, nil
	}

	var createdNames []string
	channelIDCounter := 0
	mock.GuildChannelCreateComplexFunc = func(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
		channelIDCounter++
		createdNames = append(createdNames, data.Name)
		return &discordgo.Channel{
			ID:   fmt.Sprintf("new-%d", channelIDCounter),
			Name: data.Name,
			Type: data.Type,
		}, nil
	}

	result, err := SetupChannels(mock, "guild-1", "bot-user-1", "dm-user-1")
	require.NoError(t, err)

	// initiative-tracker should be skipped (exists)
	assert.NotContains(t, createdNames, "initiative-tracker")
	assert.Equal(t, "chan-init", result["initiative-tracker"])

	// combat-log and roll-history should be created
	assert.Contains(t, createdNames, "combat-log")
	assert.Contains(t, createdNames, "roll-history")
}

func TestSetupChannels_CategoriesCreatedWithCorrectType(t *testing.T) {
	mock := newTestMock()
	mock.GuildChannelsFunc = func(guildID string) ([]*discordgo.Channel, error) {
		return nil, nil
	}

	var categoryTypes []discordgo.ChannelType
	channelIDCounter := 0
	mock.GuildChannelCreateComplexFunc = func(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
		channelIDCounter++
		if data.Type == discordgo.ChannelTypeGuildCategory {
			categoryTypes = append(categoryTypes, data.Type)
		}
		return &discordgo.Channel{
			ID:   fmt.Sprintf("chan-%d", channelIDCounter),
			Name: data.Name,
			Type: data.Type,
		}, nil
	}

	_, err := SetupChannels(mock, "guild-1", "bot-user-1", "dm-user-1")
	require.NoError(t, err)

	// All 4 categories should be created as ChannelTypeGuildCategory
	assert.Len(t, categoryTypes, 4)
	for _, ct := range categoryTypes {
		assert.Equal(t, discordgo.ChannelTypeGuildCategory, ct)
	}
}

func TestSetupChannels_ChannelsHaveCorrectParentID(t *testing.T) {
	mock := newTestMock()
	mock.GuildChannelsFunc = func(guildID string) ([]*discordgo.Channel, error) {
		return nil, nil
	}

	parentIDs := make(map[string]string) // channel name -> parent ID
	channelIDCounter := 0
	mock.GuildChannelCreateComplexFunc = func(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
		channelIDCounter++
		if data.Type == discordgo.ChannelTypeGuildText {
			parentIDs[data.Name] = data.ParentID
		}
		return &discordgo.Channel{
			ID:   fmt.Sprintf("chan-%d", channelIDCounter),
			Name: data.Name,
			Type: data.Type,
		}, nil
	}

	_, err := SetupChannels(mock, "guild-1", "bot-user-1", "dm-user-1")
	require.NoError(t, err)

	// All text channels should have a parent ID set
	for name, pid := range parentIDs {
		assert.NotEmpty(t, pid, "channel %s should have a parent ID", name)
	}
}

// HandleSetupCommand tests

// setupMockResponder configures a mock to capture the final interaction response edit content.
// Returns a pointer to the captured content string.
func setupMockResponder(mock *MockSession) *string {
	var content string
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		return nil
	}
	mock.InteractionResponseEditFunc = func(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit) (*discordgo.Message, error) {
		if newresp.Content != nil {
			content = *newresp.Content
		}
		return &discordgo.Message{}, nil
	}
	return &content
}

type mockCampaignLookup struct {
	getCampaignFunc    func(guildID string) (SetupCampaignInfo, error)
	updateSettingsFunc func(guildID string, channelIDs map[string]string) error
}

func (m *mockCampaignLookup) GetCampaignForSetup(guildID string) (SetupCampaignInfo, error) {
	if m.getCampaignFunc != nil {
		return m.getCampaignFunc(guildID)
	}
	return SetupCampaignInfo{DMUserID: "dm-user-1"}, nil
}

func (m *mockCampaignLookup) SaveChannelIDs(guildID string, channelIDs map[string]string) error {
	if m.updateSettingsFunc != nil {
		return m.updateSettingsFunc(guildID, channelIDs)
	}
	return nil
}

func TestHandleSetupCommand_Success(t *testing.T) {
	mock := newTestMock()
	mock.GuildChannelsFunc = func(guildID string) ([]*discordgo.Channel, error) {
		return nil, nil
	}
	channelIDCounter := 0
	mock.GuildChannelCreateComplexFunc = func(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
		channelIDCounter++
		return &discordgo.Channel{
			ID:   fmt.Sprintf("chan-%d", channelIDCounter),
			Name: data.Name,
			Type: data.Type,
		}, nil
	}

	var deferredResponse bool
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Type == discordgo.InteractionResponseDeferredChannelMessageWithSource {
			deferredResponse = true
		}
		return nil
	}

	var editContent string
	mock.InteractionResponseEditFunc = func(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit) (*discordgo.Message, error) {
		if newresp.Content != nil {
			editContent = *newresp.Content
		}
		return &discordgo.Message{}, nil
	}

	var savedChannelIDs map[string]string
	campaignLookup := &mockCampaignLookup{
		updateSettingsFunc: func(guildID string, channelIDs map[string]string) error {
			savedChannelIDs = channelIDs
			return nil
		},
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	handler := NewSetupHandler(bot, campaignLookup)
	handler.Handle(&discordgo.Interaction{GuildID: "guild-1"})

	assert.True(t, deferredResponse, "should send deferred response")
	assert.Contains(t, editContent, "created")
	assert.NotNil(t, savedChannelIDs, "should save channel IDs")
	assert.Len(t, savedChannelIDs, 10) // 10 text channels
}

func TestHandleSetupCommand_NoCampaign(t *testing.T) {
	mock := newTestMock()
	editContent := setupMockResponder(mock)

	campaignLookup := &mockCampaignLookup{
		getCampaignFunc: func(guildID string) (SetupCampaignInfo, error) {
			return SetupCampaignInfo{}, fmt.Errorf("no campaign found")
		},
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	handler := NewSetupHandler(bot, campaignLookup)
	handler.Handle(&discordgo.Interaction{GuildID: "guild-1"})

	assert.Contains(t, *editContent, "no campaign")
}

func TestHandleSetupCommand_SetupError(t *testing.T) {
	mock := newTestMock()
	mock.GuildChannelsFunc = func(guildID string) ([]*discordgo.Channel, error) {
		return nil, fmt.Errorf("api error")
	}
	editContent := setupMockResponder(mock)

	bot := NewBot(mock, "app-1", newTestLogger())
	handler := NewSetupHandler(bot, &mockCampaignLookup{})
	handler.Handle(&discordgo.Interaction{GuildID: "guild-1"})

	assert.Contains(t, *editContent, "Failed")
}

func TestHandleSetupCommand_SaveError(t *testing.T) {
	mock := newTestMock()
	mock.GuildChannelsFunc = func(guildID string) ([]*discordgo.Channel, error) {
		return nil, nil
	}
	channelIDCounter := 0
	mock.GuildChannelCreateComplexFunc = func(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
		channelIDCounter++
		return &discordgo.Channel{
			ID:   fmt.Sprintf("chan-%d", channelIDCounter),
			Name: data.Name,
			Type: data.Type,
		}, nil
	}
	editContent := setupMockResponder(mock)

	campaignLookup := &mockCampaignLookup{
		updateSettingsFunc: func(guildID string, channelIDs map[string]string) error {
			return fmt.Errorf("db error")
		},
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	handler := NewSetupHandler(bot, campaignLookup)
	handler.Handle(&discordgo.Interaction{GuildID: "guild-1"})

	assert.Contains(t, *editContent, "created")
	assert.Contains(t, *editContent, "failed to save")
}
