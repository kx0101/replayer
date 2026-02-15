DROP INDEX IF EXISTS idx_runs_baseline;
CREATE INDEX idx_runs_baseline ON runs (environment, is_baseline) WHERE is_baseline = TRUE;

DROP INDEX IF EXISTS idx_runs_user_id;
ALTER TABLE runs DROP COLUMN IF EXISTS user_id;
