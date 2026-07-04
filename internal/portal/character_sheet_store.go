package portal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/rest"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
)

// CharacterQuerier is the subset of refdata.Queries needed for character sheet loading.
type CharacterQuerier interface {
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	GetPlayerCharacterByCharacter(ctx context.Context, arg refdata.GetPlayerCharacterByCharacterParams) (refdata.PlayerCharacter, error)
	GetSpellsByIDs(ctx context.Context, ids []string) ([]refdata.Spell, error)
	GetActiveCombatantByCharacterID(ctx context.Context, characterID uuid.NullUUID) (refdata.Combatant, error)
	GetCampaignByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error)
	GetPlayerCharacterByDiscordUser(ctx context.Context, arg refdata.GetPlayerCharacterByDiscordUserParams) (refdata.PlayerCharacter, error)
	ListWeapons(ctx context.Context) ([]refdata.Weapon, error)
	ListArmor(ctx context.Context) ([]refdata.Armor, error)
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

// CanViewCharacter reports whether requestingUserID may view the character's
// sheet without owning it: true for the campaign DM or any non-retired player
// in the character's campaign.
func (a *CharacterSheetStoreAdapter) CanViewCharacter(ctx context.Context, characterID, requestingUserID string) (bool, error) {
	charUUID, err := uuid.Parse(characterID)
	if err != nil {
		return false, fmt.Errorf("invalid character ID: %w", err)
	}

	ch, err := a.q.GetCharacter(ctx, charUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrCharacterNotFound
		}
		return false, fmt.Errorf("getting character: %w", err)
	}

	camp, err := a.q.GetCampaignByID(ctx, ch.CampaignID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("getting campaign: %w", err)
	}
	if err == nil && camp.DmUserID == requestingUserID {
		return true, nil
	}

	// Any non-retired player in the same campaign may view the sheet.
	_, err = a.q.GetPlayerCharacterByDiscordUser(ctx, refdata.GetPlayerCharacterByDiscordUserParams{
		CampaignID:    ch.CampaignID,
		DiscordUserID: requestingUserID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("getting player character: %w", err)
	}
	return true, nil
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

	// Join weapon/armor stats onto inventory items and equipped slots.
	a.enrichEquipment(ctx, data)

	return data, nil
}

// enrichEquipment joins reference weapon/armor stats onto inventory items (by
// item id) and resolves each equipped slot's id to a display name plus its stat
// block. Best-effort, like enrichSpells: a missing ref table or an unmatched id
// just leaves the item name-only, never failing the sheet load.
func (a *CharacterSheetStoreAdapter) enrichEquipment(ctx context.Context, data *CharacterSheetData) {
	weapons := a.loadWeaponStats(ctx)
	armor := a.loadArmorStats(ctx)
	catalog := refdata.ItemCatalogByID()

	for i := range data.Inventory {
		id := data.Inventory[i].ItemID
		// Resolve the display name from the catalog so legacy rows that stored a
		// raw id as the name (e.g. "crossbow-bolt") render as "Crossbow Bolts".
		// Off-catalog ids (magic items) keep their stored name.
		if e, ok := catalog[id]; ok {
			data.Inventory[i].Name = e.Name
		}
		if w, ok := weapons[id]; ok {
			data.Inventory[i].Weapon = w
		}
		if ar, ok := armor[id]; ok {
			data.Inventory[i].Armor = ar
		}
	}

	data.WeaponMasteries = resolveWeaponMasteries(data.masteryWeaponIDs, weapons, catalog)

	for _, slot := range []*EquippedSlot{&data.EquippedMainHand, &data.EquippedOffHand, &data.EquippedArmor} {
		if slot.ItemID == "" {
			continue
		}
		if e, ok := catalog[slot.ItemID]; ok {
			slot.Name = e.Name
		}
		if slot.Name == "" {
			slot.Name = slot.ItemID
		}
		if w, ok := weapons[slot.ItemID]; ok {
			slot.Weapon = w
		}
		if ar, ok := armor[slot.ItemID]; ok {
			slot.Armor = ar
		}
	}
}

// loadWeaponStats returns reference weapon stat blocks keyed by item id, or nil
// when the lookup fails (enrichment degrades to name-only).
func (a *CharacterSheetStoreAdapter) loadWeaponStats(ctx context.Context) map[string]*WeaponStats {
	rows, err := a.q.ListWeapons(ctx)
	if err != nil {
		return nil
	}
	out := make(map[string]*WeaponStats, len(rows))
	for _, w := range rows {
		out[w.ID] = weaponStatsFrom(w)
	}
	return out
}

// loadArmorStats returns reference armor stat blocks keyed by item id, or nil
// when the lookup fails.
func (a *CharacterSheetStoreAdapter) loadArmorStats(ctx context.Context) map[string]*ArmorStats {
	rows, err := a.q.ListArmor(ctx)
	if err != nil {
		return nil
	}
	out := make(map[string]*ArmorStats, len(rows))
	for _, ar := range rows {
		out[ar.ID] = armorStatsFrom(ar)
	}
	return out
}

// resolveWeaponMasteries pairs each chosen weapon-mastery id with the weapon's
// display name (from the item catalog) and mastery property (from the reference
// weapon stat block). Ids that resolve to no reference weapon, or to a weapon
// without a mastery property, are skipped — best-effort degradation mirroring
// enrichEquipment. Returns nil when nothing resolves so the template hides the
// section.
func resolveWeaponMasteries(ids []string, weapons map[string]*WeaponStats, catalog map[string]refdata.ItemCatalogEntry) []WeaponMasteryDisplay {
	if len(ids) == 0 {
		return nil
	}
	out := make([]WeaponMasteryDisplay, 0, len(ids))
	for _, id := range ids {
		w, ok := weapons[id]
		if !ok || w.Mastery == "" {
			continue
		}
		name := id
		if e, ok := catalog[id]; ok {
			name = e.Name
		}
		out = append(out, WeaponMasteryDisplay{Weapon: name, Mastery: w.Mastery})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// extractWeaponMasteries pulls the chosen weapon-mastery ids from the
// character_data JSONB bag. Returns nil when absent or malformed.
func extractWeaponMasteries(charData pqtype.NullRawMessage) []string {
	if !charData.Valid || len(charData.RawMessage) == 0 {
		return nil
	}
	var cd struct {
		WeaponMasteries []string `json:"weapon_masteries"`
	}
	if err := json.Unmarshal(charData.RawMessage, &cd); err != nil {
		return nil
	}
	return cd.WeaponMasteries
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

	// Equipped slots hold item ids; name + stats are resolved by enrichEquipment.
	if ch.EquippedMainHand.Valid {
		data.EquippedMainHand = EquippedSlot{ItemID: ch.EquippedMainHand.String}
	}
	if ch.EquippedOffHand.Valid {
		data.EquippedOffHand = EquippedSlot{ItemID: ch.EquippedOffHand.String}
	}
	if ch.EquippedArmor.Valid {
		data.EquippedArmor = EquippedSlot{ItemID: ch.EquippedArmor.String}
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
	data.Inventory = wrapInventory(parseNullJSON[[]character.InventoryItem](ch.Inventory))
	data.AttunementSlots = parseNullJSON[[]character.AttunementSlot](ch.AttunementSlots)
	data.SpellSlots = parseNullJSON[map[string]character.SlotInfo](ch.SpellSlots)
	data.PactMagicSlots = parseNullJSONPtr[character.PactMagicSlots](ch.PactMagicSlots)
	data.FeatureUses = parseNullJSON[map[string]character.FeatureUse](ch.FeatureUses)
	data.HitDiceRemaining = parseHitDiceRemaining(ch.HitDiceRemaining)
	data.Spells = extractSpells(ch.CharacterData)
	data.masteryWeaponIDs = extractWeaponMasteries(ch.CharacterData)

	// Persistent (out-of-combat) condition + exhaustion state from the character
	// row. hydrateFromCombatant later overrides these from the live combatant.
	data.Conditions = conditionNamesFromJSON(ch.Conditions)
	level, _ := rest.ExhaustionLevelFromCharacterData(ch.CharacterData.RawMessage)
	data.ExhaustionLevel = level

	profile := character.ProfileFromCharacterData(ch.CharacterData.RawMessage)
	data.Appearance = profile.Appearance
	data.Backstory = profile.Backstory

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
		spells[i].Components = ref.Components
		spells[i].Duration = ref.Duration
		spells[i].Description = ref.Description
		spells[i].Concentration = ref.Concentration.Valid && ref.Concentration.Bool
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
		return appendGrantedSpells(entries, data)
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

// appendGrantedSpells appends Warlock Eldritch-Invocation granted spells
// (character_data.granted_spells, stored as []string separate from the
// known-spell budget) to the display list. Granted-only spells are tagged
// Source "invocation"; ids already present (manually known) are skipped so
// there are no duplicates. Best-effort: a missing or malformed granted_spells
// leaves entries unchanged.
func appendGrantedSpells(entries []SpellDisplayEntry, data map[string]json.RawMessage) []SpellDisplayEntry {
	grantedRaw, ok := data["granted_spells"]
	if !ok {
		return entries
	}
	var granted []string
	if err := json.Unmarshal(grantedRaw, &granted); err != nil {
		return entries
	}
	present := make(map[string]bool, len(entries))
	for _, e := range entries {
		present[e.ID] = true
	}
	for _, id := range granted {
		if present[id] {
			continue
		}
		present[id] = true
		entries = append(entries, SpellDisplayEntry{
			ID:     id,
			Name:   id,
			Source: "invocation",
		})
	}
	return entries
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

// hydrateFromCombatant populates live combat state (HP, conditions, exhaustion,
// concentration) from the character's active combatant row. No-op when the
// character is not in an active encounter.
func (a *CharacterSheetStoreAdapter) hydrateFromCombatant(ctx context.Context, charID uuid.UUID, data *CharacterSheetData) {
	cb, err := a.q.GetActiveCombatantByCharacterID(ctx, uuid.NullUUID{UUID: charID, Valid: true})
	if err != nil {
		return // not in combat or query error — leave defaults
	}

	// HP is the live combat snapshot: combat seeds the combatant from the
	// character at start and never writes back, so the character row is stale
	// mid-fight. Overlay it from the combatant.
	data.HpCurrent = int(cb.HpCurrent)
	data.HpMax = int(cb.HpMax)
	data.TempHP = int(cb.TempHp)

	data.ExhaustionLevel = int(cb.ExhaustionLevel)
	if cb.ConcentrationSpellName.Valid {
		data.ConcentrationOn = cb.ConcentrationSpellName.String
	}

	// The combatant is the live source of truth during combat: replace the
	// persistent conditions carried in from the character row, do not append
	// (combat seeds the combatant from the character at start, so appending
	// would double any carried-in condition).
	data.Conditions = conditionNamesFromJSON(cb.Conditions)
}

// conditionNamesFromJSON parses a JSON array of condition objects — each shaped
// like {"condition": "<name>", ...} — into a slice of condition names. Empty,
// nil, or invalid input yields a nil slice.
func conditionNamesFromJSON(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var entries []struct {
		Condition string `json:"condition"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Condition)
	}
	return names
}
