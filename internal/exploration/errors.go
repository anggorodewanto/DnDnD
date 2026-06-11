package exploration

import "errors"

// ErrNoPlayerSpawnZones and ErrNotEnoughSpawnTiles now live in
// internal/spawnzone; they are re-exported from spawn_alias.go.

// ErrEncounterNotExploration is returned when an operation requires an
// exploration-mode encounter but the referenced encounter is in a different mode.
var ErrEncounterNotExploration = errors.New("encounter is not in exploration mode")
