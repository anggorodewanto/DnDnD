-- +goose Up
CREATE TABLE maps (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id         UUID NOT NULL REFERENCES campaigns(id),
    name                TEXT NOT NULL,
    width_squares       INTEGER NOT NULL CHECK (width_squares >= 1),
    height_squares      INTEGER NOT NULL CHECK (height_squares >= 1),
    tiled_json          JSONB NOT NULL,
    background_image_id UUID,
    tileset_refs        JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS maps;
