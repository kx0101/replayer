//go:build integration

package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kx0101/replayer-cloud/internal/models"
)

func setupTestDB(t *testing.T) *PostgresStore {
	t.Helper()

	dbURL := os.Getenv("REPLAYER_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("REPLAYER_TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connecting to test db: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	s := NewPostgresStore(pool)
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	_, err = pool.Exec(ctx, "DELETE FROM runs")
	if err != nil {
		t.Fatalf("cleaning runs table: %v", err)
	}

	return s
}

func TestPostgresStore_CreateAndGetRun(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	run := &models.Run{
		Environment:   "staging",
		Targets:       []string{"http://localhost:8080"},
		TotalRequests: 10,
		Succeeded:     9,
		Failed:        1,
		LatencyStats:  models.LatencyStats{P50: 10, P90: 20, P95: 25, P99: 30, Min: 5, Max: 50, Avg: 15},
		ByTarget:      map[string]models.TargetStats{},
		Results:       []models.MultiEnvResult{},
		Labels:        map[string]string{"branch": "main"},
	}

	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("creating run: %v", err)
	}

	if run.ID == uuid.Nil {
		t.Fatal("expected run ID to be set")
	}
	if run.CreatedAt.IsZero() {
		t.Fatal("expected created_at to be set")
	}

	got, err := s.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("getting run: %v", err)
	}
	if got == nil {
		t.Fatal("expected run, got nil")
	}
	if got.Environment != "staging" {
		t.Errorf("got environment %q, want staging", got.Environment)
	}
	if got.TotalRequests != 10 {
		t.Errorf("got total_requests %d, want 10", got.TotalRequests)
	}
}

func TestPostgresStore_ListRuns(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		run := &models.Run{
			Environment:   "staging",
			Targets:       []string{"http://localhost:8080"},
			TotalRequests: i + 1,
			ByTarget:      map[string]models.TargetStats{},
			Results:       []models.MultiEnvResult{},
			Labels:        map[string]string{},
			LatencyStats:  models.LatencyStats{},
		}
		if err := s.CreateRun(ctx, run); err != nil {
			t.Fatalf("creating run %d: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	items, total, err := s.ListRuns(ctx, ListFilter{Environment: "staging", Limit: 2})
	if err != nil {
		t.Fatalf("listing runs: %v", err)
	}
	if total != 3 {
		t.Errorf("got total %d, want 3", total)
	}
	if len(items) != 2 {
		t.Errorf("got %d items, want 2", len(items))
	}
}

func TestPostgresStore_SetBaseline(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	run1 := &models.Run{
		Environment: "prod",
		Targets:     []string{"http://localhost:8080"},
		ByTarget:    map[string]models.TargetStats{},
		Results:     []models.MultiEnvResult{},
		Labels:      map[string]string{},
		LatencyStats: models.LatencyStats{},
	}
	run2 := &models.Run{
		Environment: "prod",
		Targets:     []string{"http://localhost:8080"},
		ByTarget:    map[string]models.TargetStats{},
		Results:     []models.MultiEnvResult{},
		Labels:      map[string]string{},
		LatencyStats: models.LatencyStats{},
	}

	if err := s.CreateRun(ctx, run1); err != nil {
		t.Fatalf("creating run1: %v", err)
	}
	if err := s.CreateRun(ctx, run2); err != nil {
		t.Fatalf("creating run2: %v", err)
	}

	if err := s.SetBaseline(ctx, run1.ID); err != nil {
		t.Fatalf("setting baseline to run1: %v", err)
	}

	baseline, err := s.GetBaseline(ctx, "prod")
	if err != nil {
		t.Fatalf("getting baseline: %v", err)
	}
	if baseline.ID != run1.ID {
		t.Errorf("got baseline %s, want %s", baseline.ID, run1.ID)
	}

	if err := s.SetBaseline(ctx, run2.ID); err != nil {
		t.Fatalf("setting baseline to run2: %v", err)
	}

	baseline, err = s.GetBaseline(ctx, "prod")
	if err != nil {
		t.Fatalf("getting baseline after switch: %v", err)
	}
	if baseline.ID != run2.ID {
		t.Errorf("got baseline %s, want %s", baseline.ID, run2.ID)
	}
}
