-- name: ListOpen5eCustomSources :many
SELECT * FROM open5e_custom_sources ORDER BY title;

-- name: UpsertOpen5eCustomSource :one
INSERT INTO open5e_custom_sources (slug, title, publisher, description)
VALUES ($1, $2, $3, $4)
ON CONFLICT (slug) DO UPDATE
  SET title = EXCLUDED.title,
      publisher = EXCLUDED.publisher,
      description = EXCLUDED.description
RETURNING *;

-- name: DeleteOpen5eCustomSource :execrows
DELETE FROM open5e_custom_sources WHERE slug = $1;
