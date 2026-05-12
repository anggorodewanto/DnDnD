package portal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ab/dndnd/internal/character"
)

var (
	// ErrPointBuyOverspent is returned when point-buy total exceeds 27.
	ErrPointBuyOverspent = errors.New("point buy: spent more than 27 points")
	// ErrScoreOutOfRange is returned when a score is below 8 or above 15.
	ErrScoreOutOfRange = errors.New("point buy: score out of range (must be 8-15)")
)

// CharacterSubmission is the payload sent by the character builder form.
//
// Classes is the multiclass entry — when non-empty it overrides
// Class/Subclass for persistence. Class/Subclass remain on the payload
// so older single-class submitters keep working.
type CharacterSubmission struct {
	Name          string                  `json:"name"`
	Race          string                  `json:"race"`
	Subrace       string                  `json:"subrace,omitempty"`
	Background    string                  `json:"background"`
	Class         string                  `json:"class"`
	Subclass      string                  `json:"subclass,omitempty"`
	Classes       []character.ClassEntry  `json:"classes,omitempty"`
	AbilityScores PointBuyScores          `json:"ability_scores"`
	Skills        []string                `json:"skills"`
	Equipment     []string                `json:"equipment,omitempty"`
	Spells        []string                `json:"spells,omitempty"`
}

// ValidateSubmission returns a list of validation error messages.
func ValidateSubmission(s CharacterSubmission) []string {
	var errs []string
	if s.Name == "" {
		errs = append(errs, "name is required")
	}
	if s.Race == "" {
		errs = append(errs, "race is required")
	}
	if s.Class == "" {
		errs = append(errs, "class is required")
	}
	if err := ValidatePointBuy(s.AbilityScores); err != nil {
		errs = append(errs, err.Error())
	}
	return errs
}

// CreateCharacterParams holds params for creating a character record.
//
// Classes drives the JSONB classes column when non-empty; otherwise the
// adapter falls back to a single ClassEntry built from Class/Subclass.
type CreateCharacterParams struct {
	CampaignID    string
	Name          string
	Race          string
	Subrace       string
	Class         string
	Subclass      string
	Classes       []character.ClassEntry
	Background    string
	AbilityScores character.AbilityScores
	HPMax         int
	AC            int
	SpeedFt       int
	ProfBonus     int
	Skills        []string
	Saves         []string
	Equipment      []string
	Spells         []string
	Languages      []string
	Features       []character.Feature
	EquippedWeapon string
	WornArmor      string
}

// CreatePlayerCharacterParams holds params for the player_characters record.
type CreatePlayerCharacterParams struct {
	CampaignID  string
	CharacterID string
	DiscordUserID string
	Status      string
	CreatedVia  string
}

// CreateCharacterResult holds the IDs from character creation.
type CreateCharacterResult struct {
	CharacterID       string
	PlayerCharacterID string
}

// BuilderStore abstracts the persistence layer for character creation.
type BuilderStore interface {
	CreateCharacterRecord(ctx context.Context, p CreateCharacterParams) (string, error)
	CreatePlayerCharacterRecord(ctx context.Context, p CreatePlayerCharacterParams) (string, error)
	RedeemToken(ctx context.Context, token string) error
}

// DMQueueNotifier sends notifications to the DM queue channel.
type DMQueueNotifier interface {
	NotifyDMQueue(ctx context.Context, characterName, playerDiscordID, via string) error
}

// BuilderServiceOption configures optional features of BuilderService.
type BuilderServiceOption func(*BuilderService)

// WithNotifier adds a DM queue notifier to the BuilderService.
func WithNotifier(n DMQueueNotifier) BuilderServiceOption {
	return func(svc *BuilderService) {
		svc.notifier = n
	}
}

// WithLogger adds a logger to the BuilderService.
func WithLogger(l *slog.Logger) BuilderServiceOption {
	return func(svc *BuilderService) {
		svc.logger = l
	}
}

// BuilderService handles character creation from the portal form.
type BuilderService struct {
	store    BuilderStore
	notifier DMQueueNotifier
	logger   *slog.Logger
}

// NewBuilderService creates a new BuilderService.
func NewBuilderService(store BuilderStore, opts ...BuilderServiceOption) *BuilderService {
	svc := &BuilderService{store: store}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// CreateCharacter validates the submission, calculates derived stats,
// creates the character + player_character records, and redeems the token.
func (svc *BuilderService) CreateCharacter(ctx context.Context, campaignID, discordUserID, token string, sub CharacterSubmission) (CreateCharacterResult, error) {
	errs := ValidateSubmission(sub)
	if len(errs) > 0 {
		return CreateCharacterResult{}, fmt.Errorf("validation failed: %s", strings.Join(errs, "; "))
	}

	scores := character.AbilityScores{
		STR: sub.AbilityScores.STR,
		DEX: sub.AbilityScores.DEX,
		CON: sub.AbilityScores.CON,
		INT: sub.AbilityScores.INT,
		WIS: sub.AbilityScores.WIS,
		CHA: sub.AbilityScores.CHA,
	}

	hp := DeriveHP(sub.Class, scores)
	ac := DeriveAC(scores)
	profBonus := character.ProficiencyBonus(1)

	charParams := CreateCharacterParams{
		CampaignID:    campaignID,
		Name:          sub.Name,
		Race:          sub.Race,
		Subrace:       sub.Subrace,
		Class:         sub.Class,
		Subclass:      sub.Subclass,
		Classes:       sub.Classes,
		Background:    sub.Background,
		AbilityScores: scores,
		HPMax:         hp,
		AC:            ac,
		SpeedFt:       DeriveSpeed(sub.Race),
		ProfBonus:     profBonus,
		Skills:        sub.Skills,
		Equipment:     sub.Equipment,
		Spells:        sub.Spells,
	}

	charID, err := svc.store.CreateCharacterRecord(ctx, charParams)
	if err != nil {
		return CreateCharacterResult{}, fmt.Errorf("creating character: %w", err)
	}

	pcParams := CreatePlayerCharacterParams{
		CampaignID:    campaignID,
		CharacterID:   charID,
		DiscordUserID: discordUserID,
		Status:        "pending",
		CreatedVia:    "create",
	}

	pcID, err := svc.store.CreatePlayerCharacterRecord(ctx, pcParams)
	if err != nil {
		return CreateCharacterResult{}, fmt.Errorf("creating player character: %w", err)
	}

	if err := svc.store.RedeemToken(ctx, token); err != nil && svc.logger != nil {
		svc.logger.Warn("redeeming token after character creation", "token", token, "error", err)
	}

	if svc.notifier != nil {
		if err := svc.notifier.NotifyDMQueue(ctx, sub.Name, discordUserID, "portal-create"); err != nil && svc.logger != nil {
			svc.logger.Warn("notifying dm queue", "error", err)
		}
	}

	return CreateCharacterResult{
		CharacterID:       charID,
		PlayerCharacterID: pcID,
	}, nil
}

// PointBuyScores holds the six ability scores for point-buy validation.
type PointBuyScores struct {
	STR int `json:"str"`
	DEX int `json:"dex"`
	CON int `json:"con"`
	INT int `json:"int"`
	WIS int `json:"wis"`
	CHA int `json:"cha"`
}

// PointBuyCost returns the point cost for a single ability score value.
func PointBuyCost(score int) (int, error) {
	if score < 8 || score > 15 {
		return 0, ErrScoreOutOfRange
	}
	// 8=0, 9=1, 10=2, 11=3, 12=4, 13=5, 14=7, 15=9
	base := score - 8
	if score <= 13 {
		return base, nil
	}
	// 14 costs 7 (5 + 2 extra), 15 costs 9 (5 + 2 + 2 extra)
	return 5 + (score-13)*2, nil
}

// ValidatePointBuy checks that the given scores are valid under 5e point-buy rules.
func ValidatePointBuy(scores PointBuyScores) error {
	vals := []int{scores.STR, scores.DEX, scores.CON, scores.INT, scores.WIS, scores.CHA}
	total := 0
	for _, v := range vals {
		cost, err := PointBuyCost(v)
		if err != nil {
			return fmt.Errorf("%w: %d", err, v)
		}
		total += cost
	}
	if total > 27 {
		return ErrPointBuyOverspent
	}
	return nil
}
