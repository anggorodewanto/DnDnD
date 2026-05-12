-- name: UpsertPendingASI :exec
INSERT INTO pending_asi (character_id, snapshot_json)
VALUES ($1, $2)
ON CONFLICT (character_id) DO UPDATE
    SET snapshot_json = EXCLUDED.snapshot_json,
        updated_at    = now();

-- name: GetPendingASI :one
SELECT character_id, snapshot_json, created_at, updated_at
FROM pending_asi
WHERE character_id = $1;

-- name: DeletePendingASI :exec
DELETE FROM pending_asi WHERE character_id = $1;

-- name: ListPendingASI :many
SELECT character_id, snapshot_json, created_at, updated_at
FROM pending_asi
ORDER BY created_at ASC;
