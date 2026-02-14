package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestSetBaseline(t *testing.T) {
	testID := uuid.New()
	userID := uuid.New()

	tests := []struct {
		name       string
		id         string
		storeFn    func(ctx context.Context, uid, rid uuid.UUID) error
		wantStatus int
	}{
		{
			name: "success",
			id:   testID.String(),
			storeFn: func(ctx context.Context, uid, rid uuid.UUID) error {
				return nil
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
			storeFn: func(ctx context.Context, uid, rid uuid.UUID) error {
				return fmt.Errorf("run not found")
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "store error",
			id:   testID.String(),
			storeFn: func(ctx context.Context, uid, rid uuid.UUID) error {
				return fmt.Errorf("db error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockStore{setBaselineForUserFn: tt.storeFn}
			h := newTestHandler(ms)

			r := chi.NewRouter()
			r.Post("/api/v1/runs/{id}/baseline", func(w http.ResponseWriter, r *http.Request) {
				r = withUserID(r, userID)
				h.SetBaseline(w, r)
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/runs/"+tt.id+"/baseline", nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d, body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}
