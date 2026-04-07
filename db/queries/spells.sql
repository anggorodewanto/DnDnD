-- name: GetSpell :one
SELECT * FROM spells WHERE id = $1;

-- name: ListSpells :many
SELECT * FROM spells ORDER BY name;

-- name: CountSpells :one
SELECT count(*) FROM spells;

-- name: UpsertSpell :exec
INSERT INTO spells (id, name, level, school, casting_time, range_ft, range_type, components, material_description, material_cost_gp, material_consumed, duration, concentration, ritual, description, higher_levels, damage, healing, save_ability, save_effect, attack_type, area_of_effect, conditions_applied, teleport, resolution_mode, classes, campaign_id, homebrew, source)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    level = EXCLUDED.level,
    school = EXCLUDED.school,
    casting_time = EXCLUDED.casting_time,
    range_ft = EXCLUDED.range_ft,
    range_type = EXCLUDED.range_type,
    components = EXCLUDED.components,
    material_description = EXCLUDED.material_description,
    material_cost_gp = EXCLUDED.material_cost_gp,
    material_consumed = EXCLUDED.material_consumed,
    duration = EXCLUDED.duration,
    concentration = EXCLUDED.concentration,
    ritual = EXCLUDED.ritual,
    description = EXCLUDED.description,
    higher_levels = EXCLUDED.higher_levels,
    damage = EXCLUDED.damage,
    healing = EXCLUDED.healing,
    save_ability = EXCLUDED.save_ability,
    save_effect = EXCLUDED.save_effect,
    attack_type = EXCLUDED.attack_type,
    area_of_effect = EXCLUDED.area_of_effect,
    conditions_applied = EXCLUDED.conditions_applied,
    teleport = EXCLUDED.teleport,
    resolution_mode = EXCLUDED.resolution_mode,
    classes = EXCLUDED.classes,
    campaign_id = EXCLUDED.campaign_id,
    homebrew = EXCLUDED.homebrew,
    source = EXCLUDED.source,
    updated_at = now();

-- name: DeleteHomebrewSpell :execrows
DELETE FROM spells WHERE id = $1 AND homebrew = true AND campaign_id = $2;

-- name: ListSpellsByClass :many
SELECT * FROM spells WHERE $1::text = ANY(classes) ORDER BY level, name;

-- name: ListSpellsBySchool :many
SELECT * FROM spells WHERE school = $1 ORDER BY level, name;

-- name: ListSpellsByLevel :many
SELECT * FROM spells WHERE level = $1 ORDER BY name;

-- name: ListSpellsByResolutionMode :many
SELECT * FROM spells WHERE resolution_mode = $1 ORDER BY level, name;

-- name: GetSpellsByIDs :many
SELECT * FROM spells WHERE id = ANY($1::text[]) ORDER BY level, name;
