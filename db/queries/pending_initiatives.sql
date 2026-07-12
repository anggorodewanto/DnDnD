-- name: UpsertPendingInitiative :one
-- APP-5: stage (or edit) one player's own initiative before combat starts.
-- Re-submitting the same (campaign, character) overwrites the prior roll.
INSERT INTO pending_initiatives (campaign_id, character_id, roll)
VALUES ($1, $2, $3)
ON CONFLICT (campaign_id, character_id)
DO UPDATE SET roll = EXCLUDED.roll, updated_at = now()
RETURNING *;

-- name: GetPendingInitiative :one
SELECT * FROM pending_initiatives
WHERE campaign_id = $1 AND character_id = $2;

-- name: DeletePendingInitiative :exec
DELETE FROM pending_initiatives
WHERE campaign_id = $1 AND character_id = $2;

-- name: ClearAndReturnPendingInitiatives :many
-- APP-5: StartCombat consumes every staged roll for the campaign and clears the
-- rows in one statement — read-and-delete atomically, so nothing leaks into the
-- next combat and the start path pays a single round trip.
DELETE FROM pending_initiatives
WHERE campaign_id = $1
RETURNING character_id, roll;
