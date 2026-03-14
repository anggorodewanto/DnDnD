package refdata

import "github.com/google/uuid"

// UpdateCombatantAutoResolveCountParams holds params for updating auto-resolve tracking.
type UpdateCombatantAutoResolveCountParams struct {
	ID                      uuid.UUID `json:"id"`
	ConsecutiveAutoResolves int32     `json:"consecutive_auto_resolves"`
	IsAbsent                bool      `json:"is_absent"`
}
