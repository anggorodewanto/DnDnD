-- +goose Up
CREATE TABLE player_characters (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id     UUID NOT NULL REFERENCES campaigns(id),
    character_id    UUID NOT NULL REFERENCES characters(id),
    discord_user_id TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'changes_requested', 'rejected', 'retired')),
    dm_feedback     TEXT,
    created_via     TEXT NOT NULL CHECK (created_via IN ('register', 'import', 'create', 'dm_dashboard')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(campaign_id, discord_user_id),
    UNIQUE(campaign_id, character_id)
);

CREATE INDEX idx_player_characters_campaign_id ON player_characters(campaign_id);
CREATE INDEX idx_player_characters_discord_user_id ON player_characters(discord_user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_player_characters_discord_user_id;
DROP INDEX IF EXISTS idx_player_characters_campaign_id;
DROP TABLE IF EXISTS player_characters;
