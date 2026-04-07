-- name: GetClass :one
SELECT * FROM classes WHERE id = $1;

-- name: ListClasses :many
SELECT * FROM classes ORDER BY name;

-- name: CountClasses :one
SELECT count(*) FROM classes;

-- name: UpsertClass :exec
INSERT INTO classes (id, name, hit_die, primary_ability, save_proficiencies, armor_proficiencies, weapon_proficiencies, skill_choices, spellcasting, features_by_level, attacks_per_action, subclass_level, subclasses, multiclass_prereqs, multiclass_proficiencies, campaign_id, homebrew, source)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    hit_die = EXCLUDED.hit_die,
    primary_ability = EXCLUDED.primary_ability,
    save_proficiencies = EXCLUDED.save_proficiencies,
    armor_proficiencies = EXCLUDED.armor_proficiencies,
    weapon_proficiencies = EXCLUDED.weapon_proficiencies,
    skill_choices = EXCLUDED.skill_choices,
    spellcasting = EXCLUDED.spellcasting,
    features_by_level = EXCLUDED.features_by_level,
    attacks_per_action = EXCLUDED.attacks_per_action,
    subclass_level = EXCLUDED.subclass_level,
    subclasses = EXCLUDED.subclasses,
    multiclass_prereqs = EXCLUDED.multiclass_prereqs,
    multiclass_proficiencies = EXCLUDED.multiclass_proficiencies,
    campaign_id = EXCLUDED.campaign_id,
    homebrew = EXCLUDED.homebrew,
    source = EXCLUDED.source,
    updated_at = now();

-- name: DeleteHomebrewClass :execrows
DELETE FROM classes WHERE id = $1 AND homebrew = true AND campaign_id = $2;
