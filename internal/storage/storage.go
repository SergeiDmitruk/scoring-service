package storage

import (
	"context"

	"github.com/scoring-service/pkg/models"
)

type StorageInterface interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUserByLogin(ctx context.Context, login string) (*models.User, error)
	GetUserOrders(ctx context.Context, userID int) ([]models.Order, error)
	GetUserWithdrawals(ctx context.Context, userID int) ([]models.Withdrawal, error)
	GetUserBalance(ctx context.Context, userID int) (models.Balance, error)
	SaveOrder(ctx context.Context, user int, order *models.Order) error
	UpdateOrder(ctx context.Context, accrual *models.AccrualResponse) error
	IsOrderExists(ctx context.Context, orderNum string) (int, error)
	Withdraw(ctx context.Context, userID int, order string, sum float64) error
}
