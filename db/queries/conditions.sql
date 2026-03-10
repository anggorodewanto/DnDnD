-- name: GetCondition :one
SELECT * FROM conditions_ref WHERE id = $1;

-- name: ListConditions :many
SELECT * FROM conditions_ref ORDER BY name;

-- name: CountConditions :one
SELECT count(*) FROM conditions_ref;

-- name: UpsertCondition :exec
INSERT INTO conditions_ref (id, name, description, mechanical_effects)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    mechanical_effects = EXCLUDED.mechanical_effects,
    updated_at = now();
