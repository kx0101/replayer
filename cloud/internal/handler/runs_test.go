package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kx0101/replayer-cloud/internal/middleware"
	"github.com/kx0101/replayer-cloud/internal/models"
	"github.com/kx0101/replayer-cloud/internal/store"
)

type mockStore struct {
	createRunFn          func(ctx context.Context, run *models.Run) error
	getRunFn             func(ctx context.Context, id uuid.UUID) (*models.Run, error)
	listRunsFn           func(ctx context.Context, filter store.ListFilter) ([]models.RunListItem, int, error)
	setBaselineFn        func(ctx context.Context, id uuid.UUID) error
	getBaselineFn        func(ctx context.Context, environment string) (*models.Run, error)
	createRunForUserFn   func(ctx context.Context, userID uuid.UUID, run *models.Run) error
	getRunForUserFn      func(ctx context.Context, userID, runID uuid.UUID) (*models.Run, error)
	listRunsForUserFn    func(ctx context.Context, userID uuid.UUID, filter store.ListFilter) ([]models.RunListItem, int, error)
	setBaselineForUserFn func(ctx context.Context, userID, runID uuid.UUID) error
	getBaselineForUserFn func(ctx context.Context, userID uuid.UUID, env string) (*models.Run, error)
	createUserFn         func(ctx context.Context, user *models.User) error
	getUserByEmailFn     func(ctx context.Context, email string) (*models.User, error)
	getUserByIDFn        func(ctx context.Context, id uuid.UUID) (*models.User, error)
	getUserByVerifyTokenFn func(ctx context.Context, token string) (*models.User, error)
	verifyUserFn         func(ctx context.Context, userID uuid.UUID) error
	createAPIKeyFn       func(ctx context.Context, key *models.APIKey) error
	getAPIKeyByHashFn    func(ctx context.Context, hash string) (*models.APIKey, error)
	listAPIKeysForUserFn func(ctx context.Context, userID uuid.UUID) ([]models.APIKey, error)
	deleteAPIKeyFn       func(ctx context.Context, userID, keyID uuid.UUID) error
	updateAPIKeyLastUsedFn func(ctx context.Context, keyID uuid.UUID) error
}

func (m *mockStore) CreateRun(ctx context.Context, run *models.Run) error {
	if m.createRunFn != nil {
		return m.createRunFn(ctx, run)
	}
	return nil
}

func (m *mockStore) GetRun(ctx context.Context, id uuid.UUID) (*models.Run, error) {
	if m.getRunFn != nil {
		return m.getRunFn(ctx, id)
	}
	return nil, nil
}

func (m *mockStore) ListRuns(ctx context.Context, filter store.ListFilter) ([]models.RunListItem, int, error) {
	if m.listRunsFn != nil {
		return m.listRunsFn(ctx, filter)
	}
	return nil, 0, nil
}

func (m *mockStore) SetBaseline(ctx context.Context, id uuid.UUID) error {
	if m.setBaselineFn != nil {
		return m.setBaselineFn(ctx, id)
	}
	return nil
}

func (m *mockStore) GetBaseline(ctx context.Context, environment string) (*models.Run, error) {
	if m.getBaselineFn != nil {
		return m.getBaselineFn(ctx, environment)
	}
	return nil, nil
}

func (m *mockStore) CreateRunForUser(ctx context.Context, userID uuid.UUID, run *models.Run) error {
	if m.createRunForUserFn != nil {
		return m.createRunForUserFn(ctx, userID, run)
	}
	return nil
}

func (m *mockStore) GetRunForUser(ctx context.Context, userID, runID uuid.UUID) (*models.Run, error) {
	if m.getRunForUserFn != nil {
		return m.getRunForUserFn(ctx, userID, runID)
	}
	return nil, nil
}

func (m *mockStore) ListRunsForUser(ctx context.Context, userID uuid.UUID, filter store.ListFilter) ([]models.RunListItem, int, error) {
	if m.listRunsForUserFn != nil {
		return m.listRunsForUserFn(ctx, userID, filter)
	}
	return nil, 0, nil
}

func (m *mockStore) SetBaselineForUser(ctx context.Context, userID, runID uuid.UUID) error {
	if m.setBaselineForUserFn != nil {
		return m.setBaselineForUserFn(ctx, userID, runID)
	}
	return nil
}

func (m *mockStore) GetBaselineForUser(ctx context.Context, userID uuid.UUID, env string) (*models.Run, error) {
	if m.getBaselineForUserFn != nil {
		return m.getBaselineForUserFn(ctx, userID, env)
	}
	return nil, nil
}

func (m *mockStore) CreateUser(ctx context.Context, user *models.User) error {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, user)
	}
	return nil
}

func (m *mockStore) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.getUserByEmailFn != nil {
		return m.getUserByEmailFn(ctx, email)
	}
	return nil, nil
}

func (m *mockStore) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	if m.getUserByIDFn != nil {
		return m.getUserByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockStore) GetUserByVerifyToken(ctx context.Context, token string) (*models.User, error) {
	if m.getUserByVerifyTokenFn != nil {
		return m.getUserByVerifyTokenFn(ctx, token)
	}
	return nil, nil
}

func (m *mockStore) VerifyUser(ctx context.Context, userID uuid.UUID) error {
	if m.verifyUserFn != nil {
		return m.verifyUserFn(ctx, userID)
	}
	return nil
}

func (m *mockStore) CreateAPIKey(ctx context.Context, key *models.APIKey) error {
	if m.createAPIKeyFn != nil {
		return m.createAPIKeyFn(ctx, key)
	}
	return nil
}

func (m *mockStore) GetAPIKeyByHash(ctx context.Context, hash string) (*models.APIKey, error) {
	if m.getAPIKeyByHashFn != nil {
		return m.getAPIKeyByHashFn(ctx, hash)
	}
	return nil, nil
}

func (m *mockStore) ListAPIKeysForUser(ctx context.Context, userID uuid.UUID) ([]models.APIKey, error) {
	if m.listAPIKeysForUserFn != nil {
		return m.listAPIKeysForUserFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockStore) DeleteAPIKey(ctx context.Context, userID, keyID uuid.UUID) error {
	if m.deleteAPIKeyFn != nil {
		return m.deleteAPIKeyFn(ctx, userID, keyID)
	}
	return nil
}

func (m *mockStore) UpdateAPIKeyLastUsed(ctx context.Context, keyID uuid.UUID) error {
	if m.updateAPIKeyLastUsedFn != nil {
		return m.updateAPIKeyLastUsedFn(ctx, keyID)
	}
	return nil
}

func newTestHandler(ms *mockStore) *Handler {
	return &Handler{store: ms}
}

func withUserID(r *http.Request, userID uuid.UUID) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey, userID)
	return r.WithContext(ctx)
}

func TestCreateRun(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name       string
		body       string
		storeFn    func(ctx context.Context, uid uuid.UUID, run *models.Run) error
		wantStatus int
	}{
		{
			name: "success",
			body: `{"environment":"staging","targets":["http://localhost:8080"],"summary":{"total_requests":10,"succeeded":9,"failed":1,"latency":{"p50":10,"p90":20,"p95":25,"p99":30,"min":5,"max":50,"avg":15},"by_target":{}},"results":[]}`,
			storeFn: func(ctx context.Context, uid uuid.UUID, run *models.Run) error {
				run.ID = uuid.New()
				run.CreatedAt = time.Now()
				return nil
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid json",
			body:       `{invalid`,
			storeFn:    nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing environment",
			body:       `{"targets":["http://localhost:8080"],"summary":{"total_requests":1},"results":[]}`,
			storeFn:    nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing targets",
			body:       `{"environment":"staging","summary":{"total_requests":1},"results":[]}`,
			storeFn:    nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "store error",
			body: `{"environment":"staging","targets":["http://localhost:8080"],"summary":{"total_requests":1},"results":[]}`,
			storeFn: func(ctx context.Context, uid uuid.UUID, run *models.Run) error {
				return fmt.Errorf("db error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockStore{createRunForUserFn: tt.storeFn}
			h := newTestHandler(ms)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/runs", bytes.NewBufferString(tt.body))
			req = withUserID(req, userID)
			rec := httptest.NewRecorder()

			h.CreateRun(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d, body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestGetRun(t *testing.T) {
	testID := uuid.New()
	userID := uuid.New()

	tests := []struct {
		name       string
		id         string
		storeFn    func(ctx context.Context, uid, rid uuid.UUID) (*models.Run, error)
		wantStatus int
	}{
		{
			name: "success",
			id:   testID.String(),
			storeFn: func(ctx context.Context, uid, rid uuid.UUID) (*models.Run, error) {
				return &models.Run{
					ID:          testID,
					Environment: "staging",
					Targets:     []string{"http://localhost:8080"},
					CreatedAt:   time.Now(),
					ByTarget:    map[string]models.TargetStats{},
					Results:     []models.MultiEnvResult{},
					Labels:      map[string]string{},
				}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid id",
			id:         "not-a-uuid",
			storeFn:    nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "not found",
			id:   testID.String(),
			storeFn: func(ctx context.Context, uid, rid uuid.UUID) (*models.Run, error) {
				return nil, nil
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "store error",
			id:   testID.String(),
			storeFn: func(ctx context.Context, uid, rid uuid.UUID) (*models.Run, error) {
				return nil, fmt.Errorf("db error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockStore{getRunForUserFn: tt.storeFn}
			h := newTestHandler(ms)

			r := chi.NewRouter()
			r.Get("/api/v1/runs/{id}", func(w http.ResponseWriter, r *http.Request) {
				r = withUserID(r, userID)
				h.GetRun(w, r)
			})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/runs/"+tt.id, nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d, body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestListRuns(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name       string
		query      string
		storeFn    func(ctx context.Context, uid uuid.UUID, filter store.ListFilter) ([]models.RunListItem, int, error)
		wantStatus int
		wantTotal  int
	}{
		{
			name:  "success with results",
			query: "?environment=staging&limit=10",
			storeFn: func(ctx context.Context, uid uuid.UUID, filter store.ListFilter) ([]models.RunListItem, int, error) {
				if filter.Environment != "staging" {
					t.Errorf("expected environment staging, got %q", filter.Environment)
				}
				if filter.Limit != 10 {
					t.Errorf("expected limit 10, got %d", filter.Limit)
				}
				return []models.RunListItem{
					{
						ID:          uuid.New(),
						Environment: "staging",
						Targets:     []string{"http://localhost:8080"},
						ByTarget:    map[string]models.TargetStats{},
						Labels:      map[string]string{},
					},
				}, 1, nil
			},
			wantStatus: http.StatusOK,
			wantTotal:  1,
		},
		{
			name:  "empty results",
			query: "",
			storeFn: func(ctx context.Context, uid uuid.UUID, filter store.ListFilter) ([]models.RunListItem, int, error) {
				return []models.RunListItem{}, 0, nil
			},
			wantStatus: http.StatusOK,
			wantTotal:  0,
		},
		{
			name:  "store error",
			query: "",
			storeFn: func(ctx context.Context, uid uuid.UUID, filter store.ListFilter) ([]models.RunListItem, int, error) {
				return nil, 0, fmt.Errorf("db error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockStore{listRunsForUserFn: tt.storeFn}
			h := newTestHandler(ms)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/runs"+tt.query, nil)
			req = withUserID(req, userID)
			rec := httptest.NewRecorder()

			h.ListRuns(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d, body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp map[string]json.RawMessage
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				var total int
				if err := json.Unmarshal(resp["total"], &total); err != nil {
					t.Fatalf("failed to unmarshal total: %v", err)
				}
				if total != tt.wantTotal {
					t.Errorf("got total %d, want %d", total, tt.wantTotal)
				}
			}
		})
	}
}
