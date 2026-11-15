-- A cruise is an edition (edycja) of a series: it holds what is common to
-- all of its legs (etapy). Dates, ports, crew limit and prices belong to
-- each leg, so the cruise copies of them go; an edition's dates and route
-- come from its legs. The crew group chat is dropped altogether. The kind
-- (sea or inland) belongs to the whole series; a cruise without a series
-- counts as a sea cruise, as standalone trips always did.

ALTER TABLE cruise_series
    ADD COLUMN kind TEXT NOT NULL DEFAULT 'sea'
        CHECK (kind IN ('sea', 'inland'));

UPDATE cruise_series s SET kind = 'inland'
WHERE EXISTS (SELECT 1 FROM cruises c WHERE c.series_id = s.id AND c.kind = 'inland');

ALTER TABLE cruises
    DROP COLUMN kind,
    DROP COLUMN embark_date,
    DROP COLUMN disembark_date,
    DROP COLUMN start_port,
    DROP COLUMN end_port,
    DROP COLUMN max_crew,
    DROP COLUMN cost_per_person,
    DROP COLUMN cost_per_person_non_member,
    DROP COLUMN currency,
    DROP COLUMN yacht_fund,
    DROP COLUMN deposit,
    DROP COLUMN group_chat_url;
