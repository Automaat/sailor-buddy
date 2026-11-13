-- A crew entry added by accepting a leg sign-up. Taking the sign-up out of
-- accepted removes only these entries; crew added by hand stays.

ALTER TABLE crew_assignments
    ADD COLUMN added_by_acceptance BOOLEAN NOT NULL DEFAULT FALSE;
