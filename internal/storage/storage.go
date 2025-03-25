package storage

import (
	"context"

	"github.com/scoring-service/pkg/models"
)

type StorageInterface interface {
	CreateUser(ctx context.Context, user models.User) error
	GetUserByLogin(ctx context.Context, login string) (*models.User, error)
}
