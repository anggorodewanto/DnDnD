-- +goose Up
CREATE TABLE characters (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id         UUID NOT NULL REFERENCES campaigns(id),
    name                TEXT NOT NULL,
    race                TEXT NOT NULL,
    classes             JSONB NOT NULL,
    level               INTEGER NOT NULL DEFAULT 1,
    ability_scores      JSONB NOT NULL,
    hp_max              INTEGER NOT NULL,
    hp_current          INTEGER NOT NULL,
    temp_hp             INTEGER NOT NULL DEFAULT 0,
    ac                  INTEGER NOT NULL,
    ac_formula          TEXT,
    speed_ft            INTEGER NOT NULL DEFAULT 30,
    proficiency_bonus   INTEGER NOT NULL,
    equipped_main_hand  TEXT,
    equipped_off_hand   TEXT,
    equipped_armor      TEXT,
    spell_slots         JSONB,
    pact_magic_slots    JSONB,
    hit_dice_remaining  JSONB NOT NULL,
    feature_uses        JSONB,
    features            JSONB,
    proficiencies       JSONB,
    gold                INTEGER NOT NULL DEFAULT 0,
    attunement_slots    JSONB,
    languages           TEXT[] NOT NULL,
    inventory           JSONB,
    character_data      JSONB,
    ddb_url             TEXT,
    homebrew            BOOLEAN DEFAULT false,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_characters_campaign_id ON characters(campaign_id);

-- +goose Down
DROP INDEX IF EXISTS idx_characters_campaign_id;
DROP TABLE IF EXISTS characters;
