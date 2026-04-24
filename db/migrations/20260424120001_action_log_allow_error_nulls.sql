-- +goose Up
--
-- Phase 112 — Error Handling & Observability.
--
-- Errors are stored in action_log with action_type='error' (spec §Monitoring
-- & Observability, lines 2968-2970). These errors can originate outside of
-- combat (e.g. /register, /import, dashboard requests, PNG rendering), where
-- turn_id, encounter_id, and actor_id have no meaningful value. Drop their
-- NOT NULL constraints so error rows can be persisted alongside the existing
-- combat action rows, which continue to populate all three columns.

ALTER TABLE action_log ALTER COLUMN turn_id DROP NOT NULL;
ALTER TABLE action_log ALTER COLUMN encounter_id DROP NOT NULL;
ALTER TABLE action_log ALTER COLUMN actor_id DROP NOT NULL;

-- +goose Down
--
-- Clear any rows that only exist because of the loosened constraint before
-- restoring NOT NULL, otherwise the ALTER would fail. action_type='error'
-- rows are the only callers intended to rely on the loosened schema; combat
-- rows always populate every column.

DELETE FROM action_log WHERE action_type = 'error';
ALTER TABLE action_log ALTER COLUMN turn_id SET NOT NULL;
ALTER TABLE action_log ALTER COLUMN encounter_id SET NOT NULL;
ALTER TABLE action_log ALTER COLUMN actor_id SET NOT NULL;
