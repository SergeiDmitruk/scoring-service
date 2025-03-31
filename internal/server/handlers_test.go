package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scoring-service/internal/auth"
	"github.com/scoring-service/internal/mocks"
	"github.com/scoring-service/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRegister(t *testing.T) {
	type testCase struct {
		name         string
		input        models.User
		mockSetup    func(storage *mocks.StorageInterface)
		expectedCode int
		expectedBody string
	}

	tests := []testCase{
		{
			name: "Успешная регистрация",
			input: models.User{
				Login:    "testuser",
				Password: "password123",
			},
			mockSetup: func(storage *mocks.StorageInterface) {
				storage.On("GetUserByLogin", mock.Anything, "testuser").Return(nil, nil)
				storage.On("CreateUser", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
			},
			expectedCode: http.StatusOK,
			expectedBody: "Пользователь успешно зарегистрирован и аутентифицирован",
		},
		{
			name: "Пустые логин и пароль",
			input: models.User{
				Login:    "",
				Password: "",
			},
			mockSetup:    func(storage *mocks.StorageInterface) {},
			expectedCode: http.StatusBadRequest,
			expectedBody: "Невалидный логин или пароль",
		},
		{
			name: "Логин уже занят",
			input: models.User{
				Login:    "existinguser",
				Password: "password123",
			},
			mockSetup: func(storage *mocks.StorageInterface) {
				storage.On("GetUserByLogin", mock.Anything, "existinguser").Return(&models.User{Login: "existinguser"}, nil)
			},
			expectedCode: http.StatusConflict,
			expectedBody: "Логин уже занят",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storageMock := new(mocks.StorageInterface)
			tt.mockSetup(storageMock)

			h := NewHandler(storageMock)
			body, _ := json.Marshal(tt.input)
			r := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
			w := httptest.NewRecorder()

			h.Register(w, r)

			resp := w.Result()
			assert.Equal(t, tt.expectedCode, resp.StatusCode)

			responseBody := w.Body.String()
			assert.Contains(t, responseBody, tt.expectedBody)
		})
	}
}
func TestLogin(t *testing.T) {
	type testCase struct {
		name           string
		inputUser      models.User
		mockSetup      func(storage *mocks.StorageInterface)
		expectedStatus int
		expectedBody   string
	}
	hash, _ := auth.HashPassword("password123")
	tests := []testCase{
		{
			name: "успешная авторизация",
			inputUser: models.User{
				Login:    "testuser",
				Password: "password123",
			},
			mockSetup: func(storage *mocks.StorageInterface) {
				storage.On("GetUserByLogin", mock.Anything, "testuser").Return(&models.User{
					ID:       1,
					Login:    "testuser",
					Password: hash,
				}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "Пользователь успешно аутентифицирован",
		},
		{
			name: "неверный пароль",
			inputUser: models.User{
				Login:    "testuser",
				Password: "wrongpassword",
			},
			mockSetup: func(storage *mocks.StorageInterface) {
				storage.On("GetUserByLogin", mock.Anything, "testuser").Return(&models.User{
					ID:       1,
					Login:    "testuser",
					Password: "$2a$10$A4ZzYopqPweqKGHpmhPbXcYzxt.AZHs5hOZ1I/x8Cz6gAVO65v9m6",
				}, nil)
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Неверная пара логин/пароль",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storageMock := new(mocks.StorageInterface)
			tt.mockSetup(storageMock)

			h := NewHandler(storageMock)

			body, _ := json.Marshal(tt.inputUser)
			r := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
			w := httptest.NewRecorder()

			if tt.expectedStatus == http.StatusOK {
				validToken, _ := auth.GenerateJWT(&models.User{ID: 1})
				r.Header.Set("Authorization", "Bearer "+validToken)
			}

			h.Login(w, r)

			resp := w.Result()
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
			responseBody := w.Body.String()
			assert.Contains(t, responseBody, tt.expectedBody)
		})
	}
}

func TestGetUserOrders(t *testing.T) {
	type testCase struct {
		name           string
		userID         int
		mockSetup      func(storage *mocks.StorageInterface)
		expectedStatus int
		expectedBody   string
	}

	validToken, _ := auth.GenerateJWT(&models.User{ID: 1})

	tests := []testCase{
		{
			name:   "успешное получение заказов",
			userID: 1,
			mockSetup: func(storage *mocks.StorageInterface) {
				storage.On("GetUserOrders", mock.Anything, 1).Return([]models.Order{
					{Number: "123", Status: "NEW", Accrual: 100.5, UploadedAt: time.Now()},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"number":"123"`,
		},
		{
			name:   "у пользователя нет заказов",
			userID: 1,
			mockSetup: func(storage *mocks.StorageInterface) {
				storage.On("GetUserOrders", mock.Anything, 1).Return([]models.Order{}, nil)
			},
			expectedStatus: http.StatusNoContent,
			expectedBody:   "",
		},
		{
			name:   "ошибка базы данных",
			userID: 1,
			mockSetup: func(storage *mocks.StorageInterface) {
				storage.On("GetUserOrders", mock.Anything, 1).Return(nil, errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "internal server error",
		},
		{
			name:           "отсутствие авторизации",
			userID:         0,
			mockSetup:      func(storage *mocks.StorageInterface) {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			storageMock := new(mocks.StorageInterface)
			tt.mockSetup(storageMock)

			h := NewHandler(storageMock)

			r := httptest.NewRequest(http.MethodGet, "/orders", nil)
			w := httptest.NewRecorder()

			if tt.userID > 0 {
				r.Header.Set("Authorization", "Bearer "+validToken)
				ctx := context.WithValue(r.Context(), auth.UserIDKey, tt.userID)
				r = r.WithContext(ctx)
			}

			h.GetUserOrders(w, r)

			resp := w.Result()
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
			if tt.expectedBody != "" {
				body, _ := io.ReadAll(resp.Body)
				assert.Contains(t, string(body), tt.expectedBody)
			}
		})
	}
}
func TestGetUserWithdrawals(t *testing.T) {
	type testCase struct {
		name           string
		userID         int
		mockSetup      func(storage *mocks.StorageInterface)
		expectedStatus int
		expectedBody   string
	}

	validToken, _ := auth.GenerateJWT(&models.User{ID: 1})

	tests := []testCase{
		{
			name:   "успешное получение выводов средств",
			userID: 1,
			mockSetup: func(storage *mocks.StorageInterface) {
				storage.On("GetUserWithdrawals", mock.Anything, 1).Return([]models.Withdrawal{
					{Order: "12345", Sum: 500.75, ProcessedAt: time.Now()},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"order":"12345"`,
		},
		{
			name:   "у пользователя нет выводов",
			userID: 1,
			mockSetup: func(storage *mocks.StorageInterface) {
				storage.On("GetUserWithdrawals", mock.Anything, 1).Return([]models.Withdrawal{}, nil)
			},
			expectedStatus: http.StatusNoContent,
			expectedBody:   "",
		},
		{
			name:   "ошибка базы данных",
			userID: 1,
			mockSetup: func(storage *mocks.StorageInterface) {
				storage.On("GetUserWithdrawals", mock.Anything, 1).Return(nil, errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "internal server error",
		},
		{
			name:           "отсутствие авторизации",
			userID:         0,
			mockSetup:      func(storage *mocks.StorageInterface) {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storageMock := new(mocks.StorageInterface)
			tt.mockSetup(storageMock)

			h := NewHandler(storageMock)

			r := httptest.NewRequest(http.MethodGet, "/withdrawals", nil)
			w := httptest.NewRecorder()

			if tt.userID > 0 {
				r.Header.Set("Authorization", "Bearer "+validToken)
				ctx := context.WithValue(r.Context(), auth.UserIDKey, tt.userID)
				r = r.WithContext(ctx)
			}

			h.GetUserWithdrawals(w, r)

			resp := w.Result()
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			if tt.expectedBody != "" {
				body, _ := io.ReadAll(resp.Body)
				assert.Contains(t, string(body), tt.expectedBody)
			}
		})
	}
}
func TestGetUserBalance(t *testing.T) {
	type testCase struct {
		name           string
		userID         int
		mockSetup      func(storage *mocks.StorageInterface)
		expectedStatus int
		expectedBody   string
	}

	validToken, _ := auth.GenerateJWT(&models.User{ID: 1})

	tests := []testCase{
		{
			name:   "успешное получение баланса",
			userID: 1,
			mockSetup: func(storage *mocks.StorageInterface) {
				storage.On("GetUserBalance", mock.Anything, 1).Return(models.Balance{
					Current:   1500.75,
					Withdrawn: 500.25,
				}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"current":1500.75`,
		},
		{
			name:   "ошибка базы данных",
			userID: 1,
			mockSetup: func(storage *mocks.StorageInterface) {
				storage.On("GetUserBalance", mock.Anything, 1).Return(models.Balance{}, errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "internal server error",
		},
		{
			name:           "отсутствие авторизации",
			userID:         0, // Нет userID в контексте
			mockSetup:      func(storage *mocks.StorageInterface) {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storageMock := new(mocks.StorageInterface)
			tt.mockSetup(storageMock)

			h := NewHandler(storageMock)

			r := httptest.NewRequest(http.MethodGet, "/balance", nil)
			w := httptest.NewRecorder()

			if tt.userID > 0 {
				r.Header.Set("Authorization", "Bearer "+validToken)
				ctx := context.WithValue(r.Context(), auth.UserIDKey, tt.userID)
				r = r.WithContext(ctx)
			}

			h.GetUserBalance(w, r)

			resp := w.Result()
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			if tt.expectedBody != "" {
				body, _ := io.ReadAll(resp.Body)
				assert.Contains(t, string(body), tt.expectedBody)
			}
		})
	}
}
func TestWithdrawBalance(t *testing.T) {
	type testCase struct {
		name           string
		userID         int
		requestBody    string
		mockSetup      func(storage *mocks.StorageInterface)
		expectedStatus int
		expectedBody   string
	}

	validToken, _ := auth.GenerateJWT(&models.User{ID: 1})

	tests := []testCase{
		{
			name:        "успешное списание",
			userID:      1,
			requestBody: `{"order":"79927398713","sum":500.50}`,
			mockSetup: func(storage *mocks.StorageInterface) {
				storage.On("GetUserBalance", mock.Anything, 1).Return(models.Balance{Current: 1000.0, Withdrawn: 0}, nil)
				storage.On("Withdraw", mock.Anything, 1, "79927398713", 500.50).Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "некорректный JSON",
			userID:         1,
			requestBody:    `{"order":79927398713,"sum":500.50}`,
			mockSetup:      func(storage *mocks.StorageInterface) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid request format",
		},
		{
			name:           "некорректный номер заказа",
			userID:         1,
			requestBody:    `{"order":"123456","sum":500.50}`,
			mockSetup:      func(storage *mocks.StorageInterface) {},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedBody:   "invalid order number",
		},
		{
			name:           "сумма меньше нуля",
			userID:         1,
			requestBody:    `{"order":"79927398713","sum":0}`,
			mockSetup:      func(storage *mocks.StorageInterface) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "sum must be greater than zero",
		},
		{
			name:        "ошибка при получении баланса",
			userID:      1,
			requestBody: `{"order":"79927398713","sum":500.50}`,
			mockSetup: func(storage *mocks.StorageInterface) {
				storage.On("GetUserBalance", mock.Anything, 1).Return(models.Balance{}, errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "internal server error",
		},
		{
			name:        "недостаточно средств",
			userID:      1,
			requestBody: `{"order":"79927398713","sum":2000.50}`,
			mockSetup: func(storage *mocks.StorageInterface) {
				storage.On("GetUserBalance", mock.Anything, 1).Return(models.Balance{Current: 1000.0, Withdrawn: 0}, nil)
			},
			expectedStatus: http.StatusPaymentRequired,
			expectedBody:   "insufficient funds",
		},
		{
			name:        "ошибка при списании",
			userID:      1,
			requestBody: `{"order":"79927398713","sum":500.50}`,
			mockSetup: func(storage *mocks.StorageInterface) {
				storage.On("GetUserBalance", mock.Anything, 1).Return(models.Balance{Current: 1000.0, Withdrawn: 0}, nil)
				storage.On("Withdraw", mock.Anything, 1, "79927398713", 500.50).Return(errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "internal server error",
		},
		{
			name:           "отсутствие авторизации",
			userID:         0,
			requestBody:    `{"order":"79927398713","sum":500.50}`,
			mockSetup:      func(storage *mocks.StorageInterface) {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storageMock := new(mocks.StorageInterface)
			tt.mockSetup(storageMock)
			h := NewHandler(storageMock)

			r := httptest.NewRequest(http.MethodPost, "/withdraw", strings.NewReader(tt.requestBody))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			if tt.userID > 0 {
				r.Header.Set("Authorization", "Bearer "+validToken)
				ctx := context.WithValue(r.Context(), auth.UserIDKey, tt.userID)
				r = r.WithContext(ctx)
			}

			h.WithdrawBalance(w, r)

			resp := w.Result()
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			if tt.expectedBody != "" {
				body, _ := io.ReadAll(resp.Body)
				assert.Contains(t, string(body), tt.expectedBody)
			}
		})
	}
}

type mockQueue struct {
	mock.Mock
}

func (m *mockQueue) EnqueueOrder(userID int, orderNum string) {
	m.Called(userID, orderNum)
}

func TestPostOrder(t *testing.T) {
	type testCase struct {
		name           string
		userID         int
		orderNum       string
		mockSetup      func(storage *mocks.StorageInterface, queue *mocks.QueueInterface)
		expectedStatus int
		expectedBody   string
	}

	validToken, _ := auth.GenerateJWT(&models.User{ID: 1})

	tests := []testCase{
		{
			name:     "успешное добавление нового заказа",
			userID:   1,
			orderNum: "79927398713",
			mockSetup: func(storage *mocks.StorageInterface, queue *mocks.QueueInterface) {
				storage.On("IsOrderExists", mock.Anything, "79927398713").Return(0, nil)
				storage.On("SaveOrder", mock.Anything, 1, mock.Anything).Return(nil)
				queue.On("EnqueueOrder", 1, "79927398713").Return()
			},
			expectedStatus: http.StatusAccepted,
		},
		{
			name:     "заказ уже существует для этого пользователя",
			userID:   1,
			orderNum: "79927398713",
			mockSetup: func(storage *mocks.StorageInterface, queue *mocks.QueueInterface) {
				storage.On("IsOrderExists", mock.Anything, "79927398713").Return(1, nil)
				queue.On("EnqueueOrder", mock.Anything, mock.Anything).Maybe()
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:     "ошибка БД при проверке заказа",
			userID:   1,
			orderNum: "79927398713",
			mockSetup: func(storage *mocks.StorageInterface, queue *mocks.QueueInterface) {
				storage.On("IsOrderExists", mock.Anything, "79927398713").Return(0, errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "internal server error",
		},
		{
			name:     "некорректный формат номера заказа (Luhn)",
			userID:   1,
			orderNum: "12345",
			mockSetup: func(storage *mocks.StorageInterface, queue *mocks.QueueInterface) {
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedBody:   "invalid order number format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storageMock := new(mocks.StorageInterface)
			queueMock := new(mocks.QueueInterface)
			tt.mockSetup(storageMock, queueMock)

			h := &handler{
				storage: storageMock,
				queue:   queueMock,
			}

			r := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(tt.orderNum))
			w := httptest.NewRecorder()

			if tt.userID > 0 {
				r.Header.Set("Authorization", "Bearer "+validToken)
				ctx := context.WithValue(r.Context(), auth.UserIDKey, tt.userID)
				r = r.WithContext(ctx)
			}

			h.PostOrder(w, r)

			resp := w.Result()
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			if tt.expectedBody != "" {
				body, _ := io.ReadAll(resp.Body)
				assert.Contains(t, string(body), tt.expectedBody)
			}

			queueMock.AssertExpectations(t)
			storageMock.AssertExpectations(t)
		})
	}
}
