package character

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// MaxSpellSlotLevel is the highest leveled spell-slot level in 5e (9th).
const MaxSpellSlotLevel = 9

// MaxPactSlotLevel is the highest Warlock pact-magic slot level in 5e (5th).
const MaxPactSlotLevel = 5

// ValidateSpellSlots checks a DM-supplied spell-slot map. Each level must be
// 1..MaxSpellSlotLevel, max must be >= 0, and current must be within 0..max.
// A nil or empty map is valid — it represents a caster with no leveled slots.
func ValidateSpellSlots(slots map[int]SlotInfo) error {
	for level, info := range slots {
		if level < 1 || level > MaxSpellSlotLevel {
			return fmt.Errorf("spell slot level %d out of range (1-%d)", level, MaxSpellSlotLevel)
		}
		if info.Max < 0 {
			return fmt.Errorf("spell slot level %d: max must be >= 0", level)
		}
		if info.Current < 0 || info.Current > info.Max {
			return fmt.Errorf("spell slot level %d: current %d out of range (0-%d)", level, info.Current, info.Max)
		}
	}
	return nil
}

// ValidatePactSlots checks DM-supplied Warlock pact-magic slots: slot level
// 1..MaxPactSlotLevel, max >= 0, and current within 0..max. The zero value
// (all fields 0) is valid and represents a character with no pact magic.
func ValidatePactSlots(p PactMagicSlots) error {
	if p == (PactMagicSlots{}) {
		return nil
	}
	if p.SlotLevel < 1 || p.SlotLevel > MaxPactSlotLevel {
		return fmt.Errorf("pact slot level %d out of range (1-%d)", p.SlotLevel, MaxPactSlotLevel)
	}
	if p.Max < 0 {
		return fmt.Errorf("pact magic: max must be >= 0")
	}
	if p.Current < 0 || p.Current > p.Max {
		return fmt.Errorf("pact magic: current %d out of range (0-%d)", p.Current, p.Max)
	}
	return nil
}

// ParseSpellSlotsJSON parses the wire/storage JSON for spell slots — a
// string-keyed object like {"1":{"current":2,"max":4}} — into an int-keyed
// map. Empty or nil input yields an empty map. Returns an error if a key is
// not an integer.
func ParseSpellSlotsJSON(raw []byte) (map[int]SlotInfo, error) {
	out := map[int]SlotInfo{}
	if len(raw) == 0 {
		return out, nil
	}
	var strKeyed map[string]SlotInfo
	if err := json.Unmarshal(raw, &strKeyed); err != nil {
		return nil, fmt.Errorf("parsing spell slots: %w", err)
	}
	for k, v := range strKeyed {
		level, err := strconv.Atoi(k)
		if err != nil {
			return nil, fmt.Errorf("spell slot key %q is not an integer level", k)
		}
		out[level] = v
	}
	return out, nil
}

// MarshalSpellSlotsJSON marshals an int-keyed slot map to the string-keyed
// storage JSON ({"1":{"current":2,"max":4}}).
func MarshalSpellSlotsJSON(slots map[int]SlotInfo) ([]byte, error) {
	strKeyed := make(map[string]SlotInfo, len(slots))
	for level, info := range slots {
		strKeyed[strconv.Itoa(level)] = info
	}
	return json.Marshal(strKeyed)
}
