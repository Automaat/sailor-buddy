-- name: CreateCruiseSeries :one
INSERT INTO cruise_series (created_by, name, description)
VALUES ($1, $2, $3) RETURNING *;

-- name: GetCruiseSeries :one
SELECT * FROM cruise_series WHERE id = $1;

-- name: ListCruiseSeries :many
SELECT * FROM cruise_series ORDER BY name, id LIMIT $1 OFFSET $2;

-- name: CountCruiseSeries :one
SELECT COUNT(*)::BIGINT FROM cruise_series;

-- name: UpdateCruiseSeries :exec
UPDATE cruise_series SET name = $1, description = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $3;

-- name: DeleteCruiseSeries :exec
DELETE FROM cruise_series WHERE id = $1;

-- name: ListCruiseSeriesEditions :many
-- Editions of a series newest first, each with the miles and hours summed
-- over its voyages. An edition without an embark date is still being
-- planned, so it sorts before every dated one.
SELECT
    c.id,
    c.name,
    c.edition,
    c.kind,
    c.embark_date,
    c.disembark_date,
    COUNT(v.id)::BIGINT AS voyage_count,
    COALESCE(SUM(v.miles), 0)::DOUBLE PRECISION AS total_miles,
    COALESCE(SUM(v.hours_total), 0)::DOUBLE PRECISION AS total_hours
FROM cruises c
LEFT JOIN voyages v ON v.cruise_id = c.id
WHERE c.series_id = $1
GROUP BY c.id
ORDER BY c.embark_date DESC NULLS FIRST, c.edition DESC NULLS LAST, c.id DESC;
