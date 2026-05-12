package portal

import (
	"context"
	"database/sql"
	"encoding/json"
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
			lvl := c.Level
			if lvl < 1 {
				lvl = 1
			}
			out = append(out, character.ClassEntry{Class: c.Class, Subclass: c.Subclass, Level: lvl})
		}
		if len(out) > 0 {
			return out
		}
	}
	return []character.ClassEntry{{Class: p.Class, Subclass: p.Subclass, Level: 1}}
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
		Skills: p.Skills,
		Saves:  p.Saves,
	})

	var inventoryMsg pqtype.NullRawMessage
	if items := EquipmentToInventoryWithEquipped(p.Equipment, p.EquippedWeapon, p.WornArmor); len(items) > 0 {
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
		campID = uuid.New()
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
		SpeedFt:          int32(p.SpeedFt),
		ProficiencyBonus: int32(p.ProfBonus),
		EquippedMainHand: equippedMainHand,
		EquippedArmor:    equippedArmor,
		HitDiceRemaining: hitDiceJSON,
		Features:         featuresMsg,
		Proficiencies:    pqtype.NullRawMessage{RawMessage: profJSON, Valid: true},
		Languages:        p.Languages,
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
		campID = uuid.New()
	}
	charID, err := uuid.Parse(p.CharacterID)
	if err != nil {
		charID = uuid.New()
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

// RedeemToken marks the token as used.
func (a *BuilderStoreAdapter) RedeemToken(ctx context.Context, token string) error {
	if a.tokenSvc == nil {
		return nil
	}
	return a.tokenSvc.RedeemToken(ctx, token)
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
// marking the equipped weapon and worn armor items.
func EquipmentToInventoryWithEquipped(equipment []string, equippedWeapon, wornArmor string) []character.InventoryItem {
	if len(equipment) == 0 {
		return nil
	}
	items := make([]character.InventoryItem, len(equipment))
	for i, id := range equipment {
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
		items[i] = item
	}
	return items
}

// DeriveSpeed returns the default speed for a race (30 ft default).
func DeriveSpeed(_ string) int {
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

// DeriveAC calculates AC for a character with no armor.
func DeriveAC(scores character.AbilityScores) int {
	return character.CalculateAC(scores, nil, false, "")
}
