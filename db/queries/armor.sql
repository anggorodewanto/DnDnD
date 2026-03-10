-- name: GetArmor :one
SELECT * FROM armor WHERE id = $1;

-- name: ListArmor :many
SELECT * FROM armor ORDER BY name;

-- name: CountArmor :one
SELECT count(*) FROM armor;

-- name: UpsertArmor :exec
INSERT INTO armor (id, name, ac_base, ac_dex_bonus, ac_dex_max, strength_req, stealth_disadv, armor_type, weight_lb)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    ac_base = EXCLUDED.ac_base,
    ac_dex_bonus = EXCLUDED.ac_dex_bonus,
    ac_dex_max = EXCLUDED.ac_dex_max,
    strength_req = EXCLUDED.strength_req,
    stealth_disadv = EXCLUDED.stealth_disadv,
    armor_type = EXCLUDED.armor_type,
    weight_lb = EXCLUDED.weight_lb,
    updated_at = now();
