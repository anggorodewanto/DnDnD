-- name: InsertDMQueueItem :one
INSERT INTO dm_queue_items (
    id,
    campaign_id,
    guild_id,
    channel_id,
    message_id,
    kind,
    player_name,
    summary,
    resolve_path,
    extra
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetDMQueueItem :one
SELECT *
FROM dm_queue_items
WHERE id = $1;

-- name: ListPendingDMQueueItems :many
SELECT *
FROM dm_queue_items
WHERE campaign_id = $1 AND status = 'pending'
ORDER BY created_at ASC;

-- name: ListAllPendingDMQueueItems :many
SELECT *
FROM dm_queue_items
WHERE status = 'pending'
ORDER BY created_at ASC;

-- name: UpdateDMQueueItemMessageID :one
UPDATE dm_queue_items
SET message_id = $2
WHERE id = $1
RETURNING *;

-- name: MarkDMQueueItemResolved :one
UPDATE dm_queue_items
SET status = 'resolved',
    outcome = $2,
    resolved_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkDMQueueItemCancelled :one
UPDATE dm_queue_items
SET status = 'cancelled',
    outcome = $2,
    resolved_at = now()
WHERE id = $1
RETURNING *;

-- name: CountPendingDMQueueItemsByCampaign :one
-- SR-032: counts unresolved dm-queue items for a campaign so the Combat
-- Workspace tab badges + Encounter Overview "queued" pill can render the
-- pending-action backlog. Resolved/cancelled rows are excluded by the
-- status filter.
SELECT count(*)::BIGINT AS pending_count
FROM dm_queue_items
WHERE campaign_id = $1 AND status = 'pending';
