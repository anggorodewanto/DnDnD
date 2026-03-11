-- +goose Up
CREATE TABLE creatures (
    id                      TEXT PRIMARY KEY,
    campaign_id             UUID REFERENCES campaigns(id),
    name                    TEXT NOT NULL,
    size                    TEXT NOT NULL,
    type                    TEXT NOT NULL,
    alignment               TEXT,
    ac                      INTEGER NOT NULL,
    ac_type                 TEXT,
    hp_formula              TEXT NOT NULL,
    hp_average              INTEGER NOT NULL,
    speed                   JSONB NOT NULL,
    ability_scores          JSONB NOT NULL,
    saving_throws           JSONB,
    skills                  JSONB,
    damage_resistances      TEXT[],
    damage_immunities       TEXT[],
    damage_vulnerabilities  TEXT[],
    condition_immunities    TEXT[],
    senses                  JSONB,
    languages               TEXT[],
    cr                      TEXT NOT NULL,
    attacks                 JSONB NOT NULL,
    abilities               JSONB,
    homebrew                BOOLEAN DEFAULT false,
    source                  TEXT DEFAULT 'srd',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE magic_items (
    id                      TEXT PRIMARY KEY,
    campaign_id             UUID REFERENCES campaigns(id),
    name                    TEXT NOT NULL,
    base_item_type          TEXT,
    base_item_id            TEXT,
    rarity                  TEXT NOT NULL,
    requires_attunement     BOOLEAN DEFAULT false,
    attunement_restriction  TEXT,
    magic_bonus             INTEGER,
    passive_effects         JSONB,
    active_abilities        JSONB,
    charges                 JSONB,
    description             TEXT NOT NULL,
    homebrew                BOOLEAN DEFAULT false,
    source                  TEXT DEFAULT 'srd',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS magic_items;
DROP TABLE IF EXISTS creatures;
