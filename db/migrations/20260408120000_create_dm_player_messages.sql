-- +goose Up
CREATE TABLE dm_player_messages (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id          UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    player_character_id  UUID NOT NULL REFERENCES player_characters(id) ON DELETE CASCADE,
    author_user_id       TEXT NOT NULL,
    body                 TEXT NOT NULL,
    discord_message_ids  TEXT[] NOT NULL DEFAULT '{}',
    sent_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX dm_player_messages_campaign_player_idx
    ON dm_player_messages (campaign_id, player_character_id, sent_at DESC);

-- +goose Down
DROP INDEX IF EXISTS dm_player_messages_campaign_player_idx;
DROP TABLE IF EXISTS dm_player_messages;
