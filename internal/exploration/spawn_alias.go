package exploration

import "github.com/ab/dndnd/internal/spawnzone"

// Spawn-zone parsing and PC seating moved to the neutral internal/spawnzone
// package so combat mode can reuse it without an exploration<->combat import
// cycle. These aliases keep the exploration package's existing API stable.
type (
	// SpawnZone re-exports spawnzone.SpawnZone.
	SpawnZone = spawnzone.SpawnZone
	// TilePos re-exports spawnzone.TilePos.
	TilePos = spawnzone.TilePos
)

var (
	// ParseSpawnZones re-exports spawnzone.ParseSpawnZones.
	ParseSpawnZones = spawnzone.ParseSpawnZones
	// AssignPCsToSpawnZones re-exports spawnzone.AssignPCsToSpawnZones.
	AssignPCsToSpawnZones = spawnzone.AssignPCsToSpawnZones
	// ErrNoPlayerSpawnZones re-exports spawnzone.ErrNoPlayerSpawnZones.
	ErrNoPlayerSpawnZones = spawnzone.ErrNoPlayerSpawnZones
	// ErrNotEnoughSpawnTiles re-exports spawnzone.ErrNotEnoughSpawnTiles.
	ErrNotEnoughSpawnTiles = spawnzone.ErrNotEnoughSpawnTiles
)
