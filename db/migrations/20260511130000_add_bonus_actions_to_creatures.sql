-- +goose Up
-- F-78c: structured bonus_actions JSONB column on creatures.
-- Replaces (or augments) runtime parsing of the abilities column for
-- entries whose description mentions "bonus action". The combat turn
-- builder prefers this structured column; rows that leave it NULL fall
-- back to the legacy ParseBonusActions scan so existing imported creature
-- data keeps working unchanged.
ALTER TABLE creatures ADD COLUMN bonus_actions JSONB;

-- +goose Down
ALTER TABLE creatures DROP COLUMN bonus_actions;
