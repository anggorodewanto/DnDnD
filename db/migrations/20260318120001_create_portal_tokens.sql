-- +goose Up
CREATE TABLE portal_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token TEXT UNIQUE NOT NULL,
    campaign_id UUID NOT NULL REFERENCES campaigns(id),
    discord_user_id TEXT NOT NULL,
    purpose TEXT NOT NULL,
    used BOOLEAN NOT NULL DEFAULT false,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_portal_tokens_token ON portal_tokens (token);
CREATE INDEX idx_portal_tokens_expires_at ON portal_tokens (expires_at);

-- +goose Down
DROP INDEX IF EXISTS idx_portal_tokens_expires_at;
DROP INDEX IF EXISTS idx_portal_tokens_token;
DROP TABLE IF EXISTS portal_tokens;
