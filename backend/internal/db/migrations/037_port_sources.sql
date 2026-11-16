-- Port search over every port the club entered: the visited ports of
-- voyages and the start and end ports of trips and voyages.

-- Stored port names follow the API name rule: no surrounding blanks, one
-- space between words, and a blank start or end port means no port.
UPDATE trips SET start_port = NULLIF(btrim(regexp_replace(start_port, '\s+', ' ', 'g')), '')
WHERE start_port IS DISTINCT FROM NULLIF(btrim(regexp_replace(start_port, '\s+', ' ', 'g')), '');
UPDATE trips SET end_port = NULLIF(btrim(regexp_replace(end_port, '\s+', ' ', 'g')), '')
WHERE end_port IS DISTINCT FROM NULLIF(btrim(regexp_replace(end_port, '\s+', ' ', 'g')), '');
UPDATE voyages SET start_port = NULLIF(btrim(regexp_replace(start_port, '\s+', ' ', 'g')), '')
WHERE start_port IS DISTINCT FROM NULLIF(btrim(regexp_replace(start_port, '\s+', ' ', 'g')), '');
UPDATE voyages SET end_port = NULLIF(btrim(regexp_replace(end_port, '\s+', ' ', 'g')), '')
WHERE end_port IS DISTINCT FROM NULLIF(btrim(regexp_replace(end_port, '\s+', ' ', 'g')), '');
-- A visited port's name is required, so a blank one stays as it is.
UPDATE voyage_ports SET name = btrim(regexp_replace(name, '\s+', ' ', 'g'))
WHERE name <> btrim(regexp_replace(name, '\s+', ' ', 'g'))
  AND btrim(regexp_replace(name, '\s+', ' ', 'g')) <> '';

-- A port added without a position now goes after the voyage's last one
-- (max + 1), and the API caps explicit positions. Positions written before
-- had no cap, so renumber each voyage's ports 0..n in their current order
-- (position, then id, as they are listed): a position near the BIGINT limit
-- would otherwise make every later append overflow.
UPDATE voyage_ports p SET position = r.seq
FROM (
    SELECT id, row_number() OVER (PARTITION BY voyage_id ORDER BY position, id) - 1 AS seq
    FROM voyage_ports
) r
WHERE p.id = r.id AND p.position <> r.seq;

-- port_search_prefix turns a query into a LIKE pattern for names that start
-- with it. LIKE, unlike starts_with, can use a trigram index; the query's
-- own wildcards and escape character match literally.
CREATE FUNCTION port_search_prefix(query TEXT) RETURNS TEXT
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    AS $$ SELECT replace(replace(replace(port_search_norm(query), E'\\', E'\\\\'), '%', E'\\%'), '_', E'\\_') || '%' $$;

-- port_search_matches reports whether a stored port name matches a typed
-- query: it starts with the query, is similar to it (typos), or holds a
-- word similar to it. port_search_similarity ranks the matches.
--
-- pg_trgm may live outside search_path (CREATE EXTENSION IF NOT EXISTS in
-- 036 keeps an existing install), so its schema is read from pg_extension
-- and its operators, function and operator class are named with it. Both
-- functions are plain SQL expressions, which the planner inlines, so a
-- trigram index on port_search_norm(column) serves port_search_matches.
DO $do$
DECLARE
    trgm TEXT;
BEGIN
    SELECT n.nspname INTO STRICT trgm
    FROM pg_extension e
    JOIN pg_namespace n ON n.oid = e.extnamespace
    WHERE e.extname = 'pg_trgm';

    EXECUTE format(
        'CREATE FUNCTION port_search_matches(name TEXT, query TEXT) RETURNS BOOLEAN
             LANGUAGE sql STABLE PARALLEL SAFE
             AS %L',
        format(' SELECT port_search_norm($1) LIKE port_search_prefix($2)'
               ' OR port_search_norm($1) OPERATOR(%1$I.%%) port_search_norm($2)'
               ' OR port_search_norm($2) OPERATOR(%1$I.<%%) port_search_norm($1) ', trgm));

    EXECUTE format(
        'CREATE FUNCTION port_search_similarity(a TEXT, b TEXT) RETURNS REAL
             LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
             AS %L',
        format(' SELECT %I.similarity($1, $2) ', trgm));

    EXECUTE format('CREATE INDEX trips_start_port_trgm_idx
        ON trips USING gin (port_search_norm(start_port) %I.gin_trgm_ops)', trgm);
    EXECUTE format('CREATE INDEX trips_end_port_trgm_idx
        ON trips USING gin (port_search_norm(end_port) %I.gin_trgm_ops)', trgm);
    EXECUTE format('CREATE INDEX voyages_start_port_trgm_idx
        ON voyages USING gin (port_search_norm(start_port) %I.gin_trgm_ops)', trgm);
    EXECUTE format('CREATE INDEX voyages_end_port_trgm_idx
        ON voyages USING gin (port_search_norm(end_port) %I.gin_trgm_ops)', trgm);
END
$do$;
