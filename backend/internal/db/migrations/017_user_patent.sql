ALTER TABLE users ADD COLUMN patent_type TEXT
  CHECK (patent_type IS NULL OR patent_type IN
    ('zeglarz_jachtowy', 'jachtowy_sternik_morski', 'kapitan_jachtowy'));
ALTER TABLE users ADD COLUMN patent_number TEXT;
