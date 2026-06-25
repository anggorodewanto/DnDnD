package discord

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
)

// mockCharacterLookup implements CharacterLookup for testing.
type mockCharacterLookup struct {
	pc        refdata.PlayerCharacter
	pcErr     error
	char      refdata.Character
	charErr   error
	spells    []refdata.Spell
	spellsErr error
}

func (m *mockCharacterLookup) GetPlayerCharacterByDiscordUser(_ context.Context, _ refdata.GetPlayerCharacterByDiscordUserParams) (refdata.PlayerCharacter, error) {
	return m.pc, m.pcErr
}

func (m *mockCharacterLookup) GetCharacter(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
	return m.char, m.charErr
}

func (m *mockCharacterLookup) GetSpellsByIDs(_ context.Context, _ []string) ([]refdata.Spell, error) {
	return m.spells, m.spellsErr
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
	if len(rc.Embeds) == 0 {
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

func TestCharacterHandler_WithSpells(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	charID := uuid.New()
	campID := uuid.New()

	scoresJSON, _ := json.Marshal(character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Wizard", Level: 5}})

	// character_data with spells (DDB format)
	type ddbSpell struct {
		Name   string `json:"name"`
		Level  int    `json:"level"`
		Source string `json:"source"`
	}
	charData := map[string]any{"spells": []ddbSpell{
		{Name: "Fire Bolt", Level: 0, Source: "class"},
		{Name: "Mage Hand", Level: 0, Source: "class"},
		{Name: "Magic Missile", Level: 1, Source: "class"},
		{Name: "Shield", Level: 1, Source: "class"},
		{Name: "Fireball", Level: 3, Source: "class"},
	}}
	charDataJSON, _ := json.Marshal(charData)

	lookup := &mockCharacterLookup{
		pc: refdata.PlayerCharacter{
			CharacterID:   charID,
			CampaignID:    campID,
			DiscordUserID: "player-1",
			Status:        "approved",
		},
		char: refdata.Character{
			ID:            charID,
			CampaignID:    campID,
			Name:          "Gandalf",
			Race:          "Elf",
			Level:         5,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			HpMax:         22,
			HpCurrent:     22,
			Ac:            12,
			SpeedFt:       30,
			Languages:     []string{"Common"},
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		},
	}

	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.dndnd.app")
	handler.Handle(makeInteraction("character", "player-1", "guild-1"))

	if len(rc.Embeds) == 0 {
		t.Fatal("expected embeds in response")
	}

	embed := rc.Embeds[0]
	// Should contain spell count summary
	if !strings.Contains(embed.Description, "Spells") {
		t.Errorf("expected spell summary in description, got: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "Cantrips: 2") {
		t.Errorf("expected 'Cantrips: 2' in description, got: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "1st: 2") {
		t.Errorf("expected '1st: 2' in description, got: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "3rd: 1") {
		t.Errorf("expected '3rd: 1' in description, got: %s", embed.Description)
	}
}

func TestCharacterHandler_WithPortalSpells(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	charID := uuid.New()
	campID := uuid.New()

	scoresJSON, _ := json.Marshal(character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Wizard", Level: 3}})

	// Portal format: array of spell IDs (strings)
	charData := map[string]any{"spells": []string{"fire-bolt", "magic-missile", "shield"}}
	charDataJSON, _ := json.Marshal(charData)

	lookup := &mockCharacterLookup{
		pc: refdata.PlayerCharacter{
			CharacterID:   charID,
			CampaignID:    campID,
			DiscordUserID: "player-1",
			Status:        "approved",
		},
		char: refdata.Character{
			ID:            charID,
			CampaignID:    campID,
			Name:          "Wizard",
			Race:          "Elf",
			Level:         3,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		},
		spells: []refdata.Spell{
			{ID: "fire-bolt", Name: "Fire Bolt", Level: 0, School: "Evocation", CastingTime: "1 action", RangeType: "ranged"},
			{ID: "magic-missile", Name: "Magic Missile", Level: 1, School: "Evocation", CastingTime: "1 action", RangeType: "ranged"},
			{ID: "shield", Name: "Shield", Level: 1, School: "Abjuration", CastingTime: "1 reaction", RangeType: "self"},
		},
	}

	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.test")
	handler.Handle(makeInteraction("character", "player-1", "guild-1"))

	if len(rc.Embeds) == 0 {
		t.Fatal("expected embeds in response")
	}

	embed := rc.Embeds[0]
	// Portal spells should now show level-based breakdown after enrichment
	if !strings.Contains(embed.Description, "Cantrips: 1") {
		t.Errorf("expected 'Cantrips: 1' for enriched portal spells, got: %s", embed.Description)
	}
	if !strings.Contains(embed.Description, "1st: 2") {
		t.Errorf("expected '1st: 2' for enriched portal spells, got: %s", embed.Description)
	}
}

func TestCharacterHandler_WithPortalSpells_EnrichmentError(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	charID := uuid.New()
	campID := uuid.New()

	scoresJSON, _ := json.Marshal(character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Wizard", Level: 3}})

	charData := map[string]any{"spells": []string{"fire-bolt", "magic-missile"}}
	charDataJSON, _ := json.Marshal(charData)

	lookup := &mockCharacterLookup{
		pc: refdata.PlayerCharacter{
			CharacterID:   charID,
			CampaignID:    campID,
			DiscordUserID: "player-1",
			Status:        "approved",
		},
		char: refdata.Character{
			ID:            charID,
			CampaignID:    campID,
			Name:          "Wizard",
			Race:          "Elf",
			Level:         3,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		},
		spellsErr: fmt.Errorf("db error"),
	}

	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.test")
	handler.Handle(makeInteraction("character", "player-1", "guild-1"))

	if len(rc.Embeds) == 0 {
		t.Fatal("expected embeds in response")
	}

	embed := rc.Embeds[0]
	// When enrichment fails, portal spells fall back to Cantrips (level 0)
	if !strings.Contains(embed.Description, "Cantrips: 2") {
		t.Errorf("expected 'Cantrips: 2' fallback for portal spells with enrichment error, got: %s", embed.Description)
	}
}

func TestCharacterHandler_NoSpells(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	charID := uuid.New()
	campID := uuid.New()

	scoresJSON, _ := json.Marshal(character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 10, CHA: 10})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 1}})

	lookup := &mockCharacterLookup{
		pc: refdata.PlayerCharacter{
			CharacterID:   charID,
			CampaignID:    campID,
			DiscordUserID: "player-1",
			Status:        "approved",
		},
		char: refdata.Character{
			ID:            charID,
			CampaignID:    campID,
			Name:          "Tank",
			Race:          "Human",
			Level:         1,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
		},
	}

	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.test")
	handler.Handle(makeInteraction("character", "player-1", "guild-1"))

	embed := rc.Embeds[0]
	if strings.Contains(embed.Description, "Spells:") {
		t.Errorf("expected no spell line for fighter, got: %s", embed.Description)
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

func TestCharacterHandler_ChangesRequested_ShowsFeedbackAndResubmitHint(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	lookup := &mockCharacterLookup{
		pc: refdata.PlayerCharacter{
			Status:        "changes_requested",
			CharacterID:   uuid.New(),
			DiscordUserID: "player-1",
			DmFeedback:    sql.NullString{String: "Please pick a subclass", Valid: true},
		},
	}

	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.dndnd.app")
	handler.Handle(makeInteraction("character", "player-1", "guild-1"))

	if !strings.Contains(rc.Content, "changes_requested") {
		t.Errorf("expected status in message, got: %s", rc.Content)
	}
	if !strings.Contains(rc.Content, "Please pick a subclass") {
		t.Errorf("expected DM feedback in message, got: %s", rc.Content)
	}
	if !strings.Contains(rc.Content, "DM feedback") {
		t.Errorf("expected 'DM feedback' label in message, got: %s", rc.Content)
	}
	if !strings.Contains(rc.Content, "/create-character") {
		t.Errorf("expected resubmit hint mentioning /create-character, got: %s", rc.Content)
	}
}

func TestCharacterHandler_NotApproved_NoFeedback_OmitsFeedbackSection(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	lookup := &mockCharacterLookup{
		pc: refdata.PlayerCharacter{
			Status:        "pending",
			CharacterID:   uuid.New(),
			DiscordUserID: "player-1",
			// DmFeedback deliberately unset.
		},
	}

	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.dndnd.app")
	handler.Handle(makeInteraction("character", "player-1", "guild-1"))

	if !strings.Contains(rc.Content, "pending") {
		t.Errorf("expected pending message, got: %s", rc.Content)
	}
	if strings.Contains(rc.Content, "DM feedback") {
		t.Errorf("expected no DM feedback section when none set, got: %s", rc.Content)
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
	if len(rc.Embeds) == 0 {
		t.Fatal("expected embeds from character handler, got stub response")
	}
}

func TestCharacterHandler_WarlockPactSlots(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	charID := uuid.New()
	campID := uuid.New()

	scoresJSON, _ := json.Marshal(character.AbilityScores{STR: 8, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 18})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Warlock", Subclass: "Fiend", Level: 3}})
	pactJSON, _ := json.Marshal(character.PactMagicSlots{SlotLevel: 2, Current: 2, Max: 2})

	lookup := &mockCharacterLookup{
		pc: refdata.PlayerCharacter{
			CharacterID:   charID,
			CampaignID:    campID,
			DiscordUserID: "player-1",
			Status:        "approved",
		},
		char: refdata.Character{
			ID:            charID,
			CampaignID:    campID,
			Name:          "Hexblood",
			Race:          "Tiefling",
			Level:         3,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			HpMax:         21,
			HpCurrent:     21,
			Ac:            13,
			SpeedFt:       30,
			Languages:     []string{"Common"},
			// spell_slots empty; pact_magic_slots carries the warlock's slots.
			PactMagicSlots: pqtype.NullRawMessage{RawMessage: pactJSON, Valid: true},
		},
	}

	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.dndnd.app")
	handler.Handle(makeInteraction("character", "player-1", "guild-1"))

	if len(rc.Embeds) == 0 {
		t.Fatal("expected embeds in response")
	}

	embed := rc.Embeds[0]
	if !strings.Contains(embed.Description, "Pact Magic: 2 × Lvl 2") {
		t.Errorf("expected pact magic slot line in description, got: %s", embed.Description)
	}
}

// userOption builds an ApplicationCommandOptionUser option whose value is the
// snowflake string Discord sends over the wire.
func userOption(name, snowflake string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionUser,
		Value: snowflake,
	}
}

func approvedLookup(t *testing.T, discordUserID, name string) *mockCharacterLookup {
	t.Helper()
	charID := uuid.New()
	campID := uuid.New()
	scoresJSON, _ := json.Marshal(character.AbilityScores{STR: 16, DEX: 14, CON: 12, INT: 10, WIS: 8, CHA: 13})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 5}})
	return &mockCharacterLookup{
		pc: refdata.PlayerCharacter{
			CharacterID:   charID,
			CampaignID:    campID,
			DiscordUserID: discordUserID,
			Status:        "approved",
		},
		char: refdata.Character{
			ID:            charID,
			CampaignID:    campID,
			Name:          name,
			Race:          "Human",
			Level:         5,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			HpMax:         42,
			HpCurrent:     35,
			Ac:            18,
			SpeedFt:       30,
			Languages:     []string{"Common"},
		},
	}
}

func TestCharacterHandler_SelfView_IncludesPortalLink(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	lookup := approvedLookup(t, "player-1", "Thorn")
	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.dndnd.app")
	// No target option -> self view.
	handler.Handle(makeInteraction("character", "player-1", "guild-1"))

	if len(rc.Embeds) == 0 {
		t.Fatal("expected embeds in response")
	}
	if !strings.Contains(rc.Content, "https://portal.dndnd.app/portal/character/") {
		t.Errorf("expected portal link for self view, got: %s", rc.Content)
	}
	if rc.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Error("expected ephemeral response")
	}
}

func TestCharacterHandler_TargetView_ApprovedMember_NoPortalLink(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	// The mock resolves whatever lookup ID is passed; the target's character.
	lookup := approvedLookup(t, "player-2", "Aria")
	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.dndnd.app")
	handler.Handle(makeInteraction("character", "player-1", "guild-1", userOption("target", "player-2")))

	if len(rc.Embeds) == 0 {
		t.Fatal("expected embeds for approved target view")
	}
	if !strings.Contains(rc.Embeds[0].Title, "Aria") {
		t.Errorf("expected target's character in embed, got title: %s", rc.Embeds[0].Title)
	}
	if strings.Contains(rc.Content, "/portal/character/") {
		t.Errorf("did not expect portal link when viewing a target, got: %s", rc.Content)
	}
	if rc.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Error("expected ephemeral response for target view")
	}
}

func TestCharacterHandler_TargetView_NoCharacter_FriendlyMessage(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	lookup := &mockCharacterLookup{pcErr: sql.ErrNoRows}
	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.dndnd.app")
	handler.Handle(makeInteraction("character", "player-1", "guild-1", userOption("target", "player-2")))

	if !strings.Contains(rc.Content, "doesn't have a character in this campaign") {
		t.Errorf("expected friendly target-missing message, got: %s", rc.Content)
	}
	// Must not fall back to the self-oriented register hint.
	if strings.Contains(rc.Content, "/register") {
		t.Errorf("did not expect self register hint for a target, got: %s", rc.Content)
	}
}

func TestCharacterHandler_TargetView_NotApproved_HidesDMFeedback(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	const secretFeedback = "Please pick a subclass"
	lookup := &mockCharacterLookup{
		pc: refdata.PlayerCharacter{
			Status:        "changes_requested",
			CharacterID:   uuid.New(),
			DiscordUserID: "player-2",
			DmFeedback:    sql.NullString{String: secretFeedback, Valid: true},
		},
	}
	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.dndnd.app")
	handler.Handle(makeInteraction("character", "player-1", "guild-1", userOption("target", "player-2")))

	if !strings.Contains(rc.Content, "hasn't been approved yet") {
		t.Errorf("expected generic not-approved message, got: %s", rc.Content)
	}
	if strings.Contains(rc.Content, secretFeedback) {
		t.Errorf("DM feedback leaked to a non-owner, got: %s", rc.Content)
	}
	if strings.Contains(rc.Content, "DM feedback") {
		t.Errorf("did not expect DM feedback label for a target, got: %s", rc.Content)
	}
}

func TestCharacterHandler_EmbedIncludesAppearanceAndBackstory(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	charID := uuid.New()
	campID := uuid.New()
	scoresJSON, _ := json.Marshal(character.AbilityScores{STR: 16, DEX: 14, CON: 12, INT: 10, WIS: 8, CHA: 13})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 5}})
	charData := map[string]any{
		"appearance": "Tall   and\nscarred",
		"backstory":  "Orphaned at sea, raised by smugglers.",
	}
	charDataJSON, _ := json.Marshal(charData)

	lookup := &mockCharacterLookup{
		pc: refdata.PlayerCharacter{
			CharacterID:   charID,
			CampaignID:    campID,
			DiscordUserID: "player-1",
			Status:        "approved",
		},
		char: refdata.Character{
			ID:            charID,
			CampaignID:    campID,
			Name:          "Thorn",
			Race:          "Human",
			Level:         5,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		},
	}
	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.dndnd.app")
	handler.Handle(makeInteraction("character", "player-1", "guild-1"))

	if len(rc.Embeds) == 0 {
		t.Fatal("expected embeds in response")
	}
	desc := rc.Embeds[0].Description
	if !strings.Contains(desc, "Appearance: Tall and scarred") {
		t.Errorf("expected collapsed appearance line, got: %s", desc)
	}
	if !strings.Contains(desc, "Backstory: Orphaned at sea, raised by smugglers.") {
		t.Errorf("expected backstory line, got: %s", desc)
	}
}

func TestCharacterHandler_EmbedOmitsAppearanceAndBackstoryWhenAbsent(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	lookup := approvedLookup(t, "player-1", "Thorn")
	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.dndnd.app")
	handler.Handle(makeInteraction("character", "player-1", "guild-1"))

	if len(rc.Embeds) == 0 {
		t.Fatal("expected embeds in response")
	}
	desc := rc.Embeds[0].Description
	if strings.Contains(desc, "Appearance:") {
		t.Errorf("did not expect appearance line when absent, got: %s", desc)
	}
	if strings.Contains(desc, "Backstory:") {
		t.Errorf("did not expect backstory line when absent, got: %s", desc)
	}
}

func TestTruncate(t *testing.T) {
	// Shorter than the limit is returned unchanged.
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("expected unchanged short string, got: %q", got)
	}
	// Exactly at the limit is unchanged (no ellipsis).
	if got := truncate("hello", 5); got != "hello" {
		t.Errorf("expected unchanged at-limit string, got: %q", got)
	}
	// Longer than the limit is cut and gets an ellipsis.
	got := truncate("hello world", 5)
	if got != "hello…" {
		t.Errorf("expected truncated string with ellipsis, got: %q", got)
	}
}

func TestCharacterHandler_FullCasterStandardSlots(t *testing.T) {
	mock := newTestMock()
	rc := captureFullResponse(mock)

	charID := uuid.New()
	campID := uuid.New()

	scoresJSON, _ := json.Marshal(character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Wizard", Level: 5}})
	slotsJSON, _ := json.Marshal(map[string]character.SlotInfo{
		"1": {Current: 4, Max: 4},
		"3": {Current: 2, Max: 2},
	})

	lookup := &mockCharacterLookup{
		pc: refdata.PlayerCharacter{
			CharacterID:   charID,
			CampaignID:    campID,
			DiscordUserID: "player-1",
			Status:        "approved",
		},
		char: refdata.Character{
			ID:            charID,
			CampaignID:    campID,
			Name:          "Merlin",
			Race:          "Elf",
			Level:         5,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			HpMax:         22,
			HpCurrent:     22,
			Ac:            12,
			SpeedFt:       30,
			Languages:     []string{"Common"},
			SpellSlots:    pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		},
	}

	handler := NewCharacterHandler(mock, newMockCampaignProvider(), lookup, "https://portal.dndnd.app")
	handler.Handle(makeInteraction("character", "player-1", "guild-1"))

	if len(rc.Embeds) == 0 {
		t.Fatal("expected embeds in response")
	}

	embed := rc.Embeds[0]
	if !strings.Contains(embed.Description, "Spell Slots: 1st: 4/4 | 3rd: 2/2") {
		t.Errorf("expected standard spell slot line in description, got: %s", embed.Description)
	}
	if strings.Contains(embed.Description, "Pact Magic") {
		t.Errorf("did not expect pact magic line for a full caster, got: %s", embed.Description)
	}
}
