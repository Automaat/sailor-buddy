-- name: CreateVoyagePort :one
INSERT INTO voyage_ports (voyage_id, name, latitude, longitude, position, anchorage)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: ListVoyagePorts :many
SELECT * FROM voyage_ports WHERE voyage_id = $1 ORDER BY position, id;

-- name: DeleteVoyagePort :exec
DELETE FROM voyage_ports WHERE id = $1 AND voyage_id = $2;

-- name: SetVoyagePortPosition :exec
UPDATE voyage_ports SET position = $3 WHERE id = $1 AND voyage_id = $2;

-- name: SuggestPorts :many
-- Port names the club already entered that match a typed query, ignoring
-- case and accents: names starting with the query first, then by trigram
-- similarity (typos), then by use count. Spellings of one name collapse
-- into one row named after the most used spelling, with the coordinates of
-- the latest port that has them. Without include_hidden only ports of
-- voyages visible to members (voyage and its cruise published) count.
WITH needle AS (
    SELECT port_search_norm(sqlc.arg(query)::TEXT) AS q
),
matched AS (
    SELECT p.id, p.name, p.latitude, p.longitude,
           port_search_norm(p.name) AS norm,
           n.q,
           COUNT(*) OVER (PARTITION BY p.name) AS spelling_uses
    FROM voyage_ports p
    JOIN voyages v ON v.id = p.voyage_id
    CROSS JOIN needle n
    WHERE (sqlc.arg(include_hidden)::BOOLEAN OR (v.published_at IS NOT NULL AND EXISTS (
            SELECT 1 FROM cruises c WHERE c.id = v.cruise_id AND c.published_at IS NOT NULL)))
      AND (starts_with(port_search_norm(p.name), n.q)
           OR port_search_norm(p.name) % n.q
           OR n.q <% port_search_norm(p.name))
),
grouped AS (
    SELECT norm, q,
           (array_agg(name ORDER BY spelling_uses DESC, id DESC))[1] AS name,
           COUNT(*) AS uses
    FROM matched
    GROUP BY norm, q
)
SELECT g.name::TEXT AS name, coords.latitude, coords.longitude, g.uses::BIGINT AS uses
FROM grouped g
LEFT JOIN LATERAL (
    SELECT m.latitude, m.longitude
    FROM matched m
    WHERE m.norm = g.norm AND m.latitude IS NOT NULL
    ORDER BY m.id DESC
    LIMIT 1
) coords ON TRUE
ORDER BY starts_with(g.norm, g.q) DESC, similarity(g.norm, g.q) DESC, g.uses DESC, g.norm
LIMIT sqlc.arg(row_limit);
