package discord

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

// mockCharacterLookup implements CharacterLookup for testing.
type mockCharacterLookup struct {
	pc      refdata.PlayerCharacter
	pcErr   error
	char    refdata.Character
	charErr error
}

func (m *mockCharacterLookup) GetPlayerCharacterByDiscordUser(_ context.Context, _ refdata.GetPlayerCharacterByDiscordUserParams) (refdata.PlayerCharacter, error) {
	return m.pc, m.pcErr
}

func (m *mockCharacterLookup) GetCharacter(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
	return m.char, m.charErr
}

// captureFullResponse captures the full interaction response including embeds.
type fullResponseCapture struct {
	Content string
	Embeds  []*discordgo.MessageEmbed
	Flags   discordgo.MessageFlags
}

func captureFullResponse(mock *MockSession) *fullResponseCapture {
	rc := &fullResponseCapture{}
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Data != nil {
			rc.Content = resp.Data.Content
			rc.Embeds = resp.Data.Embeds
			rc.Flags = resp.Data.Flags
		}
		return nil
	}
	return rc
}

func TestCharacterHandler_Success(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	charID := uuid.New()
	campID := uuid.New()

	scoresJSON, _ := json.Marshal(character.AbilityScores{STR: 16, DEX: 14, CON: 12, INT: 10, WIS: 8, CHA: 13})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 5}})

	lookup := &mockCharacterLookup{
		pc: refdata.PlayerCharacter{
			CharacterID:   charID,
			CampaignID:    campID,
			DiscordUserID: "player-1",
			Status:        "approved",
		},
		char: refdata.Character{
			ID:               charID,
			CampaignID:       campID,
			Name:             "Thorn",
			Race:             "Human",
			Level:            5,
			Classes:          classesJSON,
			AbilityScores:    scoresJSON,
			HpMax:            42,
			HpCurrent:        35,
			TempHp:           0,
			Ac:               18,
			SpeedFt:          30,
			ProficiencyBonus: 3,
			EquippedMainHand: sql.NullString{String: "Longsword", Valid: true},
			Languages:        []string{"Common", "Elvish"},
			Gold:             50,
		},
	}

	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.dndnd.app")
	handler.Handle(makeInteraction("character", "player-1", "guild-1"))

	// Should respond with an embed
	if rc.Embeds == nil || len(rc.Embeds) == 0 {
		t.Fatal("expected embeds in response")
	}

	embed := rc.Embeds[0]
	if !strings.Contains(embed.Title, "Thorn") {
		t.Errorf("expected embed title to contain 'Thorn', got: %s", embed.Title)
	}
	if !strings.Contains(embed.Description, "HP: 35/42") {
		t.Errorf("expected HP in description, got: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "AC: 18") {
		t.Errorf("expected AC in description, got: %s", embed.Description)
	}

	// Should contain portal link in content
	if !strings.Contains(rc.Content, "https://portal.dndnd.app/portal/character/") {
		t.Errorf("expected portal link in content, got: %s", rc.Content)
	}

	// Should be ephemeral
	if rc.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Error("expected ephemeral response")
	}
}

func TestCharacterHandler_NoCampaign(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	campProv := &mockCampaignProvider{
		GetCampaignByGuildIDFunc: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{}, sql.ErrNoRows
		},
	}
	handler := NewCharacterHandler(mock, campProv, nil, "https://portal.dndnd.app")
	handler.Handle(makeInteraction("character", "player-1", "guild-1"))

	if !strings.Contains(rc.Content, "No campaign found") {
		t.Errorf("expected no-campaign message, got: %s", rc.Content)
	}
}

func TestCharacterHandler_NoCharacter(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	lookup := &mockCharacterLookup{
		pcErr: sql.ErrNoRows,
	}

	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.dndnd.app")
	handler.Handle(makeInteraction("character", "player-1", "guild-1"))

	if !strings.Contains(rc.Content, "No character found") {
		t.Errorf("expected no-character message, got: %s", rc.Content)
	}
}

func TestCharacterHandler_NotApproved(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	lookup := &mockCharacterLookup{
		pc: refdata.PlayerCharacter{
			Status:        "pending",
			CharacterID:   uuid.New(),
			DiscordUserID: "player-1",
		},
	}

	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.dndnd.app")
	handler.Handle(makeInteraction("character", "player-1", "guild-1"))

	if !strings.Contains(rc.Content, "pending") {
		t.Errorf("expected pending message, got: %s", rc.Content)
	}
}

func TestCharacterHandler_CharacterLoadError(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	lookup := &mockCharacterLookup{
		pc: refdata.PlayerCharacter{
			Status:        "approved",
			CharacterID:   uuid.New(),
			DiscordUserID: "player-1",
		},
		charErr: sql.ErrConnDone,
	}

	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.dndnd.app")
	handler.Handle(makeInteraction("character", "player-1", "guild-1"))

	if !strings.Contains(rc.Content, "load your character") {
		t.Errorf("expected error message, got: %s", rc.Content)
	}
}

func TestCommandRouter_SetCharacterHandler(t *testing.T) {
	mock := newTestMock()

	charID := uuid.New()
	scoresJSON, _ := json.Marshal(character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 1}})

	lookup := &mockCharacterLookup{
		pc: refdata.PlayerCharacter{
			CharacterID:   charID,
			DiscordUserID: "player-1",
			Status:        "approved",
		},
		char: refdata.Character{
			ID:            charID,
			Name:          "Test",
			Level:         1,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
		},
	}

	bot := &Bot{session: mock}
	router := NewCommandRouter(bot, nil)
	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.test")
	router.SetCharacterHandler(handler)

	rc := captureFullResponse(mock)
	router.Handle(makeInteraction("character", "player-1", "guild-1"))

	// Should have used the real handler, not stub
	if rc.Embeds == nil || len(rc.Embeds) == 0 {
		t.Fatal("expected embeds from character handler, got stub response")
	}
}
