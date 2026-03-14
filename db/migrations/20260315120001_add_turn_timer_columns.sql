-- +goose Up
ALTER TABLE turns ADD COLUMN nudge_sent_at TIMESTAMPTZ;
ALTER TABLE turns ADD COLUMN warning_sent_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE turns DROP COLUMN IF EXISTS nudge_sent_at;
ALTER TABLE turns DROP COLUMN IF EXISTS warning_sent_at;
