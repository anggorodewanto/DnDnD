-- +goose Up
--
-- Phase 119 — Error Log Schema Follow-up.
--
-- Splits errors out of action_log into a dedicated error_log table. The
-- previous Phase 112 approach overloaded action_log.before_state JSONB with
-- {command, user_id} metadata for action_type='error' rows and required
-- dropping the NOT NULL constraints on turn_id/encounter_id/actor_id since
-- non-combat errors have no parent. Phase 119 keeps action_log combat-only
-- and surfaces command/user_id as first-class columns here.

CREATE TABLE error_log (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  command       TEXT NOT NULL,
  user_id       TEXT,                    -- nullable: system-context errors have no user
  summary       TEXT NOT NULL,
  error_detail  JSONB,                   -- optional structured detail (full error msg / stack / fields)
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX error_log_created_at_idx ON error_log (created_at DESC);

-- +goose Down

DROP INDEX IF EXISTS error_log_created_at_idx;
DROP TABLE IF EXISTS error_log;
