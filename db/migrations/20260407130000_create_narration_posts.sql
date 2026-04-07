-- +goose Up
CREATE TABLE narration_posts (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id           UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    author_user_id        TEXT NOT NULL,
    body                  TEXT NOT NULL,
    attachment_asset_ids  UUID[] NOT NULL DEFAULT '{}',
    discord_message_ids   TEXT[] NOT NULL DEFAULT '{}',
    posted_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX narration_posts_campaign_posted_at_idx
    ON narration_posts (campaign_id, posted_at DESC);

-- +goose Down
DROP INDEX IF EXISTS narration_posts_campaign_posted_at_idx;
DROP TABLE IF EXISTS narration_posts;
