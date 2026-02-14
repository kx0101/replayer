package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kx0101/replayer-cloud/internal/models"
)

func TestCompareRun(t *testing.T) {
	runID := uuid.New()
	baselineID := uuid.New()
	userID := uuid.New()

	tests := []struct {
		name        string
		id          string
		getRunFn    func(ctx context.Context, uid, rid uuid.UUID) (*models.Run, error)
		getBaseFn   func(ctx context.Context, uid uuid.UUID, env string) (*models.Run, error)
		wantStatus  int
	}{
		{
			name: "success",
			id:   runID.String(),
			getRunFn: func(ctx context.Context, uid, rid uuid.UUID) (*models.Run, error) {
				return &models.Run{
					ID:            runID,
					Environment:   "staging",
					Targets:       []string{"http://localhost:8080"},
					CreatedAt:     time.Now(),
					TotalRequests: 10,
					Succeeded:     9,
					Failed:        1,
					LatencyStats:  models.LatencyStats{P50: 15, P90: 25, P95: 30, P99: 40, Min: 5, Max: 50, Avg: 20},
					ByTarget: map[string]models.TargetStats{
						"http://localhost:8080": {
							Succeeded: 9,
							Failed:    1,
							Latency:   models.LatencyStats{P50: 15, P90: 25, P95: 30, P99: 40, Min: 5, Max: 50, Avg: 20},
						},
					},
					Results: []models.MultiEnvResult{},
					Labels:  map[string]string{},
				}, nil
			},
			getBaseFn: func(ctx context.Context, uid uuid.UUID, env string) (*models.Run, error) {
				return &models.Run{
					ID:            baselineID,
					Environment:   "staging",
					Targets:       []string{"http://localhost:8080"},
					CreatedAt:     time.Now(),
					TotalRequests: 10,
					Succeeded:     10,
					Failed:        0,
					LatencyStats:  models.LatencyStats{P50: 10, P90: 20, P95: 25, P99: 30, Min: 3, Max: 40, Avg: 15},
					ByTarget: map[string]models.TargetStats{
						"http://localhost:8080": {
							Succeeded: 10,
							Failed:    0,
							Latency:   models.LatencyStats{P50: 10, P90: 20, P95: 25, P99: 30, Min: 3, Max: 40, Avg: 15},
						},
					},
					Results: []models.MultiEnvResult{},
					Labels:  map[string]string{},
				}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid id",
			id:         "not-a-uuid",
			getRunFn:   nil,
			getBaseFn:  nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "run not found",
			id:   runID.String(),
			getRunFn: func(ctx context.Context, uid, rid uuid.UUID) (*models.Run, error) {
				return nil, nil
			},
			getBaseFn:  nil,
			wantStatus: http.StatusNotFound,
		},
		{
			name: "no baseline",
			id:   runID.String(),
			getRunFn: func(ctx context.Context, uid, rid uuid.UUID) (*models.Run, error) {
				return &models.Run{
					ID:          runID,
					Environment: "staging",
					Targets:     []string{"http://localhost:8080"},
					ByTarget:    map[string]models.TargetStats{},
					Results:     []models.MultiEnvResult{},
					Labels:      map[string]string{},
				}, nil
			},
			getBaseFn: func(ctx context.Context, uid uuid.UUID, env string) (*models.Run, error) {
				return nil, nil
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "store error on get run",
			id:   runID.String(),
			getRunFn: func(ctx context.Context, uid, rid uuid.UUID) (*models.Run, error) {
				return nil, fmt.Errorf("db error")
			},
			getBaseFn:  nil,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockStore{
				getRunForUserFn:      tt.getRunFn,
				getBaselineForUserFn: tt.getBaseFn,
			}
			h := newTestHandler(ms)

			r := chi.NewRouter()
			r.Get("/api/v1/compare/{id}", func(w http.ResponseWriter, r *http.Request) {
				r = withUserID(r, userID)
				h.CompareRun(w, r)
			})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/compare/"+tt.id, nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d, body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var result models.ComparisonResult
				if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
					t.Fatalf("failed to unmarshal comparison: %v", err)
				}
				if result.RunID != runID {
					t.Errorf("got run_id %s, want %s", result.RunID, runID)
				}
				if result.BaselineID != baselineID {
					t.Errorf("got baseline_id %s, want %s", result.BaselineID, baselineID)
				}
				if _, ok := result.LatencyDelta["http://localhost:8080"]; !ok {
					t.Error("expected latency delta for target")
				}
			}
		})
	}
}

func TestBuildComparison(t *testing.T) {
	t.Run("latency deltas computed correctly", func(t *testing.T) {
		run := &models.Run{
			ID:            uuid.New(),
			TotalRequests: 10,
			Succeeded:     9,
			Failed:        1,
			LatencyStats:  models.LatencyStats{P50: 20, P90: 40, P95: 50, P99: 60, Avg: 30},
			ByTarget: map[string]models.TargetStats{
				"target1": {Latency: models.LatencyStats{P50: 20, P90: 40, P95: 50, P99: 60, Avg: 30}},
			},
			Results: []models.MultiEnvResult{},
		}
		baseline := &models.Run{
			ID:            uuid.New(),
			TotalRequests: 10,
			Succeeded:     10,
			Failed:        0,
			LatencyStats:  models.LatencyStats{P50: 10, P90: 20, P95: 25, P99: 30, Avg: 15},
			ByTarget: map[string]models.TargetStats{
				"target1": {Latency: models.LatencyStats{P50: 10, P90: 20, P95: 25, P99: 30, Avg: 15}},
			},
			Results: []models.MultiEnvResult{},
		}

		result := buildComparison(run, baseline)

		delta, ok := result.LatencyDelta["target1"]
		if !ok {
			t.Fatal("expected latency delta for target1")
		}
		if delta.P50Change != 100.0 {
			t.Errorf("expected P50 change 100%%, got %.1f%%", delta.P50Change)
		}
		if delta.AvgChange != 100.0 {
			t.Errorf("expected Avg change 100%%, got %.1f%%", delta.AvgChange)
		}
	})

	t.Run("diff count from request_id matching", func(t *testing.T) {
		status200 := 200
		status500 := 500

		run := &models.Run{
			ID: uuid.New(),
			ByTarget: map[string]models.TargetStats{},
			Results: []models.MultiEnvResult{
				{
					RequestID: "req-1",
					Responses: map[string]models.ReplayResult{
						"target1": {Status: &status500},
					},
				},
				{
					RequestID: "req-2",
					Responses: map[string]models.ReplayResult{
						"target1": {Status: &status200},
					},
				},
			},
		}
		baseline := &models.Run{
			ID: uuid.New(),
			ByTarget: map[string]models.TargetStats{},
			Results: []models.MultiEnvResult{
				{
					RequestID: "req-1",
					Responses: map[string]models.ReplayResult{
						"target1": {Status: &status200},
					},
				},
				{
					RequestID: "req-2",
					Responses: map[string]models.ReplayResult{
						"target1": {Status: &status200},
					},
				},
			},
		}

		result := buildComparison(run, baseline)
		if result.DiffCount != 1 {
			t.Errorf("expected 1 diff, got %d", result.DiffCount)
		}
	})
}
