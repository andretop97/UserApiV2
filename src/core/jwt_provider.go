package core

type JwtProvider interface {
	GenerateToken(userID string) (string, error)
	ValidateToken(token string) (string, error)
}
