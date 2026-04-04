package ddbimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
)

// CharacterStore abstracts the database operations needed for import.
type CharacterStore interface {
	CreateCharacter(ctx context.Context, params refdata.CreateCharacterParams) (refdata.Character, error)
	GetCharacterByDdbURL(ctx context.Context, params refdata.GetCharacterByDdbURLParams) (refdata.Character, error)
	UpdateCharacterFull(ctx context.Context, params refdata.UpdateCharacterFullParams) (refdata.Character, error)
}

// ImportResult contains the result of an import operation.
type ImportResult struct {
	Character refdata.Character
	Parsed    *ParsedCharacter
	Warnings  []Warning
	Preview   string
	IsResync  bool
	Changes   []string // non-empty for re-sync
}

// Service orchestrates the DDB import flow.
type Service struct {
	client Client
	store  CharacterStore
}

// NewService creates a new import service.
func NewService(client Client, store CharacterStore) *Service {
	return &Service{
		client: client,
		store:  store,
	}
}

// Import performs the full import flow: parse URL -> fetch -> parse JSON -> validate -> create/update.
func (s *Service) Import(ctx context.Context, campaignID uuid.UUID, ddbURL string) (*ImportResult, error) {
	charID, err := ParseCharacterURL(ddbURL)
	if err != nil {
		return nil, fmt.Errorf("invalid DDB URL: %w", err)
	}

	rawJSON, err := s.client.FetchCharacter(ctx, charID)
	if err != nil {
		return nil, fmt.Errorf("fetching character: %w", err)
	}

	parsed, err := ParseDDBJSON(rawJSON)
	if err != nil {
		return nil, fmt.Errorf("parsing DDB JSON: %w", err)
	}

	warnings, err := Validate(parsed)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	params, err := buildCreateParams(campaignID, ddbURL, parsed)
	if err != nil {
		return nil, fmt.Errorf("building character params: %w", err)
	}

	result := &ImportResult{
		Parsed:   parsed,
		Warnings: warnings,
	}

	// Check for existing character (re-sync)
	existing, getErr := s.store.GetCharacterByDdbURL(ctx, refdata.GetCharacterByDdbURLParams{
		CampaignID: campaignID,
		DdbUrl:     sql.NullString{String: ddbURL, Valid: true},
	})
	if getErr == nil {
		// Re-sync: diff and update
		result.IsResync = true
		oldParsed := characterToParseResult(&existing)
		result.Changes = GenerateDiff(oldParsed, parsed)

		updateParams := buildUpdateParams(existing.ID, params)
		updated, updateErr := s.store.UpdateCharacterFull(ctx, updateParams)
		if updateErr != nil {
			return nil, fmt.Errorf("updating character: %w", updateErr)
		}
		result.Character = updated
	} else {
		// New import
		char, createErr := s.store.CreateCharacter(ctx, params)
		if createErr != nil {
			return nil, fmt.Errorf("creating character: %w", createErr)
		}
		result.Character = char
	}

	result.Preview = FormatPreviewWithWarnings(parsed, warnings)

	return result, nil
}

func marshalField(name string, v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshaling %s: %w", name, err)
	}
	return data, nil
}

func buildCreateParams(campaignID uuid.UUID, ddbURL string, pc *ParsedCharacter) (refdata.CreateCharacterParams, error) {
	scoresJSON, err := marshalField("ability scores", pc.AbilityScores)
	if err != nil {
		return refdata.CreateCharacterParams{}, err
	}

	classesJSON, err := marshalField("classes", pc.Classes)
	if err != nil {
		return refdata.CreateCharacterParams{}, err
	}

	inventoryJSON, err := marshalField("inventory", pc.Inventory)
	if err != nil {
		return refdata.CreateCharacterParams{}, err
	}

	profsJSON, err := marshalField("proficiencies", pc.Proficiencies)
	if err != nil {
		return refdata.CreateCharacterParams{}, err
	}

	featuresJSON, err := marshalField("features", pc.Features)
	if err != nil {
		return refdata.CreateCharacterParams{}, err
	}

	charDataJSON, err := marshalField("character data", map[string]interface{}{"spells": pc.Spells})
	if err != nil {
		return refdata.CreateCharacterParams{}, err
	}

	// Compute proficiency bonus from level
	profBonus := int32(2)
	if pc.Level >= 17 {
		profBonus = 6
	} else if pc.Level >= 13 {
		profBonus = 5
	} else if pc.Level >= 9 {
		profBonus = 4
	} else if pc.Level >= 5 {
		profBonus = 3
	}

	// Build hit dice remaining
	hitDice := make(map[string]int)
	for _, c := range pc.Classes {
		die := classHitDie(c.Class)
		if die != "" {
			hitDice[die] += c.Level
		}
	}
	hitDiceJSON, _ := json.Marshal(hitDice)

	langs := pc.Languages
	if len(langs) == 0 {
		langs = []string{"Common"}
	}

	return refdata.CreateCharacterParams{
		CampaignID:       campaignID,
		Name:             pc.Name,
		Race:             pc.Race,
		Classes:          classesJSON,
		Level:            int32(pc.Level),
		AbilityScores:    scoresJSON,
		HpMax:            int32(pc.HPMax),
		HpCurrent:        int32(pc.HPCurrent),
		TempHp:           int32(pc.TempHP),
		Ac:               int32(pc.AC),
		SpeedFt:          int32(pc.SpeedFt),
		ProficiencyBonus: profBonus,
		HitDiceRemaining: hitDiceJSON,
		Gold:             int32(pc.Gold),
		Languages:        langs,
		Inventory:        pqtype.NullRawMessage{RawMessage: inventoryJSON, Valid: true},
		Proficiencies:    pqtype.NullRawMessage{RawMessage: profsJSON, Valid: true},
		Features:         pqtype.NullRawMessage{RawMessage: featuresJSON, Valid: true},
		CharacterData:    pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		DdbUrl:           sql.NullString{String: ddbURL, Valid: true},
	}, nil
}

// buildUpdateParams converts CreateCharacterParams to UpdateCharacterFullParams with the given ID.
func buildUpdateParams(id uuid.UUID, p refdata.CreateCharacterParams) refdata.UpdateCharacterFullParams {
	return refdata.UpdateCharacterFullParams{
		ID:               id,
		Name:             p.Name,
		Race:             p.Race,
		Classes:          p.Classes,
		Level:            p.Level,
		AbilityScores:    p.AbilityScores,
		HpMax:            p.HpMax,
		HpCurrent:        p.HpCurrent,
		TempHp:           p.TempHp,
		Ac:               p.Ac,
		AcFormula:        p.AcFormula,
		SpeedFt:          p.SpeedFt,
		ProficiencyBonus: p.ProficiencyBonus,
		EquippedMainHand: p.EquippedMainHand,
		EquippedOffHand:  p.EquippedOffHand,
		EquippedArmor:    p.EquippedArmor,
		SpellSlots:       p.SpellSlots,
		PactMagicSlots:   p.PactMagicSlots,
		HitDiceRemaining: p.HitDiceRemaining,
		FeatureUses:      p.FeatureUses,
		Features:         p.Features,
		Proficiencies:    p.Proficiencies,
		Gold:             p.Gold,
		AttunementSlots:  p.AttunementSlots,
		Languages:        p.Languages,
		Inventory:        p.Inventory,
		CharacterData:    p.CharacterData,
		DdbUrl:           p.DdbUrl,
		Homebrew:         p.Homebrew,
	}
}

func classHitDie(className string) string {
	switch className {
	case "Barbarian":
		return "d12"
	case "Fighter", "Paladin", "Ranger":
		return "d10"
	case "Bard", "Cleric", "Druid", "Monk", "Rogue", "Warlock":
		return "d8"
	case "Sorcerer", "Wizard":
		return "d6"
	}
	return "d8" // default
}

// characterToParseResult converts a refdata.Character back to ParsedCharacter for diffing.
func characterToParseResult(c *refdata.Character) *ParsedCharacter {
	pc := &ParsedCharacter{
		Name:      c.Name,
		Race:      c.Race,
		Level:     int(c.Level),
		HPMax:     int(c.HpMax),
		HPCurrent: int(c.HpCurrent),
		TempHP:    int(c.TempHp),
		AC:        int(c.Ac),
		SpeedFt:   int(c.SpeedFt),
		Gold:      int(c.Gold),
	}

	if len(c.AbilityScores) > 0 {
		_ = json.Unmarshal(c.AbilityScores, &pc.AbilityScores)
	}
	if len(c.Classes) > 0 {
		_ = json.Unmarshal(c.Classes, &pc.Classes)
	}
	if c.Inventory.Valid && len(c.Inventory.RawMessage) > 0 {
		_ = json.Unmarshal(c.Inventory.RawMessage, &pc.Inventory)
	}
	if c.Proficiencies.Valid && len(c.Proficiencies.RawMessage) > 0 {
		_ = json.Unmarshal(c.Proficiencies.RawMessage, &pc.Proficiencies)
	}

	pc.Languages = c.Languages

	if c.Features.Valid && len(c.Features.RawMessage) > 0 {
		_ = json.Unmarshal(c.Features.RawMessage, &pc.Features)
	}
	if c.CharacterData.Valid && len(c.CharacterData.RawMessage) > 0 {
		var charData map[string]json.RawMessage
		if err := json.Unmarshal(c.CharacterData.RawMessage, &charData); err == nil {
			if spellsRaw, ok := charData["spells"]; ok {
				_ = json.Unmarshal(spellsRaw, &pc.Spells)
			}
		}
	}

	return pc
}
