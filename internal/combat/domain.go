package combat

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/rest"
)

// CombatantParams holds the data needed to create a combatant,
// independent of the database layer.
type CombatantParams struct {
	CharacterID     string
	CreatureRefID   string
	ShortID         string
	DisplayName     string
	HPMax           int32
	HPCurrent       int32
	TempHP          int32
	AC              int32
	SpeedFt         int32
	PositionCol     string
	PositionRow     int32
	IsNPC           bool
	IsAlive         bool
	IsVisible       bool
	DeathSaves      json.RawMessage
	ExhaustionLevel int32
	Conditions      json.RawMessage
}

// defaultConditions returns raw unchanged, or an empty JSON array when raw is
// nil/empty, so a combatant always starts combat with a valid conditions array.
func defaultConditions(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("[]")
	}
	return raw
}

// DeathSaves tracks PC death saving throws.
type DeathSaves struct {
	Successes int `json:"successes"`
	Failures  int `json:"failures"`
}

// TemplateCreature represents a creature placement in an encounter template.
type TemplateCreature struct {
	CreatureRefID string `json:"creature_ref_id"`
	ShortID       string `json:"short_id"`
	DisplayName   string `json:"display_name"`
	PositionCol   string `json:"position_col"`
	PositionRow   int    `json:"position_row"`
	Quantity      int    `json:"quantity"`
}

// UnmarshalJSON accepts position_col as either a letter string ("A", seeded
// templates) or a 0-based integer (the dashboard encounter builder emits a
// numeric column index from canvasTileCoords). Numbers are normalized to the
// letter label so StartCombat's colToIndex round-trips; without this, a placed
// creature makes Start Combat fail with "cannot unmarshal number into ...
// position_col of type string".
func (tc *TemplateCreature) UnmarshalJSON(data []byte) error {
	// alias drops TemplateCreature's methods, avoiding recursion; the outer
	// position_col (RawMessage) shadows the embedded string field.
	type alias TemplateCreature
	aux := &struct {
		PositionCol json.RawMessage `json:"position_col"`
		*alias
	}{alias: (*alias)(tc)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	col, err := normalizeTemplateCol(aux.PositionCol)
	if err != nil {
		return err
	}
	tc.PositionCol = col
	return nil
}

// normalizeTemplateCol converts a position_col JSON value to a letter label.
// Strings pass through; non-negative numbers map via the 0-based index→label
// convention (0→"A"); null/missing yields "".
func normalizeTemplateCol(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	switch val := v.(type) {
	case nil:
		return "", nil
	case string:
		return val, nil
	case float64:
		if val < 0 {
			return "", fmt.Errorf("position_col index must be non-negative, got %v", val)
		}
		return indexToColLabel(int(val)), nil
	default:
		return "", fmt.Errorf("position_col must be a string or integer, got %T", val)
	}
}

// CombatantFromCreature builds CombatantParams from a creature stat block.
func CombatantFromCreature(creature refdata.Creature, shortID, displayName, posCol string, posRow int32) CombatantParams {
	return CombatantParams{
		CreatureRefID: creature.ID,
		ShortID:       shortID,
		DisplayName:   displayName,
		HPMax:         creature.HpAverage,
		HPCurrent:     creature.HpAverage,
		AC:            creature.Ac,
		SpeedFt:       ParseWalkSpeed(creature.Speed),
		PositionCol:   posCol,
		PositionRow:   posRow,
		IsNPC:         true,
		IsAlive:       true,
		IsVisible:     true,
	}
}

// CombatantFromCharacter builds CombatantParams from a player character.
func CombatantFromCharacter(char refdata.Character, shortID, posCol string, posRow int32) CombatantParams {
	ds, _ := json.Marshal(DeathSaves{Successes: 0, Failures: 0})
	exhaustionLevel := 0
	if char.CharacterData.Valid {
		exhaustionLevel, _ = rest.ExhaustionLevelFromCharacterData(char.CharacterData.RawMessage)
	}
	return CombatantParams{
		CharacterID:     char.ID.String(),
		ShortID:         shortID,
		DisplayName:     char.Name,
		HPMax:           char.HpMax,
		HPCurrent:       char.HpCurrent,
		TempHP:          char.TempHp,
		AC:              char.Ac,
		SpeedFt:         char.SpeedFt,
		PositionCol:     posCol,
		PositionRow:     posRow,
		IsNPC:           false,
		IsAlive:         true,
		IsVisible:       true,
		DeathSaves:      ds,
		ExhaustionLevel: int32(exhaustionLevel),
		Conditions:      defaultConditions(char.Conditions),
	}
}

// Position represents a map grid position.
type Position struct {
	Col string `json:"col"`
	Row int32  `json:"row"`
}

// InitiativeInput is a caller-supplied initiative for one player character at
// combat start (APP-1). Roll is the player's own total (their reported d20 plus
// their modifier); the app uses it verbatim and does NOT auto-roll that PC.
// Order optionally pins the combatant to a specific 1-based seat; when nil the
// seat is derived from the rolls (roll DESC → DEX → name → uuid).
type InitiativeInput struct {
	Roll  int32  `json:"roll"`
	Order *int32 `json:"order,omitempty"`
}

// StartCombatInput holds parameters for the StartCombat flow.
type StartCombatInput struct {
	TemplateID         uuid.UUID              `json:"template_id"`
	CharacterIDs       []uuid.UUID            `json:"character_ids"`
	CharacterPositions map[uuid.UUID]Position `json:"character_positions"`
	SurprisedShortIDs  []string               `json:"surprised_short_ids,omitempty"`
	// CharacterInitiatives, keyed by character UUID, supplies player-authoritative
	// initiative for PCs so the DM never rolls a player's die (APP-1). Combatants
	// without an entry (NPCs, and any un-supplied PC) auto-roll as before.
	CharacterInitiatives map[uuid.UUID]InitiativeInput `json:"character_initiatives,omitempty"`
}

// StartCombatResult holds the result of the StartCombat flow.
type StartCombatResult struct {
	Encounter         refdata.Encounter   `json:"encounter"`
	Combatants        []refdata.Combatant `json:"combatants"`
	InitiativeTracker string              `json:"initiative_tracker"`
	FirstTurn         TurnInfo            `json:"first_turn"`
}

// EndCombatResult holds the result of the EndCombat operation.
type EndCombatResult struct {
	Encounter         refdata.Encounter   `json:"encounter"`
	Combatants        []refdata.Combatant `json:"combatants"`
	Summary           string              `json:"summary"`
	Casualties        int                 `json:"casualties"`
	RoundsElapsed     int32               `json:"rounds_elapsed"`
	InitiativeTracker string              `json:"initiative_tracker"`
}

// ParseTemplateCreatures parses the JSONB creatures array from an encounter template.
func ParseTemplateCreatures(raw json.RawMessage) ([]TemplateCreature, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var creatures []TemplateCreature
	if err := json.Unmarshal(raw, &creatures); err != nil {
		return nil, err
	}
	return creatures, nil
}

// ParseWalkSpeed extracts the walk speed from a creature's speed JSON.
// Returns 30 as default if not found or unparseable.
func ParseWalkSpeed(speedJSON json.RawMessage) int32 {
	var speeds map[string]int32
	if err := json.Unmarshal(speedJSON, &speeds); err != nil {
		return 30
	}
	if walk, ok := speeds["walk"]; ok {
		return walk
	}
	return 30
}
