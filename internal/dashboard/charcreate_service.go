package dashboard

import (
	"context"
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/portal"
)

// CharCreateStore abstracts the persistence layer for DM character creation.
type CharCreateStore interface {
	CreateCharacterRecord(ctx context.Context, p portal.CreateCharacterParams) (string, error)
	CreatePlayerCharacterRecord(ctx context.Context, p portal.CreatePlayerCharacterParams) (string, error)
}

// FeatureProvider supplies class/subclass/racial feature data for character creation.
type FeatureProvider interface {
	ClassFeatures() map[string]map[string][]character.Feature
	SubclassFeatures() map[string]map[string]map[string][]character.Feature
	RacialTraits(race string) []character.Feature
}

// DMCharCreateServiceOption configures optional features of DMCharCreateService.
type DMCharCreateServiceOption func(*DMCharCreateService)

// WithFeatureProvider adds a feature provider to the service.
func WithFeatureProvider(fp FeatureProvider) DMCharCreateServiceOption {
	return func(svc *DMCharCreateService) {
		svc.featureProvider = fp
	}
}

// DMCharCreateService handles DM character creation.
type DMCharCreateService struct {
	store           CharCreateStore
	featureProvider FeatureProvider
}

// NewDMCharCreateService creates a new DMCharCreateService.
func NewDMCharCreateService(store CharCreateStore, opts ...DMCharCreateServiceOption) *DMCharCreateService {
	svc := &DMCharCreateService{store: store}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// CreateCharacter validates the submission, calculates derived stats,
// and creates the character + player_character records as pre-approved.
func (svc *DMCharCreateService) CreateCharacter(ctx context.Context, campaignID string, sub DMCharacterSubmission) (portal.CreateCharacterResult, error) {
	errs := ValidateDMSubmission(sub)
	if len(errs) > 0 {
		return portal.CreateCharacterResult{}, fmt.Errorf("validation failed: %s", strings.Join(errs, "; "))
	}

	stats := DeriveDMStats(sub)

	// Collect features from provider if available
	var features []character.Feature
	if svc.featureProvider != nil {
		features = CollectFeatures(
			sub.Classes,
			svc.featureProvider.ClassFeatures(),
			svc.featureProvider.SubclassFeatures(),
			svc.featureProvider.RacialTraits(sub.Race),
		)
	}

	primaryClass := sub.Classes[0].Class
	primarySubclass := sub.Classes[0].Subclass

	charParams := portal.CreateCharacterParams{
		CampaignID:    campaignID,
		Name:          sub.Name,
		Race:          sub.Race,
		Class:         primaryClass,
		Subclass:      primarySubclass,
		Background:    sub.Background,
		AbilityScores: sub.AbilityScores,
		HPMax:         stats.HPMax,
		AC:            stats.AC,
		SpeedFt:       stats.SpeedFt,
		ProfBonus:     stats.ProficiencyBonus,
		Saves:         stats.SaveProficiencies,
		Equipment:      sub.Equipment,
		Spells:         sub.Spells,
		Languages:      sub.Languages,
		Features:       features,
		EquippedWeapon: sub.EquippedWeapon,
		WornArmor:      sub.WornArmor,
	}

	charID, err := svc.store.CreateCharacterRecord(ctx, charParams)
	if err != nil {
		return portal.CreateCharacterResult{}, fmt.Errorf("creating character: %w", err)
	}

	pcParams := portal.CreatePlayerCharacterParams{
		CampaignID:    campaignID,
		CharacterID:   charID,
		DiscordUserID: "", // No player linked yet; player links via /register
		Status:        "approved",
		CreatedVia:    "dm_dashboard",
	}

	pcID, err := svc.store.CreatePlayerCharacterRecord(ctx, pcParams)
	if err != nil {
		return portal.CreateCharacterResult{}, fmt.Errorf("creating player character: %w", err)
	}

	return portal.CreateCharacterResult{
		CharacterID:       charID,
		PlayerCharacterID: pcID,
	}, nil
}
