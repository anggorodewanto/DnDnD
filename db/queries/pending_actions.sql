-- name: CreatePendingAction :one
INSERT INTO pending_actions (encounter_id, combatant_id, action_text, dm_queue_message_id, dm_queue_channel_id, status)
VALUES ($1, $2, $3, $4, $5, 'pending')
RETURNING *;

-- name: GetPendingAction :one
SELECT * FROM pending_actions WHERE id = $1;

-- name: GetPendingActionByCombatant :one
SELECT * FROM pending_actions
WHERE combatant_id = $1 AND status = 'pending'
ORDER BY created_at DESC
LIMIT 1;

-- name: UpdatePendingActionStatus :one
UPDATE pending_actions
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdatePendingActionDMQueueMessage :one
UPDATE pending_actions
SET dm_queue_message_id = $2, dm_queue_channel_id = $3, updated_at = now()
WHERE id = $1
RETURNING *;
