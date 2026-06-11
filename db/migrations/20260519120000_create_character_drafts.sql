-- +goose Up
-- T11 / Finding 4·b (lost-work cluster): server-side persistence of the
-- in-progress character-builder draft. The localStorage draft is cleared on a
-- successful submit and is browser-local, so the "request changes" cycle
-- (re-run /create-character -> fresh token -> new builder page) and a
-- cross-device resume both came up with a blank form. This table stores the
-- exact, lossless builder draft blob keyed by (campaign, player, mode) so the
-- builder can rehydrate it on mount.
--
-- The draft is opaque JSON owned by the Svelte builder (serializeDraft): the
-- server never interprets it, which keeps the table decoupled from builder
-- field churn. Keyed by mode as well as (campaign, user) so a single Discord
-- account that is both a player (PC draft) and a DM (NPC draft) for the same
-- campaign does not clobber one draft with the other — mirroring the client's
-- draftScope() namespacing.
CREATE TABLE character_drafts (
    campaign_id     UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    discord_user_id TEXT NOT NULL,
    mode            TEXT NOT NULL DEFAULT 'player',
    draft           JSONB NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (campaign_id, discord_user_id, mode)
);

-- +goose Down
DROP TABLE IF EXISTS character_drafts;
