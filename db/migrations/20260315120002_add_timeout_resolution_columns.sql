-- +goose Up
ALTER TABLE turns ADD COLUMN dm_decision_sent_at TIMESTAMPTZ;
ALTER TABLE turns ADD COLUMN dm_decision_deadline TIMESTAMPTZ;
ALTER TABLE turns ADD COLUMN wait_extended BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE turns ADD COLUMN auto_resolved BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE combatants ADD COLUMN consecutive_auto_resolves INT NOT NULL DEFAULT 0;
ALTER TABLE combatants ADD COLUMN is_absent BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE turns DROP COLUMN IF EXISTS dm_decision_sent_at;
ALTER TABLE turns DROP COLUMN IF EXISTS dm_decision_deadline;
ALTER TABLE turns DROP COLUMN IF EXISTS wait_extended;
ALTER TABLE turns DROP COLUMN IF EXISTS auto_resolved;

ALTER TABLE combatants DROP COLUMN IF EXISTS consecutive_auto_resolves;
ALTER TABLE combatants DROP COLUMN IF EXISTS is_absent;
