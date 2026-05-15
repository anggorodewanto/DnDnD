-- name: CreateMap :one
INSERT INTO maps (campaign_id, name, width_squares, height_squares, tiled_json, background_image_id, tileset_refs)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetMapByID :one
SELECT * FROM maps WHERE id = $1 AND campaign_id = $2;

-- name: GetMapByIDUnchecked :one
SELECT * FROM maps WHERE id = $1;

-- name: ListMapsByCampaignID :many
SELECT * FROM maps WHERE campaign_id = $1 ORDER BY created_at DESC;

-- name: UpdateMap :one
UPDATE maps SET
    name = $2,
    width_squares = $3,
    height_squares = $4,
    tiled_json = $5,
    background_image_id = $6,
    tileset_refs = $7,
    updated_at = now()
WHERE id = $1 AND campaign_id = $8
RETURNING *;

-- name: DeleteMap :exec
DELETE FROM maps WHERE id = $1 AND campaign_id = $2;
