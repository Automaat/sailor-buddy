-- Most club cruises are yearly editions of a recurring series (for example
-- AGH Winter Sail Expedition or May I Sail). A cruise may belong to one
-- series and carries a free-form edition label such as "2025" or "2026/27".
-- Deleting a series keeps its cruises and clears their link.

CREATE TABLE cruise_series (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE cruises
    ADD COLUMN series_id BIGINT REFERENCES cruise_series(id) ON DELETE SET NULL,
    ADD COLUMN edition TEXT;

CREATE INDEX idx_cruises_series_id ON cruises(series_id) WHERE series_id IS NOT NULL;
