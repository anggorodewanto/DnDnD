-- name: CreateCampaign :one
INSERT INTO campaigns (guild_id, dm_user_id, name, settings)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetCampaignByGuildID :one
SELECT * FROM campaigns WHERE guild_id = $1;

-- name: GetCampaignByID :one
SELECT * FROM campaigns WHERE id = $1;

-- name: UpdateCampaignStatus :one
UPDATE campaigns SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateCampaignSettings :one
UPDATE campaigns SET settings = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateCampaignName :one
UPDATE campaigns SET name = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListCampaigns :many
SELECT * FROM campaigns ORDER BY created_at DESC;
