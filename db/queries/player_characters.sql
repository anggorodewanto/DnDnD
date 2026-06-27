-- name: CreatePlayerCharacter :one
INSERT INTO player_characters (
    campaign_id, character_id, discord_user_id, status, dm_feedback, created_via
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetPlayerCharacter :one
SELECT * FROM player_characters WHERE id = $1;

-- name: GetPlayerCharacterByDiscordUser :one
SELECT * FROM player_characters
WHERE campaign_id = $1 AND discord_user_id = $2 AND status != 'retired';

-- name: GetPlayerCharacterByCharacter :one
SELECT * FROM player_characters
WHERE campaign_id = $1 AND character_id = $2;

-- name: UpdatePlayerCharacterStatus :one
UPDATE player_characters
SET status = $2, dm_feedback = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListPlayerCharactersByCampaign :many
SELECT * FROM player_characters
WHERE campaign_id = $1
ORDER BY created_at;

-- name: FindCharacterByNameCaseInsensitive :one
SELECT * FROM characters
WHERE campaign_id = $1 AND LOWER(name) = LOWER($2);

-- name: ListCharacterNamesByCampaign :many
SELECT id, name FROM characters
WHERE campaign_id = $1
ORDER BY name;

-- name: ListPlayerCharactersByStatus :many
SELECT pc.id, pc.campaign_id, pc.character_id, pc.discord_user_id, pc.status,
       pc.dm_feedback, pc.created_via, pc.created_at, pc.updated_at,
       c.name AS character_name, c.race, c.level, c.classes, c.hp_max,
       c.hp_current, c.temp_hp, c.ac, c.speed_ft, c.ability_scores,
       c.languages, c.ddb_url, c.conditions, c.character_data,
       c.spell_slots, c.pact_magic_slots
FROM player_characters pc
JOIN characters c ON c.id = pc.character_id
WHERE pc.campaign_id = $1 AND pc.status = $2
ORDER BY pc.created_at;

-- name: GetPlayerCharacterWithCharacter :one
SELECT pc.id, pc.campaign_id, pc.character_id, pc.discord_user_id, pc.status,
       pc.dm_feedback, pc.created_via, pc.created_at, pc.updated_at,
       pc.review_before,
       c.name AS character_name, c.race, c.level, c.classes, c.hp_max,
       c.hp_current, c.ac, c.speed_ft, c.ability_scores, c.languages, c.ddb_url,
       c.character_data
FROM player_characters pc
JOIN characters c ON c.id = pc.character_id
WHERE pc.id = $1;

-- name: SetPlayerCharacterReviewBefore :exec
-- Stores the pre-edit DM-review baseline (ReviewCharacter JSON) captured when an
-- approved character is edited back to pending. See docs/dm-character-review-diff.md.
UPDATE player_characters
SET review_before = $2, updated_at = now()
WHERE id = $1;

-- name: ClearPlayerCharacterReviewBefore :exec
-- Clears the review baseline on approve: the newly approved state becomes the
-- implicit baseline for the next edit.
UPDATE player_characters
SET review_before = NULL, updated_at = now()
WHERE id = $1;

-- name: MarkPlayerCharacterRetireRequested :one
-- Sets created_via='retire' on the row matching (campaign_id, discord_user_id)
-- so the Phase 16 dashboard approval branch can route it through the retire
-- path. Status is left untouched (typically 'approved').
UPDATE player_characters
SET created_via = 'retire', updated_at = now()
WHERE campaign_id = $1 AND discord_user_id = $2
RETURNING *;

-- name: ListPlayerCharactersAwaitingApproval :many
-- Returns rows in the DM approval queue: anything still 'pending' plus any
-- row flagged as a retire request (created_via='retire'). Joined with the
-- character row for the queue UI.
SELECT pc.id, pc.campaign_id, pc.character_id, pc.discord_user_id, pc.status,
       pc.dm_feedback, pc.created_via, pc.created_at, pc.updated_at,
       c.name AS character_name, c.race, c.level, c.classes, c.hp_max,
       c.hp_current, c.ac, c.speed_ft, c.ability_scores, c.languages, c.ddb_url
FROM player_characters pc
JOIN characters c ON c.id = pc.character_id
WHERE pc.campaign_id = $1
  AND (pc.status = 'pending' OR pc.created_via = 'retire')
ORDER BY pc.created_at;

-- name: RelinkPlayerCharacter :one
-- Re-points an existing non-retired player_characters row at a freshly built
-- character and resets it to 'pending' for DM approval. Used by the portal
-- builder so a re-submit or resumed build reuses the existing row instead of
-- INSERTing a second one — which the partial unique index
-- idx_player_characters_unique_active_discord_user forbids.
UPDATE player_characters
SET character_id = $2, status = 'pending', dm_feedback = NULL, created_via = $3, updated_at = now()
WHERE id = $1
RETURNING *;
