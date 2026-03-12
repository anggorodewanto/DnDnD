-- +goose Up
ALTER TABLE characters ADD COLUMN card_message_id TEXT;

-- +goose Down
ALTER TABLE characters DROP COLUMN IF EXISTS card_message_id;
