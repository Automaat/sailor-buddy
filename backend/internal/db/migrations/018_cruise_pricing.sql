-- Club cruises are priced in two tiers and two parts: the berth price for
-- members (czlonek, sympatyk) and non-members (gosc), plus an estimated
-- yacht fund (kasa jachtowa: food, fuel, marinas) and a refundable deposit
-- (kaucja). cost_per_person stays and is the member price; existing values
-- are PLN, the column default. Prices are informational only.

ALTER TABLE cruises
    ADD COLUMN currency TEXT NOT NULL DEFAULT 'PLN'
        CHECK (currency IN ('PLN', 'EUR')),
    ADD COLUMN cost_per_person_non_member DOUBLE PRECISION
        CHECK (cost_per_person_non_member >= 0),
    ADD COLUMN yacht_fund DOUBLE PRECISION CHECK (yacht_fund >= 0),
    ADD COLUMN deposit DOUBLE PRECISION CHECK (deposit >= 0),
    ADD COLUMN included JSONB NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(included) = 'array'),
    ADD COLUMN not_included JSONB NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(not_included) = 'array');

ALTER TABLE trips
    ADD COLUMN currency TEXT NOT NULL DEFAULT 'PLN'
        CHECK (currency IN ('PLN', 'EUR')),
    ADD COLUMN cost_per_person_non_member DOUBLE PRECISION
        CHECK (cost_per_person_non_member >= 0),
    ADD COLUMN yacht_fund DOUBLE PRECISION CHECK (yacht_fund >= 0),
    ADD COLUMN deposit DOUBLE PRECISION CHECK (deposit >= 0);
