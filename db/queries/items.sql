-- name: GetItem :one
SELECT * FROM items WHERE id = $1;

-- name: ListItems :many
SELECT * FROM items ORDER BY name;

-- name: CountItems :one
SELECT count(*) FROM items;

-- name: UpsertItem :exec
INSERT INTO items (id, name, category, default_quantity, stackable)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    category = EXCLUDED.category,
    default_quantity = EXCLUDED.default_quantity,
    stackable = EXCLUDED.stackable,
    updated_at = now();
