package security

import (
	"errors"
	"fmt"
	"time"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JwtConfig struct {
	SecretKey []byte
	TTL       time.Duration
	Issuer    string
	Audience  string
}

type claims[T any] struct {
	Data T `json:"data"`
	jwt.RegisteredClaims
}
type JwtProvider[T any] struct {
	cfg    JwtConfig
	parser *jwt.Parser
}

func NewJwtProvider[T any](cfg JwtConfig) (core.JwtProvider[T], error) {
	if len(cfg.SecretKey) < 32 {
		return nil, fmt.Errorf("SecretKey must be at least 32 bytes long")
	}
	return &JwtProvider[T]{
		cfg: cfg,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithIssuer(cfg.Issuer),
			jwt.WithAudience(cfg.Audience),
			jwt.WithExpirationRequired(),
		),
	}, nil
}

func (jp *JwtProvider[T]) GenerateToken(data T) (string, error) {
	now := time.Now()
	claims := claims[T]{
		Data: data,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Issuer:    jp.cfg.Issuer,
			Audience:  jwt.ClaimStrings{jp.cfg.Audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(jp.cfg.TTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jp.cfg.SecretKey)
}

func (jp *JwtProvider[T]) ValidateToken(token string) (T, error) {
	var zero T
	claims := &claims[T]{}
	_, err := jp.parser.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return jp.cfg.SecretKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return zero, core.ErrTokenExpired
		}
		return zero, err
	}
	return claims.Data, nil
}

func (jp *JwtProvider[T]) TTL() time.Duration {
	return jp.cfg.TTL
}
