-- name: CreateActionLog :one
INSERT INTO action_log (turn_id, encounter_id, action_type, actor_id, target_id, description, before_state, after_state, dice_rolls)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListActionLogByEncounterID :many
SELECT * FROM action_log WHERE encounter_id = $1 ORDER BY created_at ASC;

-- name: ListActionLogByTurnID :many
SELECT * FROM action_log WHERE turn_id = $1 ORDER BY created_at ASC;
