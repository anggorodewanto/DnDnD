-- +goose Up
ALTER TABLE maps ADD CONSTRAINT maps_dimension_limit
    CHECK (width_squares BETWEEN 1 AND 200 AND height_squares BETWEEN 1 AND 200);

-- +goose Down
ALTER TABLE maps DROP CONSTRAINT IF EXISTS maps_dimension_limit;
