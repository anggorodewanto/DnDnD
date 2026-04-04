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

// DMCharCreateService handles DM character creation.
type DMCharCreateService struct {
	store CharCreateStore
}

// NewDMCharCreateService creates a new DMCharCreateService.
func NewDMCharCreateService(store CharCreateStore) *DMCharCreateService {
	return &DMCharCreateService{store: store}
}

// CreateCharacter validates the submission, calculates derived stats,
// and creates the character + player_character records as pre-approved.
func (svc *DMCharCreateService) CreateCharacter(ctx context.Context, campaignID string, sub DMCharacterSubmission) (portal.CreateCharacterResult, error) {
	errs := ValidateDMSubmission(sub)
	if len(errs) > 0 {
		return portal.CreateCharacterResult{}, fmt.Errorf("validation failed: %s", strings.Join(errs, "; "))
	}

	stats := DeriveDMStats(sub)

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

// DMCreateStoreAdapter adapts the portal BuilderStoreAdapter for DM character creation.
// It reuses the same underlying store but customizes the character record creation
// to support multiclass and proper derived stats.
type DMCreateStoreAdapter struct {
	inner CharCreateStore
}

// NewDMCreateStoreAdapter wraps an existing CharCreateStore.
func NewDMCreateStoreAdapter(inner CharCreateStore) *DMCreateStoreAdapter {
	return &DMCreateStoreAdapter{inner: inner}
}

// CreateCharacterRecord delegates to the inner store.
func (a *DMCreateStoreAdapter) CreateCharacterRecord(ctx context.Context, p portal.CreateCharacterParams) (string, error) {
	return a.inner.CreateCharacterRecord(ctx, p)
}

// CreatePlayerCharacterRecord delegates to the inner store.
func (a *DMCreateStoreAdapter) CreatePlayerCharacterRecord(ctx context.Context, p portal.CreatePlayerCharacterParams) (string, error) {
	return a.inner.CreatePlayerCharacterRecord(ctx, p)
}

// classSaveProficienciesLookup returns save proficiencies for a class name (for use in store adapters).
func classSaveProficienciesLookup(className string) []string {
	return classSaveProficiencies([]character.ClassEntry{{Class: className, Level: 1}})
}