package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/scoring-service/internal/auth"
	"github.com/scoring-service/pkg/logger"
	"github.com/scoring-service/pkg/models"
)

//go:generate go tool mockery --name=Storage --inpackage --filename=storageinterface_test.go --with-expecter
type Storage interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUserByLogin(ctx context.Context, login string) (*models.User, error)
	GetUserOrders(ctx context.Context, userID int) ([]models.Order, error)
	GetUserWithdrawals(ctx context.Context, userID int) ([]models.Withdrawal, error)
	GetUserBalance(ctx context.Context, userID int) (models.Balance, error)
	SaveOrder(ctx context.Context, user int, order *models.Order) error
	UpdateOrder(ctx context.Context, accrual *models.AccrualResponse) error
	IsOrderExists(ctx context.Context, orderNum string) (int, error)
	Withdraw(ctx context.Context, userID int, order string, sum float64) error
	GetPendingOrders(ctx context.Context) ([]string, error)
}

type AccrualService struct {
	db     Storage
	client *http.Client
	apiURL string
}
type CreateStatus int

const (
	StatusOK CreateStatus = iota
	StatusAlreadyExist
	StatusConflict
	StatusInvalid
	StatusError
)

func NewAccrualService(db Storage, apiURL string) *AccrualService {
	serviceInstance := AccrualService{
		db:     db,
		client: &http.Client{},
		apiURL: apiURL,
	}
	return &serviceInstance
}

func (s *AccrualService) FetchAccrual(ctx context.Context, orderNumber string) error {
	attempts := 0
	maxAttempts := 5
	backoff := time.Second
	for {
		if attempts >= maxAttempts {
			logger.Log.Error("max retries reached")
			return errors.New("max retries reached")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		u, err := url.Parse(s.apiURL)
		if err != nil {
			return fmt.Errorf("неверный API URL: %w", err)
		}
		u.Path = path.Join(u.Path, "api/orders", orderNumber)

		resp, err := s.client.Get(u.String())
		if err != nil {
			logger.Log.Error(err.Error())

			time.Sleep(backoff)
			backoff *= 2
			attempts++
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNoContent {
			logger.Log.Error("order not registered")
			return errors.New("order not registered")
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			retryKey := resp.Header.Get("Retry-After")
			if retryKey != "" {
				retry, err := strconv.Atoi(retryKey)
				if err == nil {
					backoff = time.Second
					time.Sleep(time.Second * time.Duration(retry))
				}

			}
			continue
		}

		if resp.StatusCode == http.StatusInternalServerError {
			logger.Log.Error("internal server error")
			return errors.New("internal server error")
		}

		var accrual models.AccrualResponse
		if err := json.NewDecoder(resp.Body).Decode(&accrual); err != nil {
			logger.Log.Error(err.Error())
			return err
		}
		if err := s.db.UpdateOrder(ctx, &accrual); err != nil {
			logger.Log.Error(err.Error())
			return err
		}

		return nil

	}
}

func (s *AccrualService) UserExist(ctx context.Context, login string) (bool, error) {
	user, err := s.db.GetUserByLogin(ctx, login)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, nil
	}
	return true, nil
}

func (s *AccrualService) ReagisterUser(ctx context.Context, user *models.User) error {
	hashedPassword, err := auth.HashPassword(user.Password)
	if err != nil {
		return err
	}
	user.Password = hashedPassword
	err = s.db.CreateUser(ctx, user)
	if err != nil {
		return err
	}
	return nil
}

func (s *AccrualService) AuthorizeUser(ctx context.Context, newUser *models.User) error {
	user, err := s.db.GetUserByLogin(ctx, newUser.Login)
	if err != nil {
		return err
	}
	if !auth.CheckPasswordHash(newUser.Password, user.Password) {
		logger.Log.Error("Неверная пара логин/пароль", zap.String("login", newUser.Login), zap.String("Password", newUser.Password))
		return errors.New("неверная пара логин/пароль")
	}
	newUser.ID = user.ID
	return nil
}

func (s *AccrualService) GetUserOrders(ctx context.Context, id int) ([]models.Order, error) {
	return s.db.GetUserOrders(ctx, id)
}
func (s *AccrualService) GetUserWithdrawals(ctx context.Context, id int) ([]models.Withdrawal, error) {
	return s.db.GetUserWithdrawals(ctx, id)
}
func (s *AccrualService) GetUserBalance(ctx context.Context, id int) (models.Balance, error) {
	return s.db.GetUserBalance(ctx, id)
}
func (s *AccrualService) CreateOrder(ctx context.Context, userID int, orderNum string) CreateStatus {
	if !auth.IsValidLuhn(orderNum) {
		logger.Log.Error("invalid order number format", zap.String("order", orderNum))
		return StatusInvalid
	}
	realUserID, err := s.db.IsOrderExists(ctx, orderNum)
	if err != nil {
		return StatusError
	}
	if realUserID == 0 {
		newOrder := models.Order{
			Number: orderNum,
			Status: models.OrderNew,
		}
		if err := s.db.SaveOrder(ctx, userID, &newOrder); err != nil {
			return StatusError
		}
		return StatusOK
	} else {
		if userID != realUserID {
			return StatusConflict
		}
		return StatusAlreadyExist
	}

}
func (s *AccrualService) CreateWithdraw(ctx context.Context, userID int, withdraw models.Withdraw) CreateStatus {
	if !auth.IsValidLuhn(withdraw.Order) {
		logger.Log.Error("invalid order number format", zap.String("order", withdraw.Order))
		return StatusInvalid
	}
	balance, err := s.db.GetUserBalance(ctx, userID)
	if err != nil {
		return StatusError
	}
	if balance.Current < withdraw.Sum {
		return StatusConflict
	}

	err = s.db.Withdraw(ctx, userID, withdraw.Order, withdraw.Sum)
	if err != nil {
		return StatusError
	}
	return StatusOK

}
