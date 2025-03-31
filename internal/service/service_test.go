package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scoring-service/internal/mocks"
	"github.com/scoring-service/pkg/models"
	"github.com/stretchr/testify/mock"
)

func TestFetchAccrual(t *testing.T) {
	mockDB := new(mocks.StorageInterface)
	service := &AccrualService{
		client: &http.Client{Timeout: 3 * time.Second},
		apiURL: "http://test-api",
		db:     mockDB,
	}

	tests := []struct {
		name         string
		setupServer  func() *httptest.Server
		expectedErr  string
		expectedCall bool
	}{
		{
			name: "успешное обновление заказа",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(models.AccrualResponse{Status: models.OrderProcessed})
				}))
			},
			expectedErr:  "",
			expectedCall: true,
		},
		{
			name: "заказ не найден",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}))
			},
			expectedErr:  "order not registered",
			expectedCall: false,
		},
		{
			name: "внутренняя ошибка сервера",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
			},
			expectedErr:  "internal server error",
			expectedCall: false,
		},
		{
			name: "контекст завершён",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(3 * time.Second)
				}))
			},
			expectedErr:  "context deadline exceeded",
			expectedCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			service.apiURL = server.URL

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			if tt.expectedCall {
				mockDB.On("UpdateOrder", mock.Anything, mock.Anything).Return(nil).Once()
			}

			err := service.FetchAccrual(ctx, "123456")

			if tt.expectedErr == "" && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.expectedErr != "" && (err == nil || !strings.Contains(err.Error(), tt.expectedErr)) {
				t.Errorf("expected error: %v, got: %v", tt.expectedErr, err)
			}

			mockDB.AssertExpectations(t)
		})
	}
}
