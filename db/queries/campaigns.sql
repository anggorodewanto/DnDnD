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

-- name: SetActiveCampaign :exec
-- Records (or replaces) the DM's explicitly-selected active campaign so the
-- dashboard stops silently following created_at DESC (T20 / Finding 12).
INSERT INTO dm_active_campaign (dm_user_id, active_campaign_id, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (dm_user_id)
DO UPDATE SET active_campaign_id = EXCLUDED.active_campaign_id, updated_at = now();

-- name: GetActiveCampaign :one
-- Returns the DM's stored active campaign id, or sql.ErrNoRows when the DM has
-- never made an explicit selection (the resolver then falls back to most-recent).
SELECT active_campaign_id FROM dm_active_campaign WHERE dm_user_id = $1;
