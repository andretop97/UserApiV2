package services

import (
	"context"
	"errors"

	"github.com/andretop97/UserApiV2/src/core"
)

type AuthService struct {
	userRepository     core.UserRepository
	authRepository     core.AuthRepository
	passkeyRepository  core.PasskeyRepository
	passwordEncryption core.PasswordEncryption
	mfaTokens          core.JwtProvider[core.MfaClaims]
	sessionTokens      core.JwtProvider[core.SessionClaims]
}

func NewAuthService(
	userRepository core.UserRepository,
	authRepository core.AuthRepository,
	passkeyRepository core.PasskeyRepository,
	passwordEncryption core.PasswordEncryption,
	mfaTokens core.JwtProvider[core.MfaClaims],
	sessionTokens core.JwtProvider[core.SessionClaims],
) core.AuthService {
	return &AuthService{
		userRepository:     userRepository,
		authRepository:     authRepository,
		passkeyRepository:  passkeyRepository,
		passwordEncryption: passwordEncryption,
		mfaTokens:          mfaTokens,
		sessionTokens:      sessionTokens,
	}
}

func (as *AuthService) Login(ctx context.Context, email, password string) (*core.LoginResult, error) {
	email = core.NormalizeEmail(email)
	user, err := as.userRepository.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, core.ErrUserNotFound) {
			return nil, core.ErrInvalidCredentials
		}
		return nil, err
	}
	if user == nil {
		return nil, core.ErrInvalidCredentials
	}
	isMatch, err := as.passwordEncryption.Verify(password, user.Password, user.PepperVersion)
	if err != nil {
		return nil, err
	}
	if !isMatch {
		return nil, core.ErrInvalidCredentials
	}

	passkeys, err := as.passkeyRepository.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	token, err := as.mfaTokens.GenerateToken(core.MfaClaims{UserID: user.ID})
	if err != nil {
		return nil, err
	}

	err = as.authRepository.SetTemporaryToken(ctx, user.ID, token, as.mfaTokens.TTL())
	if err != nil {
		return nil, err
	}

	result := core.LoginResult{
		Token:     token,
		ExpiresIn: as.mfaTokens.TTL(),
		NextStep:  nextLoginStep(passkeys),
	}
	return &result, nil
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

func nextLoginStep(passkeys []*core.Passkey) core.AuthStep {
	if len(passkeys) == 0 {
		return core.StepWebAuthnRegister
	}
	return core.StepWebAuthn
}
