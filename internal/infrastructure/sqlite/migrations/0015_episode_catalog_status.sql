ALTER TABLE episodes ADD COLUMN active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1));
ALTER TABLE episodes ADD COLUMN original_episode_number INTEGER;
