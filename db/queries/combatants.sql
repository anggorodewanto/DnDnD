-- name: CreateCombatant :one
INSERT INTO combatants (
    encounter_id, character_id, creature_ref_id, short_id, display_name,
    initiative_roll, initiative_order, position_col, position_row, altitude_ft,
    hp_max, hp_current, temp_hp, ac, conditions, exhaustion_level, death_saves,
    is_visible, is_alive, is_npc, is_raging, rage_rounds_remaining,
    rage_attacked_this_round, rage_took_damage_this_round,
    is_wild_shaped, wild_shape_creature_ref, wild_shape_original, summoner_id
)
VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17,
    $18, $19, $20, $21, $22,
    $23, $24,
    $25, $26, $27, $28
)
RETURNING *;

-- name: GetCombatant :one
SELECT * FROM combatants WHERE id = $1;

-- name: ListCombatantsByEncounterID :many
SELECT * FROM combatants WHERE encounter_id = $1 ORDER BY initiative_order ASC;

-- name: UpdateCombatantHP :one
UPDATE combatants SET hp_current = $2, temp_hp = $3, is_alive = $4, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: UpdateCombatantConditions :one
UPDATE combatants SET conditions = $2, exhaustion_level = $3, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: UpdateCombatantPosition :one
UPDATE combatants SET position_col = $2, position_row = $3, altitude_ft = $4, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: UpdateCombatantInitiative :one
UPDATE combatants SET initiative_roll = $2, initiative_order = $3, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: UpdateCombatantDeathSaves :one
UPDATE combatants SET death_saves = $2, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: DeleteCombatant :exec
DELETE FROM combatants WHERE id = $1;
