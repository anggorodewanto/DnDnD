-- +goose Up
ALTER TABLE spells
    ADD COLUMN campaign_id UUID REFERENCES campaigns(id),
    ADD COLUMN homebrew BOOLEAN DEFAULT false,
    ADD COLUMN source TEXT DEFAULT 'srd';

ALTER TABLE weapons
    ADD COLUMN campaign_id UUID REFERENCES campaigns(id),
    ADD COLUMN homebrew BOOLEAN DEFAULT false,
    ADD COLUMN source TEXT DEFAULT 'srd';

ALTER TABLE races
    ADD COLUMN campaign_id UUID REFERENCES campaigns(id),
    ADD COLUMN homebrew BOOLEAN DEFAULT false,
    ADD COLUMN source TEXT DEFAULT 'srd';

ALTER TABLE feats
    ADD COLUMN campaign_id UUID REFERENCES campaigns(id),
    ADD COLUMN homebrew BOOLEAN DEFAULT false,
    ADD COLUMN source TEXT DEFAULT 'srd';

ALTER TABLE classes
    ADD COLUMN campaign_id UUID REFERENCES campaigns(id),
    ADD COLUMN homebrew BOOLEAN DEFAULT false,
    ADD COLUMN source TEXT DEFAULT 'srd';

-- +goose Down
ALTER TABLE classes
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS homebrew,
    DROP COLUMN IF EXISTS campaign_id;

ALTER TABLE feats
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS homebrew,
    DROP COLUMN IF EXISTS campaign_id;

ALTER TABLE races
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS homebrew,
    DROP COLUMN IF EXISTS campaign_id;

ALTER TABLE weapons
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS homebrew,
    DROP COLUMN IF EXISTS campaign_id;

ALTER TABLE spells
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS homebrew,
    DROP COLUMN IF EXISTS campaign_id;
