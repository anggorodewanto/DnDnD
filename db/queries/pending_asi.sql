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

-- name: ListPendingASIByCampaign :many
-- Pending level-up / ASI prompts for one campaign, joined to the character's
-- name for display. pending_asi only carries character_id, so the campaign is
-- scoped via the player_characters membership row. Feeds the DM Console's
-- unified pending list (internal/situation).
SELECT pa.character_id, c.name AS character_name, pa.created_at
FROM pending_asi pa
JOIN player_characters pc ON pc.character_id = pa.character_id
JOIN characters c ON c.id = pa.character_id
WHERE pc.campaign_id = $1
ORDER BY pa.created_at ASC;
