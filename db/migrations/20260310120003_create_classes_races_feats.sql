-- +goose Up
CREATE TABLE classes (
    id                      TEXT PRIMARY KEY,
    name                    TEXT NOT NULL,
    hit_die                 TEXT NOT NULL,
    primary_ability         TEXT NOT NULL,
    save_proficiencies      TEXT[] NOT NULL,
    armor_proficiencies     TEXT[],
    weapon_proficiencies    TEXT[],
    skill_choices           JSONB,
    spellcasting            JSONB,
    features_by_level       JSONB NOT NULL,
    attacks_per_action      JSONB NOT NULL,
    subclass_level          INTEGER NOT NULL,
    subclasses              JSONB NOT NULL,
    multiclass_prereqs      JSONB,
    multiclass_proficiencies JSONB,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE races (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    speed_ft        INTEGER NOT NULL,
    size            TEXT NOT NULL,
    ability_bonuses JSONB NOT NULL,
    darkvision_ft   INTEGER NOT NULL DEFAULT 0,
    traits          JSONB NOT NULL,
    languages       TEXT[],
    subraces        JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE feats (
    id                TEXT PRIMARY KEY,
    name              TEXT NOT NULL,
    description       TEXT NOT NULL,
    prerequisites     JSONB,
    asi_bonus         JSONB,
    mechanical_effect JSONB,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS feats;
DROP TABLE IF EXISTS races;
DROP TABLE IF EXISTS classes;
