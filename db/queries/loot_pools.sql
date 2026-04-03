-- name: CreateLootPool :one
INSERT INTO loot_pools (encounter_id, campaign_id, gold_total, status)
VALUES ($1, $2, $3, 'open')
RETURNING *;

-- name: GetLootPool :one
SELECT * FROM loot_pools WHERE id = $1;

-- name: GetLootPoolByEncounter :one
SELECT * FROM loot_pools WHERE encounter_id = $1;

-- name: UpdateLootPoolGold :one
UPDATE loot_pools SET gold_total = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateLootPoolStatus :one
UPDATE loot_pools SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteLootPool :exec
DELETE FROM loot_pools WHERE id = $1;

-- name: CreateLootPoolItem :one
INSERT INTO loot_pool_items (
    loot_pool_id, item_id, name, description, quantity, type,
    is_magic, magic_bonus, magic_properties, requires_attunement, rarity
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetLootPoolItem :one
SELECT * FROM loot_pool_items WHERE id = $1;

-- name: ListLootPoolItems :many
SELECT * FROM loot_pool_items WHERE loot_pool_id = $1 ORDER BY created_at;

-- name: ClaimLootPoolItem :one
UPDATE loot_pool_items SET claimed_by = $2, claimed_at = now(), updated_at = now()
WHERE id = $1 AND claimed_by IS NULL
RETURNING *;

-- name: DeleteLootPoolItem :exec
DELETE FROM loot_pool_items WHERE id = $1;

-- name: DeleteUnclaimedLootPoolItems :exec
DELETE FROM loot_pool_items WHERE loot_pool_id = $1 AND claimed_by IS NULL;

-- name: ListPlayerCharactersByCampaignApproved :many
SELECT pc.*, c.name AS character_name, c.gold
FROM player_characters pc
JOIN characters c ON c.id = pc.character_id
WHERE pc.campaign_id = $1 AND pc.status = 'approved'
ORDER BY c.name;
