package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kx0101/replayer-cloud/internal/models"
)

const migrationUp = `
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
`

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Migrate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, migrationUp)
	if err != nil {
		return fmt.Errorf("executing migration: %w", err)
	}
	return nil
}

func (s *PostgresStore) CreateRun(ctx context.Context, run *models.Run) error {
	latencyJSON, err := json.Marshal(run.LatencyStats)
	if err != nil {
		return fmt.Errorf("marshaling latency_stats: %w", err)
	}

	byTargetJSON, err := json.Marshal(run.ByTarget)
	if err != nil {
		return fmt.Errorf("marshaling by_target: %w", err)
	}

	resultsJSON, err := json.Marshal(run.Results)
	if err != nil {
		return fmt.Errorf("marshaling results: %w", err)
	}

	labelsJSON, err := json.Marshal(run.Labels)
	if err != nil {
		return fmt.Errorf("marshaling labels: %w", err)
	}

	query := `
		INSERT INTO runs (environment, targets, total_requests, succeeded, failed, latency_stats, by_target, results, labels)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`

	err = s.pool.QueryRow(ctx, query,
		run.Environment,
		run.Targets,
		run.TotalRequests,
		run.Succeeded,
		run.Failed,
		latencyJSON,
		byTargetJSON,
		resultsJSON,
		labelsJSON,
	).Scan(&run.ID, &run.CreatedAt)

	if err != nil {
		return fmt.Errorf("inserting run: %w", err)
	}

	return nil
}

func (s *PostgresStore) GetRun(ctx context.Context, id uuid.UUID) (*models.Run, error) {
	query := `
		SELECT id, environment, targets, created_at, total_requests, succeeded, failed,
		       latency_stats, by_target, results, is_baseline, baseline_id, labels
		FROM runs
		WHERE id = $1`

	var run models.Run
	var latencyJSON, byTargetJSON, resultsJSON, labelsJSON []byte
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&run.ID,
		&run.Environment,
		&run.Targets,
		&run.CreatedAt,
		&run.TotalRequests,
		&run.Succeeded,
		&run.Failed,
		&latencyJSON,
		&byTargetJSON,
		&resultsJSON,
		&run.IsBaseline,
		&run.BaselineID,
		&labelsJSON,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying run: %w", err)
	}

	if err := json.Unmarshal(latencyJSON, &run.LatencyStats); err != nil {
		return nil, fmt.Errorf("unmarshaling latency_stats: %w", err)
	}
	if err := json.Unmarshal(byTargetJSON, &run.ByTarget); err != nil {
		return nil, fmt.Errorf("unmarshaling by_target: %w", err)
	}
	if err := json.Unmarshal(resultsJSON, &run.Results); err != nil {
		return nil, fmt.Errorf("unmarshaling results: %w", err)
	}
	if err := json.Unmarshal(labelsJSON, &run.Labels); err != nil {
		return nil, fmt.Errorf("unmarshaling labels: %w", err)
	}

	return &run, nil
}

func (s *PostgresStore) ListRuns(ctx context.Context, filter ListFilter) ([]models.RunListItem, int, error) {
	filter.Normalize()

	where := "WHERE 1=1"
	args := []any{}
	argIdx := 1

	if filter.Environment != "" {
		where += fmt.Sprintf(" AND environment = $%d", argIdx)
		args = append(args, filter.Environment)
		argIdx++
	}
	if filter.After != nil {
		where += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *filter.After)
		argIdx++
	}
	if filter.Before != nil {
		where += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *filter.Before)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM runs " + where
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting runs: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, environment, targets, created_at, total_requests, succeeded, failed,
		       latency_stats, by_target, is_baseline, baseline_id, labels
		FROM runs %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying runs: %w", err)
	}
	defer rows.Close()

	var items []models.RunListItem
	for rows.Next() {
		var item models.RunListItem
		var latencyJSON, byTargetJSON, labelsJSON []byte
		err := rows.Scan(
			&item.ID,
			&item.Environment,
			&item.Targets,
			&item.CreatedAt,
			&item.TotalRequests,
			&item.Succeeded,
			&item.Failed,
			&latencyJSON,
			&byTargetJSON,
			&item.IsBaseline,
			&item.BaselineID,
			&labelsJSON,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scanning run: %w", err)
		}

		if err := json.Unmarshal(latencyJSON, &item.LatencyStats); err != nil {
			return nil, 0, fmt.Errorf("unmarshaling latency_stats: %w", err)
		}
		if err := json.Unmarshal(byTargetJSON, &item.ByTarget); err != nil {
			return nil, 0, fmt.Errorf("unmarshaling by_target: %w", err)
		}
		if err := json.Unmarshal(labelsJSON, &item.Labels); err != nil {
			return nil, 0, fmt.Errorf("unmarshaling labels: %w", err)
		}

		items = append(items, item)
	}

	return items, total, nil
}

func (s *PostgresStore) SetBaseline(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		err = tx.Rollback(ctx)
		if err != nil {
			fmt.Printf("Failed to rollback transaction: %v\n", err)
		}
	}()

	var env string
	err = tx.QueryRow(ctx, "SELECT environment FROM runs WHERE id = $1", id).Scan(&env)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("run not found")
		}
		return fmt.Errorf("querying run: %w", err)
	}

	_, err = tx.Exec(ctx, "UPDATE runs SET is_baseline = FALSE WHERE environment = $1 AND is_baseline = TRUE", env)
	if err != nil {
		return fmt.Errorf("clearing old baseline: %w", err)
	}

	_, err = tx.Exec(ctx, "UPDATE runs SET is_baseline = TRUE WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("setting new baseline: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) GetBaseline(ctx context.Context, environment string) (*models.Run, error) {
	query := `
		SELECT id, environment, targets, created_at, total_requests, succeeded, failed,
		       latency_stats, by_target, results, is_baseline, baseline_id, labels
		FROM runs
		WHERE environment = $1 AND is_baseline = TRUE
		LIMIT 1`

	var run models.Run
	var latencyJSON, byTargetJSON, resultsJSON, labelsJSON []byte
	err := s.pool.QueryRow(ctx, query, environment).Scan(
		&run.ID,
		&run.Environment,
		&run.Targets,
		&run.CreatedAt,
		&run.TotalRequests,
		&run.Succeeded,
		&run.Failed,
		&latencyJSON,
		&byTargetJSON,
		&resultsJSON,
		&run.IsBaseline,
		&run.BaselineID,
		&labelsJSON,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying baseline: %w", err)
	}

	if err := json.Unmarshal(latencyJSON, &run.LatencyStats); err != nil {
		return nil, fmt.Errorf("unmarshaling latency_stats: %w", err)
	}
	if err := json.Unmarshal(byTargetJSON, &run.ByTarget); err != nil {
		return nil, fmt.Errorf("unmarshaling by_target: %w", err)
	}
	if err := json.Unmarshal(resultsJSON, &run.Results); err != nil {
		return nil, fmt.Errorf("unmarshaling results: %w", err)
	}
	if err := json.Unmarshal(labelsJSON, &run.Labels); err != nil {
		return nil, fmt.Errorf("unmarshaling labels: %w", err)
	}

	return &run, nil
}

// User-scoped run methods

func (s *PostgresStore) CreateRunForUser(ctx context.Context, userID uuid.UUID, run *models.Run) error {
	latencyJSON, err := json.Marshal(run.LatencyStats)
	if err != nil {
		return fmt.Errorf("marshaling latency_stats: %w", err)
	}

	byTargetJSON, err := json.Marshal(run.ByTarget)
	if err != nil {
		return fmt.Errorf("marshaling by_target: %w", err)
	}

	resultsJSON, err := json.Marshal(run.Results)
	if err != nil {
		return fmt.Errorf("marshaling results: %w", err)
	}

	labelsJSON, err := json.Marshal(run.Labels)
	if err != nil {
		return fmt.Errorf("marshaling labels: %w", err)
	}

	query := `
		INSERT INTO runs (user_id, environment, targets, total_requests, succeeded, failed, latency_stats, by_target, results, labels)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at`

	run.UserID = &userID
	err = s.pool.QueryRow(ctx, query,
		userID,
		run.Environment,
		run.Targets,
		run.TotalRequests,
		run.Succeeded,
		run.Failed,
		latencyJSON,
		byTargetJSON,
		resultsJSON,
		labelsJSON,
	).Scan(&run.ID, &run.CreatedAt)

	if err != nil {
		return fmt.Errorf("inserting run: %w", err)
	}

	return nil
}

func (s *PostgresStore) GetRunForUser(ctx context.Context, userID, runID uuid.UUID) (*models.Run, error) {
	query := `
		SELECT id, user_id, environment, targets, created_at, total_requests, succeeded, failed,
		       latency_stats, by_target, results, is_baseline, baseline_id, labels
		FROM runs
		WHERE id = $1 AND user_id = $2`

	var run models.Run
	var latencyJSON, byTargetJSON, resultsJSON, labelsJSON []byte
	err := s.pool.QueryRow(ctx, query, runID, userID).Scan(
		&run.ID,
		&run.UserID,
		&run.Environment,
		&run.Targets,
		&run.CreatedAt,
		&run.TotalRequests,
		&run.Succeeded,
		&run.Failed,
		&latencyJSON,
		&byTargetJSON,
		&resultsJSON,
		&run.IsBaseline,
		&run.BaselineID,
		&labelsJSON,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying run: %w", err)
	}

	if err := json.Unmarshal(latencyJSON, &run.LatencyStats); err != nil {
		return nil, fmt.Errorf("unmarshaling latency_stats: %w", err)
	}
	if err := json.Unmarshal(byTargetJSON, &run.ByTarget); err != nil {
		return nil, fmt.Errorf("unmarshaling by_target: %w", err)
	}
	if err := json.Unmarshal(resultsJSON, &run.Results); err != nil {
		return nil, fmt.Errorf("unmarshaling results: %w", err)
	}
	if err := json.Unmarshal(labelsJSON, &run.Labels); err != nil {
		return nil, fmt.Errorf("unmarshaling labels: %w", err)
	}

	return &run, nil
}

func (s *PostgresStore) ListRunsForUser(ctx context.Context, userID uuid.UUID, filter ListFilter) ([]models.RunListItem, int, error) {
	filter.Normalize()

	where := "WHERE user_id = $1"
	args := []any{userID}
	argIdx := 2

	if filter.Environment != "" {
		where += fmt.Sprintf(" AND environment = $%d", argIdx)
		args = append(args, filter.Environment)
		argIdx++
	}
	if filter.After != nil {
		where += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *filter.After)
		argIdx++
	}
	if filter.Before != nil {
		where += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *filter.Before)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM runs " + where
	var total int
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting runs: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, environment, targets, created_at, total_requests, succeeded, failed,
		       latency_stats, by_target, is_baseline, baseline_id, labels
		FROM runs %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying runs: %w", err)
	}
	defer rows.Close()

	var items []models.RunListItem
	for rows.Next() {
		var item models.RunListItem
		var latencyJSON, byTargetJSON, labelsJSON []byte
		err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Environment,
			&item.Targets,
			&item.CreatedAt,
			&item.TotalRequests,
			&item.Succeeded,
			&item.Failed,
			&latencyJSON,
			&byTargetJSON,
			&item.IsBaseline,
			&item.BaselineID,
			&labelsJSON,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scanning run: %w", err)
		}

		if err := json.Unmarshal(latencyJSON, &item.LatencyStats); err != nil {
			return nil, 0, fmt.Errorf("unmarshaling latency_stats: %w", err)
		}
		if err := json.Unmarshal(byTargetJSON, &item.ByTarget); err != nil {
			return nil, 0, fmt.Errorf("unmarshaling by_target: %w", err)
		}
		if err := json.Unmarshal(labelsJSON, &item.Labels); err != nil {
			return nil, 0, fmt.Errorf("unmarshaling labels: %w", err)
		}

		items = append(items, item)
	}

	return items, total, nil
}

func (s *PostgresStore) SetBaselineForUser(ctx context.Context, userID, runID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		err = tx.Rollback(ctx)
		if err != nil {
			fmt.Printf("Failed to rollback transaction: %v\n", err)
		}
	}()

	var env string
	err = tx.QueryRow(ctx, "SELECT environment FROM runs WHERE id = $1 AND user_id = $2", runID, userID).Scan(&env)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("run not found")
		}
		return fmt.Errorf("querying run: %w", err)
	}

	_, err = tx.Exec(ctx, "UPDATE runs SET is_baseline = FALSE WHERE user_id = $1 AND environment = $2 AND is_baseline = TRUE", userID, env)
	if err != nil {
		return fmt.Errorf("clearing old baseline: %w", err)
	}

	_, err = tx.Exec(ctx, "UPDATE runs SET is_baseline = TRUE WHERE id = $1 AND user_id = $2", runID, userID)
	if err != nil {
		return fmt.Errorf("setting new baseline: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) GetBaselineForUser(ctx context.Context, userID uuid.UUID, env string) (*models.Run, error) {
	query := `
		SELECT id, user_id, environment, targets, created_at, total_requests, succeeded, failed,
		       latency_stats, by_target, results, is_baseline, baseline_id, labels
		FROM runs
		WHERE user_id = $1 AND environment = $2 AND is_baseline = TRUE
		LIMIT 1`

	var run models.Run
	var latencyJSON, byTargetJSON, resultsJSON, labelsJSON []byte
	err := s.pool.QueryRow(ctx, query, userID, env).Scan(
		&run.ID,
		&run.UserID,
		&run.Environment,
		&run.Targets,
		&run.CreatedAt,
		&run.TotalRequests,
		&run.Succeeded,
		&run.Failed,
		&latencyJSON,
		&byTargetJSON,
		&resultsJSON,
		&run.IsBaseline,
		&run.BaselineID,
		&labelsJSON,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying baseline: %w", err)
	}

	if err := json.Unmarshal(latencyJSON, &run.LatencyStats); err != nil {
		return nil, fmt.Errorf("unmarshaling latency_stats: %w", err)
	}
	if err := json.Unmarshal(byTargetJSON, &run.ByTarget); err != nil {
		return nil, fmt.Errorf("unmarshaling by_target: %w", err)
	}
	if err := json.Unmarshal(resultsJSON, &run.Results); err != nil {
		return nil, fmt.Errorf("unmarshaling results: %w", err)
	}
	if err := json.Unmarshal(labelsJSON, &run.Labels); err != nil {
		return nil, fmt.Errorf("unmarshaling labels: %w", err)
	}

	return &run, nil
}

func (s *PostgresStore) CreateUser(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (email, password_hash, verify_token)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at`

	err := s.pool.QueryRow(ctx, query,
		user.Email,
		user.PasswordHash,
		user.VerifyToken,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting user: %w", err)
	}

	return nil
}

func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, verified_at, verify_token, created_at, updated_at
		FROM users
		WHERE email = $1`

	var user models.User
	err := s.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.VerifiedAt,
		&user.VerifyToken,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying user: %w", err)
	}

	return &user, nil
}

func (s *PostgresStore) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, verified_at, verify_token, created_at, updated_at
		FROM users
		WHERE id = $1`

	var user models.User
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.VerifiedAt,
		&user.VerifyToken,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying user: %w", err)
	}

	return &user, nil
}

func (s *PostgresStore) GetUserByVerifyToken(ctx context.Context, token string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, verified_at, verify_token, created_at, updated_at
		FROM users
		WHERE verify_token = $1`

	var user models.User
	err := s.pool.QueryRow(ctx, query, token).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.VerifiedAt,
		&user.VerifyToken,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying user: %w", err)
	}

	return &user, nil
}

func (s *PostgresStore) VerifyUser(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE users
		SET verified_at = now(), verify_token = NULL, updated_at = now()
		WHERE id = $1`

	_, err := s.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("verifying user: %w", err)
	}

	return nil
}

func (s *PostgresStore) CreateAPIKey(ctx context.Context, key *models.APIKey) error {
	query := `
		INSERT INTO api_keys (user_id, key_hash, key_prefix, name)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`

	err := s.pool.QueryRow(ctx, query,
		key.UserID,
		key.KeyHash,
		key.KeyPrefix,
		key.Name,
	).Scan(&key.ID, &key.CreatedAt)

	if err != nil {
		return fmt.Errorf("inserting api key: %w", err)
	}

	return nil
}

func (s *PostgresStore) GetAPIKeyByHash(ctx context.Context, hash string) (*models.APIKey, error) {
	query := `
		SELECT id, user_id, key_hash, key_prefix, name, created_at, last_used_at, expires_at
		FROM api_keys
		WHERE key_hash = $1`

	var key models.APIKey
	err := s.pool.QueryRow(ctx, query, hash).Scan(
		&key.ID,
		&key.UserID,
		&key.KeyHash,
		&key.KeyPrefix,
		&key.Name,
		&key.CreatedAt,
		&key.LastUsedAt,
		&key.ExpiresAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying api key: %w", err)
	}

	return &key, nil
}

func (s *PostgresStore) ListAPIKeysForUser(ctx context.Context, userID uuid.UUID) ([]models.APIKey, error) {
	query := `
		SELECT id, user_id, key_hash, key_prefix, name, created_at, last_used_at, expires_at
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("querying api keys: %w", err)
	}
	defer rows.Close()

	var keys []models.APIKey
	for rows.Next() {
		var key models.APIKey
		err := rows.Scan(
			&key.ID,
			&key.UserID,
			&key.KeyHash,
			&key.KeyPrefix,
			&key.Name,
			&key.CreatedAt,
			&key.LastUsedAt,
			&key.ExpiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning api key: %w", err)
		}
		keys = append(keys, key)
	}

	return keys, nil
}

func (s *PostgresStore) DeleteAPIKey(ctx context.Context, userID, keyID uuid.UUID) error {
	query := `DELETE FROM api_keys WHERE id = $1 AND user_id = $2`

	result, err := s.pool.Exec(ctx, query, keyID, userID)
	if err != nil {
		return fmt.Errorf("deleting api key: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("api key not found")
	}

	return nil
}

func (s *PostgresStore) UpdateAPIKeyLastUsed(ctx context.Context, keyID uuid.UUID) error {
	query := `UPDATE api_keys SET last_used_at = now() WHERE id = $1`

	_, err := s.pool.Exec(ctx, query, keyID)
	if err != nil {
		return fmt.Errorf("updating api key last used: %w", err)
	}

	return nil
}
