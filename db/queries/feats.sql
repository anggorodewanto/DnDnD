-- name: GetFeat :one
SELECT * FROM feats WHERE id = $1;

-- name: ListFeats :many
SELECT * FROM feats ORDER BY name;

-- name: CountFeats :one
SELECT count(*) FROM feats;

-- name: UpsertFeat :exec
INSERT INTO feats (id, name, description, prerequisites, asi_bonus, mechanical_effect)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    prerequisites = EXCLUDED.prerequisites,
    asi_bonus = EXCLUDED.asi_bonus,
    mechanical_effect = EXCLUDED.mechanical_effect,
    updated_at = now();
