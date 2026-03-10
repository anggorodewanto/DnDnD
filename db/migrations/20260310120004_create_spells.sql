-- +goose Up
CREATE TABLE spells (
    id                   TEXT PRIMARY KEY,
    name                 TEXT NOT NULL,
    level                INTEGER NOT NULL,
    school               TEXT NOT NULL,
    casting_time         TEXT NOT NULL,
    range_ft             INTEGER,
    range_type           TEXT NOT NULL,
    components           TEXT[] NOT NULL,
    material_description TEXT,
    material_cost_gp     REAL,
    material_consumed    BOOLEAN DEFAULT false,
    duration             TEXT NOT NULL,
    concentration        BOOLEAN DEFAULT false,
    ritual               BOOLEAN DEFAULT false,
    description          TEXT NOT NULL,
    higher_levels        TEXT,
    damage               JSONB,
    healing              JSONB,
    save_ability         TEXT,
    save_effect          TEXT,
    attack_type          TEXT,
    area_of_effect       JSONB,
    conditions_applied   TEXT[],
    teleport             JSONB,
    resolution_mode      TEXT NOT NULL DEFAULT 'dm_required',
    classes              TEXT[] NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS spells;
