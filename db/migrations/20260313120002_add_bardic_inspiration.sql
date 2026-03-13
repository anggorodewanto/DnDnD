-- +goose Up
ALTER TABLE combatants
    ADD COLUMN bardic_inspiration_die TEXT,
    ADD COLUMN bardic_inspiration_source TEXT,
    ADD COLUMN bardic_inspiration_granted_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE combatants
    DROP COLUMN bardic_inspiration_die,
    DROP COLUMN bardic_inspiration_source,
    DROP COLUMN bardic_inspiration_granted_at;
