-- name: CreateCruiseSeries :one
INSERT INTO cruise_series (created_by, name, description, kind)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetCruiseSeries :one
SELECT * FROM cruise_series WHERE id = $1;

-- name: ListCruiseSeries :many
-- A set after_id starts the page after the cursor's (name, id) sort key.
SELECT * FROM cruise_series
WHERE sqlc.narg(after_id)::BIGINT IS NULL
   OR (name, id) > (sqlc.narg(after_name)::TEXT, sqlc.narg(after_id)::BIGINT)
ORDER BY name, id
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountCruiseSeries :one
SELECT COUNT(*)::BIGINT FROM cruise_series;

-- name: UpdateCruiseSeries :exec
UPDATE cruise_series SET name = $1, description = $2, kind = $3, updated_at = CURRENT_TIMESTAMP
WHERE id = $4;

-- name: DeleteCruiseSeries :exec
DELETE FROM cruise_series WHERE id = $1;

-- name: ListCruiseSeriesEditions :many
-- Editions of a series newest first by their first leg's embark date. An
-- edition without a dated leg (NULL, or the empty string the leg forms send)
-- is still being planned, so it sorts before every dated one. Without
-- include_hidden only published editions are listed, and only their
-- published legs set the order.
SELECT c.id, c.name, c.edition, c.published_at
FROM cruises c
WHERE c.series_id = sqlc.arg(series_id)
  AND (sqlc.arg(include_hidden)::BOOLEAN OR c.published_at IS NOT NULL)
ORDER BY (
    SELECT MIN(NULLIF(l.embark_date, '')) FROM (
        SELECT t.embark_date FROM trips t
        WHERE t.cruise_id = c.id
          AND (sqlc.arg(include_hidden)::BOOLEAN OR t.published_at IS NOT NULL)
        UNION ALL
        SELECT lv.embark_date FROM voyages lv
        WHERE lv.cruise_id = c.id
          AND (sqlc.arg(include_hidden)::BOOLEAN OR lv.published_at IS NOT NULL)
    ) l
) DESC NULLS FIRST, c.edition DESC NULLS LAST, c.id DESC;
