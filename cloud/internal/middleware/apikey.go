package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/kx0101/replayer-cloud/internal/auth"
	"github.com/kx0101/replayer-cloud/internal/store"
)

func APIKeyAuth(s store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := r.Header.Get("X-API-Key")
			if provided == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			keyHash := auth.HashAPIKey(provided)
			apiKey, err := s.GetAPIKeyByHash(r.Context(), keyHash)
			if err != nil || apiKey == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
				http.Error(w, `{"error":"api key expired"}`, http.StatusUnauthorized)
				return
			}

			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = s.UpdateAPIKeyLastUsed(ctx, apiKey.ID)
			}()

			ctx := context.WithValue(r.Context(), UserIDContextKey, apiKey.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
