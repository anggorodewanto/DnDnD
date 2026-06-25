-- +goose Up
-- Persist the #initiative-tracker Discord message id per encounter so the live
-- tracker is edited in place across bot restarts. Previously the (channel_id,
-- message_id) pair lived only in an in-memory map on the notifier (med-18 /
-- Phase 25), so a restart mid-combat orphaned the tracker message and the next
-- update posted a duplicate instead of editing the existing one.
--
-- One row per encounter (PK = encounter_id, ON DELETE CASCADE) so the mapping
-- is dropped automatically when the encounter is deleted.
CREATE TABLE initiative_tracker_messages (
    encounter_id UUID PRIMARY KEY REFERENCES encounters(id) ON DELETE CASCADE,
    channel_id   TEXT NOT NULL,
    message_id   TEXT NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS initiative_tracker_messages;
