-- name: CreateEncounterZone :one
INSERT INTO encounter_zones (
    encounter_id, source_combatant_id, source_spell, shape,
    origin_col, origin_row, dimensions, anchor_mode, anchor_combatant_id,
    zone_type, overlay_color, marker_icon, requires_concentration,
    expires_at_round, zone_triggers, triggered_this_round
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
) RETURNING *;

-- name: GetEncounterZone :one
SELECT * FROM encounter_zones WHERE id = $1;

-- name: ListEncounterZonesByEncounterID :many
SELECT * FROM encounter_zones WHERE encounter_id = $1 ORDER BY created_at;

-- name: ListConcentrationZonesByCombatant :many
SELECT * FROM encounter_zones
WHERE source_combatant_id = $1 AND requires_concentration = true
ORDER BY created_at;

-- name: DeleteEncounterZone :exec
DELETE FROM encounter_zones WHERE id = $1;

-- name: DeleteEncounterZonesByEncounterID :exec
DELETE FROM encounter_zones WHERE encounter_id = $1;

-- name: DeleteConcentrationZonesByCombatant :exec
DELETE FROM encounter_zones
WHERE source_combatant_id = $1 AND requires_concentration = true;

-- name: DeleteExpiredZones :exec
DELETE FROM encounter_zones
WHERE encounter_id = $1 AND expires_at_round IS NOT NULL AND expires_at_round <= $2;

-- name: UpdateEncounterZoneOrigin :one
UPDATE encounter_zones
SET origin_col = $2, origin_row = $3
WHERE id = $1
RETURNING *;

-- name: UpdateEncounterZoneTriggeredThisRound :one
UPDATE encounter_zones
SET triggered_this_round = $2
WHERE id = $1
RETURNING *;

-- name: ResetAllTriggeredThisRound :exec
UPDATE encounter_zones
SET triggered_this_round = '{}'
WHERE encounter_id = $1;
