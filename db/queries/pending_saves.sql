-- name: CreatePendingSave :one
INSERT INTO pending_saves (encounter_id, combatant_id, ability, dc, source, cover_bonus, status)
VALUES ($1, $2, $3, $4, $5, $6, 'pending')
RETURNING *;

-- name: GetPendingSave :one
SELECT * FROM pending_saves WHERE id = $1;

-- name: ListPendingSavesByCombatant :many
SELECT * FROM pending_saves
WHERE combatant_id = $1 AND status = 'pending'
ORDER BY created_at ASC;

-- name: ListPendingSavesByEncounter :many
SELECT * FROM pending_saves
WHERE encounter_id = $1 AND status = 'pending'
ORDER BY created_at ASC;

-- name: ListSavesByEncounter :many
-- ISSUE-044: lists every save row for an encounter REGARDLESS of status. The
-- AoE damage-apply gate must see rows that have already flipped to 'rolled'
-- (the pending-only list above hides them, so damage never lands).
SELECT * FROM pending_saves
WHERE encounter_id = $1
ORDER BY created_at ASC;

-- name: MarkPendingSaveApplied :exec
-- ISSUE-044: closes the lifecycle pending→rolled→applied so a repeated
-- damage-apply drive is an idempotent no-op.
UPDATE pending_saves
SET status = 'applied', updated_at = now()
WHERE id = $1;

-- name: UpdatePendingSaveResult :one
UPDATE pending_saves
SET roll_result = $2, success = $3, status = 'rolled', updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ForfeitPendingSave :one
UPDATE pending_saves
SET status = 'forfeited', updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CancelAllPendingSavesByCombatant :exec
UPDATE pending_saves
SET status = 'forfeited', updated_at = now()
WHERE combatant_id = $1 AND encounter_id = $2 AND status = 'pending';
