CREATE TABLE runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    environment     TEXT        NOT NULL,
    targets         TEXT[]      NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    total_requests  INT         NOT NULL,
    succeeded       INT         NOT NULL,
    failed          INT         NOT NULL,
    latency_stats   JSONB       NOT NULL,
    by_target       JSONB       NOT NULL,
    results         JSONB       NOT NULL,
    is_baseline     BOOLEAN     NOT NULL DEFAULT FALSE,
    baseline_id     UUID        REFERENCES runs(id),
    labels          JSONB       DEFAULT '{}'
);

CREATE INDEX idx_runs_environment ON runs (environment);
CREATE INDEX idx_runs_created_at  ON runs (created_at DESC);
CREATE INDEX idx_runs_baseline    ON runs (environment, is_baseline) WHERE is_baseline = TRUE;
