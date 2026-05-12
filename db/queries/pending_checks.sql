-- name: UpsertPendingCheck :one
INSERT INTO pending_checks (encounter_id, combatant_id, skill, dc, reason, status)
VALUES ($1, $2, $3, $4, $5, 'pending')
RETURNING *;

-- name: GetPendingCheck :one
SELECT * FROM pending_checks WHERE id = $1;

-- name: ListPendingChecksByCombatant :many
SELECT * FROM pending_checks
WHERE combatant_id = $1 AND status = 'pending'
ORDER BY created_at ASC;

-- name: ListPendingChecksByEncounter :many
SELECT * FROM pending_checks
WHERE encounter_id = $1 AND status = 'pending'
ORDER BY created_at ASC;

-- name: UpdatePendingCheckResult :one
UPDATE pending_checks
SET roll_result = $2, success = $3, status = 'rolled', updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ForfeitPendingCheck :one
UPDATE pending_checks
SET status = 'forfeited', updated_at = now()
WHERE id = $1
RETURNING *;
