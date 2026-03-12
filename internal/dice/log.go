package dice

import (
	"encoding/json"
	"time"
)

// RollLogEntry represents a complete roll event for logging to #roll-history.
type RollLogEntry struct {
	DiceRolls  []GroupResult `json:"dice_rolls"`
	Total      int           `json:"total"`
	Expression string        `json:"expression"`
	Roller     string        `json:"roller"`
	Purpose    string        `json:"purpose"`
	Breakdown  string        `json:"breakdown"`
	Timestamp  time.Time     `json:"timestamp"`
}

// ToJSONRolls serializes the dice_rolls field as JSONB for action_log storage.
func (e RollLogEntry) ToJSONRolls() []byte {
	data, _ := json.Marshal(e.DiceRolls)
	return data
}

// RollHistoryLogger is the interface for posting rolls to #roll-history.
// The actual Discord implementation will be wired in combat phases.
type RollHistoryLogger interface {
	LogRoll(entry RollLogEntry) error
}
