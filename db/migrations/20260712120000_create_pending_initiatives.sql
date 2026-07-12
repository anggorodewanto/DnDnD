-- +goose Up
-- APP-5: player-staged initiative collected BEFORE combat starts. A player runs
-- /initiative <roll> to submit their own total; StartCombat reads these rows for
-- the campaign, folds them into the APP-1 character_initiatives map (so the PC's
-- reported total is used verbatim and is never auto-rolled), then clears them.
-- One row per (campaign, character) so re-submitting upserts — a player can edit
-- their staged value, or clear it with /initiative clear:true.
CREATE TABLE pending_initiatives (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    roll INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (campaign_id, character_id)
);

CREATE INDEX idx_pending_initiatives_campaign ON pending_initiatives(campaign_id);

-- +goose Down
DROP TABLE IF EXISTS pending_initiatives;
