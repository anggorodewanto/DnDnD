-- name: InsertDMPlayerMessage :one
INSERT INTO dm_player_messages (
    campaign_id,
    player_character_id,
    author_user_id,
    body,
    discord_message_ids
)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListDMPlayerMessages :many
SELECT *
FROM dm_player_messages
WHERE campaign_id = $1 AND player_character_id = $2
ORDER BY sent_at DESC
LIMIT $3 OFFSET $4;
