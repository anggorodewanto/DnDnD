-- +goose Up
-- T20 / Finding 12 (campaign switcher): an explicit, persisted per-DM active
-- campaign selection. Previously the dashboard bound to LookupActiveCampaign's
-- "most recently created, non-archived" heuristic, so creating any second
-- campaign (a test, a typo retry, a dashboard-form orphan) instantly flipped
-- the Maps/Encounters/Party context for the whole dashboard with no way back
-- except archiving. This table records the DM's deliberate choice so the
-- resolver can honor it instead of silently following created_at DESC.
--
-- One row per DM (the PK). active_campaign_id references campaigns(id) with
-- ON DELETE CASCADE so a deleted campaign cleanly drops the stale pointer; the
-- resolver additionally ignores a stored choice that has since been archived
-- and falls back to the most-recent non-archived campaign.
CREATE TABLE dm_active_campaign (
    dm_user_id         TEXT PRIMARY KEY,
    active_campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS dm_active_campaign;
