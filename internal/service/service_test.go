package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/scoring-service/internal/auth"
	"github.com/scoring-service/pkg/models"
)

func TestFetchAccrual(t *testing.T) {
	mockDB := NewMockStorage(t)
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
		{
			name: "Retry-After заголовок",
			setupServer: func() *httptest.Server {
				attempts := 0
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if attempts == 0 {
						w.Header().Set("Retry-After", "1")
						w.WriteHeader(http.StatusTooManyRequests)
						attempts++
						return
					}
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(models.AccrualResponse{Status: models.OrderProcessed})
				}))
			},
			expectedErr:  "",
			expectedCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			service.apiURL = server.URL

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if tt.expectedCall {
				mockDB.EXPECT().UpdateOrder(mock.Anything, mock.Anything).Return(nil).Once()
			}

			err := service.FetchAccrual(ctx, "123456")

			if tt.expectedErr == "" && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.expectedErr != "" && (err == nil || !strings.Contains(err.Error(), tt.expectedErr)) {
				t.Errorf("expected error: %v, got: %v", tt.expectedErr, err)
			}
		})
	}
}
func TestUserExist(t *testing.T) {
	mockDB := NewMockStorage(t)
	service := &AccrualService{db: mockDB}

	tests := []struct {
		name        string
		login       string
		setupMock   func()
		expectedRes bool
		expectedErr error
	}{
		{
			name:  "пользователь найден",
			login: "user1",
			setupMock: func() {
				mockDB.EXPECT().
					GetUserByLogin(mock.Anything, "user1").
					Return(&models.User{Login: "user1"}, nil).
					Once()
			},
			expectedRes: true,
			expectedErr: nil,
		},
		{
			name:  "пользователь не найден",
			login: "unknown",
			setupMock: func() {
				mockDB.EXPECT().
					GetUserByLogin(mock.Anything, "unknown").
					Return(nil, nil).
					Once()
			},
			expectedRes: false,
			expectedErr: nil,
		},
		{
			name:  "ошибка при запросе",
			login: "error_user",
			setupMock: func() {
				mockDB.EXPECT().
					GetUserByLogin(mock.Anything, "error_user").
					Return(nil, errors.New("db error")).
					Once()
			},
			expectedRes: false,
			expectedErr: errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			res, err := service.UserExist(context.Background(), tt.login)

			assert.Equal(t, tt.expectedRes, res)
			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
func TestRegisterUser(t *testing.T) {
	mockDB := NewMockStorage(t)
	service := &AccrualService{db: mockDB}

	tests := []struct {
		name        string
		user        *models.User
		mockDBFunc  func()
		wantErr     bool
		errContains string
	}{
		{
			name: "успешная регистрация",
			user: &models.User{Login: "test", Password: "plainpass"},
			mockDBFunc: func() {
				mockDB.EXPECT().
					CreateUser(mock.Anything, mock.MatchedBy(func(u *models.User) bool {
						return u.Login == "test" && u.Password != "plainpass"
					})).
					Return(nil).
					Once()
			},
			wantErr: false,
		},
		{
			name: "ошибка хеширования пароля",
			user: &models.User{Login: "fail", Password: string(make([]byte, 1000000))},
			mockDBFunc: func() {
			},
			wantErr:     true,
			errContains: "bcrypt",
		},
		{
			name: "ошибка создания пользователя в БД",
			user: &models.User{Login: "dberror", Password: "pass123"},
			mockDBFunc: func() {
				mockDB.EXPECT().
					CreateUser(mock.Anything, mock.Anything).
					Return(errors.New("db error")).
					Once()
			},
			wantErr:     true,
			errContains: "db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockDBFunc()

			err := service.ReagisterUser(context.Background(), tt.user)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
func TestAuthorizeUser(t *testing.T) {
	mockDB := NewMockStorage(t)
	service := &AccrualService{db: mockDB}

	hashedPassword, _ := auth.HashPassword("correctpassword")

	tests := []struct {
		name        string
		inputUser   *models.User
		storedUser  *models.User
		dbErr       error
		wantErr     bool
		errContains string
	}{
		{
			name:      "успешная авторизация",
			inputUser: &models.User{Login: "test", Password: "correctpassword"},
			storedUser: &models.User{
				Login:    "test",
				Password: hashedPassword,
			},
			wantErr: false,
		},
		{
			name:      "неверный пароль",
			inputUser: &models.User{Login: "test", Password: "wrongpassword"},
			storedUser: &models.User{
				Login:    "test",
				Password: hashedPassword,
			},
			wantErr:     true,
			errContains: "неверная пара логин/пароль",
		},
		{
			name:        "ошибка из БД",
			inputUser:   &models.User{Login: "dberror", Password: "pass"},
			storedUser:  nil,
			dbErr:       errors.New("db failure"),
			wantErr:     true,
			errContains: "db failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.dbErr != nil {
				mockDB.EXPECT().
					GetUserByLogin(mock.Anything, tt.inputUser.Login).
					Return(nil, tt.dbErr).
					Once()
			} else {
				mockDB.EXPECT().
					GetUserByLogin(mock.Anything, tt.inputUser.Login).
					Return(tt.storedUser, nil).
					Once()
			}

			err := service.AuthorizeUser(context.Background(), tt.inputUser)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
func TestCreateOrder(t *testing.T) {
	mockDB := NewMockStorage(t)
	service := &AccrualService{db: mockDB}

	validOrder := "79927398713"

	tests := []struct {
		name           string
		userID         int
		orderNum       string
		existingUser   int
		isOrderErr     error
		saveOrderErr   error
		expectedStatus CreateStatus
	}{
		{
			name:           "ошибка при проверке существования заказа",
			userID:         1,
			orderNum:       validOrder,
			isOrderErr:     errors.New("db error"),
			expectedStatus: StatusError,
		},
		{
			name:           "новый заказ сохраняется успешно",
			userID:         1,
			orderNum:       validOrder,
			existingUser:   0,
			expectedStatus: StatusOK,
		},
		{
			name:           "ошибка при сохранении заказа",
			userID:         1,
			orderNum:       validOrder,
			existingUser:   0,
			saveOrderErr:   errors.New("save error"),
			expectedStatus: StatusError,
		},
		{
			name:           "заказ уже принадлежит другому пользователю",
			userID:         1,
			orderNum:       validOrder,
			existingUser:   2,
			expectedStatus: StatusConflict,
		},
		{
			name:           "заказ уже существует и принадлежит этому пользователю",
			userID:         1,
			orderNum:       validOrder,
			existingUser:   1,
			expectedStatus: StatusAlreadyExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB.On("IsOrderExists", mock.Anything, tt.orderNum).
				Return(tt.existingUser, tt.isOrderErr).
				Once()

			if tt.existingUser == 0 && tt.isOrderErr == nil {
				mockDB.On("SaveOrder", mock.Anything, tt.userID, mock.Anything).
					Return(tt.saveOrderErr).
					Once()
			}

			status := service.CreateOrder(context.Background(), tt.userID, tt.orderNum)
			require.Equal(t, tt.expectedStatus, status)

			mockDB.AssertExpectations(t)
		})
	}
}
func TestCreateWithdraw(t *testing.T) {
	mockDB := NewMockStorage(t)
	service := &AccrualService{db: mockDB}

	validOrder := "79927398713"

	tests := []struct {
		name           string
		userID         int
		withdraw       models.Withdraw
		balance        models.Balance
		balanceErr     error
		withdrawErr    error
		expectedStatus CreateStatus
	}{
		{
			name:   "некорректный номер заказа",
			userID: 1,
			withdraw: models.Withdraw{
				Order: "123456789",
				Sum:   100,
			},
			expectedStatus: StatusInvalid,
		},
		{
			name:   "ошибка при получении баланса",
			userID: 1,
			withdraw: models.Withdraw{
				Order: validOrder,
				Sum:   100,
			},
			balanceErr:     errors.New("db error"),
			expectedStatus: StatusError,
		},
		{
			name:   "недостаточно средств",
			userID: 1,
			withdraw: models.Withdraw{
				Order: validOrder,
				Sum:   200,
			},
			balance:        models.Balance{Current: 100},
			expectedStatus: StatusConflict,
		},
		{
			name:   "ошибка при списании",
			userID: 1,
			withdraw: models.Withdraw{
				Order: validOrder,
				Sum:   100,
			},
			balance:        models.Balance{Current: 200},
			withdrawErr:    errors.New("withdraw error"),
			expectedStatus: StatusError,
		},
		{
			name:   "успешное списание",
			userID: 1,
			withdraw: models.Withdraw{
				Order: validOrder,
				Sum:   100,
			},
			balance:        models.Balance{Current: 200},
			expectedStatus: StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if auth.IsValidLuhn(tt.withdraw.Order) {
				mockDB.On("GetUserBalance", mock.Anything, tt.userID).
					Return(tt.balance, tt.balanceErr).
					Once()

				if tt.balanceErr == nil && tt.balance.Current >= tt.withdraw.Sum {
					mockDB.On("Withdraw", mock.Anything, tt.userID, tt.withdraw.Order, tt.withdraw.Sum).
						Return(tt.withdrawErr).
						Once()
				}
			}

			status := service.CreateWithdraw(context.Background(), tt.userID, tt.withdraw)
			require.Equal(t, tt.expectedStatus, status)

			mockDB.AssertExpectations(t)
		})
	}
}
