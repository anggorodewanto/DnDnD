package exploration

import "errors"

// ErrNoPlayerSpawnZones is returned when a map has no "player" spawn zones
// but the caller asked to place PCs.
var ErrNoPlayerSpawnZones = errors.New("map has no player spawn zones")

// ErrNotEnoughSpawnTiles is returned when the player spawn zones do not have
// enough tiles to seat every PC.
var ErrNotEnoughSpawnTiles = errors.New("not enough player spawn tiles")

// ErrEncounterNotExploration is returned when an operation requires an
// exploration-mode encounter but the referenced encounter is in a different mode.
var ErrEncounterNotExploration = errors.New("encounter is not in exploration mode")
