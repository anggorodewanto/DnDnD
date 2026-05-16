-- +goose Up
ALTER TABLE pending_saves ADD COLUMN cover_bonus INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE pending_saves DROP COLUMN cover_bonus;
