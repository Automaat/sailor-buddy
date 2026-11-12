-- Logistics shown on club cruise pages: a link to the crew group chat
-- (Discord, WhatsApp, ...), free-form travel notes (cheap flights,
-- recommended transport) and a minimum crew age. The minimum age is
-- information only: the app stores no dates of birth and checks nothing.

ALTER TABLE cruises
    ADD COLUMN group_chat_url TEXT,
    ADD COLUMN travel_notes TEXT,
    ADD COLUMN min_age BIGINT CHECK (min_age BETWEEN 0 AND 99);
