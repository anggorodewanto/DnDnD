-- name: CreateEncounterTemplate :one
INSERT INTO encounter_templates (campaign_id, map_id, name, display_name, creatures)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetEncounterTemplate :one
SELECT * FROM encounter_templates WHERE id = $1;

-- name: ListEncounterTemplatesByCampaignID :many
SELECT * FROM encounter_templates WHERE campaign_id = $1 ORDER BY created_at DESC;

-- name: UpdateEncounterTemplate :one
UPDATE encounter_templates SET
    map_id = $2,
    name = $3,
    display_name = $4,
    creatures = $5,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteEncounterTemplate :exec
DELETE FROM encounter_templates WHERE id = $1;
