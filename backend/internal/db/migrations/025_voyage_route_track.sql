-- Voyage records from the club sheet: route ports written by name only (no
-- coordinates), anchorages, a link to the track recording and the voyage
-- card note ("Karta rejsu": who holds it or where it is).

ALTER TABLE voyage_ports
    ALTER COLUMN latitude DROP NOT NULL,
    ALTER COLUMN longitude DROP NOT NULL,
    ADD COLUMN anchorage BOOLEAN NOT NULL DEFAULT FALSE,
    ADD CONSTRAINT voyage_ports_coords_paired
        CHECK ((latitude IS NULL) = (longitude IS NULL));

ALTER TABLE voyages
    ADD COLUMN track_url TEXT,
    ADD COLUMN voyage_card TEXT;
