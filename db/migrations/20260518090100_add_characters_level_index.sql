-- +goose Up
CREATE INDEX idx_characters_level ON characters(level);

-- +goose Down
DROP INDEX IF EXISTS idx_characters_level;
