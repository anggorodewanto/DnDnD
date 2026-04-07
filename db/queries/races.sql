-- name: GetRace :one
SELECT * FROM races WHERE id = $1;

-- name: ListRaces :many
SELECT * FROM races ORDER BY name;

-- name: CountRaces :one
SELECT count(*) FROM races;

-- name: UpsertRace :exec
INSERT INTO races (id, name, speed_ft, size, ability_bonuses, darkvision_ft, traits, languages, subraces, campaign_id, homebrew, source)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    speed_ft = EXCLUDED.speed_ft,
    size = EXCLUDED.size,
    ability_bonuses = EXCLUDED.ability_bonuses,
    darkvision_ft = EXCLUDED.darkvision_ft,
    traits = EXCLUDED.traits,
    languages = EXCLUDED.languages,
    subraces = EXCLUDED.subraces,
    campaign_id = EXCLUDED.campaign_id,
    homebrew = EXCLUDED.homebrew,
    source = EXCLUDED.source,
    updated_at = now();

-- name: DeleteHomebrewRace :execrows
DELETE FROM races WHERE id = $1 AND homebrew = true AND campaign_id = $2;
