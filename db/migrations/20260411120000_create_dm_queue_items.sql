-- +goose Up
CREATE TABLE dm_queue_items (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id  UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    guild_id     TEXT NOT NULL,
    channel_id   TEXT NOT NULL,
    message_id   TEXT NOT NULL,
    kind         TEXT NOT NULL,
    player_name  TEXT NOT NULL,
    summary      TEXT NOT NULL,
    resolve_path TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'resolved', 'cancelled')),
    outcome      TEXT NOT NULL DEFAULT '',
    extra        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at  TIMESTAMPTZ
);

CREATE INDEX dm_queue_items_campaign_status_idx
    ON dm_queue_items (campaign_id, status);

CREATE UNIQUE INDEX dm_queue_items_channel_message_uniq
    ON dm_queue_items (channel_id, message_id);

-- +goose Down
DROP INDEX IF EXISTS dm_queue_items_channel_message_uniq;
DROP INDEX IF EXISTS dm_queue_items_campaign_status_idx;
DROP TABLE IF EXISTS dm_queue_items;
