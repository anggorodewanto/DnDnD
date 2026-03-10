-- +goose Up
CREATE TABLE weapons (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    damage          TEXT NOT NULL,
    damage_type     TEXT NOT NULL,
    weight_lb       REAL,
    properties      TEXT[] NOT NULL DEFAULT '{}',
    range_normal_ft INTEGER,
    range_long_ft   INTEGER,
    versatile_damage TEXT,
    weapon_type     TEXT NOT NULL CHECK (weapon_type IN ('simple_melee', 'simple_ranged', 'martial_melee', 'martial_ranged')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE armor (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    ac_base         INTEGER NOT NULL,
    ac_dex_bonus    BOOLEAN DEFAULT true,
    ac_dex_max      INTEGER,
    strength_req    INTEGER,
    stealth_disadv  BOOLEAN DEFAULT false,
    armor_type      TEXT NOT NULL CHECK (armor_type IN ('light', 'medium', 'heavy', 'shield')),
    weight_lb       REAL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE conditions_ref (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL,
    mechanical_effects JSONB NOT NULL DEFAULT '[]',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS conditions_ref;
DROP TABLE IF EXISTS armor;
DROP TABLE IF EXISTS weapons;
