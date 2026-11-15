-- Port name suggestions: fuzzy, accent-insensitive matching of the port names
-- the club already entered. Both extensions ship with PostgreSQL contrib and
-- are trusted, so the database owner can create them without superuser.

CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;

-- port_search_norm folds a port name for matching: no accents, lower case,
-- no surrounding blanks. unaccent() is only STABLE because its dictionary
-- could change; naming the dictionary and its schema pins the result, so the
-- wrapper can be IMMUTABLE and back an index.
CREATE FUNCTION port_search_norm(name TEXT) RETURNS TEXT
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    AS $$ SELECT lower(public.unaccent('public.unaccent'::regdictionary, btrim($1))) $$;

CREATE INDEX voyage_ports_name_trgm_idx
    ON voyage_ports USING gin (port_search_norm(name) gin_trgm_ops);
