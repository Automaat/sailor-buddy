ALTER TABLE voyage_opinions ADD CONSTRAINT uq_voyage_opinion_cruise_crew
UNIQUE (cruise_id, crew_member_id);
