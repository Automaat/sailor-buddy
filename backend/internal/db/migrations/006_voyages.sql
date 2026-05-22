-- Voyages are completed sailing trips with logged sailing statistics.

CREATE TABLE voyages (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    cruise_id BIGINT REFERENCES cruises(id) ON DELETE SET NULL,
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

CREATE INDEX idx_voyages_cruise_id ON voyages(cruise_id) WHERE cruise_id IS NOT NULL;
