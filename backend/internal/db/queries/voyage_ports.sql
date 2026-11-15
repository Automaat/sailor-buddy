-- name: CreateVoyagePort :one
INSERT INTO voyage_ports (voyage_id, name, latitude, longitude, position, anchorage)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: ListVoyagePorts :many
SELECT * FROM voyage_ports WHERE voyage_id = $1 ORDER BY position, id;

-- name: AddVoyagePort :one
-- Adds a port for an admin or the account linked to the voyage's captain
-- person, else inserts no row. The check runs in the statement, so a
-- captain change committed before it refuses the write.
INSERT INTO voyage_ports (voyage_id, name, latitude, longitude, position, anchorage)
SELECT sqlc.arg(voyage_id)::BIGINT, sqlc.arg(name)::TEXT, sqlc.narg(latitude)::DOUBLE PRECISION,
    sqlc.narg(longitude)::DOUBLE PRECISION, sqlc.arg(position)::BIGINT, sqlc.arg(anchorage)::BOOLEAN
WHERE sqlc.arg(caller_is_admin)::BOOLEAN OR EXISTS (
    SELECT 1 FROM voyages v JOIN crew_members cm ON cm.id = v.captain_person_id
    WHERE v.id = sqlc.arg(voyage_id)::BIGINT AND cm.user_id = sqlc.arg(caller_id)::BIGINT)
RETURNING *;

-- name: DeleteVoyagePort :one
-- Deletes the port for an admin or the account linked to the voyage's
-- captain person and answers whether the caller passed that check, since a
-- port already gone is no error.
WITH allowed AS (
    SELECT (sqlc.arg(caller_is_admin)::BOOLEAN OR EXISTS (
        SELECT 1 FROM voyages v JOIN crew_members cm ON cm.id = v.captain_person_id
        WHERE v.id = sqlc.arg(voyage_id)::BIGINT AND cm.user_id = sqlc.arg(caller_id)::BIGINT)) AS ok
), deleted AS (
    DELETE FROM voyage_ports p
    WHERE p.id = sqlc.arg(id)::BIGINT AND p.voyage_id = sqlc.arg(voyage_id)::BIGINT
      AND (SELECT ok FROM allowed)
)
SELECT ok::BOOLEAN AS ok FROM allowed;

-- name: SetVoyagePortPosition :execrows
-- Moves the port for an admin or the account linked to the voyage's captain
-- person, else updates no row.
UPDATE voyage_ports SET position = sqlc.arg(position)::BIGINT
WHERE id = sqlc.arg(id)::BIGINT AND voyage_id = sqlc.arg(voyage_id)::BIGINT
  AND (sqlc.arg(caller_is_admin)::BOOLEAN OR EXISTS (
      SELECT 1 FROM voyages v JOIN crew_members cm ON cm.id = v.captain_person_id
      WHERE v.id = sqlc.arg(voyage_id)::BIGINT AND cm.user_id = sqlc.arg(caller_id)::BIGINT));

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
