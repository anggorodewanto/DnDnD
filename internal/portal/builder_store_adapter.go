package portal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
)

// CharacterCreator is the subset of refdata.Queries for character creation.
type CharacterCreator interface {
	CreateCharacter(ctx context.Context, arg refdata.CreateCharacterParams) (refdata.Character, error)
	CreatePlayerCharacter(ctx context.Context, arg refdata.CreatePlayerCharacterParams) (refdata.PlayerCharacter, error)
	GetPlayerCharacterByDiscordUser(ctx context.Context, arg refdata.GetPlayerCharacterByDiscordUserParams) (refdata.PlayerCharacter, error)
	RelinkPlayerCharacter(ctx context.Context, arg refdata.RelinkPlayerCharacterParams) (refdata.PlayerCharacter, error)
	UpsertCharacterDraft(ctx context.Context, arg refdata.UpsertCharacterDraftParams) error
	GetCharacterDraft(ctx context.Context, arg refdata.GetCharacterDraftParams) (json.RawMessage, error)
}

type campaignGetter interface {
	GetCampaignByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error)
}

// BuilderStoreAdapter adapts refdata.Queries + TokenService to BuilderStore.
type BuilderStoreAdapter struct {
	q        CharacterCreator
	tokenSvc *TokenService
}

// resolveClassEntries returns the multiclass entries to persist. When the
// caller supplied a non-empty Classes slice it is filtered to drop empty
// rows; otherwise a single ClassEntry is constructed from Class/Subclass
// at level 1 (the legacy single-class path).
func resolveClassEntries(p CreateCharacterParams) []character.ClassEntry {
	if len(p.Classes) > 0 {
		out := make([]character.ClassEntry, 0, len(p.Classes))
		for _, c := range p.Classes {
			if c.Class == "" {
				continue
			}
			lvl := max(c.Level, 1)
			out = append(out, character.ClassEntry{Class: c.Class, Subclass: c.Subclass, Level: lvl, IsPrimary: c.IsPrimary})
		}
		if len(out) > 0 {
			ensurePrimary(out)
			return out
		}
	}
	return []character.ClassEntry{{Class: p.Class, Subclass: p.Subclass, Level: 1, IsPrimary: true}}
}

// ensurePrimary sets IsPrimary on the first entry if no entry has it set.
func ensurePrimary(classes []character.ClassEntry) {
	for _, c := range classes {
		if c.IsPrimary {
			return
		}
	}
	classes[0].IsPrimary = true
}

// NewBuilderStoreAdapter creates a new BuilderStoreAdapter.
func NewBuilderStoreAdapter(q CharacterCreator, tokenSvc *TokenService) *BuilderStoreAdapter {
	return &BuilderStoreAdapter{q: q, tokenSvc: tokenSvc}
}

// CreateCharacterRecord creates a character in the database.
func (a *BuilderStoreAdapter) CreateCharacterRecord(ctx context.Context, p CreateCharacterParams) (string, error) {
	scoresJSON, _ := json.Marshal(p.AbilityScores)
	classEntries := resolveClassEntries(p)
	classesJSON, _ := json.Marshal(classEntries)
	hitDice := make(map[string]int, len(classEntries))
	totalLevel := 0
	for _, ce := range classEntries {
		hitDice[ce.Class] = ce.Level
		totalLevel += ce.Level
	}
	if totalLevel < 1 {
		totalLevel = 1
	}
	hitDiceJSON, _ := json.Marshal(hitDice)
	profJSON, _ := json.Marshal(character.Proficiencies{
		Skills:    p.Skills,
		Saves:     p.Saves,
		Expertise: p.Expertise,
	})

	var inventoryMsg pqtype.NullRawMessage
	allEquipment := p.Equipment
	if bgItems := BackgroundEquipmentPack(p.Background); len(bgItems) > 0 {
		allEquipment = append(allEquipment, bgItems...)
	}
	if items := EquipmentToInventoryWithEquipped(allEquipment, p.EquippedWeapon, p.WornArmor); len(items) > 0 {
		invJSON, _ := json.Marshal(items)
		inventoryMsg = pqtype.NullRawMessage{RawMessage: invJSON, Valid: true}
	}

	var equippedMainHand sql.NullString
	if p.EquippedWeapon != "" {
		equippedMainHand = sql.NullString{String: p.EquippedWeapon, Valid: true}
	}
	var equippedArmor sql.NullString
	if p.WornArmor != "" {
		equippedArmor = sql.NullString{String: p.WornArmor, Valid: true}
	}
	// ISSUE-011: an equipped shield (flagged off_hand by the inventory builder)
	// must also populate the dedicated equipped_off_hand column. Mirror the
	// shield detection used for the Unarmored Defense AC formula below.
	var equippedOffHand sql.NullString
	if hasEquipmentItem(p.Equipment, "shield") {
		equippedOffHand = sql.NullString{String: "shield", Valid: true}
	}

	var featureUsesMsg pqtype.NullRawMessage
	if fu := InitFeatureUses(classEntries, p.AbilityScores); len(fu) > 0 {
		fuJSON, _ := json.Marshal(fu)
		featureUsesMsg = pqtype.NullRawMessage{RawMessage: fuJSON, Valid: true}
	}

	var pactMagicMsg pqtype.NullRawMessage
	if pact := pactMagicSlotsForClasses(classEntries); pact != nil {
		pactJSON, _ := json.Marshal(pact)
		pactMagicMsg = pqtype.NullRawMessage{RawMessage: pactJSON, Valid: true}
	}

	// Standard spell slots (ISSUE-002). Persist in the canonical string-keyed
	// {current,max} shape the play/read path consumes; leave NULL for
	// non-casters so fighters/rogues/barbarians are unaffected.
	var spellSlotsMsg pqtype.NullRawMessage
	if slots := spellSlotsForClasses(classEntries); slots != nil {
		slotsJSON, _ := json.Marshal(slots)
		spellSlotsMsg = pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true}
	}

	var featuresMsg pqtype.NullRawMessage
	if len(p.Features) > 0 {
		featJSON, _ := json.Marshal(p.Features)
		featuresMsg = pqtype.NullRawMessage{RawMessage: featJSON, Valid: true}
	}

	// character_data carries spells today; we also stash subrace and
	// background here since the characters table has no dedicated column
	// for them. Downstream code can read these without a migration.
	charData := map[string]any{}
	if len(p.Spells) > 0 {
		charData["spells"] = p.Spells
	}
	if len(p.WeaponMasteries) > 0 {
		charData["weapon_masteries"] = p.WeaponMasteries
	}
	if p.Subrace != "" {
		charData["subrace"] = p.Subrace
	}
	if p.Background != "" {
		charData["background"] = p.Background
	}
	var charDataMsg pqtype.NullRawMessage
	if len(charData) > 0 {
		charDataJSON, _ := json.Marshal(charData)
		charDataMsg = pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true}
	}

	campID, err := uuid.Parse(p.CampaignID)
	if err != nil {
		return "", fmt.Errorf("invalid campaign_id %q: %w", p.CampaignID, err)
	}

	// ISSUE-004: persist the Unarmored Defense ac_formula so an unarmored
	// Barbarian/Monk gets the right AC at creation and on every play-time
	// recompute (combat.RecalculateAC reads char.AcFormula). Shield detection
	// mirrors the inventory builder (EquipmentToInventoryWithEquipped marks any
	// "shield" item equipped). NULL for armored characters and non-UD classes
	// so fighters/wizards/etc. are unchanged.
	var acFormula sql.NullString
	if f := unarmoredDefenseFormula(classEntries, p.WornArmor, hasEquipmentItem(p.Equipment, "shield")); f != "" {
		acFormula = sql.NullString{String: f, Valid: true}
	}

	// characters.languages is TEXT[] NOT NULL. pq.Array of a nil slice writes
	// SQL NULL, which violates the constraint and 500s the create. The builder
	// does not yet collect concrete languages, so default to a non-nil empty
	// array. (Real race/background language population is a follow-up.)
	languages := p.Languages
	if languages == nil {
		languages = []string{}
	}

	ch, err := a.q.CreateCharacter(ctx, refdata.CreateCharacterParams{
		CampaignID:       campID,
		Name:             p.Name,
		Race:             p.Race,
		Classes:          classesJSON,
		Level:            int32(totalLevel),
		AbilityScores:    scoresJSON,
		HpMax:            int32(p.HPMax),
		HpCurrent:        int32(p.HPMax),
		TempHp:           0,
		Ac:               int32(p.AC),
		AcFormula:        acFormula,
		SpeedFt:          int32(p.SpeedFt),
		ProficiencyBonus: int32(p.ProfBonus),
		EquippedMainHand: equippedMainHand,
		EquippedOffHand:  equippedOffHand,
		EquippedArmor:    equippedArmor,
		HitDiceRemaining: hitDiceJSON,
		FeatureUses:      featureUsesMsg,
		SpellSlots:       spellSlotsMsg,
		PactMagicSlots:   pactMagicMsg,
		Features:         featuresMsg,
		Proficiencies:    pqtype.NullRawMessage{RawMessage: profJSON, Valid: true},
		Languages:        languages,
		Inventory:        inventoryMsg,
		CharacterData:    charDataMsg,
		Gold:             0,
		Homebrew:         sql.NullBool{Bool: false, Valid: true},
	})
	if err != nil {
		return "", err
	}
	return ch.ID.String(), nil
}

// CreatePlayerCharacterRecord creates a player_characters row.
func (a *BuilderStoreAdapter) CreatePlayerCharacterRecord(ctx context.Context, p CreatePlayerCharacterParams) (string, error) {
	campID, err := uuid.Parse(p.CampaignID)
	if err != nil {
		return "", fmt.Errorf("invalid campaign_id %q: %w", p.CampaignID, err)
	}
	charID, err := uuid.Parse(p.CharacterID)
	if err != nil {
		return "", fmt.Errorf("invalid character_id %q: %w", p.CharacterID, err)
	}

	pc, err := a.q.CreatePlayerCharacter(ctx, refdata.CreatePlayerCharacterParams{
		CampaignID:    campID,
		CharacterID:   charID,
		DiscordUserID: p.DiscordUserID,
		Status:        p.Status,
		CreatedVia:    p.CreatedVia,
	})
	if err != nil {
		return "", err
	}
	return pc.ID.String(), nil
}

// ActivePlayerCharacter returns the non-retired player_characters row for the
// (campaign, player), or (nil, nil) when none exists.
func (a *BuilderStoreAdapter) ActivePlayerCharacter(ctx context.Context, campaignID, discordUserID string) (*ActivePlayerCharacter, error) {
	campID, err := uuid.Parse(campaignID)
	if err != nil {
		return nil, err
	}
	pc, err := a.q.GetPlayerCharacterByDiscordUser(ctx, refdata.GetPlayerCharacterByDiscordUserParams{
		CampaignID:    campID,
		DiscordUserID: discordUserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ActivePlayerCharacter{ID: pc.ID.String(), Status: pc.Status}, nil
}

// RelinkPlayerCharacterRecord re-points an existing row at a freshly built
// character and resets it to pending.
func (a *BuilderStoreAdapter) RelinkPlayerCharacterRecord(ctx context.Context, pcID, characterID, createdVia string) (string, error) {
	id, err := uuid.Parse(pcID)
	if err != nil {
		return "", err
	}
	charID, err := uuid.Parse(characterID)
	if err != nil {
		return "", err
	}
	pc, err := a.q.RelinkPlayerCharacter(ctx, refdata.RelinkPlayerCharacterParams{
		ID:          id,
		CharacterID: charID,
		CreatedVia:  createdVia,
	})
	if err != nil {
		return "", err
	}
	return pc.ID.String(), nil
}

// ValidateToken checks that the token is valid and returns the token record.
func (a *BuilderStoreAdapter) ValidateToken(ctx context.Context, token string) (*PortalToken, error) {
	if a.tokenSvc == nil {
		return nil, nil
	}
	return a.tokenSvc.ValidateToken(ctx, token)
}

// RedeemToken marks the token as used.
func (a *BuilderStoreAdapter) RedeemToken(ctx context.Context, token string) error {
	if a.tokenSvc == nil {
		return nil
	}
	return a.tokenSvc.RedeemToken(ctx, token)
}

// SaveCharacterDraft upserts the in-progress builder draft for the
// (campaign, player, mode) key (T11 / Finding 4·b).
func (a *BuilderStoreAdapter) SaveCharacterDraft(ctx context.Context, campaignID, discordUserID, mode string, draft json.RawMessage) error {
	campID, err := uuid.Parse(campaignID)
	if err != nil {
		return fmt.Errorf("invalid campaign_id %q: %w", campaignID, err)
	}
	return a.q.UpsertCharacterDraft(ctx, refdata.UpsertCharacterDraftParams{
		CampaignID:    campID,
		DiscordUserID: discordUserID,
		Mode:          mode,
		Draft:         draft,
	})
}

// LoadCharacterDraft returns the stored builder draft for (campaign, player,
// mode), or (nil, nil) when no draft row exists.
func (a *BuilderStoreAdapter) LoadCharacterDraft(ctx context.Context, campaignID, discordUserID, mode string) (json.RawMessage, error) {
	campID, err := uuid.Parse(campaignID)
	if err != nil {
		return nil, fmt.Errorf("invalid campaign_id %q: %w", campaignID, err)
	}
	draft, err := a.q.GetCharacterDraft(ctx, refdata.GetCharacterDraftParams{
		CampaignID:    campID,
		DiscordUserID: discordUserID,
		Mode:          mode,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return draft, nil
}

// AllowedAbilityScoreMethods reads campaign settings for creation method gating.
func (a *BuilderStoreAdapter) AllowedAbilityScoreMethods(ctx context.Context, campaignID string) ([]AbilityScoreMethod, error) {
	getter, ok := a.q.(campaignGetter)
	if !ok {
		return DefaultAbilityScoreMethods(), nil
	}
	id, err := uuid.Parse(campaignID)
	if err != nil {
		return DefaultAbilityScoreMethods(), nil
	}
	campaign, err := getter.GetCampaignByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !campaign.Settings.Valid {
		return DefaultAbilityScoreMethods(), nil
	}
	return AbilityScoreMethodsFromSettings(campaign.Settings.RawMessage), nil
}

// knownWeapons is the set of weapon IDs from the SRD.
var knownWeapons = map[string]bool{
	"club": true, "dagger": true, "greatclub": true, "handaxe": true, "javelin": true,
	"light-hammer": true, "mace": true, "quarterstaff": true, "sickle": true, "spear": true,
	"light-crossbow": true, "dart": true, "shortbow": true, "sling": true,
	"battleaxe": true, "flail": true, "glaive": true, "greataxe": true, "greatsword": true,
	"halberd": true, "lance": true, "longsword": true, "maul": true, "morningstar": true,
	"pike": true, "rapier": true, "scimitar": true, "shortsword": true, "trident": true,
	"war-pick": true, "warhammer": true, "whip": true, "blowgun": true, "hand-crossbow": true,
	"heavy-crossbow": true, "longbow": true, "net": true,
}

// knownArmor is the set of armor IDs from the SRD.
var knownArmor = map[string]bool{
	"padded": true, "leather": true, "studded-leather": true,
	"hide": true, "chain-shirt": true, "scale-mail": true, "breastplate": true, "half-plate": true,
	"ring-mail": true, "chain-mail": true, "splint": true, "plate": true,
	"shield": true,
}

// itemType returns "weapon", "armor", or "gear" for an item ID.
func itemType(id string) string {
	if knownWeapons[id] {
		return "weapon"
	}
	if knownArmor[id] {
		return "armor"
	}
	return "gear"
}

// EquipmentToInventory converts equipment ID strings to InventoryItem structs.
func EquipmentToInventory(equipment []string) []character.InventoryItem {
	return EquipmentToInventoryWithEquipped(equipment, "", "")
}

// EquipmentToInventoryWithEquipped converts equipment IDs to InventoryItems,
// marking the equipped weapon and worn armor items. Unresolved placeholder IDs
// (e.g. "any-martial", "any-simple-melee") are skipped — the client must
// resolve these to concrete item IDs before submission.
func EquipmentToInventoryWithEquipped(equipment []string, equippedWeapon, wornArmor string) []character.InventoryItem {
	if len(equipment) == 0 {
		return nil
	}
	items := make([]character.InventoryItem, 0, len(equipment))
	for _, id := range equipment {
		if strings.HasPrefix(id, "any-") {
			continue // skip unresolved placeholder
		}
		item := character.InventoryItem{
			ItemID:   id,
			Name:     id,
			Quantity: 1,
			Type:     itemType(id),
		}
		if equippedWeapon != "" && strings.EqualFold(id, equippedWeapon) {
			item.Equipped = true
			item.EquipSlot = "main_hand"
		}
		if wornArmor != "" && strings.EqualFold(id, wornArmor) {
			item.Equipped = true
			item.EquipSlot = "armor"
		}
		if strings.EqualFold(id, "shield") {
			item.Equipped = true
			item.EquipSlot = "off_hand"
		}
		items = append(items, item)
	}
	return items
}

// raceSpeed maps race IDs to their base walking speed in feet.
var raceSpeed = map[string]int{
	"dwarf":    25,
	"halfling": 25,
	"gnome":    25,
}

// DeriveSpeed returns the base walking speed for a race (30 ft default).
func DeriveSpeed(race string) int {
	if spd, ok := raceSpeed[strings.ToLower(race)]; ok {
		return spd
	}
	return 30
}

// ClassHitDie returns the hit die string for a class.
func ClassHitDie(class string) string {
	switch strings.ToLower(class) {
	case "barbarian":
		return "d12"
	case "fighter", "paladin", "ranger":
		return "d10"
	case "sorcerer", "wizard":
		return "d6"
	default:
		return "d8"
	}
}

// DeriveHP calculates HP for a level 1 character.
func DeriveHP(class string, scores character.AbilityScores) int {
	classes := []character.ClassEntry{{Class: class, Level: 1}}
	hitDice := map[string]string{class: ClassHitDie(class)}
	return character.CalculateHP(classes, hitDice, scores)
}

// DeriveHPMulticlass calculates HP for a multiclass character using all class entries.
func DeriveHPMulticlass(classes []character.ClassEntry, scores character.AbilityScores) int {
	hitDice := make(map[string]string, len(classes))
	for _, c := range classes {
		hitDice[c.Class] = ClassHitDie(c.Class)
	}
	return character.CalculateHP(classes, hitDice, scores)
}

// DeriveAC calculates AC for a character with no armor.
func DeriveAC(scores character.AbilityScores) int {
	return character.CalculateAC(scores, nil, false, "")
}
