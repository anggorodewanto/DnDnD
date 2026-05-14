-- name: CreateReactionDeclaration :one
INSERT INTO reaction_declarations (encounter_id, combatant_id, description)
VALUES ($1, $2, $3)
RETURNING *;

-- name: CreateReadiedActionDeclaration :one
INSERT INTO reaction_declarations (encounter_id, combatant_id, description, is_readied_action, spell_name, spell_slot_level)
VALUES ($1, $2, $3, true, $4, $5)
RETURNING *;

-- name: GetReactionDeclaration :one
SELECT * FROM reaction_declarations WHERE id = $1;

-- name: ListReactionDeclarationsByEncounter :many
SELECT * FROM reaction_declarations WHERE encounter_id = $1 ORDER BY created_at ASC;

-- name: ListReactionDeclarationsByCombatant :many
SELECT * FROM reaction_declarations WHERE combatant_id = $1 AND encounter_id = $2 ORDER BY created_at ASC;

-- name: ListActiveReactionDeclarationsByEncounter :many
SELECT * FROM reaction_declarations WHERE encounter_id = $1 AND status = 'active' ORDER BY created_at ASC;

-- name: ListActiveReactionDeclarationsByCombatant :many
SELECT * FROM reaction_declarations WHERE combatant_id = $1 AND encounter_id = $2 AND status = 'active' ORDER BY created_at ASC;

-- name: UpdateReactionDeclarationStatusUsed :one
UPDATE reaction_declarations
SET status = 'used', used_at = now(), used_on_round = $2
WHERE id = $1
RETURNING *;

-- name: CancelReactionDeclaration :one
UPDATE reaction_declarations
SET status = 'cancelled'
WHERE id = $1 AND status = 'active'
RETURNING *;

-- name: CancelAllReactionDeclarationsByCombatant :exec
UPDATE reaction_declarations
SET status = 'cancelled'
WHERE combatant_id = $1 AND encounter_id = $2 AND status = 'active';

-- name: UpdateReactionDeclarationCounterspellPrompt :one
UPDATE reaction_declarations
SET counterspell_enemy_spell = $2, counterspell_enemy_level = $3, counterspell_enemy_caster_id = $4, counterspell_status = 'prompted'
WHERE id = $1
RETURNING *;

-- name: UpdateReactionDeclarationCounterspellResolved :one
UPDATE reaction_declarations
SET counterspell_slot_used = $2, counterspell_status = $3, counterspell_dc = $4
WHERE id = $1
RETURNING *;

-- name: DeleteReactionDeclarationsByEncounter :exec
DELETE FROM reaction_declarations WHERE encounter_id = $1;
