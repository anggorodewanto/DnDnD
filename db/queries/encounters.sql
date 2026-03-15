-- name: CreateEncounter :one
INSERT INTO encounters (campaign_id, map_id, name, display_name, template_id, status, round_number)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

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
