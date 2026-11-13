-- Onboarding: structured name and contact phone collected once after the
-- first sign-in. onboarded_at NULL sends the user through the onboarding form;
-- existing accounts go through it once with their name pre-split below.
-- Self-registered accounts start as guests until an admin sets membership.

ALTER TABLE users
    ADD COLUMN first_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN last_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN phone TEXT,
    ADD COLUMN onboarded_at TIMESTAMPTZ,
    ALTER COLUMN membership SET DEFAULT 'gosc';

UPDATE users SET
    first_name = split_part(btrim(name), ' ', 1),
    last_name = btrim(substr(btrim(name), length(split_part(btrim(name), ' ', 1)) + 1));
