CREATE TYPE cruise_status AS ENUM ('planned', 'completed', 'cancelled');
ALTER TABLE cruises ADD COLUMN status cruise_status NOT NULL DEFAULT 'completed';
