-- +goose Up
-- Phase 110: exploration mode.
-- Exploration reuses the encounters table but skips initiative/turns.
ALTER TABLE encounters
    ADD COLUMN mode TEXT NOT NULL DEFAULT 'combat'
    CHECK (mode IN ('combat', 'exploration'));

CREATE INDEX idx_encounters_mode ON encounters(mode);

-- +goose Down
DROP INDEX IF EXISTS idx_encounters_mode;
ALTER TABLE encounters DROP COLUMN IF EXISTS mode;
