package security

import (
	"fmt"
	"time"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/golang-jwt/jwt/v5"
)

type JwtProvider struct {
}

func NewJwtProvider() core.JwtProvider {
	return &JwtProvider{}
}

func (jp *JwtProvider) GenerateToken(payload string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"payload": payload,
		"exp":     time.Now().Add(time.Hour * 72).Unix(),
	})
	tokenString, err := token.SignedString([]byte("your-secret-key"))
	if err != nil {
		return "", err
	}
	return tokenString, err
}

func (jp *JwtProvider) ValidateToken(token string) (string, error) {
	queryToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte("your-secret-key"), nil
	})
	if err != nil {
		return "", err
	}
	if !queryToken.Valid {
		return "", fmt.Errorf("invalid token")
	}
	claims, ok := queryToken.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("failed to extract claims")
	}
	payload, ok := claims["payload"].(string)
	if !ok {
		return "", fmt.Errorf("failed to extract payload")
	}
	return payload, nil
}
