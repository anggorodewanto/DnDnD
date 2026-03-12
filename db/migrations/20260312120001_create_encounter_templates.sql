-- +goose Up
CREATE TABLE encounter_templates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id     UUID NOT NULL REFERENCES campaigns(id),
    map_id          UUID REFERENCES maps(id),
    name            TEXT NOT NULL,
    display_name    TEXT,
    creatures       JSONB NOT NULL DEFAULT '[]',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS encounter_templates;
