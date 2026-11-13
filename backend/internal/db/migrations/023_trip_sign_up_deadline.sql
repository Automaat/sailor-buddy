-- Sign-up closing date per leg: the last calendar day (YYYY-MM-DD, Europe/
-- Warsaw) on which members can sign up. NULL keeps sign-ups open until an
-- admin switches them off. Stored as TEXT like embark_date.

ALTER TABLE trips ADD COLUMN sign_up_deadline TEXT;
