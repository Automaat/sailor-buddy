-- Split cruises into trips (planned/cancelled) and voyages (completed with sailing stats).

CREATE TYPE trip_status AS ENUM ('planned', 'cancelled');

CREATE TABLE trips (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id BIGINT REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    embark_date TEXT,
    disembark_date TEXT,
    countries TEXT,
    start_port TEXT,
    end_port TEXT,
    captain_name TEXT,
    yacht_id BIGINT REFERENCES yachts(id),
    cost_total DOUBLE PRECISION DEFAULT 0,
    cost_per_person DOUBLE PRECISION DEFAULT 0,
    max_crew BIGINT,
    image_logo_url TEXT,
    image_photo_url TEXT,
    image_route_url TEXT,
    description TEXT,
    status trip_status NOT NULL DEFAULT 'planned',
    enroll_token TEXT UNIQUE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_trips_org_id ON trips(org_id) WHERE org_id IS NOT NULL;

CREATE TABLE voyages (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id BIGINT REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    year BIGINT,
    embark_date TEXT,
    disembark_date TEXT,
    countries TEXT,
    start_port TEXT,
    end_port TEXT,
    captain_name TEXT,
    yacht_id BIGINT REFERENCES yachts(id),
    hours_total DOUBLE PRECISION NOT NULL DEFAULT 0,
    hours_sail DOUBLE PRECISION NOT NULL DEFAULT 0,
    hours_engine DOUBLE PRECISION NOT NULL DEFAULT 0,
    hours_over_6bf DOUBLE PRECISION NOT NULL DEFAULT 0,
    miles DOUBLE PRECISION NOT NULL DEFAULT 0,
    days BIGINT NOT NULL DEFAULT 0,
    tidal_waters BIGINT NOT NULL DEFAULT 0,
    cost_total DOUBLE PRECISION DEFAULT 0,
    cost_per_person DOUBLE PRECISION DEFAULT 0,
    image_logo_url TEXT,
    image_photo_url TEXT,
    image_route_url TEXT,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_voyages_org_id ON voyages(org_id) WHERE org_id IS NOT NULL;

-- Backfill trips from planned + cancelled cruises (preserve IDs so FK rewiring works).
INSERT INTO trips (
    id, owner_id, org_id, name, embark_date, disembark_date, countries, start_port, end_port,
    captain_name, yacht_id, cost_total, cost_per_person, max_crew,
    image_logo_url, image_photo_url, image_route_url, description,
    status, enroll_token, created_at, updated_at
) OVERRIDING SYSTEM VALUE
SELECT
    id, owner_id, org_id, name, embark_date, disembark_date, countries, start_port, end_port,
    captain_name, yacht_id, COALESCE(cost_total, 0), COALESCE(cost_per_person, 0), max_crew,
    image_logo_url, image_photo_url, image_route_url, description,
    status::TEXT::trip_status, enroll_token, created_at, updated_at
FROM cruises WHERE status IN ('planned', 'cancelled');

-- Backfill voyages from completed cruises.
INSERT INTO voyages (
    id, owner_id, org_id, name, year, embark_date, disembark_date, countries, start_port, end_port,
    captain_name, yacht_id,
    hours_total, hours_sail, hours_engine, hours_over_6bf, miles, days, tidal_waters,
    cost_total, cost_per_person,
    image_logo_url, image_photo_url, image_route_url, description,
    created_at, updated_at
) OVERRIDING SYSTEM VALUE
SELECT
    id, owner_id, org_id, name, year, embark_date, disembark_date, countries, start_port, end_port,
    captain_name, yacht_id,
    COALESCE(hours_total, 0), COALESCE(hours_sail, 0), COALESCE(hours_engine, 0),
    COALESCE(hours_over_6bf, 0), COALESCE(miles, 0), COALESCE(days, 0), COALESCE(tidal_waters, 0),
    COALESCE(cost_total, 0), COALESCE(cost_per_person, 0),
    image_logo_url, image_photo_url, image_route_url, description,
    created_at, updated_at
FROM cruises WHERE status = 'completed';

-- Restart identity sequences past the backfilled max id.
SELECT setval(pg_get_serial_sequence('trips', 'id'),
    GREATEST((SELECT COALESCE(MAX(id), 0) FROM trips), 1));
SELECT setval(pg_get_serial_sequence('voyages', 'id'),
    GREATEST((SELECT COALESCE(MAX(id), 0) FROM voyages), 1));

-- crew_assignments: split FK into trip_id/voyage_id (exactly one set).
ALTER TABLE crew_assignments ADD COLUMN trip_id BIGINT REFERENCES trips(id) ON DELETE CASCADE;
ALTER TABLE crew_assignments ADD COLUMN voyage_id BIGINT REFERENCES voyages(id) ON DELETE CASCADE;

UPDATE crew_assignments ca SET trip_id = ca.cruise_id
WHERE EXISTS (SELECT 1 FROM trips t WHERE t.id = ca.cruise_id);

UPDATE crew_assignments ca SET voyage_id = ca.cruise_id
WHERE EXISTS (SELECT 1 FROM voyages v WHERE v.id = ca.cruise_id);

-- Drop any orphan rows (defensive; cascade from cruises should have prevented this).
DELETE FROM crew_assignments WHERE trip_id IS NULL AND voyage_id IS NULL;

ALTER TABLE crew_assignments DROP CONSTRAINT crew_assignments_cruise_id_crew_member_id_key;
ALTER TABLE crew_assignments DROP COLUMN cruise_id;
ALTER TABLE crew_assignments ADD CONSTRAINT crew_assignments_one_parent
    CHECK ((trip_id IS NULL) <> (voyage_id IS NULL));
CREATE UNIQUE INDEX crew_assignments_trip_member_uniq
    ON crew_assignments(trip_id, crew_member_id) WHERE trip_id IS NOT NULL;
CREATE UNIQUE INDEX crew_assignments_voyage_member_uniq
    ON crew_assignments(voyage_id, crew_member_id) WHERE voyage_id IS NOT NULL;

-- cruise_enrollments -> trip_enrollments (only for surviving trips; drop the rest).
DELETE FROM cruise_enrollments ce
WHERE NOT EXISTS (SELECT 1 FROM trips t WHERE t.id = ce.cruise_id);

ALTER TABLE cruise_enrollments RENAME TO trip_enrollments;
ALTER TABLE trip_enrollments RENAME COLUMN cruise_id TO trip_id;
ALTER TABLE trip_enrollments DROP CONSTRAINT cruise_enrollments_cruise_id_fkey;
ALTER TABLE trip_enrollments ADD CONSTRAINT trip_enrollments_trip_id_fkey
    FOREIGN KEY (trip_id) REFERENCES trips(id) ON DELETE CASCADE;
ALTER TABLE trip_enrollments RENAME CONSTRAINT cruise_enrollments_cruise_id_user_id_key
    TO trip_enrollments_trip_id_user_id_key;

-- voyage_opinions: rewire FK to voyages.
DELETE FROM voyage_opinions vo
WHERE NOT EXISTS (SELECT 1 FROM voyages v WHERE v.id = vo.cruise_id);

ALTER TABLE voyage_opinions RENAME COLUMN cruise_id TO voyage_id;
ALTER TABLE voyage_opinions DROP CONSTRAINT voyage_opinions_cruise_id_fkey;
ALTER TABLE voyage_opinions ADD CONSTRAINT voyage_opinions_voyage_id_fkey
    FOREIGN KEY (voyage_id) REFERENCES voyages(id) ON DELETE CASCADE;
ALTER TABLE voyage_opinions RENAME CONSTRAINT uq_voyage_opinion_cruise_crew
    TO uq_voyage_opinion_voyage_crew;

-- Drop the old cruises table and its enum.
DROP TABLE cruises;
DROP TYPE cruise_status;
