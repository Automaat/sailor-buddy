-- Trips are planned/cancelled sailing events. Completing a trip produces a voyage.

CREATE TYPE trip_status AS ENUM ('planned', 'cancelled');

CREATE TABLE trips (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    cruise_id BIGINT REFERENCES cruises(id) ON DELETE SET NULL,
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

CREATE INDEX idx_trips_cruise_id ON trips(cruise_id) WHERE cruise_id IS NOT NULL;
