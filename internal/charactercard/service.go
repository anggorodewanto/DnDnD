package charactercard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// DiscordSender is the subset of the Discord session needed for character cards.
type DiscordSender interface {
	ChannelMessageSend(channelID, content string) (*discordgo.Message, error)
	ChannelMessageEdit(channelID, messageID, content string) (*discordgo.Message, error)
}

// Store defines the database operations needed by the character card service.
type Store interface {
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	GetCampaignByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error)
	GetCharacterCardMessageID(ctx context.Context, id uuid.UUID) (sql.NullString, error)
	SetCharacterCardMessageID(ctx context.Context, arg refdata.SetCharacterCardMessageIDParams) error
	ListCharactersByCampaign(ctx context.Context, campaignID uuid.UUID) ([]refdata.Character, error)
}

// Service implements CharacterCardPoster and handles card updates.
type Service struct {
	discord DiscordSender
	store   Store
	logger  *slog.Logger
}

// NewService creates a new character card Service.
func NewService(discord DiscordSender, store Store, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{discord: discord, store: store, logger: logger}
}

// PostCharacterCard creates a new character card message in the #character-cards channel.
func (s *Service) PostCharacterCard(ctx context.Context, characterID uuid.UUID, characterName, discordUserID string) error {
	char, channelID, err := s.fetchCharacterAndChannel(ctx, characterID)
	if err != nil {
		return err
	}

	shortID, err := s.generateShortID(ctx, char)
	if err != nil {
		return err
	}

	data := buildCardData(char, shortID, false)
	content := FormatCard(data)

	msg, err := s.discord.ChannelMessageSend(channelID, content)
	if err != nil {
		return fmt.Errorf("sending character card: %w", err)
	}

	err = s.store.SetCharacterCardMessageID(ctx, refdata.SetCharacterCardMessageIDParams{
		ID:            characterID,
		CardMessageID: sql.NullString{String: msg.ID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("storing card message ID: %w", err)
	}

	return nil
}

// UpdateCardRetired edits the existing card message to add a RETIRED badge.
func (s *Service) UpdateCardRetired(ctx context.Context, characterID uuid.UUID, characterName, discordUserID string) error {
	return s.editCard(ctx, characterID, true)
}

// UpdateCard re-fetches character data and edits the existing Discord message.
func (s *Service) UpdateCard(ctx context.Context, characterID uuid.UUID) error {
	return s.editCard(ctx, characterID, false)
}

// editCard fetches the character, resolves its short ID, and edits the existing card message.
func (s *Service) editCard(ctx context.Context, characterID uuid.UUID, retired bool) error {
	char, channelID, err := s.fetchCharacterAndChannel(ctx, characterID)
	if err != nil {
		return err
	}

	msgID, err := s.store.GetCharacterCardMessageID(ctx, characterID)
	if err != nil {
		return fmt.Errorf("fetching card message ID: %w", err)
	}
	if !msgID.Valid || msgID.String == "" {
		return fmt.Errorf("no existing card message for character %s", characterID)
	}

	shortID, err := s.generateShortID(ctx, char)
	if err != nil {
		return err
	}

	data := buildCardData(char, shortID, retired)
	content := FormatCard(data)

	_, err = s.discord.ChannelMessageEdit(channelID, msgID.String, content)
	if err != nil {
		return fmt.Errorf("editing character card: %w", err)
	}

	return nil
}

// OnCharacterUpdated is a hook that should be called when character state changes
// (HP, equipment, conditions, level, etc.). It updates the character's card if one exists.
// If no card message exists yet (character not approved), this is a silent no-op.
func (s *Service) OnCharacterUpdated(ctx context.Context, characterID uuid.UUID) error {
	// Check if a card message exists; if not, silently return
	msgID, err := s.store.GetCharacterCardMessageID(ctx, characterID)
	if err != nil {
		s.logger.Warn("checking card message ID for auto-update", "character_id", characterID, "error", err)
		return nil
	}
	if !msgID.Valid || msgID.String == "" {
		return nil
	}

	return s.UpdateCard(ctx, characterID)
}

func (s *Service) fetchCharacterAndChannel(ctx context.Context, characterID uuid.UUID) (refdata.Character, string, error) {
	char, err := s.store.GetCharacter(ctx, characterID)
	if err != nil {
		return refdata.Character{}, "", fmt.Errorf("fetching character: %w", err)
	}

	campaign, err := s.store.GetCampaignByID(ctx, char.CampaignID)
	if err != nil {
		return refdata.Character{}, "", fmt.Errorf("fetching campaign: %w", err)
	}

	channelID, err := getCharacterCardsChannelID(campaign)
	if err != nil {
		return refdata.Character{}, "", err
	}

	return char, channelID, nil
}

func (s *Service) generateShortID(ctx context.Context, char refdata.Character) (string, error) {
	chars, err := s.store.ListCharactersByCampaign(ctx, char.CampaignID)
	if err != nil {
		return "", fmt.Errorf("listing characters for short ID: %w", err)
	}

	// Sort all characters by ID for stable, deterministic assignment
	sort.Slice(chars, func(i, j int) bool {
		return chars[i].ID.String() < chars[j].ID.String()
	})

	// Assign short IDs in order; earlier characters claim the base ID first
	var assigned []string
	for _, c := range chars {
		id := ShortID(c.Name, assigned)
		if c.ID == char.ID {
			return id, nil
		}
		assigned = append(assigned, id)
	}

	// Character not in the list (shouldn't happen), fall back
	return ShortID(char.Name, assigned), nil
}

type campaignSettings struct {
	ChannelIDs map[string]string `json:"channel_ids,omitempty"`
}

func getCharacterCardsChannelID(campaign refdata.Campaign) (string, error) {
	if !campaign.Settings.Valid {
		return "", fmt.Errorf("character-cards channel not configured")
	}

	var settings campaignSettings
	if err := json.Unmarshal(campaign.Settings.RawMessage, &settings); err != nil {
		return "", fmt.Errorf("parsing campaign settings: %w", err)
	}

	channelID, ok := settings.ChannelIDs["character-cards"]
	if !ok || channelID == "" {
		return "", fmt.Errorf("character-cards channel not configured")
	}

	return channelID, nil
}

func buildCardData(char refdata.Character, shortID string, retired bool) CardData {
	var classes []character.ClassEntry
	_ = json.Unmarshal(char.Classes, &classes)

	var abilities character.AbilityScores
	_ = json.Unmarshal(char.AbilityScores, &abilities)

	var spellSlots map[string]character.SlotInfo
	if char.SpellSlots.Valid {
		_ = json.Unmarshal(char.SpellSlots.RawMessage, &spellSlots)
	}

	spellCount, preparedCount := extractSpellCounts(char)

	return CardData{
		Name:             char.Name,
		ShortID:          shortID,
		Level:            int(char.Level),
		Race:             char.Race,
		Classes:          classes,
		HpCurrent:        int(char.HpCurrent),
		HpMax:            int(char.HpMax),
		TempHP:           int(char.TempHp),
		AC:               int(char.Ac),
		SpeedFt:          int(char.SpeedFt),
		AbilityScores:    abilities,
		EquippedMainHand: char.EquippedMainHand.String,
		EquippedOffHand:  char.EquippedOffHand.String,
		SpellSlots:       spellSlots,
		SpellCount:       spellCount,
		PreparedCount:    preparedCount,
		Gold:             int(char.Gold),
		Languages:        char.Languages,
		Retired:          retired,
	}
}

// extractSpellCounts counts spells and prepared spells from character_data.
func extractSpellCounts(char refdata.Character) (spellCount int, preparedCount int) {
	if !char.CharacterData.Valid || len(char.CharacterData.RawMessage) == 0 {
		return 0, 0
	}

	var data map[string]json.RawMessage
	if err := json.Unmarshal(char.CharacterData.RawMessage, &data); err != nil {
		return 0, 0
	}

	spellsRaw, ok := data["spells"]
	if !ok {
		return 0, 0
	}

	// Try DDB format first: []{"name":..., "level":...}
	var ddbSpells []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(spellsRaw, &ddbSpells); err == nil && len(ddbSpells) > 0 && ddbSpells[0].Name != "" {
		spellCount = len(ddbSpells)
	} else {
		// Try portal format: []string
		var portalSpells []string
		if err := json.Unmarshal(spellsRaw, &portalSpells); err == nil {
			spellCount = len(portalSpells)
		}
	}

	// Count prepared spells
	if prepRaw, ok := data["prepared_spells"]; ok {
		var prepared []string
		if err := json.Unmarshal(prepRaw, &prepared); err == nil {
			preparedCount = len(prepared)
		}
	}

	return spellCount, preparedCount
}
