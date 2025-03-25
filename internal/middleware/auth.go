package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/scoring-service/internal/auth"
	"github.com/scoring-service/pkg/logger"
)

type contextKey string

const userIDKey contextKey = "userID"

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		tokenString, err := getTokenFromRequest(r)
		if err != nil {
			logger.Log.Error(err.Error())
			http.Error(w, "Пользователь не авторизован", http.StatusUnauthorized)
			return
		}

		userID, err := auth.ValidateJWT(tokenString)
		if err != nil {
			logger.Log.Error(err.Error())
			http.Error(w, "Неудачная аутентификация", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUserIDFromContext(ctx context.Context) (int, error) {
	userID, ok := ctx.Value(userIDKey).(int)
	if !ok {
		logger.Log.Error("не удалось извлечь userID из контекста")
		return 0, fmt.Errorf("не удалось извлечь userID из контекста")
	}
	return userID, nil
}

func getTokenFromRequest(r *http.Request) (string, error) {

	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {

		split := strings.Split(authHeader, "Bearer ")
		if len(split) != 2 {
			logger.Log.Error("неверный формат заголовка Authorization")
			return "", fmt.Errorf("неверный формат заголовка Authorization")
		}
		return split[1], nil
	}
	logger.Log.Error("не найден токен авторизации")
	return "", fmt.Errorf("не найден токен авторизации")
}
