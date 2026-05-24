-- +goose Up
ALTER TABLE weapons ADD COLUMN mastery TEXT NOT NULL DEFAULT '';
ALTER TABLE classes ADD COLUMN weapon_mastery_count INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE weapons DROP COLUMN mastery;
ALTER TABLE classes DROP COLUMN weapon_mastery_count;
