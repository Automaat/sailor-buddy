-- A cruise is a multi-yacht club event container. Each cruise has many trips
-- (one per yacht); members enroll at the cruise level and an admin assigns each
-- accepted enrollment to a specific trip.

CREATE TABLE cruises (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
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
