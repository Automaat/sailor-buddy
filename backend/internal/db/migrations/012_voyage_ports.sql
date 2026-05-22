-- Ports visited during a voyage, captured when a trip is completed.

CREATE TABLE voyage_ports (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    voyage_id  BIGINT NOT NULL REFERENCES voyages(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    latitude   DOUBLE PRECISION NOT NULL,
    longitude  DOUBLE PRECISION NOT NULL,
    position   BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX voyage_ports_voyage_id_idx ON voyage_ports(voyage_id);
