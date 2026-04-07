-- name: InsertNarrationPost :one
INSERT INTO narration_posts (
    campaign_id,
    author_user_id,
    body,
    attachment_asset_ids,
    discord_message_ids
)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListNarrationPostsByCampaign :many
SELECT *
FROM narration_posts
WHERE campaign_id = $1
ORDER BY posted_at DESC
LIMIT $2 OFFSET $3;
