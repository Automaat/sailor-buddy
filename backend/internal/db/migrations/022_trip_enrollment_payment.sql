-- Manual payment status of a leg sign-up. The club confirms a place after
-- the first payment and an admin checks transfers by hand: unpaid (default),
-- deposit (first payment received) or paid (paid in full). Independent of
-- the review status; no amounts are stored.

ALTER TABLE trip_enrollments
    ADD COLUMN payment_status TEXT NOT NULL DEFAULT 'unpaid'
        CONSTRAINT trip_enrollments_payment_status_check
        CHECK (payment_status IN ('unpaid', 'deposit', 'paid'));
