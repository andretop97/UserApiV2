package services

import (
	"context"

	"github.com/andretop97/UserApiV2/src/core"
)

type AuthService struct {
	userRepository     core.UserRepository
	passwordEncryption core.PasswordEncryption
}

func NewAuthService(userRepository core.UserRepository, passwordEncryption core.PasswordEncryption) core.AuthService {
	return &AuthService{
		userRepository:     userRepository,
		passwordEncryption: passwordEncryption,
	}
}

func (as *AuthService) Login(ctx context.Context, email, password string) (token string, err error) {
	user, err := as.userRepository.GetUserByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", core.ErrUserNotFound
	}
	isMatch, err := as.passwordEncryption.Verify(password, user.Password, user.PepperVersion)
	if err != nil {
		return "", err
	}
	if !isMatch {
		return "", core.ErrInvalidCredentials
	}

	return user.Name, nil
}

func (as *AuthService) RegisterWebAuthnBegin(ctx context.Context, email string) (string, error) {
	return "", nil
}

func (as *AuthService) RegisterWebAuthnFinish(ctx context.Context, email, credential string) error {
	return nil
}

func (as *AuthService) WebAuthnBegin(ctx context.Context, email string) (string, error) {
	return "", nil
}

func (as *AuthService) WebAuthnFinish(ctx context.Context, email, credential string) error {
	return nil
}
