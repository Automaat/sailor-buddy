-- name: CreateVoyagePort :one
INSERT INTO voyage_ports (voyage_id, name, latitude, longitude, position, anchorage)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: ListVoyagePorts :many
SELECT * FROM voyage_ports WHERE voyage_id = $1 ORDER BY position, id;

-- name: LockVoyagePorts :exec
-- Serializes port writes on one voyage until the transaction ends, so
-- writers appending a port take turns and each sees the ports the one
-- before added. An advisory lock, not a row lock on the voyage, so a
-- captain change does not wait for it.
SELECT pg_advisory_xact_lock(hashtextextended('voyage-ports:' || sqlc.arg(voyage_id)::BIGINT, 0));

-- name: AddVoyagePort :one
-- Adds a port for an admin or the account linked to the voyage's captain
-- person, else inserts no row. The check runs in the statement, so a
-- captain change committed before it refuses the write. Without a position
-- the port goes after the voyage's last port; run LockVoyagePorts first in
-- the same transaction, so concurrent appends do not share a position.
INSERT INTO voyage_ports (voyage_id, name, latitude, longitude, position, anchorage)
SELECT sqlc.arg(voyage_id)::BIGINT, sqlc.arg(name)::TEXT, sqlc.narg(latitude)::DOUBLE PRECISION,
    sqlc.narg(longitude)::DOUBLE PRECISION,
    COALESCE(sqlc.narg(position)::BIGINT, (
        SELECT COALESCE(MAX(p.position) + 1, 0) FROM voyage_ports p
        WHERE p.voyage_id = sqlc.arg(voyage_id)::BIGINT)),
    sqlc.arg(anchorage)::BOOLEAN
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
-- case and accents: visited ports of voyages and the start and end ports of
-- trips and voyages. Names starting with the query come first, then by
-- trigram similarity (typos), then by use count. Spellings of one name
-- collapse into one row named after the most used spelling, with the
-- coordinates of the latest visited port that has them; start and end
-- ports carry none. Without include_hidden only ports of legs visible to
-- members (leg and its cruise published) count. Each source is a branch of
-- its own, so each can use the trigram index on its column.
WITH matched AS (
    SELECT p.id, p.name, p.latitude, p.longitude
    FROM voyage_ports p
    JOIN voyages v ON v.id = p.voyage_id
    JOIN cruises c ON c.id = v.cruise_id
    WHERE port_search_matches(p.name, sqlc.arg(query)::TEXT)
      AND (sqlc.arg(include_hidden)::BOOLEAN OR (v.published_at IS NOT NULL AND c.published_at IS NOT NULL))
    UNION ALL
    SELECT t.id, t.start_port, NULL::DOUBLE PRECISION, NULL::DOUBLE PRECISION
    FROM trips t
    JOIN cruises c ON c.id = t.cruise_id
    WHERE port_search_matches(t.start_port, sqlc.arg(query)::TEXT)
      AND (sqlc.arg(include_hidden)::BOOLEAN OR (t.published_at IS NOT NULL AND c.published_at IS NOT NULL))
    UNION ALL
    SELECT t.id, t.end_port, NULL::DOUBLE PRECISION, NULL::DOUBLE PRECISION
    FROM trips t
    JOIN cruises c ON c.id = t.cruise_id
    WHERE port_search_matches(t.end_port, sqlc.arg(query)::TEXT)
      AND (sqlc.arg(include_hidden)::BOOLEAN OR (t.published_at IS NOT NULL AND c.published_at IS NOT NULL))
    UNION ALL
    SELECT v.id, v.start_port, NULL::DOUBLE PRECISION, NULL::DOUBLE PRECISION
    FROM voyages v
    JOIN cruises c ON c.id = v.cruise_id
    WHERE port_search_matches(v.start_port, sqlc.arg(query)::TEXT)
      AND (sqlc.arg(include_hidden)::BOOLEAN OR (v.published_at IS NOT NULL AND c.published_at IS NOT NULL))
    UNION ALL
    SELECT v.id, v.end_port, NULL::DOUBLE PRECISION, NULL::DOUBLE PRECISION
    FROM voyages v
    JOIN cruises c ON c.id = v.cruise_id
    WHERE port_search_matches(v.end_port, sqlc.arg(query)::TEXT)
      AND (sqlc.arg(include_hidden)::BOOLEAN OR (v.published_at IS NOT NULL AND c.published_at IS NOT NULL))
),
spelled AS (
    SELECT m.id, m.name, m.latitude, m.longitude,
           port_search_norm(m.name) AS norm,
           COUNT(*) OVER (PARTITION BY m.name) AS spelling_uses
    FROM matched m
),
grouped AS (
    SELECT norm,
           (array_agg(name ORDER BY spelling_uses DESC, id DESC, name))[1] AS name,
           COUNT(*) AS uses
    FROM spelled
    GROUP BY norm
)
SELECT g.name::TEXT AS name, coords.latitude, coords.longitude, g.uses::BIGINT AS uses
FROM grouped g
LEFT JOIN LATERAL (
    SELECT s.latitude, s.longitude
    FROM spelled s
    WHERE s.norm = g.norm AND s.latitude IS NOT NULL
    ORDER BY s.id DESC
    LIMIT 1
) coords ON TRUE
ORDER BY starts_with(g.norm, port_search_norm(sqlc.arg(query)::TEXT)) DESC,
         port_search_similarity(g.norm, port_search_norm(sqlc.arg(query)::TEXT)) DESC,
         g.uses DESC, g.norm
LIMIT sqlc.arg(row_limit);
