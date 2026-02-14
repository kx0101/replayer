package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/kx0101/replayer-cloud/internal/models"
)

type Store interface {
	CreateRun(ctx context.Context, run *models.Run) error
	GetRun(ctx context.Context, id uuid.UUID) (*models.Run, error)
	ListRuns(ctx context.Context, filter ListFilter) ([]models.RunListItem, int, error)
	SetBaseline(ctx context.Context, id uuid.UUID) error
	GetBaseline(ctx context.Context, environment string) (*models.Run, error)

	CreateRunForUser(ctx context.Context, userID uuid.UUID, run *models.Run) error
	GetRunForUser(ctx context.Context, userID, runID uuid.UUID) (*models.Run, error)
	ListRunsForUser(ctx context.Context, userID uuid.UUID, filter ListFilter) ([]models.RunListItem, int, error)
	SetBaselineForUser(ctx context.Context, userID, runID uuid.UUID) error
	GetBaselineForUser(ctx context.Context, userID uuid.UUID, env string) (*models.Run, error)

	CreateUser(ctx context.Context, user *models.User) error
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetUserByVerifyToken(ctx context.Context, token string) (*models.User, error)
	VerifyUser(ctx context.Context, userID uuid.UUID) error

	CreateAPIKey(ctx context.Context, key *models.APIKey) error
	GetAPIKeyByHash(ctx context.Context, hash string) (*models.APIKey, error)
	ListAPIKeysForUser(ctx context.Context, userID uuid.UUID) ([]models.APIKey, error)
	DeleteAPIKey(ctx context.Context, userID, keyID uuid.UUID) error
	UpdateAPIKeyLastUsed(ctx context.Context, keyID uuid.UUID) error
}

type ListFilter struct {
	Environment string
	After       *time.Time
	Before      *time.Time
	Limit       int
	Offset      int
}

func (f *ListFilter) Normalize() {
	if f.Limit <= 0 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
}
