-- Every trip and voyage belongs to a club event (cruise). Standalone rows are
-- wrapped in a one-leg cruise that copies their name, dates, route,
-- description, images and prices, then cruise_id becomes required. Deleting a cruise that still
-- has legs is refused instead of orphaning or cascading logged voyages.

DO $$
DECLARE
    r RECORD;
    new_id BIGINT;
BEGIN
    FOR r IN SELECT * FROM trips WHERE cruise_id IS NULL LOOP
        INSERT INTO cruises (
            created_by, name, embark_date, disembark_date, countries, start_port, end_port,
            description, image_logo_url, image_photo_url, image_route_url, max_crew,
            cost_per_person, cost_per_person_non_member, yacht_fund, deposit, currency
        ) VALUES (
            r.created_by, r.name, r.embark_date, r.disembark_date, r.countries, r.start_port, r.end_port,
            r.description, r.image_logo_url, r.image_photo_url, r.image_route_url, r.max_crew,
            r.cost_per_person, r.cost_per_person_non_member, r.yacht_fund, r.deposit, r.currency
        ) RETURNING id INTO new_id;
        UPDATE trips SET cruise_id = new_id WHERE id = r.id;
    END LOOP;

    FOR r IN SELECT * FROM voyages WHERE cruise_id IS NULL LOOP
        INSERT INTO cruises (
            created_by, name, embark_date, disembark_date, countries, start_port, end_port,
            description, image_logo_url, image_photo_url, image_route_url, cost_per_person
        ) VALUES (
            r.created_by, r.name, r.embark_date, r.disembark_date, r.countries, r.start_port, r.end_port,
            r.description, r.image_logo_url, r.image_photo_url, r.image_route_url, r.cost_per_person
        ) RETURNING id INTO new_id;
        UPDATE voyages SET cruise_id = new_id WHERE id = r.id;
    END LOOP;
END $$;

ALTER TABLE trips
    DROP CONSTRAINT trips_cruise_id_fkey,
    ALTER COLUMN cruise_id SET NOT NULL,
    ADD CONSTRAINT trips_cruise_id_fkey FOREIGN KEY (cruise_id) REFERENCES cruises(id);

ALTER TABLE voyages
    DROP CONSTRAINT voyages_cruise_id_fkey,
    ALTER COLUMN cruise_id SET NOT NULL,
    ADD CONSTRAINT voyages_cruise_id_fkey FOREIGN KEY (cruise_id) REFERENCES cruises(id);
