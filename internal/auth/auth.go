package auth

import (
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/scoring-service/pkg/logger"
	"github.com/scoring-service/pkg/models"
	"golang.org/x/crypto/bcrypt"
)

type Claims struct {
	UserID int `json:"user_id"`
	jwt.StandardClaims
}

const UserIDKey = "userID"
const secretKey string = "e1ed36f1c0092227653c46d94ea90bcd"

func ValidateJWT(tokenString string) (int, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {

		return []byte(secretKey), nil
	})

	if err != nil || !token.Valid {
		logger.Log.Error("неверный или просроченный токен")
		return 0, fmt.Errorf("неверный или просроченный токен")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		logger.Log.Error("не удалось извлечь данные из токена")
		return 0, fmt.Errorf("не удалось извлечь данные из токена")
	}

	return claims.UserID, nil
}
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		logger.Log.Error(err.Error())
		return "", fmt.Errorf("ошибка хеширования пароля: %v", err)
	}
	return string(hash), nil
}

func GenerateJWT(user *models.User) (string, error) {
	if user == nil {
		logger.Log.Error("передан пустой указатель")
		return "", fmt.Errorf("не удалось сгенерировать токен")

	}
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		logger.Log.Error(err.Error())
		return "", fmt.Errorf("не удалось сгенерировать токен: %v", err)
	}

	return tokenString, nil
}
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
