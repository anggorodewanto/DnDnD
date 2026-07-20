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

-- name: SpendTurnResources :one
-- Targeted compare-and-set spend of a turn's boolean resources.
--
-- UpdateTurnActions blind-writes all 11 resource columns from a struct read
-- earlier in the request, so two overlapping commands (a potion spending the
-- bonus action, a cantrip spending the action) each revert the other's column
-- to whatever it held at their own read. This statement only ever sets the
-- columns it is told to spend, so those two compose however they interleave.
--
-- The WHERE clause makes it a CAS: each guard is a no-op unless that resource
-- is actually being spent, and the update matches no row when the resource was
-- already spent. Callers therefore get sql.ErrNoRows — a real "already spent"
-- signal — instead of silently double-spending after a stale validation read.
UPDATE turns SET
    action_used        = action_used        OR sqlc.arg(spend_action)::boolean,
    bonus_action_used  = bonus_action_used  OR sqlc.arg(spend_bonus_action)::boolean,
    reaction_used      = reaction_used      OR sqlc.arg(spend_reaction)::boolean,
    free_interact_used = free_interact_used OR sqlc.arg(spend_free_interact)::boolean
WHERE id = sqlc.arg(id)
  AND (NOT sqlc.arg(spend_action)::boolean        OR NOT action_used)
  AND (NOT sqlc.arg(spend_bonus_action)::boolean  OR NOT bonus_action_used)
  AND (NOT sqlc.arg(spend_reaction)::boolean      OR NOT reaction_used)
  AND (NOT sqlc.arg(spend_free_interact)::boolean OR NOT free_interact_used)
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

-- name: ListTurnsTimedOut :many
SELECT * FROM turns
WHERE status = 'active'
  AND timeout_at IS NOT NULL
  AND started_at IS NOT NULL
  AND dm_decision_sent_at IS NULL
  AND now() >= timeout_at
ORDER BY timeout_at ASC;

-- name: UpdateTurnDMDecisionSent :one
UPDATE turns SET dm_decision_sent_at = now(), dm_decision_deadline = now() + interval '1 hour' WHERE id = $1 RETURNING *;

-- name: ListTurnsNeedingDMAutoResolve :many
SELECT * FROM turns
WHERE status = 'active'
  AND dm_decision_sent_at IS NOT NULL
  AND dm_decision_deadline IS NOT NULL
  AND now() >= dm_decision_deadline
ORDER BY dm_decision_deadline ASC;

-- name: UpdateTurnAutoResolved :one
UPDATE turns SET auto_resolved = true, status = 'completed', completed_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateTurnWaitExtended :one
UPDATE turns SET wait_extended = true WHERE id = $1 RETURNING *;

-- name: ResetTurnNudgeAndWarning :one
UPDATE turns SET nudge_sent_at = NULL, warning_sent_at = NULL, dm_decision_sent_at = NULL, dm_decision_deadline = NULL WHERE id = $1 RETURNING *;

-- name: GetLastCompletedTurnByCombatant :one
SELECT * FROM turns
WHERE encounter_id = $1
  AND combatant_id = $2
  AND status IN ('completed', 'skipped')
ORDER BY completed_at DESC
LIMIT 1;

-- name: ReseatTurn :one
-- Reassigns an existing (un-acted) active turn row to a different combatant and
-- resets its per-turn state, so the current-turn pointer moves without a raw DB
-- write (APP-2). The displaced combatant loses its turn row and is picked again
-- at its true initiative order later in the round.
UPDATE turns SET
    combatant_id = $2,
    status = 'active',
    movement_remaining_ft = $3,
    attacks_remaining = $4,
    started_at = $5,
    timeout_at = $6,
    completed_at = NULL,
    action_used = false,
    bonus_action_used = false,
    bonus_action_spell_cast = false,
    action_spell_cast = false,
    reaction_used = false,
    free_interact_used = false,
    has_disengaged = false,
    action_surged = false,
    has_stood_this_turn = false,
    nudge_sent_at = NULL,
    warning_sent_at = NULL,
    dm_decision_sent_at = NULL,
    dm_decision_deadline = NULL,
    wait_extended = false,
    auto_resolved = false
WHERE id = $1
RETURNING *;
