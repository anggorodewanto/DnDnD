-- name: GetCreature :one
SELECT * FROM creatures WHERE id = $1;

-- name: ListCreatures :many
SELECT * FROM creatures ORDER BY name;

-- name: CountCreatures :one
SELECT count(*) FROM creatures;

-- name: UpsertCreature :exec
INSERT INTO creatures (id, campaign_id, name, size, type, alignment, ac, ac_type, hp_formula, hp_average, speed, ability_scores, saving_throws, skills, damage_resistances, damage_immunities, damage_vulnerabilities, condition_immunities, senses, languages, cr, attacks, abilities, bonus_actions, homebrew, source)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26)
ON CONFLICT (id) DO UPDATE SET
    campaign_id = EXCLUDED.campaign_id,
    name = EXCLUDED.name,
    size = EXCLUDED.size,
    type = EXCLUDED.type,
    alignment = EXCLUDED.alignment,
    ac = EXCLUDED.ac,
    ac_type = EXCLUDED.ac_type,
    hp_formula = EXCLUDED.hp_formula,
    hp_average = EXCLUDED.hp_average,
    speed = EXCLUDED.speed,
    ability_scores = EXCLUDED.ability_scores,
    saving_throws = EXCLUDED.saving_throws,
    skills = EXCLUDED.skills,
    damage_resistances = EXCLUDED.damage_resistances,
    damage_immunities = EXCLUDED.damage_immunities,
    damage_vulnerabilities = EXCLUDED.damage_vulnerabilities,
    condition_immunities = EXCLUDED.condition_immunities,
    senses = EXCLUDED.senses,
    languages = EXCLUDED.languages,
    cr = EXCLUDED.cr,
    attacks = EXCLUDED.attacks,
    abilities = EXCLUDED.abilities,
    bonus_actions = EXCLUDED.bonus_actions,
    homebrew = EXCLUDED.homebrew,
    source = EXCLUDED.source,
    updated_at = now();

-- name: DeleteHomebrewCreature :execrows
DELETE FROM creatures WHERE id = $1 AND homebrew = true AND campaign_id = $2;

-- name: ListCreaturesByType :many
SELECT * FROM creatures WHERE type = $1 ORDER BY name;

-- name: ListCreaturesByCR :many
SELECT * FROM creatures WHERE cr = $1 ORDER BY name;
