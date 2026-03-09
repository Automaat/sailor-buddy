ALTER TABLE yachts ADD COLUMN org_id BIGINT REFERENCES organizations(id) ON DELETE CASCADE;
ALTER TABLE cruises ADD COLUMN org_id BIGINT REFERENCES organizations(id) ON DELETE CASCADE;
ALTER TABLE crew_members ADD COLUMN org_id BIGINT REFERENCES organizations(id) ON DELETE CASCADE;

CREATE INDEX idx_yachts_org_id ON yachts(org_id) WHERE org_id IS NOT NULL;
CREATE INDEX idx_cruises_org_id ON cruises(org_id) WHERE org_id IS NOT NULL;
CREATE INDEX idx_crew_members_org_id ON crew_members(org_id) WHERE org_id IS NOT NULL;
