-- Yacht specs shown on club cruise pages. All optional; berths feeds a new
-- trip's default crew limit.

ALTER TABLE yachts
    ADD COLUMN berths BIGINT CHECK (berths >= 1),
    ADD COLUMN length_m DOUBLE PRECISION CHECK (length_m > 0),
    ADD COLUMN build_year BIGINT CHECK (build_year BETWEEN 1800 AND 2100),
    ADD COLUMN engine TEXT,
    ADD COLUMN owner_name TEXT;
