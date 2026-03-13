-- +goose Up
ALTER TABLE turns ADD COLUMN has_stood_this_turn BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE turns DROP COLUMN has_stood_this_turn;
