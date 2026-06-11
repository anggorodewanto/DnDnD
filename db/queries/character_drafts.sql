-- name: UpsertCharacterDraft :exec
-- Persists the player's in-progress builder draft, replacing any prior draft
-- for the same (campaign, player, mode). Called from the submit path so the
-- "request changes" cycle and cross-device resume can rehydrate the form
-- (T11 / Finding 4·b).
INSERT INTO character_drafts (campaign_id, discord_user_id, mode, draft, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (campaign_id, discord_user_id, mode)
DO UPDATE SET draft = EXCLUDED.draft, updated_at = now();

-- name: GetCharacterDraft :one
-- Returns the stored builder draft blob for (campaign, player, mode), or
-- sql.ErrNoRows when none exists.
SELECT draft FROM character_drafts
WHERE campaign_id = $1 AND discord_user_id = $2 AND mode = $3;
