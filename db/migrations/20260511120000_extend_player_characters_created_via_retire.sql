-- +goose Up
--
-- Phase 8/16 — Retire flow follow-up.
--
-- Phase 8's player_characters table CHECK constraint allowed `created_via` in
-- ('register','import','create','dm_dashboard') only. Phase 16's dashboard
-- retire-approval branch (approval_handler.go) keys off `created_via='retire'`,
-- so without an extension to the constraint the retire path is unreachable.
-- This migration extends the constraint to include 'retire'.
--
-- Idempotent: drops the old constraint by its conventional name and re-adds
-- the wider one. Goose runs migrations once, but defensive DROP IF EXISTS
-- keeps re-runs (e.g. after a manual revert) safe.

ALTER TABLE player_characters
    DROP CONSTRAINT IF EXISTS player_characters_created_via_check;

ALTER TABLE player_characters
    ADD CONSTRAINT player_characters_created_via_check
    CHECK (created_via IN ('register', 'import', 'create', 'dm_dashboard', 'retire'));

-- +goose Down
ALTER TABLE player_characters
    DROP CONSTRAINT IF EXISTS player_characters_created_via_check;

ALTER TABLE player_characters
    ADD CONSTRAINT player_characters_created_via_check
    CHECK (created_via IN ('register', 'import', 'create', 'dm_dashboard'));
