package portal

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// WeaponStats is the display weapon stat block joined from refdata.weapons by
// item id. Nil on a non-weapon inventory item / equipped slot.
type WeaponStats struct {
	Name       string   // display name, e.g. "Silent Blade" (homebrew fallback)
	Damage     string   // "1d8 slashing"
	Versatile  string   // versatile two-handed damage, e.g. "1d10"; "" if none
	Range      string   // "80/320 ft" for ranged/thrown; "" for pure melee
	WeaponType string   // "Martial Melee"
	Mastery    string   // "Sap"; "" if none
	Properties []string // ["Finesse", "Light"]
}

// WeaponMasteryDisplay is one weapon mastery the character has chosen (2024
// rules): the weapon's display name paired with the mastery property it grants.
type WeaponMasteryDisplay struct {
	Weapon  string // "Longsword"
	Mastery string // "Sap"
}

// ArmorStats is the display armor stat block joined from refdata.armor by item
// id. Nil on a non-armor inventory item / equipped slot.
type ArmorStats struct {
	Name          string // display name, e.g. "Gloom Plate" (homebrew fallback)
	AC            string // "16", "12 + DEX (max 2)", "+2" (shield)
	ArmorType     string // "Light" / "Medium" / "Heavy" / "Shield"
	StrengthReq   int    // 0 = none
	StealthDisadv bool
}

// EquippedSlot is one equipment slot (main hand / off hand / armor) resolved
// from the stored item id to a display name plus, when the id matches a
// reference weapon/armor, its stat block. Empty reports an unfilled slot.
type EquippedSlot struct {
	ItemID string
	Name   string
	Weapon *WeaponStats
	Armor  *ArmorStats
}

// Empty reports whether the slot holds no item.
func (s EquippedSlot) Empty() bool { return s.ItemID == "" }

// InventoryDisplayItem wraps a stored inventory item with the weapon/armor stat
// block joined from the reference tables (nil for plain gear), so the sheet can
// render an expandable row with stats.
type InventoryDisplayItem struct {
	character.InventoryItem
	Weapon *WeaponStats
	Armor  *ArmorStats
}

// HasDetail reports whether the item has an expandable detail body — weapon or
// armor stats, or magic-item specifics — vs. a plain one-line row.
func (i InventoryDisplayItem) HasDetail() bool {
	return i.Weapon != nil || i.Armor != nil || i.IsMagic ||
		i.MagicProperties != "" || i.RequiresAttunement || i.MaxCharges > 0 ||
		i.Description != ""
}

// wrapInventory lifts stored inventory items into display items (stats attached
// later by enrichEquipment). Returns nil for an empty list so the template's
// "No items" branch and the empty-JSONB test still hold.
func wrapInventory(items []character.InventoryItem) []InventoryDisplayItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]InventoryDisplayItem, len(items))
	for i, it := range items {
		out[i] = InventoryDisplayItem{InventoryItem: it}
	}
	return out
}

// weaponStatsFrom builds a display stat block from a reference weapon row.
func weaponStatsFrom(w refdata.Weapon) *WeaponStats {
	ws := &WeaponStats{
		Name:       w.Name,
		Damage:     formatDamage(w.Damage, w.DamageType),
		WeaponType: formatUnderscored(w.WeaponType),
		Mastery:    capitalizeFirst(w.Mastery),
		Properties: titleizeAll(w.Properties),
	}
	if w.VersatileDamage.Valid {
		ws.Versatile = w.VersatileDamage.String
	}
	if w.RangeNormalFt.Valid && w.RangeLongFt.Valid {
		ws.Range = fmt.Sprintf("%d/%d ft", w.RangeNormalFt.Int32, w.RangeLongFt.Int32)
	}
	return ws
}

// armorStatsFrom builds a display stat block from a reference armor row.
func armorStatsFrom(a refdata.Armor) *ArmorStats {
	as := &ArmorStats{
		Name:          a.Name,
		AC:            formatArmorAC(a),
		ArmorType:     capitalizeFirst(a.ArmorType),
		StealthDisadv: a.StealthDisadv.Valid && a.StealthDisadv.Bool,
	}
	if a.StrengthReq.Valid {
		as.StrengthReq = int(a.StrengthReq.Int32)
	}
	return as
}

// formatArmorAC renders the AC contribution the way a player reads it: a flat
// number for heavy armor, "base + DEX" (capped for medium) for dex armor, and a
// "+N" bonus for a shield.
func formatArmorAC(a refdata.Armor) string {
	if a.ArmorType == "shield" {
		return fmt.Sprintf("+%d", a.AcBase)
	}
	if !(a.AcDexBonus.Valid && a.AcDexBonus.Bool) {
		return strconv.Itoa(int(a.AcBase))
	}
	if a.AcDexMax.Valid {
		return fmt.Sprintf("%d + DEX (max %d)", a.AcBase, a.AcDexMax.Int32)
	}
	return fmt.Sprintf("%d + DEX", a.AcBase)
}

// formatDamage joins a damage die and type for display, dropping the no-damage
// placeholder ("0"/"none") carried by oddities like the net.
func formatDamage(dice, dtype string) string {
	if dice == "" || dice == "0" {
		return ""
	}
	return strings.TrimSpace(dice + " " + dtype)
}

// formatUnderscored turns a snake_case enum ("martial_melee") into a display
// label ("Martial Melee").
func formatUnderscored(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "_")
	for i, p := range parts {
		parts[i] = capitalizeFirst(p)
	}
	return strings.Join(parts, " ")
}

// titleizeAll capitalizes each entry, returning nil for an empty input.
func titleizeAll(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = capitalizeFirst(s)
	}
	return out
}
