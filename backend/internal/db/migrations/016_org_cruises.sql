-- Org-level cruises: a multi-yacht event container. Each cruise has many trips
-- (one per yacht); trips can be enrolled into at the cruise level and admin
-- assigns each accepted enrollment to a specific trip.

CREATE TABLE cruises (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    org_id BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    embark_date TEXT,
    disembark_date TEXT,
    countries TEXT,
    start_port TEXT,
    end_port TEXT,
    description TEXT,
    image_logo_url TEXT,
    image_photo_url TEXT,
    image_route_url TEXT,
    max_crew BIGINT,
    cost_per_person DOUBLE PRECISION,
    enroll_token TEXT UNIQUE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_cruises_org_id ON cruises(org_id);

ALTER TABLE trips ADD COLUMN cruise_id BIGINT REFERENCES cruises(id) ON DELETE SET NULL;
ALTER TABLE voyages ADD COLUMN cruise_id BIGINT REFERENCES cruises(id) ON DELETE SET NULL;
CREATE INDEX idx_trips_cruise_id ON trips(cruise_id) WHERE cruise_id IS NOT NULL;
CREATE INDEX idx_voyages_cruise_id ON voyages(cruise_id) WHERE cruise_id IS NOT NULL;

CREATE TABLE cruise_enrollments (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cruise_id BIGINT NOT NULL REFERENCES cruises(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    trip_id BIGINT REFERENCES trips(id) ON DELETE SET NULL,
    note TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(cruise_id, user_id)
);

CREATE INDEX idx_cruise_enrollments_trip_id ON cruise_enrollments(trip_id) WHERE trip_id IS NOT NULL;
