-- name: CreateShop :one
INSERT INTO shops (campaign_id, name, description)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetShop :one
SELECT * FROM shops WHERE id = $1;

-- name: ListShopsByCampaign :many
SELECT * FROM shops WHERE campaign_id = $1 ORDER BY name;

-- name: UpdateShop :one
UPDATE shops SET name = $2, description = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteShop :exec
DELETE FROM shops WHERE id = $1;

-- name: CreateShopItem :one
INSERT INTO shop_items (shop_id, item_id, name, description, price_gp, quantity, type)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListShopItems :many
SELECT * FROM shop_items WHERE shop_id = $1 ORDER BY name;

-- name: UpdateShopItem :one
UPDATE shop_items SET name = $2, description = $3, price_gp = $4, quantity = $5, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteShopItem :exec
DELETE FROM shop_items WHERE id = $1;

-- name: DeleteShopItemsByShop :exec
DELETE FROM shop_items WHERE shop_id = $1;
