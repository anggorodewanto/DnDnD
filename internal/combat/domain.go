package combat

import (
	"encoding/json"

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
	}
}

// Position represents a map grid position.
type Position struct {
	Col string `json:"col"`
	Row int32  `json:"row"`
}

// StartCombatInput holds parameters for the StartCombat flow.
type StartCombatInput struct {
	TemplateID         uuid.UUID              `json:"template_id"`
	CharacterIDs       []uuid.UUID            `json:"character_ids"`
	CharacterPositions map[uuid.UUID]Position `json:"character_positions"`
	SurprisedShortIDs  []string               `json:"surprised_short_ids,omitempty"`
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
