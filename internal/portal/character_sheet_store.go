package portal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
)

// CharacterQuerier is the subset of refdata.Queries needed for character sheet loading.
type CharacterQuerier interface {
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	GetPlayerCharacterByCharacter(ctx context.Context, arg refdata.GetPlayerCharacterByCharacterParams) (refdata.PlayerCharacter, error)
}

// CharacterSheetStoreAdapter implements CharacterSheetStore using refdata.Queries.
type CharacterSheetStoreAdapter struct {
	q CharacterQuerier
}

// NewCharacterSheetStoreAdapter creates a new CharacterSheetStoreAdapter.
func NewCharacterSheetStoreAdapter(q CharacterQuerier) *CharacterSheetStoreAdapter {
	return &CharacterSheetStoreAdapter{q: q}
}

// GetCharacterOwner returns the discord user ID that owns the character.
func (a *CharacterSheetStoreAdapter) GetCharacterOwner(ctx context.Context, characterID string) (string, error) {
	charUUID, err := uuid.Parse(characterID)
	if err != nil {
		return "", fmt.Errorf("invalid character ID: %w", err)
	}

	ch, err := a.q.GetCharacter(ctx, charUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrCharacterNotFound
		}
		return "", fmt.Errorf("getting character: %w", err)
	}

	pc, err := a.q.GetPlayerCharacterByCharacter(ctx, refdata.GetPlayerCharacterByCharacterParams{
		CampaignID:  ch.CampaignID,
		CharacterID: ch.ID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrCharacterNotFound
		}
		return "", fmt.Errorf("getting player character: %w", err)
	}

	return pc.DiscordUserID, nil
}

// GetCharacterForSheet loads a character's full data for sheet display.
func (a *CharacterSheetStoreAdapter) GetCharacterForSheet(ctx context.Context, characterID string) (*CharacterSheetData, error) {
	charUUID, err := uuid.Parse(characterID)
	if err != nil {
		return nil, fmt.Errorf("invalid character ID: %w", err)
	}

	ch, err := a.q.GetCharacter(ctx, charUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCharacterNotFound
		}
		return nil, fmt.Errorf("getting character: %w", err)
	}

	return mapCharacterToSheet(ch)
}

func mapCharacterToSheet(ch refdata.Character) (*CharacterSheetData, error) {
	data := &CharacterSheetData{
		ID:               ch.ID.String(),
		Name:             ch.Name,
		Race:             ch.Race,
		Level:            int(ch.Level),
		HpMax:            int(ch.HpMax),
		HpCurrent:        int(ch.HpCurrent),
		TempHP:           int(ch.TempHp),
		AC:               int(ch.Ac),
		SpeedFt:          int(ch.SpeedFt),
		ProficiencyBonus: int(ch.ProficiencyBonus),
		Gold:             int(ch.Gold),
		Languages:        ch.Languages,
	}

	// Equipped items
	if ch.EquippedMainHand.Valid {
		data.EquippedMainHand = ch.EquippedMainHand.String
	}
	if ch.EquippedOffHand.Valid {
		data.EquippedOffHand = ch.EquippedOffHand.String
	}
	if ch.EquippedArmor.Valid {
		data.EquippedArmor = ch.EquippedArmor.String
	}
	if ch.AcFormula.Valid {
		data.ACFormula = ch.AcFormula.String
	}

	// Parse JSON fields
	if err := parseJSONField(ch.AbilityScores, &data.AbilityScores); err != nil {
		return nil, fmt.Errorf("parsing ability scores: %w", err)
	}
	if err := parseJSONField(ch.Classes, &data.Classes); err != nil {
		return nil, fmt.Errorf("parsing classes: %w", err)
	}

	data.Proficiencies = parseNullJSON[character.Proficiencies](ch.Proficiencies)
	data.Features = parseNullJSONSlice[character.Feature](ch.Features)
	data.Inventory = parseNullJSONSlice[character.InventoryItem](ch.Inventory)
	data.AttunementSlots = parseNullJSONSlice[character.AttunementSlot](ch.AttunementSlots)
	data.SpellSlots = parseNullJSONMap[character.SlotInfo](ch.SpellSlots)
	data.PactMagicSlots = parseNullJSONPtr[character.PactMagicSlots](ch.PactMagicSlots)
	data.FeatureUses = parseNullJSONMap[character.FeatureUse](ch.FeatureUses)
	data.HitDiceRemaining = parseHitDiceRemaining(ch.HitDiceRemaining)

	return data, nil
}

func parseJSONField(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, target)
}

func parseNullJSON[T any](nrm pqtype.NullRawMessage) T {
	var zero T
	if !nrm.Valid || len(nrm.RawMessage) == 0 {
		return zero
	}
	var v T
	if err := json.Unmarshal(nrm.RawMessage, &v); err != nil {
		return zero
	}
	return v
}

func parseNullJSONSlice[T any](nrm pqtype.NullRawMessage) []T {
	if !nrm.Valid || len(nrm.RawMessage) == 0 {
		return nil
	}
	var v []T
	if err := json.Unmarshal(nrm.RawMessage, &v); err != nil {
		return nil
	}
	return v
}

func parseNullJSONMap[V any](nrm pqtype.NullRawMessage) map[string]V {
	if !nrm.Valid || len(nrm.RawMessage) == 0 {
		return nil
	}
	var v map[string]V
	if err := json.Unmarshal(nrm.RawMessage, &v); err != nil {
		return nil
	}
	return v
}

func parseNullJSONPtr[T any](nrm pqtype.NullRawMessage) *T {
	if !nrm.Valid || len(nrm.RawMessage) == 0 {
		return nil
	}
	var v T
	if err := json.Unmarshal(nrm.RawMessage, &v); err != nil {
		return nil
	}
	return &v
}

func parseHitDiceRemaining(raw json.RawMessage) map[string]int {
	if len(raw) == 0 {
		return nil
	}
	var v map[string]int
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	return v
}
