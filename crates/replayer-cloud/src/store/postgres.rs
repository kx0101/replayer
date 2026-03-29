use anyhow::{anyhow, Result};
use sqlx::{PgPool, Row};
use uuid::Uuid;

use super::{ListFilter, Store};
use crate::models::{APIKey, Run, RunListItem, User};

const MIGRATION_UP: &str = r#"
CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT UNIQUE NOT NULL,
    password_hash   TEXT NOT NULL,
    verified_at     TIMESTAMPTZ,
    verify_token    TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);

CREATE TABLE IF NOT EXISTS runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID REFERENCES users(id) ON DELETE CASCADE,
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

CREATE INDEX IF NOT EXISTS idx_runs_environment ON runs (environment);
CREATE INDEX IF NOT EXISTS idx_runs_created_at  ON runs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_runs_user_id ON runs (user_id);

CREATE TABLE IF NOT EXISTS api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash        TEXT NOT NULL,
    key_prefix      TEXT NOT NULL,
    name            TEXT NOT NULL DEFAULT 'Default',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at    TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys (user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys (key_hash);
"#;

pub struct PostgresStore {
    pool: PgPool,
}

impl PostgresStore {
    pub fn new(pool: PgPool) -> Self {
        PostgresStore { pool }
    }

    pub async fn migrate(&self) -> Result<()> {
        sqlx::raw_sql(MIGRATION_UP).execute(&self.pool).await?;
        Ok(())
    }
}

fn scan_run(row: &sqlx::postgres::PgRow) -> Result<Run> {
    let latency_json: serde_json::Value = row.try_get("latency_stats")?;
    let by_target_json: serde_json::Value = row.try_get("by_target")?;
    let results_json: serde_json::Value = row.try_get("results")?;
    let labels_json: serde_json::Value = row.try_get("labels")?;

    Ok(Run {
        id: row.try_get("id")?,
        user_id: row.try_get("user_id").ok(),
        environment: row.try_get("environment")?,
        targets: row.try_get("targets")?,
        created_at: row.try_get("created_at")?,
        total_requests: row.try_get("total_requests")?,
        succeeded: row.try_get("succeeded")?,
        failed: row.try_get("failed")?,
        latency_stats: serde_json::from_value(latency_json)?,
        by_target: serde_json::from_value(by_target_json)?,
        results: serde_json::from_value(results_json)?,
        is_baseline: row.try_get("is_baseline")?,
        baseline_id: row.try_get("baseline_id").ok().flatten(),
        labels: serde_json::from_value(labels_json).ok(),
    })
}

fn scan_run_list_item(row: &sqlx::postgres::PgRow) -> Result<RunListItem> {
    let latency_json: serde_json::Value = row.try_get("latency_stats")?;
    let by_target_json: serde_json::Value = row.try_get("by_target")?;
    let labels_json: serde_json::Value = row.try_get("labels")?;

    Ok(RunListItem {
        id: row.try_get("id")?,
        user_id: row.try_get("user_id").ok(),
        environment: row.try_get("environment")?,
        targets: row.try_get("targets")?,
        created_at: row.try_get("created_at")?,
        total_requests: row.try_get("total_requests")?,
        succeeded: row.try_get("succeeded")?,
        failed: row.try_get("failed")?,
        latency_stats: serde_json::from_value(latency_json)?,
        by_target: serde_json::from_value(by_target_json)?,
        is_baseline: row.try_get("is_baseline")?,
        baseline_id: row.try_get("baseline_id").ok().flatten(),
        labels: serde_json::from_value(labels_json).ok(),
    })
}

fn scan_user(row: &sqlx::postgres::PgRow) -> Result<User> {
    Ok(User {
        id: row.try_get("id")?,
        email: row.try_get("email")?,
        password_hash: row.try_get("password_hash")?,
        verified_at: row.try_get("verified_at").ok().flatten(),
        verify_token: row.try_get("verify_token").ok().flatten(),
        created_at: row.try_get("created_at")?,
        updated_at: row.try_get("updated_at")?,
    })
}

fn scan_api_key(row: &sqlx::postgres::PgRow) -> Result<APIKey> {
    Ok(APIKey {
        id: row.try_get("id")?,
        user_id: row.try_get("user_id")?,
        key_hash: row.try_get("key_hash")?,
        key_prefix: row.try_get("key_prefix")?,
        name: row.try_get("name")?,
        created_at: row.try_get("created_at")?,
        last_used_at: row.try_get("last_used_at").ok().flatten(),
        expires_at: row.try_get("expires_at").ok().flatten(),
    })
}

#[async_trait::async_trait]
impl Store for PostgresStore {
    async fn create_run(&self, run: &mut Run) -> Result<()> {
        let latency_json = serde_json::to_value(&run.latency_stats)?;
        let by_target_json = serde_json::to_value(&run.by_target)?;
        let results_json = serde_json::to_value(&run.results)?;
        let labels_json = serde_json::to_value(&run.labels)?;

        let row = sqlx::query(
            "INSERT INTO runs (environment, targets, total_requests, succeeded, failed, latency_stats, by_target, results, labels)
             VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
             RETURNING id, created_at",
        )
        .bind(&run.environment)
        .bind(&run.targets)
        .bind(run.total_requests)
        .bind(run.succeeded)
        .bind(run.failed)
        .bind(&latency_json)
        .bind(&by_target_json)
        .bind(&results_json)
        .bind(&labels_json)
        .fetch_one(&self.pool)
        .await?;

        run.id = row.try_get("id")?;
        run.created_at = row.try_get("created_at")?;
        Ok(())
    }

    async fn get_run(&self, id: Uuid) -> Result<Option<Run>> {
        let row = sqlx::query(
            "SELECT id, user_id, environment, targets, created_at, total_requests, succeeded, failed,
                    latency_stats, by_target, results, is_baseline, baseline_id, labels
             FROM runs WHERE id = $1",
        )
        .bind(id)
        .fetch_optional(&self.pool)
        .await?;

        match row {
            Some(r) => Ok(Some(scan_run(&r)?)),
            None => Ok(None),
        }
    }

    async fn list_runs(&self, mut filter: ListFilter) -> Result<(Vec<RunListItem>, i64)> {
        filter.normalize();

        let count_row = if let Some(ref env) = filter.environment {
            sqlx::query_scalar::<_, i64>("SELECT COUNT(*) FROM runs WHERE environment = $1")
                .bind(env)
                .fetch_one(&self.pool)
                .await?
        } else {
            sqlx::query_scalar::<_, i64>("SELECT COUNT(*) FROM runs")
                .fetch_one(&self.pool)
                .await?
        };

        let rows = if let Some(ref env) = filter.environment {
            sqlx::query(
                "SELECT id, user_id, environment, targets, created_at, total_requests, succeeded, failed,
                        latency_stats, by_target, is_baseline, baseline_id, labels
                 FROM runs WHERE environment = $1
                 ORDER BY created_at DESC LIMIT $2 OFFSET $3",
            )
            .bind(env)
            .bind(filter.limit)
            .bind(filter.offset)
            .fetch_all(&self.pool)
            .await?
        } else {
            sqlx::query(
                "SELECT id, user_id, environment, targets, created_at, total_requests, succeeded, failed,
                        latency_stats, by_target, is_baseline, baseline_id, labels
                 FROM runs
                 ORDER BY created_at DESC LIMIT $1 OFFSET $2",
            )
            .bind(filter.limit)
            .bind(filter.offset)
            .fetch_all(&self.pool)
            .await?
        };

        let items: Result<Vec<_>> = rows.iter().map(scan_run_list_item).collect();
        Ok((items?, count_row))
    }

    async fn set_baseline(&self, id: Uuid) -> Result<()> {
        let mut tx = self.pool.begin().await?;

        let env: String = sqlx::query_scalar("SELECT environment FROM runs WHERE id = $1")
            .bind(id)
            .fetch_optional(&mut *tx)
            .await?
            .ok_or_else(|| anyhow!("run not found"))?;

        sqlx::query(
            "UPDATE runs SET is_baseline = FALSE WHERE environment = $1 AND is_baseline = TRUE",
        )
        .bind(&env)
        .execute(&mut *tx)
        .await?;

        sqlx::query("UPDATE runs SET is_baseline = TRUE WHERE id = $1")
            .bind(id)
            .execute(&mut *tx)
            .await?;

        tx.commit().await?;
        Ok(())
    }

    async fn get_baseline(&self, environment: &str) -> Result<Option<Run>> {
        let row = sqlx::query(
            "SELECT id, user_id, environment, targets, created_at, total_requests, succeeded, failed,
                    latency_stats, by_target, results, is_baseline, baseline_id, labels
             FROM runs WHERE environment = $1 AND is_baseline = TRUE LIMIT 1",
        )
        .bind(environment)
        .fetch_optional(&self.pool)
        .await?;

        match row {
            Some(r) => Ok(Some(scan_run(&r)?)),
            None => Ok(None),
        }
    }

    async fn create_run_for_user(&self, user_id: Uuid, run: &mut Run) -> Result<()> {
        let latency_json = serde_json::to_value(&run.latency_stats)?;
        let by_target_json = serde_json::to_value(&run.by_target)?;
        let results_json = serde_json::to_value(&run.results)?;
        let labels_json = serde_json::to_value(&run.labels)?;

        run.user_id = Some(user_id);

        let row = sqlx::query(
            "INSERT INTO runs (user_id, environment, targets, total_requests, succeeded, failed, latency_stats, by_target, results, labels)
             VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
             RETURNING id, created_at",
        )
        .bind(user_id)
        .bind(&run.environment)
        .bind(&run.targets)
        .bind(run.total_requests)
        .bind(run.succeeded)
        .bind(run.failed)
        .bind(&latency_json)
        .bind(&by_target_json)
        .bind(&results_json)
        .bind(&labels_json)
        .fetch_one(&self.pool)
        .await?;

        run.id = row.try_get("id")?;
        run.created_at = row.try_get("created_at")?;
        Ok(())
    }

    async fn get_run_for_user(&self, user_id: Uuid, run_id: Uuid) -> Result<Option<Run>> {
        let row = sqlx::query(
            "SELECT id, user_id, environment, targets, created_at, total_requests, succeeded, failed,
                    latency_stats, by_target, results, is_baseline, baseline_id, labels
             FROM runs WHERE id = $1 AND user_id = $2",
        )
        .bind(run_id)
        .bind(user_id)
        .fetch_optional(&self.pool)
        .await?;

        match row {
            Some(r) => Ok(Some(scan_run(&r)?)),
            None => Ok(None),
        }
    }

    async fn list_runs_for_user(
        &self,
        user_id: Uuid,
        mut filter: ListFilter,
    ) -> Result<(Vec<RunListItem>, i64)> {
        filter.normalize();

        let count_row = if let Some(ref env) = filter.environment {
            sqlx::query_scalar::<_, i64>(
                "SELECT COUNT(*) FROM runs WHERE user_id = $1 AND environment = $2",
            )
            .bind(user_id)
            .bind(env)
            .fetch_one(&self.pool)
            .await?
        } else {
            sqlx::query_scalar::<_, i64>("SELECT COUNT(*) FROM runs WHERE user_id = $1")
                .bind(user_id)
                .fetch_one(&self.pool)
                .await?
        };

        let rows = if let Some(ref env) = filter.environment {
            sqlx::query(
                "SELECT id, user_id, environment, targets, created_at, total_requests, succeeded, failed,
                        latency_stats, by_target, is_baseline, baseline_id, labels
                 FROM runs WHERE user_id = $1 AND environment = $2
                 ORDER BY created_at DESC LIMIT $3 OFFSET $4",
            )
            .bind(user_id)
            .bind(env)
            .bind(filter.limit)
            .bind(filter.offset)
            .fetch_all(&self.pool)
            .await?
        } else {
            sqlx::query(
                "SELECT id, user_id, environment, targets, created_at, total_requests, succeeded, failed,
                        latency_stats, by_target, is_baseline, baseline_id, labels
                 FROM runs WHERE user_id = $1
                 ORDER BY created_at DESC LIMIT $2 OFFSET $3",
            )
            .bind(user_id)
            .bind(filter.limit)
            .bind(filter.offset)
            .fetch_all(&self.pool)
            .await?
        };

        let items: Result<Vec<_>> = rows.iter().map(scan_run_list_item).collect();
        Ok((items?, count_row))
    }

    async fn set_baseline_for_user(&self, user_id: Uuid, run_id: Uuid) -> Result<()> {
        let mut tx = self.pool.begin().await?;

        let env: String =
            sqlx::query_scalar("SELECT environment FROM runs WHERE id = $1 AND user_id = $2")
                .bind(run_id)
                .bind(user_id)
                .fetch_optional(&mut *tx)
                .await?
                .ok_or_else(|| anyhow!("run not found"))?;

        sqlx::query(
            "UPDATE runs SET is_baseline = FALSE WHERE user_id = $1 AND environment = $2 AND is_baseline = TRUE",
        )
        .bind(user_id)
        .bind(&env)
        .execute(&mut *tx)
        .await?;

        sqlx::query("UPDATE runs SET is_baseline = TRUE WHERE id = $1 AND user_id = $2")
            .bind(run_id)
            .bind(user_id)
            .execute(&mut *tx)
            .await?;

        tx.commit().await?;
        Ok(())
    }

    async fn get_baseline_for_user(&self, user_id: Uuid, env: &str) -> Result<Option<Run>> {
        let row = sqlx::query(
            "SELECT id, user_id, environment, targets, created_at, total_requests, succeeded, failed,
                    latency_stats, by_target, results, is_baseline, baseline_id, labels
             FROM runs WHERE user_id = $1 AND environment = $2 AND is_baseline = TRUE LIMIT 1",
        )
        .bind(user_id)
        .bind(env)
        .fetch_optional(&self.pool)
        .await?;

        match row {
            Some(r) => Ok(Some(scan_run(&r)?)),
            None => Ok(None),
        }
    }

    async fn create_user(&self, user: &mut User) -> Result<()> {
        let row = sqlx::query(
            "INSERT INTO users (email, password_hash, verify_token)
             VALUES ($1, $2, $3)
             RETURNING id, created_at, updated_at",
        )
        .bind(&user.email)
        .bind(&user.password_hash)
        .bind(&user.verify_token)
        .fetch_one(&self.pool)
        .await?;

        user.id = row.try_get("id")?;
        user.created_at = row.try_get("created_at")?;
        user.updated_at = row.try_get("updated_at")?;
        Ok(())
    }

    async fn get_user_by_email(&self, email: &str) -> Result<Option<User>> {
        let row = sqlx::query(
            "SELECT id, email, password_hash, verified_at, verify_token, created_at, updated_at
             FROM users WHERE email = $1",
        )
        .bind(email)
        .fetch_optional(&self.pool)
        .await?;

        match row {
            Some(r) => Ok(Some(scan_user(&r)?)),
            None => Ok(None),
        }
    }

    async fn get_user_by_id(&self, id: Uuid) -> Result<Option<User>> {
        let row = sqlx::query(
            "SELECT id, email, password_hash, verified_at, verify_token, created_at, updated_at
             FROM users WHERE id = $1",
        )
        .bind(id)
        .fetch_optional(&self.pool)
        .await?;

        match row {
            Some(r) => Ok(Some(scan_user(&r)?)),
            None => Ok(None),
        }
    }

    async fn get_user_by_verify_token(&self, token: &str) -> Result<Option<User>> {
        let row = sqlx::query(
            "SELECT id, email, password_hash, verified_at, verify_token, created_at, updated_at
             FROM users WHERE verify_token = $1",
        )
        .bind(token)
        .fetch_optional(&self.pool)
        .await?;

        match row {
            Some(r) => Ok(Some(scan_user(&r)?)),
            None => Ok(None),
        }
    }

    async fn verify_user(&self, user_id: Uuid) -> Result<()> {
        sqlx::query(
            "UPDATE users SET verified_at = now(), verify_token = NULL, updated_at = now() WHERE id = $1",
        )
        .bind(user_id)
        .execute(&self.pool)
        .await?;
        Ok(())
    }

    async fn create_api_key(&self, key: &mut APIKey) -> Result<()> {
        let row = sqlx::query(
            "INSERT INTO api_keys (user_id, key_hash, key_prefix, name)
             VALUES ($1, $2, $3, $4)
             RETURNING id, created_at",
        )
        .bind(key.user_id)
        .bind(&key.key_hash)
        .bind(&key.key_prefix)
        .bind(&key.name)
        .fetch_one(&self.pool)
        .await?;

        key.id = row.try_get("id")?;
        key.created_at = row.try_get("created_at")?;
        Ok(())
    }

    async fn get_api_key_by_hash(&self, hash: &str) -> Result<Option<APIKey>> {
        let row = sqlx::query(
            "SELECT id, user_id, key_hash, key_prefix, name, created_at, last_used_at, expires_at
             FROM api_keys WHERE key_hash = $1",
        )
        .bind(hash)
        .fetch_optional(&self.pool)
        .await?;

        match row {
            Some(r) => Ok(Some(scan_api_key(&r)?)),
            None => Ok(None),
        }
    }

    async fn list_api_keys_for_user(&self, user_id: Uuid) -> Result<Vec<APIKey>> {
        let rows = sqlx::query(
            "SELECT id, user_id, key_hash, key_prefix, name, created_at, last_used_at, expires_at
             FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC",
        )
        .bind(user_id)
        .fetch_all(&self.pool)
        .await?;

        let keys: Result<Vec<_>> = rows.iter().map(scan_api_key).collect();
        keys
    }

    async fn delete_api_key(&self, user_id: Uuid, key_id: Uuid) -> Result<()> {
        let result = sqlx::query("DELETE FROM api_keys WHERE id = $1 AND user_id = $2")
            .bind(key_id)
            .bind(user_id)
            .execute(&self.pool)
            .await?;

        if result.rows_affected() == 0 {
            return Err(anyhow!("api key not found"));
        }
        Ok(())
    }

    async fn update_api_key_last_used(&self, key_id: Uuid) -> Result<()> {
        sqlx::query("UPDATE api_keys SET last_used_at = now() WHERE id = $1")
            .bind(key_id)
            .execute(&self.pool)
            .await?;
        Ok(())
    }
}
