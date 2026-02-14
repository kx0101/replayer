package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/kx0101/replayer-cloud/internal/auth"
	"github.com/kx0101/replayer-cloud/internal/models"
	"github.com/kx0101/replayer-cloud/internal/store"
)

type contextKey string

const (
	SessionContextKey contextKey = "session"
	UserContextKey    contextKey = "user"
	UserIDContextKey  contextKey = "user_id"
)

func SessionAuth(sm *auth.SessionManager, s store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := sm.GetSession(r)
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			ctx := context.WithValue(r.Context(), SessionContextKey, session)
			ctx = context.WithValue(ctx, UserIDContextKey, session.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func OptionalSession(sm *auth.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := sm.GetSession(r)
			if err == nil {
				ctx := context.WithValue(r.Context(), SessionContextKey, session)
				ctx = context.WithValue(ctx, UserIDContextKey, session.UserID)
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireVerified(sm *auth.SessionManager, s store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := sm.GetSession(r)
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			user, err := s.GetUserByID(r.Context(), session.UserID)
			if err != nil || user == nil {
				sm.ClearSession(w)
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			if user.VerifiedAt == nil {
				http.Redirect(w, r, "/verify-pending", http.StatusSeeOther)
				return
			}

			ctx := context.WithValue(r.Context(), SessionContextKey, session)
			ctx = context.WithValue(ctx, UserContextKey, user)
			ctx = context.WithValue(ctx, UserIDContextKey, session.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetSession(ctx context.Context) *models.SessionData {
	if session, ok := ctx.Value(SessionContextKey).(*models.SessionData); ok {
		return session
	}
	return nil
}

func GetUser(ctx context.Context) *models.User {
	if user, ok := ctx.Value(UserContextKey).(*models.User); ok {
		return user
	}
	return nil
}

func GetUserID(ctx context.Context) uuid.UUID {
	if id, ok := ctx.Value(UserIDContextKey).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}
