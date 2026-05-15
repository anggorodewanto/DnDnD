-- name: CreateEncounter :one
INSERT INTO encounters (campaign_id, map_id, name, display_name, template_id, status, round_number)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: CreateExplorationEncounter :one
INSERT INTO encounters (campaign_id, map_id, name, display_name, status, round_number, mode)
VALUES ($1, $2, $3, $4, 'active', 0, 'exploration')
RETURNING *;

-- name: UpdateEncounterMode :one
UPDATE encounters SET mode = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: GetEncounter :one
SELECT * FROM encounters WHERE id = $1;

-- name: ListEncountersByCampaignID :many
SELECT * FROM encounters WHERE campaign_id = $1 ORDER BY created_at DESC;

-- name: UpdateEncounterStatus :one
UPDATE encounters SET status = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateEncounterCurrentTurn :one
UPDATE encounters SET current_turn_id = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateEncounterRound :one
UPDATE encounters SET round_number = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateEncounterDisplayName :one
UPDATE encounters SET display_name = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: DeleteEncounter :exec
DELETE FROM encounters WHERE id = $1;

-- name: GetMostRecentCompletedEncounter :one
SELECT * FROM encounters
WHERE campaign_id = $1 AND status = 'completed'
ORDER BY updated_at DESC
LIMIT 1;

-- name: GetCampaignByEncounterID :one
SELECT c.* FROM campaigns c
JOIN encounters e ON e.campaign_id = c.id
WHERE e.id = $1;

-- name: GetActiveEncounterIDByCharacterID :one
SELECT e.id FROM encounters e
JOIN combatants cb ON cb.encounter_id = e.id
WHERE cb.character_id = $1 AND e.status = 'active'
ORDER BY cb.created_at DESC
LIMIT 1;

-- name: UpdateEncounterExploredCells :exec
-- SR-031: Persist the packed explored-tile set (JSON array of int indexes,
-- where each index = row*width+col) for this encounter. Called by
-- mapRegeneratorAdapter after every successful render so a bot restart
-- restores the dim "Explored" overlay on previously-seen tiles.
UPDATE encounters
SET explored_cells = $2, updated_at = now()
WHERE id = $1;
