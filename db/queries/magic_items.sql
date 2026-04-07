-- name: GetMagicItem :one
SELECT * FROM magic_items WHERE id = $1;

-- name: ListMagicItems :many
SELECT * FROM magic_items ORDER BY name;

-- name: CountMagicItems :one
SELECT count(*) FROM magic_items;

-- name: UpsertMagicItem :exec
INSERT INTO magic_items (id, campaign_id, name, base_item_type, base_item_id, rarity, requires_attunement, attunement_restriction, magic_bonus, passive_effects, active_abilities, charges, description, homebrew, source)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
ON CONFLICT (id) DO UPDATE SET
    campaign_id = EXCLUDED.campaign_id,
    name = EXCLUDED.name,
    base_item_type = EXCLUDED.base_item_type,
    base_item_id = EXCLUDED.base_item_id,
    rarity = EXCLUDED.rarity,
    requires_attunement = EXCLUDED.requires_attunement,
    attunement_restriction = EXCLUDED.attunement_restriction,
    magic_bonus = EXCLUDED.magic_bonus,
    passive_effects = EXCLUDED.passive_effects,
    active_abilities = EXCLUDED.active_abilities,
    charges = EXCLUDED.charges,
    description = EXCLUDED.description,
    homebrew = EXCLUDED.homebrew,
    source = EXCLUDED.source,
    updated_at = now();

-- name: DeleteHomebrewMagicItem :execrows
DELETE FROM magic_items WHERE id = $1 AND homebrew = true AND campaign_id = $2;

-- name: ListMagicItemsByRarity :many
SELECT * FROM magic_items WHERE rarity = $1 ORDER BY name;

-- name: ListMagicItemsByType :many
SELECT * FROM magic_items WHERE base_item_type = $1 ORDER BY name;
