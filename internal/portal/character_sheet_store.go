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
	GetSpellsByIDs(ctx context.Context, ids []string) ([]refdata.Spell, error)
	GetActiveCombatantByCharacterID(ctx context.Context, characterID uuid.NullUUID) (refdata.Combatant, error)
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

	data, err := mapCharacterToSheet(ch)
	if err != nil {
		return nil, err
	}

	// Hydrate live combat state (conditions, exhaustion, concentration)
	// from the active combatant row, if one exists.
	a.hydrateFromCombatant(ctx, charUUID, data)

	// Enrich spells from reference table
	if len(data.Spells) > 0 {
		a.enrichSpells(ctx, data.Spells)
	}

	return data, nil
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
	data.Features = parseNullJSON[[]character.Feature](ch.Features)
	data.Inventory = parseNullJSON[[]character.InventoryItem](ch.Inventory)
	data.AttunementSlots = parseNullJSON[[]character.AttunementSlot](ch.AttunementSlots)
	data.SpellSlots = parseNullJSON[map[string]character.SlotInfo](ch.SpellSlots)
	data.PactMagicSlots = parseNullJSONPtr[character.PactMagicSlots](ch.PactMagicSlots)
	data.FeatureUses = parseNullJSON[map[string]character.FeatureUse](ch.FeatureUses)
	data.HitDiceRemaining = parseHitDiceRemaining(ch.HitDiceRemaining)
	data.Spells = extractSpells(ch.CharacterData)

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

// enrichSpells looks up spell IDs in the reference table and populates
// display fields (Name, Level, School, CastingTime, Range) on each entry.
func (a *CharacterSheetStoreAdapter) enrichSpells(ctx context.Context, spells []SpellDisplayEntry) {
	ids := collectSpellIDs(spells)
	if len(ids) == 0 {
		return
	}

	refSpells, err := a.q.GetSpellsByIDs(ctx, ids)
	if err != nil || len(refSpells) == 0 {
		return
	}

	byID := make(map[string]refdata.Spell, len(refSpells))
	for _, s := range refSpells {
		byID[s.ID] = s
	}

	for i := range spells {
		ref, ok := byID[spells[i].ID]
		if !ok {
			continue
		}
		spells[i].Name = ref.Name
		spells[i].Level = int(ref.Level)
		spells[i].School = ref.School
		spells[i].CastingTime = ref.CastingTime
		spells[i].Range = formatSpellRange(ref)
	}
}

// collectSpellIDs extracts unique spell IDs from display entries.
func collectSpellIDs(spells []SpellDisplayEntry) []string {
	seen := make(map[string]bool, len(spells))
	var ids []string
	for _, s := range spells {
		if s.ID != "" && !seen[s.ID] {
			seen[s.ID] = true
			ids = append(ids, s.ID)
		}
	}
	return ids
}

// formatSpellRange converts a refdata.Spell's range fields into a display string.
func formatSpellRange(s refdata.Spell) string {
	switch s.RangeType {
	case "self":
		return "Self"
	case "touch":
		return "Touch"
	default:
		if s.RangeFt.Valid {
			return fmt.Sprintf("%dft", s.RangeFt.Int32)
		}
		return s.RangeType
	}
}

// extractSpells parses spells from character_data, handling both portal ([]string)
// and DDB ([]character.DDBSpellEntry) formats. Also extracts prepared_spells for prepared indicators.
func extractSpells(charData pqtype.NullRawMessage) []SpellDisplayEntry {
	if !charData.Valid || len(charData.RawMessage) == 0 {
		return nil
	}

	var data map[string]json.RawMessage
	if err := json.Unmarshal(charData.RawMessage, &data); err != nil {
		return nil
	}

	spellsRaw, ok := data["spells"]
	if !ok {
		return nil
	}

	// Build prepared set from prepared_spells
	preparedSet := make(map[string]bool)
	if prepRaw, ok := data["prepared_spells"]; ok {
		var prepared []string
		if err := json.Unmarshal(prepRaw, &prepared); err == nil {
			for _, id := range prepared {
				preparedSet[id] = true
			}
		}
	}

	// Try portal format first: []string
	var portalSpells []string
	if err := json.Unmarshal(spellsRaw, &portalSpells); err == nil && len(portalSpells) > 0 {
		// Check that first element is a string, not an object
		// json.Unmarshal of [{"name":"x"}] into []string would fail, so this is safe
		entries := make([]SpellDisplayEntry, len(portalSpells))
		for i, id := range portalSpells {
			entries[i] = SpellDisplayEntry{
				ID:       id,
				Name:     id, // Default to ID; enrichment with DB lookup can happen later
				Prepared: preparedSet[id],
			}
		}
		return entries
	}

	// Try DDB format: []character.DDBSpellEntry
	var ddbSpells []character.DDBSpellEntry
	if err := json.Unmarshal(spellsRaw, &ddbSpells); err == nil && len(ddbSpells) > 0 {
		entries := make([]SpellDisplayEntry, len(ddbSpells))
		for i, s := range ddbSpells {
			entries[i] = SpellDisplayEntry{
				ID:       character.Slugify(s.Name),
				Name:     s.Name,
				Level:    s.Level,
				Source:   s.Source,
				Homebrew: s.Homebrew,
				OffList:  s.OffList,
			}
		}
		return entries
	}

	return nil
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

// hydrateFromCombatant populates live combat state (conditions, exhaustion,
// concentration) from the character's active combatant row. No-op when the
// character is not in an active encounter.
func (a *CharacterSheetStoreAdapter) hydrateFromCombatant(ctx context.Context, charID uuid.UUID, data *CharacterSheetData) {
	cb, err := a.q.GetActiveCombatantByCharacterID(ctx, uuid.NullUUID{UUID: charID, Valid: true})
	if err != nil {
		return // not in combat or query error — leave defaults
	}

	data.ExhaustionLevel = int(cb.ExhaustionLevel)
	if cb.ConcentrationSpellName.Valid {
		data.ConcentrationOn = cb.ConcentrationSpellName.String
	}

	// Parse conditions JSON
	type condEntry struct {
		Condition string `json:"condition"`
	}
	var conds []condEntry
	if len(cb.Conditions) > 0 {
		if json.Unmarshal(cb.Conditions, &conds) == nil {
			for _, c := range conds {
				data.Conditions = append(data.Conditions, c.Condition)
			}
		}
	}
}
