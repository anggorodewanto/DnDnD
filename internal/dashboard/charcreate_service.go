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

// WithDMAbilityMethodProvider adds campaign ability-score method gating.
func WithDMAbilityMethodProvider(p portal.AbilityMethodProvider) DMCharCreateServiceOption {
	return func(svc *DMCharCreateService) {
		svc.abilityMethods = p
	}
}

// DMCharCreateService handles DM character creation.
type DMCharCreateService struct {
	store           CharCreateStore
	featureProvider FeatureProvider
	abilityMethods  portal.AbilityMethodProvider
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
	if sub.AbilityMethod == "" && svc.hasAbilityMethodProvider() {
		sub.AbilityMethod = portal.AbilityMethodPointBuy
	}
	errs := ValidateDMSubmission(sub)
	if err := svc.validateAllowedAbilityMethod(ctx, campaignID, sub.AbilityMethod); err != nil {
		errs = append(errs, err.Error())
	}
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
		CampaignID: campaignID,
		Name:       sub.Name,
		Race:       sub.Race,
		Class:      primaryClass,
		Subclass:   primarySubclass,
		// Forward the full multiclass list so BuilderStoreAdapter
		// persists every class/subclass/level entry (SR-015). Without
		// this, resolveClassEntries falls back to a single L1 entry.
		Classes:        sub.Classes,
		Background:     sub.Background,
		AbilityScores:  sub.AbilityScores,
		HPMax:          stats.HPMax,
		AC:             stats.AC,
		SpeedFt:        stats.SpeedFt,
		ProfBonus:      stats.ProficiencyBonus,
		Saves:          stats.SaveProficiencies,
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

func (svc *DMCharCreateService) validateAllowedAbilityMethod(ctx context.Context, campaignID string, method portal.AbilityScoreMethod) error {
	allowed, err := svc.AllowedAbilityScoreMethods(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("loading ability score methods: %w", err)
	}
	if len(allowed) == 0 {
		return nil
	}
	if method == "" {
		method = portal.AbilityMethodPointBuy
	}
	for _, allowedMethod := range allowed {
		if method == allowedMethod {
			return nil
		}
	}
	return fmt.Errorf("ability score method %s is not allowed", method)
}

func (svc *DMCharCreateService) hasAbilityMethodProvider() bool {
	if svc.abilityMethods != nil {
		return true
	}
	_, ok := svc.store.(portal.AbilityMethodProvider)
	return ok
}

// AllowedAbilityScoreMethods returns campaign-enabled methods for DM creation.
func (svc *DMCharCreateService) AllowedAbilityScoreMethods(ctx context.Context, campaignID string) ([]portal.AbilityScoreMethod, error) {
	provider := svc.abilityMethods
	if provider == nil {
		if p, ok := svc.store.(portal.AbilityMethodProvider); ok {
			provider = p
		}
	}
	if provider == nil {
		return portal.DefaultAbilityScoreMethods(), nil
	}
	allowed, err := provider.AllowedAbilityScoreMethods(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if len(allowed) > 0 {
		return allowed, nil
	}
	return portal.DefaultAbilityScoreMethods(), nil
}
