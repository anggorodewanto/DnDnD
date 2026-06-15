-- +goose Up
-- Admin-managed Open5e custom sources. The 11 built-in document slugs live in
-- Go (internal/open5e/sources.go) and stay the immutable baseline; this table
-- holds DM/admin-added slugs that extend the catalog at runtime so a new Open5e
-- document can be enabled per campaign without shipping a code change.
--
-- slug is the canonical Open5e document__slug value (lowercase, hyphenated) and
-- the PK so a slug is added at most once. The per-campaign filter
-- (internal/open5e/filter.go) compares each cached row's "open5e:<slug>" source
-- against the union of built-in + custom slugs, so adding a row here makes that
-- book selectable in every campaign's source toggle.
CREATE TABLE open5e_custom_sources (
    slug        TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    publisher   TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS open5e_custom_sources;
