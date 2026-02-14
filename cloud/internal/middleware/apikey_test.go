package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kx0101/replayer-cloud/internal/auth"
	"github.com/kx0101/replayer-cloud/internal/models"
	"github.com/kx0101/replayer-cloud/internal/store"
)

type mockStore struct {
	getAPIKeyByHashFn func(ctx context.Context, hash string) (*models.APIKey, error)
	updateLastUsedFn  func(ctx context.Context, keyID uuid.UUID) error
}

func (m *mockStore) CreateRun(ctx context.Context, run *models.Run) error { return nil }
func (m *mockStore) GetRun(ctx context.Context, id uuid.UUID) (*models.Run, error) { return nil, nil }
func (m *mockStore) ListRuns(ctx context.Context, filter store.ListFilter) ([]models.RunListItem, int, error) { return nil, 0, nil }
func (m *mockStore) SetBaseline(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockStore) GetBaseline(ctx context.Context, environment string) (*models.Run, error) { return nil, nil }
func (m *mockStore) CreateRunForUser(ctx context.Context, userID uuid.UUID, run *models.Run) error { return nil }
func (m *mockStore) GetRunForUser(ctx context.Context, userID, runID uuid.UUID) (*models.Run, error) { return nil, nil }
func (m *mockStore) ListRunsForUser(ctx context.Context, userID uuid.UUID, filter store.ListFilter) ([]models.RunListItem, int, error) { return nil, 0, nil }
func (m *mockStore) SetBaselineForUser(ctx context.Context, userID, runID uuid.UUID) error { return nil }
func (m *mockStore) GetBaselineForUser(ctx context.Context, userID uuid.UUID, env string) (*models.Run, error) { return nil, nil }
func (m *mockStore) CreateUser(ctx context.Context, user *models.User) error { return nil }
func (m *mockStore) GetUserByEmail(ctx context.Context, email string) (*models.User, error) { return nil, nil }
func (m *mockStore) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) { return nil, nil }
func (m *mockStore) GetUserByVerifyToken(ctx context.Context, token string) (*models.User, error) { return nil, nil }
func (m *mockStore) VerifyUser(ctx context.Context, userID uuid.UUID) error { return nil }
func (m *mockStore) CreateAPIKey(ctx context.Context, key *models.APIKey) error { return nil }
func (m *mockStore) ListAPIKeysForUser(ctx context.Context, userID uuid.UUID) ([]models.APIKey, error) { return nil, nil }
func (m *mockStore) DeleteAPIKey(ctx context.Context, userID, keyID uuid.UUID) error { return nil }

func (m *mockStore) GetAPIKeyByHash(ctx context.Context, hash string) (*models.APIKey, error) {
	if m.getAPIKeyByHashFn != nil {
		return m.getAPIKeyByHashFn(ctx, hash)
	}
	return nil, nil
}

func (m *mockStore) UpdateAPIKeyLastUsed(ctx context.Context, keyID uuid.UUID) error {
	if m.updateLastUsedFn != nil {
		return m.updateLastUsedFn(ctx, keyID)
	}
	return nil
}

func TestAPIKeyAuth(t *testing.T) {
	testKey := "rp_testkey1234567890abcdef"
	testKeyHash := auth.HashAPIKey(testKey)
	testUserID := uuid.New()
	testKeyID := uuid.New()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := GetUserID(r.Context())
		if uid == uuid.Nil {
			t.Error("expected user ID in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		headerVal  string
		storeFn    func(ctx context.Context, hash string) (*models.APIKey, error)
		wantStatus int
	}{
		{
			name:      "valid key",
			headerVal: testKey,
			storeFn: func(ctx context.Context, hash string) (*models.APIKey, error) {
				if hash != testKeyHash {
					t.Errorf("unexpected hash: %s", hash)
				}
				return &models.APIKey{
					ID:     testKeyID,
					UserID: testUserID,
					Name:   "Test Key",
				}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:      "invalid key",
			headerVal: "rp_wrongkey",
			storeFn: func(ctx context.Context, hash string) (*models.APIKey, error) {
				return nil, nil
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "missing key",
			headerVal:  "",
			storeFn:    nil,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:      "expired key",
			headerVal: testKey,
			storeFn: func(ctx context.Context, hash string) (*models.APIKey, error) {
				expired := time.Now().Add(-time.Hour)
				return &models.APIKey{
					ID:        testKeyID,
					UserID:    testUserID,
					Name:      "Expired Key",
					ExpiresAt: &expired,
				}, nil
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockStore{getAPIKeyByHashFn: tt.storeFn}
			mw := APIKeyAuth(ms)
			wrapped := mw(handler)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.headerVal != "" {
				req.Header.Set("X-API-Key", tt.headerVal)
			}

			rec := httptest.NewRecorder()
			wrapped.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
