package charactercard

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- Mocks ---

type mockDiscordSession struct {
	sentChannel string
	sentContent string
	sentMsg     *discordgo.Message
	sendErr     error

	editedChannel string
	editedMsgID   string
	editedContent string
	editMsg       *discordgo.Message
	editErr       error
}

func (m *mockDiscordSession) ChannelMessageSend(channelID, content string) (*discordgo.Message, error) {
	m.sentChannel = channelID
	m.sentContent = content
	if m.sendErr != nil {
		return nil, m.sendErr
	}
	if m.sentMsg != nil {
		return m.sentMsg, nil
	}
	return &discordgo.Message{ID: "msg-123"}, nil
}

func (m *mockDiscordSession) ChannelMessageEdit(channelID, messageID, content string) (*discordgo.Message, error) {
	m.editedChannel = channelID
	m.editedMsgID = messageID
	m.editedContent = content
	if m.editErr != nil {
		return nil, m.editErr
	}
	if m.editMsg != nil {
		return m.editMsg, nil
	}
	return &discordgo.Message{ID: messageID}, nil
}

type mockStore struct {
	character    refdata.Character
	characterErr error

	campaign    refdata.Campaign
	campaignErr error

	cardMsgID    sql.NullString
	cardMsgIDErr error

	setCardMsgIDParams refdata.SetCharacterCardMessageIDParams
	setCardMsgIDErr    error

	characters    []refdata.Character
	charactersErr error
}

func (m *mockStore) GetCharacter(_ context.Context, id uuid.UUID) (refdata.Character, error) {
	return m.character, m.characterErr
}

func (m *mockStore) GetCampaignByID(_ context.Context, id uuid.UUID) (refdata.Campaign, error) {
	return m.campaign, m.campaignErr
}

func (m *mockStore) GetCharacterCardMessageID(_ context.Context, id uuid.UUID) (sql.NullString, error) {
	return m.cardMsgID, m.cardMsgIDErr
}

func (m *mockStore) SetCharacterCardMessageID(_ context.Context, arg refdata.SetCharacterCardMessageIDParams) error {
	m.setCardMsgIDParams = arg
	return m.setCardMsgIDErr
}

func (m *mockStore) ListCharactersByCampaign(_ context.Context, campaignID uuid.UUID) ([]refdata.Character, error) {
	return m.characters, m.charactersErr
}

// --- Tests ---

func newTestCharacter() refdata.Character {
	classes, _ := json.Marshal([]map[string]any{
		{"class": "Fighter", "subclass": "Champion", "level": 5},
	})
	abilities, _ := json.Marshal(map[string]int{
		"str": 16, "dex": 14, "con": 14, "wis": 10, "int": 10, "cha": 10,
	})
	return refdata.Character{
		ID:         uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		CampaignID: uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		Name:       "Aria",
		Race:       "Half-Elf",
		Classes:    classes,
		Level:      5,
		AbilityScores: abilities,
		HpMax:      40,
		HpCurrent:  35,
		Ac:         16,
		SpeedFt:    30,
		EquippedMainHand: sql.NullString{String: "Longsword", Valid: true},
		Gold:       50,
		Languages:  []string{"Common", "Elvish"},
	}
}

func newTestCampaign() refdata.Campaign {
	settings, _ := json.Marshal(map[string]any{
		"channel_ids": map[string]string{
			"character-cards": "channel-123",
		},
	})
	return refdata.Campaign{
		ID:       uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		GuildID:  "guild-1",
		Settings: pqtype.NullRawMessage{RawMessage: settings, Valid: true},
	}
}

func TestService_PostCharacterCard(t *testing.T) {
	char := newTestCharacter()
	campaign := newTestCampaign()

	store := &mockStore{
		character: char,
		campaign:  campaign,
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.PostCharacterCard(context.Background(), char.ID, "Aria", "player1")
	require.NoError(t, err)

	assert.Equal(t, "channel-123", session.sentChannel)
	assert.Contains(t, session.sentContent, "Aria (AR)")
	assert.Contains(t, session.sentContent, "Half-Elf")
	assert.Equal(t, char.ID, store.setCardMsgIDParams.ID)
	assert.Equal(t, "msg-123", store.setCardMsgIDParams.CardMessageID.String)
}

func TestService_PostCharacterCard_WithSpells(t *testing.T) {
	char := newTestCharacter()

	// Add character_data with spells (DDB format)
	charData := map[string]any{
		"spells": []map[string]any{
			{"name": "Fire Bolt", "level": 0, "source": "class"},
			{"name": "Fireball", "level": 3, "source": "class"},
			{"name": "Shield", "level": 1, "source": "class"},
		},
		"prepared_spells": []string{"fireball", "shield"},
	}
	charDataJSON, _ := json.Marshal(charData)
	char.CharacterData = pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true}

	campaign := newTestCampaign()
	store := &mockStore{
		character: char,
		campaign:  campaign,
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.PostCharacterCard(context.Background(), char.ID, "Aria", "player1")
	require.NoError(t, err)

	// Card should contain spell count
	assert.Contains(t, session.sentContent, "Spells: 2 prepared / 3 known")
}

func TestService_PostCharacterCard_WithPortalSpells(t *testing.T) {
	char := newTestCharacter()

	// Portal format: array of spell IDs
	charData := map[string]any{
		"spells": []string{"fire-bolt", "magic-missile", "shield", "fireball"},
	}
	charDataJSON, _ := json.Marshal(charData)
	char.CharacterData = pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true}

	campaign := newTestCampaign()
	store := &mockStore{
		character: char,
		campaign:  campaign,
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.PostCharacterCard(context.Background(), char.ID, "Aria", "player1")
	require.NoError(t, err)

	assert.Contains(t, session.sentContent, "Spells: 4 known")
}

func TestService_PostCharacterCard_GetCharacterError(t *testing.T) {
	store := &mockStore{
		characterErr: errors.New("not found"),
		campaign:     newTestCampaign(),
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.PostCharacterCard(context.Background(), uuid.New(), "Aria", "player1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fetching character")
}

func TestService_PostCharacterCard_GetCampaignError(t *testing.T) {
	store := &mockStore{
		character:   newTestCharacter(),
		campaignErr: errors.New("not found"),
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.PostCharacterCard(context.Background(), uuid.New(), "Aria", "player1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fetching campaign")
}

func TestService_PostCharacterCard_NoChannelConfigured(t *testing.T) {
	settings, _ := json.Marshal(map[string]any{
		"channel_ids": map[string]string{},
	})
	store := &mockStore{
		character: newTestCharacter(),
		campaign: refdata.Campaign{
			ID:       uuid.MustParse("00000000-0000-0000-0000-000000000002"),
			Settings: pqtype.NullRawMessage{RawMessage: settings, Valid: true},
		},
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.PostCharacterCard(context.Background(), uuid.New(), "Aria", "player1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "character-cards channel not configured")
}

func TestService_PostCharacterCard_DiscordSendError(t *testing.T) {
	store := &mockStore{
		character: newTestCharacter(),
		campaign:  newTestCampaign(),
	}
	session := &mockDiscordSession{sendErr: errors.New("discord down")}
	svc := NewService(session, store, nil)

	err := svc.PostCharacterCard(context.Background(), uuid.New(), "Aria", "player1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sending character card")
}

func TestService_UpdateCardRetired(t *testing.T) {
	char := newTestCharacter()
	campaign := newTestCampaign()

	store := &mockStore{
		character: char,
		campaign:  campaign,
		cardMsgID: sql.NullString{String: "msg-existing", Valid: true},
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.UpdateCardRetired(context.Background(), char.ID, "Aria", "player1")
	require.NoError(t, err)

	assert.Equal(t, "channel-123", session.editedChannel)
	assert.Equal(t, "msg-existing", session.editedMsgID)
	assert.Contains(t, session.editedContent, "RETIRED")
}

func TestService_UpdateCardRetired_NoExistingMessage(t *testing.T) {
	store := &mockStore{
		character: newTestCharacter(),
		campaign:  newTestCampaign(),
		cardMsgID: sql.NullString{Valid: false},
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.UpdateCardRetired(context.Background(), uuid.New(), "Aria", "player1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no existing card message")
}

func TestService_UpdateCard(t *testing.T) {
	char := newTestCharacter()
	campaign := newTestCampaign()

	store := &mockStore{
		character: char,
		campaign:  campaign,
		cardMsgID: sql.NullString{String: "msg-existing", Valid: true},
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.UpdateCard(context.Background(), char.ID)
	require.NoError(t, err)

	assert.Equal(t, "channel-123", session.editedChannel)
	assert.Equal(t, "msg-existing", session.editedMsgID)
	assert.Contains(t, session.editedContent, "Aria (AR)")
}

func TestService_UpdateCard_NoExistingMessage(t *testing.T) {
	store := &mockStore{
		character: newTestCharacter(),
		campaign:  newTestCampaign(),
		cardMsgID: sql.NullString{Valid: false},
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.UpdateCard(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no existing card message")
}

func TestService_PostCharacterCard_NilSettings(t *testing.T) {
	store := &mockStore{
		character: newTestCharacter(),
		campaign: refdata.Campaign{
			ID: uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		},
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.PostCharacterCard(context.Background(), uuid.New(), "Aria", "player1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "character-cards channel not configured")
}

func TestService_PostCharacterCard_SpellSlots(t *testing.T) {
	char := newTestCharacter()
	spellSlots, _ := json.Marshal(map[string]map[string]int{
		"1": {"current": 3, "max": 4},
	})
	char.SpellSlots = pqtype.NullRawMessage{RawMessage: spellSlots, Valid: true}

	store := &mockStore{
		character: char,
		campaign:  newTestCampaign(),
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.PostCharacterCard(context.Background(), char.ID, "Aria", "player1")
	require.NoError(t, err)
	assert.Contains(t, session.sentContent, "1st: 3/4")
}

func TestService_UpdateCardRetired_EditError(t *testing.T) {
	store := &mockStore{
		character: newTestCharacter(),
		campaign:  newTestCampaign(),
		cardMsgID: sql.NullString{String: "msg-existing", Valid: true},
	}
	session := &mockDiscordSession{editErr: errors.New("discord down")}
	svc := NewService(session, store, nil)

	err := svc.UpdateCardRetired(context.Background(), uuid.New(), "Aria", "player1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "editing character card")
}

func TestService_ShortID_WithExistingNames(t *testing.T) {
	// Aria has a lower ID than Arthur, so Aria gets "AR" and Arthur gets "AR2"
	char := newTestCharacter()
	char.ID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	char.Name = "Aria"

	otherChar := newTestCharacter()
	otherChar.ID = uuid.MustParse("00000000-0000-0000-0000-000000000099")
	otherChar.Name = "Arthur"

	store := &mockStore{
		character:  char,
		campaign:   newTestCampaign(),
		characters: []refdata.Character{otherChar, char},
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.PostCharacterCard(context.Background(), char.ID, "Aria", "player1")
	require.NoError(t, err)
	// Aria (lower ID) gets the base "AR"
	assert.Contains(t, session.sentContent, "Aria (AR)")
}

func TestService_ShortID_SymmetricCollision(t *testing.T) {
	// When two characters have the same base initials (e.g., "Aria" and "Arthur" both produce "AR"),
	// they should get different stable short IDs regardless of which one we query.
	// The first by character ID should get "AR", the second should get "AR2".
	ariaID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	arthurID := uuid.MustParse("00000000-0000-0000-0000-000000000099")

	ariaChar := newTestCharacter()
	ariaChar.ID = ariaID
	ariaChar.Name = "Aria"

	arthurChar := newTestCharacter()
	arthurChar.ID = arthurID
	arthurChar.Name = "Arthur"

	allChars := []refdata.Character{ariaChar, arthurChar}

	// Post Aria's card
	ariaStore := &mockStore{
		character:  ariaChar,
		campaign:   newTestCampaign(),
		characters: allChars,
	}
	ariaSession := &mockDiscordSession{}
	ariaSvc := NewService(ariaSession, ariaStore, nil)
	err := ariaSvc.PostCharacterCard(context.Background(), ariaID, "Aria", "player1")
	require.NoError(t, err)

	// Post Arthur's card
	arthurStore := &mockStore{
		character:  arthurChar,
		campaign:   newTestCampaign(),
		characters: allChars,
	}
	arthurSession := &mockDiscordSession{}
	arthurSvc := NewService(arthurSession, arthurStore, nil)
	err = arthurSvc.PostCharacterCard(context.Background(), arthurID, "Arthur", "player2")
	require.NoError(t, err)

	// They must get different short IDs
	ariaShort := extractShortID(ariaSession.sentContent)
	arthurShort := extractShortID(arthurSession.sentContent)
	assert.NotEqual(t, ariaShort, arthurShort, "Aria and Arthur must get different short IDs")

	// Aria (lower ID) should get "AR", Arthur should get "AR2"
	assert.Equal(t, "AR", ariaShort, "Aria (lower ID) should get the base short ID")
	assert.Equal(t, "AR2", arthurShort, "Arthur (higher ID) should get the suffixed short ID")
}

// extractShortID pulls the short ID from a formatted card content string.
// The format is "⚔️ Name (SHORTID) — ..."
func extractShortID(content string) string {
	start := strings.Index(content, "(")
	end := strings.Index(content, ")")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return content[start+1 : end]
}

func TestService_UpdateCard_GetCardMsgIDError(t *testing.T) {
	store := &mockStore{
		character:    newTestCharacter(),
		campaign:     newTestCampaign(),
		cardMsgIDErr: errors.New("db error"),
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.UpdateCard(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fetching card message ID")
}

func TestService_UpdateCard_EditError(t *testing.T) {
	store := &mockStore{
		character: newTestCharacter(),
		campaign:  newTestCampaign(),
		cardMsgID: sql.NullString{String: "msg-existing", Valid: true},
	}
	session := &mockDiscordSession{editErr: errors.New("discord down")}
	svc := NewService(session, store, nil)

	err := svc.UpdateCard(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "editing character card")
}

func TestService_UpdateCardRetired_GetCardMsgIDError(t *testing.T) {
	store := &mockStore{
		character:    newTestCharacter(),
		campaign:     newTestCampaign(),
		cardMsgIDErr: errors.New("db error"),
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.UpdateCardRetired(context.Background(), uuid.New(), "Aria", "player1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fetching card message ID")
}

func TestService_GenerateShortID_ListError(t *testing.T) {
	store := &mockStore{
		character:     newTestCharacter(),
		campaign:      newTestCampaign(),
		charactersErr: errors.New("db error"),
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.PostCharacterCard(context.Background(), uuid.New(), "Aria", "player1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "listing characters")
}

func TestService_PostCharacterCard_InvalidSettings(t *testing.T) {
	store := &mockStore{
		character: newTestCharacter(),
		campaign: refdata.Campaign{
			ID:       uuid.MustParse("00000000-0000-0000-0000-000000000002"),
			Settings: pqtype.NullRawMessage{RawMessage: []byte(`{invalid}`), Valid: true},
		},
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.PostCharacterCard(context.Background(), uuid.New(), "Aria", "player1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing campaign settings")
}

func TestService_PostCharacterCard_WithOffHand(t *testing.T) {
	char := newTestCharacter()
	char.EquippedOffHand = sql.NullString{String: "Shield", Valid: true}

	store := &mockStore{
		character: char,
		campaign:  newTestCampaign(),
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.PostCharacterCard(context.Background(), char.ID, "Aria", "player1")
	require.NoError(t, err)
	assert.Contains(t, session.sentContent, "Shield (off-hand)")
}

func TestService_UpdateCardRetired_GenerateShortIDError(t *testing.T) {
	store := &mockStore{
		character:     newTestCharacter(),
		campaign:      newTestCampaign(),
		cardMsgID:     sql.NullString{String: "msg-existing", Valid: true},
		charactersErr: errors.New("db error"),
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.UpdateCardRetired(context.Background(), uuid.New(), "Aria", "player1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "listing characters")
}

func TestService_UpdateCard_GenerateShortIDError(t *testing.T) {
	store := &mockStore{
		character:     newTestCharacter(),
		campaign:      newTestCampaign(),
		cardMsgID:     sql.NullString{String: "msg-existing", Valid: true},
		charactersErr: errors.New("db error"),
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.UpdateCard(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "listing characters")
}

func TestService_OnCharacterUpdated_CallsUpdateCard(t *testing.T) {
	char := newTestCharacter()
	store := &mockStore{
		character: char,
		campaign:  newTestCampaign(),
		cardMsgID: sql.NullString{String: "msg-existing", Valid: true},
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.OnCharacterUpdated(context.Background(), char.ID)
	require.NoError(t, err)

	// Should have edited the existing message
	assert.Equal(t, "channel-123", session.editedChannel)
	assert.Equal(t, "msg-existing", session.editedMsgID)
	assert.Contains(t, session.editedContent, "Aria (AR)")
}

func TestService_OnCharacterUpdated_NoCardMessage_NoError(t *testing.T) {
	// If no card message exists yet (character not yet approved), OnCharacterUpdated should be a no-op
	char := newTestCharacter()
	store := &mockStore{
		character: char,
		campaign:  newTestCampaign(),
		cardMsgID: sql.NullString{Valid: false},
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.OnCharacterUpdated(context.Background(), char.ID)
	require.NoError(t, err)

	// Should NOT have tried to edit anything
	assert.Empty(t, session.editedChannel)
}

func TestService_OnCharacterUpdated_GetCardMsgIDError_SilentNoOp(t *testing.T) {
	// When GetCharacterCardMessageID fails, OnCharacterUpdated should log and return nil (no-op)
	store := &mockStore{
		character:    newTestCharacter(),
		campaign:     newTestCampaign(),
		cardMsgIDErr: errors.New("db error"),
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.OnCharacterUpdated(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, session.editedChannel)
}

func TestService_OnCharacterUpdated_UpdateCardError(t *testing.T) {
	// When UpdateCard itself fails, the error should propagate
	store := &mockStore{
		character: newTestCharacter(),
		campaign:  newTestCampaign(),
		cardMsgID: sql.NullString{String: "msg-existing", Valid: true},
	}
	session := &mockDiscordSession{editErr: errors.New("discord down")}
	svc := NewService(session, store, nil)

	err := svc.OnCharacterUpdated(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "editing character card")
}

func TestService_SetCardMsgID_Error(t *testing.T) {
	store := &mockStore{
		character:       newTestCharacter(),
		campaign:        newTestCampaign(),
		setCardMsgIDErr: errors.New("db error"),
	}
	session := &mockDiscordSession{}
	svc := NewService(session, store, nil)

	err := svc.PostCharacterCard(context.Background(), uuid.New(), "Aria", "player1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "storing card message ID")
}
