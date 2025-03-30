package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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
		mockSetup      func(storage *mocks.StorageInterface, auth *mocks.AuthInterface)
		expectedStatus int
		expectedBody   string
	}

	tests := []testCase{
		{
			name: "успешная авторизация",
			inputUser: models.User{
				Login:    "testuser",
				Password: "password123",
			},
			mockSetup: func(storage *mocks.StorageInterface, auth *mocks.AuthInterface) {
				storage.On("GetUserByLogin", mock.Anything, "testuser").Return(&models.User{
					ID:       1,
					Login:    "testuser",
					Password: "$2a$10$A4ZzYopqPweqKGHpmhPbXcYzxt.AZHs5hOZ1I/x8Cz6gAVO65v9m6",
				}, nil)

				auth.On("CheckPasswordHash", "password123", mock.Anything).Return(true)
				auth.On("GenerateJWT", mock.AnythingOfType("*models.User")).Return("valid_token", nil)
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
			mockSetup: func(storage *mocks.StorageInterface, auth *mocks.AuthInterface) {
				storage.On("GetUserByLogin", mock.Anything, "testuser").Return(&models.User{
					ID:       1,
					Login:    "testuser",
					Password: "$2a$10$A4ZzYopqPweqKGHpmhPbXcYzxt.AZHs5hOZ1I/x8Cz6gAVO65v9m6",
				}, nil)

				auth.On("CheckPasswordHash", "wrongpassword", mock.Anything).Return(false)
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Неверная пара логин/пароль",
		},
		{
			name: "пользователь не найден",
			inputUser: models.User{
				Login:    "unknownuser",
				Password: "password123",
			},
			mockSetup: func(storage *mocks.StorageInterface, auth *mocks.AuthInterface) {
				storage.On("GetUserByLogin", mock.Anything, "unknownuser").Return(nil, errors.New("пользователь не найден"))
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Неверная пара логин/пароль",
		},
		{
			name: "ошибка при генерации токена",
			inputUser: models.User{
				Login:    "testuser",
				Password: "password123",
			},
			mockSetup: func(storage *mocks.StorageInterface, auth *mocks.AuthInterface) {
				storage.On("GetUserByLogin", mock.Anything, "testuser").Return(&models.User{
					ID:       1,
					Login:    "testuser",
					Password: "$2a$10$A4ZzYopqPweqKGHpmhPbXcYzxt.AZHs5hOZ1I/x8Cz6gAVO65v9m6",
				}, nil)

				auth.On("CheckPasswordHash", "password123", mock.Anything).Return(true)
				auth.On("GenerateJWT", mock.AnythingOfType("*models.User")).Return("", errors.New("ошибка генерации токена"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Ошибка при генерации токена",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Мокаем зависимости
			storageMock := new(mocks.StorageInterface)
			authMock := new(mocks.AuthInterface)
			tt.mockSetup(storageMock, authMock)

			// Создаем обработчик с замоканными зависимостями
			h := NewHandler(storageMock)

			// Подготавливаем запрос
			body, _ := json.Marshal(tt.inputUser)
			r := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
			w := httptest.NewRecorder()

			// Вызываем хендлер
			h.Login(w, r)

			// Проверяем статус и тело ответа
			resp := w.Result()
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
			responseBody := w.Body.String()
			assert.Contains(t, responseBody, tt.expectedBody)
		})
	}
}
