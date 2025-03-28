package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/scoring-service/internal/storage"
	"github.com/scoring-service/pkg/logger"
	"github.com/scoring-service/pkg/models"
)

type AccrualService struct {
	db     storage.StorageInterface
	client *http.Client
	apiURL string
}

var serviceInstance AccrualService

func NewAccrualService(db storage.StorageInterface, apiURL string) *AccrualService {
	serviceInstance = AccrualService{
		db:     db,
		client: &http.Client{},
		apiURL: apiURL,
	}
	logger.Log.Sugar().Info("Сосать", apiURL) //убрать
	return &serviceInstance
}
func GetAccrualService() *AccrualService {
	logger.Log.Sugar().Info("Ебать", serviceInstance.apiURL) //убрать
	return &serviceInstance
}
func (s *AccrualService) FetchAccrual(ctx context.Context, orderNumber string) error {
	attempts := 0
	maxAttempts := 5
	backoff := time.Second
	oldStatus := models.OrderNew
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := s.client.Get(fmt.Sprintf("http://%s/api/orders/%s", s.apiURL, orderNumber))
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
			if err := s.db.UpdateOrder(ctx, &models.AccrualResponse{Order: orderNumber, Status: models.OrderInvalid}); err != nil {
				logger.Log.Error(err.Error())
				return err
			}
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
		if oldStatus != accrual.Status {
			if err := s.db.UpdateOrder(ctx, &accrual); err != nil {
				logger.Log.Error(err.Error())
				return err
			}
			oldStatus = accrual.Status
		}

		if accrual.Status == models.OrderProcessed || accrual.Status == models.OrderInvalid {
			return nil
		}

		if attempts >= maxAttempts {
			logger.Log.Error("max retries reached")
			return errors.New("max retries reached")
		}

	}
}
