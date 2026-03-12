-- +goose Up
CREATE TABLE assets (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id  UUID NOT NULL REFERENCES campaigns(id),
    type         TEXT NOT NULL,
    original_name TEXT NOT NULL,
    mime_type    TEXT NOT NULL,
    byte_size    BIGINT NOT NULL,
    storage_path TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE maps
    ADD CONSTRAINT maps_background_image_id_fkey
    FOREIGN KEY (background_image_id) REFERENCES assets(id);

-- +goose Down
ALTER TABLE maps DROP CONSTRAINT IF EXISTS maps_background_image_id_fkey;
DROP TABLE IF EXISTS assets;
