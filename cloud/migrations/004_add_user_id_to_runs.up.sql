ALTER TABLE runs ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;
CREATE INDEX idx_runs_user_id ON runs (user_id);

DROP INDEX IF EXISTS idx_runs_baseline;
CREATE INDEX idx_runs_baseline ON runs (user_id, environment, is_baseline) WHERE is_baseline = TRUE;
