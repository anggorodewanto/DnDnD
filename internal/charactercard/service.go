package charactercard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"

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

	data := buildCardData(char, shortID, true)
	content := FormatCard(data)

	_, err = s.discord.ChannelMessageEdit(channelID, msgID.String, content)
	if err != nil {
		return fmt.Errorf("editing character card: %w", err)
	}

	return nil
}

// UpdateCard re-fetches character data and edits the existing Discord message.
func (s *Service) UpdateCard(ctx context.Context, characterID uuid.UUID) error {
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

	data := buildCardData(char, shortID, false)
	content := FormatCard(data)

	_, err = s.discord.ChannelMessageEdit(channelID, msgID.String, content)
	if err != nil {
		return fmt.Errorf("editing character card: %w", err)
	}

	return nil
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

	// Build existing short IDs from other characters
	var existing []string
	for _, c := range chars {
		if c.ID == char.ID {
			continue
		}
		existing = append(existing, ShortID(c.Name, nil))
	}

	return ShortID(char.Name, existing), nil
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

	mainHand := ""
	if char.EquippedMainHand.Valid {
		mainHand = char.EquippedMainHand.String
	}
	offHand := ""
	if char.EquippedOffHand.Valid {
		offHand = char.EquippedOffHand.String
	}

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
		EquippedMainHand: mainHand,
		EquippedOffHand:  offHand,
		SpellSlots:       spellSlots,
		Gold:             int(char.Gold),
		Languages:        char.Languages,
		Retired:          retired,
	}
}
