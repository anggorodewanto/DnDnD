-- +goose Up
CREATE TABLE narration_templates (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    category    TEXT NOT NULL DEFAULT '',
    body        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX narration_templates_campaign_category_idx
    ON narration_templates (campaign_id, category);

-- +goose Down
DROP INDEX IF EXISTS narration_templates_campaign_category_idx;
DROP TABLE IF EXISTS narration_templates;
