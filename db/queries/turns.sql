-- name: CreateTurn :one
INSERT INTO turns (encounter_id, combatant_id, round_number, status, movement_remaining_ft, attacks_remaining, started_at, timeout_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetTurn :one
SELECT * FROM turns WHERE id = $1;

-- name: GetActiveTurnByEncounterID :one
SELECT * FROM turns WHERE encounter_id = $1 AND status = 'active' LIMIT 1;

-- name: CompleteTurn :one
UPDATE turns SET status = 'completed', completed_at = now() WHERE id = $1 RETURNING *;

-- name: SkipTurn :one
UPDATE turns SET status = 'skipped', completed_at = now() WHERE id = $1 RETURNING *;

-- name: ListTurnsByEncounterAndRound :many
SELECT * FROM turns WHERE encounter_id = $1 AND round_number = $2 ORDER BY created_at ASC;

-- name: UpdateTurnActions :one
UPDATE turns SET
    movement_remaining_ft = $2,
    action_used = $3,
    bonus_action_used = $4,
    bonus_action_spell_cast = $5,
    action_spell_cast = $6,
    reaction_used = $7,
    free_interact_used = $8,
    attacks_remaining = $9,
    has_disengaged = $10,
    action_surged = $11,
    has_stood_this_turn = $12
WHERE id = $1
RETURNING *;

-- name: ListTurnsNeedingNudge :many
SELECT * FROM turns
WHERE status = 'active'
  AND timeout_at IS NOT NULL
  AND started_at IS NOT NULL
  AND nudge_sent_at IS NULL
  AND now() >= started_at + (timeout_at - started_at) * 0.5
ORDER BY started_at ASC;

-- name: ListTurnsNeedingWarning :many
SELECT * FROM turns
WHERE status = 'active'
  AND timeout_at IS NOT NULL
  AND started_at IS NOT NULL
  AND warning_sent_at IS NULL
  AND now() >= started_at + (timeout_at - started_at) * 0.75
ORDER BY started_at ASC;

-- name: UpdateTurnNudgeSent :one
UPDATE turns SET nudge_sent_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateTurnWarningSent :one
UPDATE turns SET warning_sent_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateTurnTimeout :one
UPDATE turns SET timeout_at = $2 WHERE id = $1 RETURNING *;

-- name: ListActiveTurnsByEncounterID :many
SELECT * FROM turns WHERE encounter_id = $1 AND status = 'active';

-- name: ClearTurnTimeout :one
UPDATE turns SET timeout_at = NULL WHERE id = $1 RETURNING *;

-- name: SetTurnTimeout :one
UPDATE turns SET timeout_at = $2, started_at = COALESCE(started_at, now()) WHERE id = $1 RETURNING *;
