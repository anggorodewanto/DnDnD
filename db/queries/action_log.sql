-- name: CreateActionLog :one
INSERT INTO action_log (turn_id, encounter_id, action_type, actor_id, target_id, description, before_state, after_state, dice_rolls)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListActionLogByEncounterID :many
SELECT * FROM action_log WHERE encounter_id = $1 ORDER BY created_at ASC;

-- name: ListActionLogByTurnID :many
SELECT * FROM action_log WHERE turn_id = $1 ORDER BY created_at ASC;

-- name: ListActionLogSinceTurn :many
SELECT * FROM action_log
WHERE encounter_id = $1
  AND target_id = $2
  AND created_at > $3
ORDER BY created_at ASC;

-- name: ListActionLogWithRounds :many
SELECT al.id, al.turn_id, al.encounter_id, al.action_type, al.actor_id, al.target_id,
       al.description, al.before_state, al.after_state, al.dice_rolls, al.created_at,
       t.round_number, t.combatant_id as turn_combatant_id
FROM action_log al
JOIN turns t ON al.turn_id = t.id
WHERE al.encounter_id = $1
ORDER BY t.round_number ASC, al.created_at ASC;
