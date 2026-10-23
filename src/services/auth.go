package services

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"time"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/google/uuid"
)

type AuthService struct {
	userRepository     core.UserRepository
	authRepository     core.AuthRepository
	passkeyRepository  core.PasskeyRepository
	passwordEncryption core.PasswordEncryption
	mfaTokens          core.JwtProvider[core.MfaClaims]
	sessionTokens      core.JwtProvider[core.SessionClaims]
	webAuthnVerifier   core.WebAuthnVerifier
	ceremonyTTL        time.Duration
}

func NewAuthService(
	userRepository core.UserRepository,
	authRepository core.AuthRepository,
	passkeyRepository core.PasskeyRepository,
	passwordEncryption core.PasswordEncryption,
	mfaTokens core.JwtProvider[core.MfaClaims],
	sessionTokens core.JwtProvider[core.SessionClaims],
	webAuthnVerifier core.WebAuthnVerifier,
	ceremonyTTL time.Duration,
) core.AuthService {
	return &AuthService{
		userRepository:     userRepository,
		authRepository:     authRepository,
		passkeyRepository:  passkeyRepository,
		passwordEncryption: passwordEncryption,
		mfaTokens:          mfaTokens,
		sessionTokens:      sessionTokens,
		webAuthnVerifier:   webAuthnVerifier,
		ceremonyTTL:        ceremonyTTL,
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

func (as *AuthService) RegisterWebAuthnBegin(ctx context.Context, mfaToken string) (json.RawMessage, error) {
	user, err := as.authenticateMfa(ctx, mfaToken)
	if err != nil {
		return nil, err
	}

	passkeys, err := as.passkeyRepository.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if !canRegisterFirstPasskey(passkeys) {
		return nil, core.ErrPasskeyRegistrationNotAllowed
	}

	options, session, err := as.webAuthnVerifier.BeginRegistration(user, passkeys)
	if err != nil {
		return nil, err
	}

	if err := as.authRepository.SaveCeremonySession(ctx, user.ID, core.CeremonyRegistration, session, as.ceremonyTTL); err != nil {
		return nil, err
	}
	return options, nil
}

func (as *AuthService) RegisterWebAuthnFinish(ctx context.Context, mfaToken string, response []byte) (*core.SessionResult, error) {
	user, err := as.authenticateMfa(ctx, mfaToken)
	if err != nil {
		return nil, err
	}

	session, err := as.authRepository.ConsumeCeremonySession(ctx, user.ID, core.CeremonyRegistration)
	if err != nil {
		return nil, err
	}

	passkeys, err := as.passkeyRepository.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if !canRegisterFirstPasskey(passkeys) {
		return nil, core.ErrPasskeyRegistrationNotAllowed
	}

	passkey, err := as.webAuthnVerifier.FinishRegistration(user, passkeys, session, response)
	if err != nil {
		return nil, err
	}
	if err := as.passkeyRepository.Create(ctx, passkey); err != nil {
		return nil, err
	}

	if err := as.authRepository.DeleteTemporaryToken(ctx, user.ID); err != nil {
		return nil, err
	}
	return as.issueSession(ctx, user.ID)
}

func (as *AuthService) WebAuthnBegin(ctx context.Context, mfaToken string) (json.RawMessage, error) {
	user, err := as.authenticateMfa(ctx, mfaToken)
	if err != nil {
		return nil, err
	}

	passkeys, err := as.passkeyRepository.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if len(passkeys) == 0 {
		return nil, core.ErrPasskeyNotFound
	}

	options, session, err := as.webAuthnVerifier.BeginLogin(user, passkeys)
	if err != nil {
		return nil, err
	}
	if err := as.authRepository.SaveCeremonySession(ctx, user.ID, core.CeremonyLogin, session, as.ceremonyTTL); err != nil {
		return nil, err
	}
	return options, nil
}

func (as *AuthService) WebAuthnFinish(ctx context.Context, mfaToken string, response []byte) (*core.SessionResult, error) {
	user, err := as.authenticateMfa(ctx, mfaToken)
	if err != nil {
		return nil, err
	}

	session, err := as.authRepository.ConsumeCeremonySession(ctx, user.ID, core.CeremonyLogin)
	if err != nil {
		return nil, err
	}

	passkeys, err := as.passkeyRepository.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	passkey, err := as.webAuthnVerifier.FinishLogin(user, passkeys, session, response)
	if err != nil {
		return nil, err
	}
	if err := as.passkeyRepository.UpdateAfterLogin(ctx, passkey.ID, passkey.Data); err != nil {
		return nil, err
	}

	if err := as.authRepository.DeleteTemporaryToken(ctx, user.ID); err != nil {
		return nil, err
	}
	return as.issueSession(ctx, user.ID)
}

func nextLoginStep(passkeys []*core.Passkey) core.AuthStep {
	if len(passkeys) == 0 {
		return core.StepWebAuthnRegister
	}
	return core.StepWebAuthn
}

func (as *AuthService) authenticateMfa(ctx context.Context, token string) (*core.User, error) {
	claims, err := as.mfaTokens.ValidateToken(token)
	if err != nil {
		return nil, err // já vem ErrTokenInvalid ou ErrTokenExpired
	}

	stored, err := as.authRepository.GetTemporaryToken(ctx, claims.UserID)
	if err != nil {
		return nil, err // ErrTokenInvalid se não existe; erro de infra se o Redis falhou
	}
	if subtle.ConstantTimeCompare([]byte(stored), []byte(token)) != 1 {
		return nil, core.ErrTokenInvalid
	}

	user, err := as.userRepository.GetUserByID(ctx, claims.UserID)
	if errors.Is(err, core.ErrUserNotFound) {
		return nil, core.ErrTokenInvalid // usuário apagado depois do login
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func canRegisterFirstPasskey(passkeys []*core.Passkey) bool {
	return len(passkeys) == 0
}

func (as *AuthService) issueSession(ctx context.Context, userID uuid.UUID) (*core.SessionResult, error) {
	token, err := as.sessionTokens.GenerateToken(core.SessionClaims{UserID: userID})
	if err != nil {
		return nil, err
	}
	if err := as.authRepository.SetSessionToken(ctx, userID, token, as.sessionTokens.TTL()); err != nil {
		return nil, err
	}
	return &core.SessionResult{Token: token, ExpiresIn: as.sessionTokens.TTL()}, nil
}
