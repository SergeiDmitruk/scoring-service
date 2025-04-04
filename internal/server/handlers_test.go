package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scoring-service/internal/auth"
	mocks "github.com/scoring-service/internal/mocks/service"
	"github.com/scoring-service/internal/service"
	"github.com/scoring-service/pkg/models"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRegister(t *testing.T) {
	type want struct {
		code       int
		authHeader bool
	}

	tests := []struct {
		name      string
		body      string
		mockSetup func(serv *mocks.ServiceInterface)
		want      want
	}{
		{
			name: "valid registration",
			body: `{"login": "test", "password": "12345"}`,
			mockSetup: func(serv *mocks.ServiceInterface) {
				serv.On("UserExist", mock.Anything, "test").Return(false, nil)
				serv.On("ReagisterUser", mock.Anything, mock.Anything).Return(nil)
			},
			want: want{code: http.StatusOK, authHeader: true},
		},
		{
			name: "login already taken",
			body: `{"login": "test", "password": "12345"}`,
			mockSetup: func(serv *mocks.ServiceInterface) {
				serv.On("UserExist", mock.Anything, "test").Return(true, nil)
			},
			want: want{code: http.StatusConflict},
		},
		{
			name:      "bad request",
			body:      `{bad json}`,
			mockSetup: func(serv *mocks.ServiceInterface) {},
			want:      want{code: http.StatusBadRequest},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := mocks.NewServiceInterface(t)
			tt.mockSetup(mockService)

			h := NewHandler(mockService)

			req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			h.Register(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			require.Equal(t, tt.want.code, resp.StatusCode)
			if tt.want.authHeader {
				require.NotEmpty(t, resp.Header.Get("Authorization"))
			}
		})
	}
}
func TestLogin(t *testing.T) {
	type want struct {
		code       int
		authHeader bool
	}
	tests := []struct {
		name      string
		body      string
		mockSetup func(serv *mocks.ServiceInterface)
		want      want
	}{
		{
			name: "valid login",
			body: `{"login": "test", "password": "12345"}`,
			mockSetup: func(serv *mocks.ServiceInterface) {
				serv.On("AuthorizeUser", mock.Anything, mock.Anything).Return(nil)
			},
			want: want{code: http.StatusOK, authHeader: true},
		},
		{
			name: "invalid login",
			body: `{"login": "test", "password": "wrong"}`,
			mockSetup: func(serv *mocks.ServiceInterface) {
				serv.On("AuthorizeUser", mock.Anything, mock.Anything).Return(errors.New("bad credentials"))
			},
			want: want{code: http.StatusUnauthorized},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := mocks.NewServiceInterface(t)
			tt.mockSetup(mockService)

			h := NewHandler(mockService)

			req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			h.Login(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			require.Equal(t, tt.want.code, resp.StatusCode)
			if tt.want.authHeader {
				require.NotEmpty(t, resp.Header.Get("Authorization"))
			}
		})
	}
}
func TestPostOrder(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		userID   int
		status   service.CreateStatus
		wantCode int
	}{
		{
			name:     "valid order",
			body:     "1234567890",
			userID:   42,
			status:   service.StatusOK,
			wantCode: http.StatusAccepted,
		},
		{
			name:     "already exists",
			body:     "1234567890",
			userID:   42,
			status:   service.StatusAlreadyExist,
			wantCode: http.StatusOK,
		},
		{
			name:     "conflict",
			body:     "1234567890",
			userID:   42,
			status:   service.StatusConflict,
			wantCode: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := mocks.NewServiceInterface(t)
			mockService.On("CreateOrder", mock.Anything, tt.userID, tt.body).Return(tt.status)

			h := NewHandler(mockService)

			req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(tt.body))
			req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, tt.userID))
			w := httptest.NewRecorder()

			h.PostOrder(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			require.Equal(t, tt.wantCode, resp.StatusCode)
		})
	}
}
func TestWithdraw(t *testing.T) {
	type want struct {
		code int
	}
	tests := []struct {
		name      string
		body      string
		userID    any
		mockSetup func(serv *mocks.ServiceInterface)
		want      want
	}{
		{
			name:   "successful withdraw",
			body:   `{"order":"12345678903", "sum":100}`,
			userID: 1,
			mockSetup: func(serv *mocks.ServiceInterface) {
				serv.On("CreateWithdraw", mock.Anything, 1, models.Withdraw{
					Order: "12345678903",
					Sum:   100,
				}).Return(service.StatusOK)
			},
			want: want{code: http.StatusOK},
		},
		{
			name:   "already exists withdraw",
			body:   `{"order":"12345678903", "sum":100}`,
			userID: 1,
			mockSetup: func(serv *mocks.ServiceInterface) {
				serv.On("CreateWithdraw", mock.Anything, 1, models.Withdraw{
					Order: "12345678903",
					Sum:   100,
				}).Return(service.StatusAlreadyExist)
			},
			want: want{code: http.StatusOK},
		},
		{
			name:   "insufficient funds",
			body:   `{"order":"12345678903", "sum":1000}`,
			userID: 1,
			mockSetup: func(serv *mocks.ServiceInterface) {
				serv.On("CreateWithdraw", mock.Anything, 1, models.Withdraw{
					Order: "12345678903",
					Sum:   1000,
				}).Return(service.StatusConflict)
			},
			want: want{code: http.StatusPaymentRequired},
		},
		{
			name:   "invalid order number",
			body:   `{"order":"invalid", "sum":100}`,
			userID: 1,
			mockSetup: func(serv *mocks.ServiceInterface) {
				serv.On("CreateWithdraw", mock.Anything, 1, models.Withdraw{
					Order: "invalid",
					Sum:   100,
				}).Return(service.StatusInvalid)
			},
			want: want{code: http.StatusUnprocessableEntity},
		},
		{
			name:   "internal server error",
			body:   `{"order":"12345678903", "sum":100}`,
			userID: 1,
			mockSetup: func(serv *mocks.ServiceInterface) {
				serv.On("CreateWithdraw", mock.Anything, 1, models.Withdraw{
					Order: "12345678903",
					Sum:   100,
				}).Return(service.StatusError)
			},
			want: want{code: http.StatusInternalServerError},
		},
		{
			name:      "unauthorized user",
			body:      `{"order":"12345678903", "sum":100}`,
			userID:    nil,
			mockSetup: func(serv *mocks.ServiceInterface) {},
			want:      want{code: http.StatusUnauthorized},
		},
		{
			name:      "invalid json",
			body:      `{"order":123}`,
			userID:    1,
			mockSetup: func(serv *mocks.ServiceInterface) {},
			want:      want{code: http.StatusBadRequest},
		},
		{
			name:      "sum <= 0",
			body:      `{"order":"12345678903", "sum":0}`,
			userID:    1,
			mockSetup: func(serv *mocks.ServiceInterface) {},
			want:      want{code: http.StatusBadRequest},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := mocks.NewServiceInterface(t)
			tt.mockSetup(mockService)

			h := NewHandler(mockService)

			req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", strings.NewReader(tt.body))
			if tt.userID != nil {
				req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, tt.userID))
			}
			w := httptest.NewRecorder()

			h.Withdraw(w, req)

			res := w.Result()
			defer res.Body.Close()

			require.Equal(t, tt.want.code, res.StatusCode)
		})
	}
}
func TestGetUserWithdrawals(t *testing.T) {
	type want struct {
		code int
	}
	tests := []struct {
		name      string
		userID    any
		mockSetup func(serv *mocks.ServiceInterface)
		want      want
	}{
		{
			name:   "successful get user withdrawals",
			userID: 1,
			mockSetup: func(serv *mocks.ServiceInterface) {
				serv.On("GetUserWithdrawals", mock.Anything, 1).Return([]models.Withdrawal{
					{Order: "123", Sum: 100},
					{Order: "124", Sum: 200},
				}, nil)
			},
			want: want{code: http.StatusOK},
		},
		{
			name:   "no withdrawals found",
			userID: 1,
			mockSetup: func(serv *mocks.ServiceInterface) {
				serv.On("GetUserWithdrawals", mock.Anything, 1).Return([]models.Withdrawal{}, nil)
			},
			want: want{code: http.StatusNoContent},
		},
		{
			name:      "unauthorized user",
			userID:    nil,
			mockSetup: func(serv *mocks.ServiceInterface) {},
			want:      want{code: http.StatusUnauthorized},
		},
		{
			name:   "internal server error",
			userID: 1,
			mockSetup: func(serv *mocks.ServiceInterface) {
				serv.On("GetUserWithdrawals", mock.Anything, 1).Return(nil, fmt.Errorf("server error"))
			},
			want: want{code: http.StatusInternalServerError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := mocks.NewServiceInterface(t)
			tt.mockSetup(mockService)

			h := NewHandler(mockService)

			req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
			if tt.userID != nil {
				req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, tt.userID))
			}
			w := httptest.NewRecorder()

			h.GetUserWithdrawals(w, req)

			res := w.Result()
			defer res.Body.Close()

			require.Equal(t, tt.want.code, res.StatusCode)
		})
	}
}
func TestGetUserBalance(t *testing.T) {
	type want struct {
		code int
	}
	tests := []struct {
		name      string
		userID    any
		mockSetup func(serv *mocks.ServiceInterface)
		want      want
	}{
		{
			name:   "successful get user balance",
			userID: 1,
			mockSetup: func(serv *mocks.ServiceInterface) {
				serv.On("GetUserBalance", mock.Anything, 1).Return(models.Balance{Current: 500}, nil)
			},
			want: want{code: http.StatusOK},
		},
		{
			name:      "unauthorized user",
			userID:    nil,
			mockSetup: func(serv *mocks.ServiceInterface) {},
			want:      want{code: http.StatusUnauthorized},
		},
		{
			name:   "internal server error",
			userID: 1,
			mockSetup: func(serv *mocks.ServiceInterface) {
				serv.On("GetUserBalance", mock.Anything, 1).Return(models.Balance{}, fmt.Errorf("server error"))
			},
			want: want{code: http.StatusInternalServerError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := mocks.NewServiceInterface(t)
			tt.mockSetup(mockService)

			h := NewHandler(mockService)

			req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
			if tt.userID != nil {
				req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, tt.userID))
			}
			w := httptest.NewRecorder()

			h.GetUserBalance(w, req)

			res := w.Result()
			defer res.Body.Close()

			require.Equal(t, tt.want.code, res.StatusCode)
		})
	}
}
