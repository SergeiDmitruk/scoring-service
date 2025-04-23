package auth

import (
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/scoring-service/pkg/models"
)

func TestIsValidLuhn(t *testing.T) {
	tests := []struct {
		number string
		valid  bool
	}{

		{"79927398713", true},
		{"4539578763621486", true},
		{"6011000990139424", true},
		{"378282246310005", true},

		{"79927398710", false},
		{"4539578763621487", false},
		{"1234567812345678", false},

		{"", false},
		{"0000000000000000", true},
		{"abcdefg", false},
		{"4111x11111111111", false},
	}
	for _, tt := range tests {
		t.Run(tt.number, func(t *testing.T) {
			result := IsValidLuhn(tt.number)
			if result != tt.valid {
				t.Errorf("IsValidLuhn(%q) = %v; want %v", tt.number, result, tt.valid)
			}
		})
	}
}

func TestGenerateJWT(t *testing.T) {
	tests := []struct {
		name          string
		user          *models.User
		expectErr     bool
		expectToken   bool
		expectUserID  int
		expectExpired bool
	}{
		{
			name:          "Valid user",
			user:          &models.User{ID: 123},
			expectErr:     false,
			expectToken:   true,
			expectUserID:  123,
			expectExpired: false,
		},
		{
			name:          "Nil user",
			user:          nil,
			expectErr:     true,
			expectToken:   false,
			expectUserID:  0,
			expectExpired: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateJWT(tt.user)

			if tt.expectErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.expectToken && token == "" {
				t.Errorf("expected token, got empty string")
			}
			if !tt.expectToken && token != "" {
				t.Errorf("expected empty token, got %q", token)
			}

			if token != "" {
				parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
					return []byte(secretKey), nil
				})
				if err != nil || !parsedToken.Valid {
					t.Errorf("token is invalid: %v", err)
				}

				claims, ok := parsedToken.Claims.(jwt.MapClaims)
				if !ok {
					t.Fatal("claims are not of type MapClaims")
				}

				if claims["user_id"] != float64(tt.expectUserID) {
					t.Errorf("expected user_id %d, got %v", tt.expectUserID, claims["user_id"])
				}

				exp, ok := claims["exp"].(float64)
				if !ok {
					t.Fatal("exp is not a valid float64")
				}
				expTime := time.Unix(int64(exp), 0)
				if !expTime.After(time.Now()) {
					t.Errorf("expected exp time to be in the future, got %v", expTime)
				}
			}
		})
	}
}
func TestValidateJWT(t *testing.T) {
	tests := []struct {
		name         string
		tokenString  string
		expectErr    bool
		expectedUser int
	}{
		{
			name: "Valid token",
			tokenString: func() string {
				claims := &Claims{
					UserID: 123,
					StandardClaims: jwt.StandardClaims{
						ExpiresAt: time.Now().Add(time.Hour).Unix(),
					},
				}
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				tokenString, _ := token.SignedString([]byte(secretKey))
				return tokenString
			}(),
			expectErr:    false,
			expectedUser: 123,
		},
		{
			name: "Expired token",
			tokenString: func() string {

				claims := &Claims{
					UserID: 123,
					StandardClaims: jwt.StandardClaims{
						ExpiresAt: time.Now().Add(-time.Hour).Unix(),
					},
				}
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				tokenString, _ := token.SignedString([]byte(secretKey))
				return tokenString
			}(),
			expectErr:    true,
			expectedUser: 0,
		},
		{
			name:         "Invalid token format",
			tokenString:  "invalid_token_string",
			expectErr:    true,
			expectedUser: 0,
		},
		{
			name: "Token with missing claims",
			tokenString: func() string {
				token := jwt.New(jwt.SigningMethodHS256)
				tokenString, _ := token.SignedString([]byte(secretKey))
				return tokenString
			}(),
			expectErr:    true,
			expectedUser: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID, err := ValidateJWT(tt.tokenString)

			if tt.expectErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if userID != tt.expectedUser {
				t.Errorf("expected user ID %d, got %d", tt.expectedUser, userID)
			}
		})
	}
}
