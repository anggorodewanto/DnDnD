-- name: GetInitiativeTrackerMessage :one
SELECT encounter_id, channel_id, message_id, updated_at
FROM initiative_tracker_messages
WHERE encounter_id = $1;

-- name: UpsertInitiativeTrackerMessage :exec
INSERT INTO initiative_tracker_messages (encounter_id, channel_id, message_id)
VALUES ($1, $2, $3)
ON CONFLICT (encounter_id) DO UPDATE SET
    channel_id = EXCLUDED.channel_id,
    message_id = EXCLUDED.message_id,
    updated_at = now();

-- name: DeleteInitiativeTrackerMessage :exec
DELETE FROM initiative_tracker_messages WHERE encounter_id = $1;
