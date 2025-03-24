package auth

import (
	"fmt"

	"github.com/dgrijalva/jwt-go"
)

type Claims struct {
	UserID int `json:"user_id"`
	jwt.StandardClaims
}

func ValidateJWT(tokenString string) (int, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {

		return []byte("your_secret_key"), nil
	})

	if err != nil || !token.Valid {
		return 0, fmt.Errorf("неверный или просроченный токен")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return 0, fmt.Errorf("не удалось извлечь данные из токена")
	}

	return claims.UserID, nil
}
