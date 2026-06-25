package character

import (
	"encoding/json"
	"strings"
)

// CharacterProfile holds optional free-form descriptive text a player writes
// about their character (appearance, backstory). It is display-only flavor
// that is never queried, so it lives inside the character_data JSONB bag under
// the "appearance" and "backstory" keys rather than in dedicated columns —
// the same no-migration pattern the builder already uses for subrace and
// background. See builder_store_adapter.CreateCharacterRecord.
type CharacterProfile struct {
	Appearance string `json:"appearance,omitempty"`
	Backstory  string `json:"backstory,omitempty"`
}

// IsEmpty reports whether the player supplied no description at all (both
// fields blank or whitespace-only).
func (p CharacterProfile) IsEmpty() bool {
	return strings.TrimSpace(p.Appearance) == "" && strings.TrimSpace(p.Backstory) == ""
}

// ProfileFromCharacterData extracts the appearance/backstory fields from a
// character_data JSONB blob. It returns a zero CharacterProfile when raw is
// empty, null, malformed, or simply has neither key — callers render the zero
// value as "no description". Unrelated keys (spells, background, …) are ignored.
func ProfileFromCharacterData(raw []byte) CharacterProfile {
	var p CharacterProfile
	if len(raw) == 0 {
		return p
	}
	_ = json.Unmarshal(raw, &p)
	return p
}
