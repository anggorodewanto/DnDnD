-- name: GetWeapon :one
SELECT * FROM weapons WHERE id = $1;

-- name: ListWeapons :many
SELECT * FROM weapons ORDER BY name;

-- name: CountWeapons :one
SELECT count(*) FROM weapons;

-- name: UpsertWeapon :exec
INSERT INTO weapons (id, name, damage, damage_type, weight_lb, properties, range_normal_ft, range_long_ft, versatile_damage, weapon_type)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    damage = EXCLUDED.damage,
    damage_type = EXCLUDED.damage_type,
    weight_lb = EXCLUDED.weight_lb,
    properties = EXCLUDED.properties,
    range_normal_ft = EXCLUDED.range_normal_ft,
    range_long_ft = EXCLUDED.range_long_ft,
    versatile_damage = EXCLUDED.versatile_damage,
    weapon_type = EXCLUDED.weapon_type,
    updated_at = now();
