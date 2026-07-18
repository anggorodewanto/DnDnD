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
	// SelfContained marks a Breakdown that already names each die with its
	// rolled value and ends in the grand total (see FormatValuedBreakdown),
	// e.g. "d20(13) + 2 + 1d4(2) = 17". The #roll-history renderer then drops
	// the redundant backtick Expression and the leading "=" so the line reads
	// "Roller — Purpose: breakdown" rather than doubling up the total.
	SelfContained bool `json:"self_contained,omitempty"`
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
