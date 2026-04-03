-- +goose Up
CREATE TABLE loot_pools (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id UUID NOT NULL REFERENCES encounters(id) ON DELETE CASCADE,
    campaign_id  UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    gold_total   INTEGER NOT NULL DEFAULT 0,
    status       TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(encounter_id)
);

CREATE TABLE loot_pool_items (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    loot_pool_id      UUID NOT NULL REFERENCES loot_pools(id) ON DELETE CASCADE,
    item_id           TEXT,
    name              TEXT NOT NULL,
    description       TEXT NOT NULL DEFAULT '',
    quantity          INTEGER NOT NULL DEFAULT 1,
    type              TEXT NOT NULL DEFAULT 'other',
    claimed_by        UUID REFERENCES characters(id),
    claimed_at        TIMESTAMPTZ,
    is_magic          BOOLEAN NOT NULL DEFAULT false,
    magic_bonus       INTEGER NOT NULL DEFAULT 0,
    magic_properties  TEXT NOT NULL DEFAULT '',
    requires_attunement BOOLEAN NOT NULL DEFAULT false,
    rarity            TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_loot_pool_items_pool ON loot_pool_items(loot_pool_id);
CREATE INDEX idx_loot_pools_encounter ON loot_pools(encounter_id);

-- +goose Down
DROP TABLE IF EXISTS loot_pool_items;
DROP TABLE IF EXISTS loot_pools;
