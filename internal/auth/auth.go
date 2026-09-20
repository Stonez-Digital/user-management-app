package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var Secret []byte

const (
	AccessTokenTTL = 24 * time.Hour
	RefreshTokenTTL = 7 * 24 * time.Hour
)

type Claims struct {
	UserID string `json:"user_id"`
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func ConfigureSecret(secret string) error {
	secret = strings.TrimSpace(secret)
	if len(secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	Secret = []byte(secret)
	return nil
}

func ParseToken(tokenString string) (*Claims, error) {
	if len(Secret) == 0 {
		return nil, errors.New("authentication secret is not configured")
	}
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("invalid signing method")
		}
		return Secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid || claims.UserID == "" {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func GenerateToken(userID, role string) (string, error) {
	if len(Secret) == 0 {
		return "", errors.New("authentication secret is not configured")
	}
	claims := Claims{UserID: userID, Role: role, RegisteredClaims: jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessTokenTTL)),
		IssuedAt: jwt.NewNumericDate(time.Now()),
	}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(Secret)
}

func HashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func GenerateRefreshToken(userID string) (string, error) {
	if len(Secret) == 0 {
		return "", errors.New("authentication secret is not configured")
	}
	jtiBytes := make([]byte, 16)
	if _, err := rand.Read(jtiBytes); err != nil {
		return "", fmt.Errorf("failed to generate refresh token id: %w", err)
	}
	claims := Claims{UserID: userID, Role: "refresh", RegisteredClaims: jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(RefreshTokenTTL)),
		IssuedAt: jwt.NewNumericDate(time.Now()),
		ID: hex.EncodeToString(jtiBytes),
	}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(Secret)
}
