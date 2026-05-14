-- name: UpsertPendingDDBImport :exec
INSERT INTO pending_ddb_imports (id, character_id, params_json, created_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE
    SET character_id = EXCLUDED.character_id,
        params_json  = EXCLUDED.params_json,
        created_at   = EXCLUDED.created_at,
        updated_at   = now();

-- name: GetPendingDDBImport :one
SELECT id, character_id, params_json, created_at, updated_at
FROM pending_ddb_imports
WHERE id = $1;

-- name: DeletePendingDDBImport :exec
DELETE FROM pending_ddb_imports WHERE id = $1;
