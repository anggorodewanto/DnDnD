-- name: CreateAsset :one
INSERT INTO assets (campaign_id, type, original_name, mime_type, byte_size, storage_path)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAssetByID :one
SELECT * FROM assets WHERE id = $1;

-- name: ListAssetsByCampaignID :many
SELECT * FROM assets WHERE campaign_id = $1 ORDER BY created_at DESC;

-- name: DeleteAsset :exec
DELETE FROM assets WHERE id = $1;
