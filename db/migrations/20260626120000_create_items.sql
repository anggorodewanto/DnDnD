-- +goose Up
-- items is the canonical SSOT catalog: one row per equipment id, giving every
-- id (weapons, armor, ammunition, adventuring gear) a uniform
-- name / category / default_quantity. Weapons and armor keep their detailed
-- stat tables (weapons, armor); this catalog references them by id so the
-- builder seeder, /api/equipment, and the frontend resolve "what is item X"
-- from one place. See docs/live-play/issues.md ISSUE-017.
CREATE TABLE items (
    id               TEXT PRIMARY KEY,
    name             TEXT NOT NULL,
    category         TEXT NOT NULL CHECK (category IN ('weapon', 'armor', 'ammunition', 'gear')),
    default_quantity INTEGER NOT NULL DEFAULT 1,
    stackable        BOOLEAN NOT NULL DEFAULT false,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS items;
