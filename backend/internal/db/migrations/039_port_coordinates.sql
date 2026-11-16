-- Start and end ports of trips and voyages carry optional coordinates, as
-- visited ports do: both or neither, and only on a named port.

ALTER TABLE trips
    ADD COLUMN start_port_latitude DOUBLE PRECISION,
    ADD COLUMN start_port_longitude DOUBLE PRECISION,
    ADD COLUMN end_port_latitude DOUBLE PRECISION,
    ADD COLUMN end_port_longitude DOUBLE PRECISION,
    ADD CONSTRAINT trips_start_port_coords_paired
        CHECK ((start_port_latitude IS NULL) = (start_port_longitude IS NULL)),
    ADD CONSTRAINT trips_end_port_coords_paired
        CHECK ((end_port_latitude IS NULL) = (end_port_longitude IS NULL)),
    ADD CONSTRAINT trips_start_port_coords_named
        CHECK (start_port_latitude IS NULL OR start_port IS NOT NULL),
    ADD CONSTRAINT trips_end_port_coords_named
        CHECK (end_port_latitude IS NULL OR end_port IS NOT NULL);

ALTER TABLE voyages
    ADD COLUMN start_port_latitude DOUBLE PRECISION,
    ADD COLUMN start_port_longitude DOUBLE PRECISION,
    ADD COLUMN end_port_latitude DOUBLE PRECISION,
    ADD COLUMN end_port_longitude DOUBLE PRECISION,
    ADD CONSTRAINT voyages_start_port_coords_paired
        CHECK ((start_port_latitude IS NULL) = (start_port_longitude IS NULL)),
    ADD CONSTRAINT voyages_end_port_coords_paired
        CHECK ((end_port_latitude IS NULL) = (end_port_longitude IS NULL)),
    ADD CONSTRAINT voyages_start_port_coords_named
        CHECK (start_port_latitude IS NULL OR start_port IS NOT NULL),
    ADD CONSTRAINT voyages_end_port_coords_named
        CHECK (end_port_latitude IS NULL OR end_port IS NOT NULL);
