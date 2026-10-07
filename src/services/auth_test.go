package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/services"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type authDeps struct {
	users    *userRepositoryMock
	auth     *authRepositoryMock
	password *passwordEncryptionMock
	mfa      *jwtProviderMock[core.MfaClaims]
	session  *jwtProviderMock[core.SessionClaims]
}

func setupAuthService(t *testing.T) (core.AuthService, *authDeps) {
	t.Helper()
	d := &authDeps{
		users:    &userRepositoryMock{},
		auth:     &authRepositoryMock{},
		password: &passwordEncryptionMock{},
		mfa:      &jwtProviderMock[core.MfaClaims]{},
		session:  &jwtProviderMock[core.SessionClaims]{},
	}
	t.Cleanup(func() {
		d.users.AssertExpectations(t)
		d.auth.AssertExpectations(t)
		d.password.AssertExpectations(t)
		d.mfa.AssertExpectations(t)
		d.session.AssertExpectations(t)
	})
	return services.NewAuthService(d.users, d.auth, d.password, d.mfa, d.session), d
}

func storedUser() *core.User {
	return &core.User{
		ID:            uuid.New(),
		Name:          "João",
		Email:         "joao@example.com",
		Password:      "$argon2id$hash",
		PepperVersion: 2,
	}
}

func TestAuthServiceLogin(t *testing.T) {
	ctx := context.Background()

	t.Run("sucesso emite token MFA e grava no repositório", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		d.users.On("GetUserByEmail", ctx, user.Email).Return(user, nil)
		d.password.On("Verify", "senha", user.Password, user.PepperVersion).Return(true, nil)
		d.mfa.On("GenerateToken", core.MfaClaims{UserID: user.ID}).Return("mfa-token", nil)
		d.mfa.On("TTL").Return(10 * time.Minute)
		d.auth.On("SetTemporaryToken", ctx, user.ID, "mfa-token", 10*time.Minute).Return(nil)

		got, err := svc.Login(ctx, user.Email, "senha")

		require.NoError(t, err)
		assert.Equal(t, &core.LoginResult{
			Token:     "mfa-token",
			ExpiresIn: 10 * time.Minute,
			NextStep:  core.StepWebAuthn,
		}, got)
	})

	t.Run("nunca emite token de sessão no login", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		d.users.On("GetUserByEmail", ctx, user.Email).Return(user, nil)
		d.password.On("Verify", "senha", user.Password, user.PepperVersion).Return(true, nil)
		d.mfa.On("GenerateToken", mock.Anything).Return("mfa-token", nil)
		d.mfa.On("TTL").Return(10 * time.Minute)
		d.auth.On("SetTemporaryToken", ctx, user.ID, "mfa-token", 10*time.Minute).Return(nil)

		_, err := svc.Login(ctx, user.Email, "senha")

		require.NoError(t, err)
		d.session.AssertNotCalled(t, "GenerateToken", mock.Anything)
		d.auth.AssertNotCalled(t, "SetSessionToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("normaliza o e-mail antes de buscar", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		d.users.On("GetUserByEmail", ctx, "joao@example.com").Return(user, nil)
		d.password.On("Verify", "senha", user.Password, user.PepperVersion).Return(true, nil)
		d.mfa.On("GenerateToken", mock.Anything).Return("mfa-token", nil)
		d.mfa.On("TTL").Return(10 * time.Minute)
		d.auth.On("SetTemporaryToken", ctx, user.ID, "mfa-token", 10*time.Minute).Return(nil)

		_, err := svc.Login(ctx, "  Joao@Example.COM ", "senha")

		require.NoError(t, err)
	})

	t.Run("e-mail inexistente vira ErrInvalidCredentials", func(t *testing.T) {
		svc, d := setupAuthService(t)
		d.users.On("GetUserByEmail", ctx, "x@example.com").Return(nil, core.ErrUserNotFound)
		d.password.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Maybe()

		got, err := svc.Login(ctx, "x@example.com", "senha")

		assert.ErrorIs(t, err, core.ErrInvalidCredentials)
		assert.NotErrorIs(t, err, core.ErrUserNotFound)
		assert.Nil(t, got)
	})

	t.Run("repositório devolvendo nil sem erro vira ErrInvalidCredentials", func(t *testing.T) {
		svc, d := setupAuthService(t)
		d.users.On("GetUserByEmail", ctx, "x@example.com").Return(nil, nil)
		d.password.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Maybe()

		got, err := svc.Login(ctx, "x@example.com", "senha")

		assert.ErrorIs(t, err, core.ErrInvalidCredentials)
		assert.Nil(t, got)
	})

	t.Run("senha errada vira ErrInvalidCredentials e não emite token", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		d.users.On("GetUserByEmail", ctx, user.Email).Return(user, nil)
		d.password.On("Verify", "errada", user.Password, user.PepperVersion).Return(false, nil)

		got, err := svc.Login(ctx, user.Email, "errada")

		assert.ErrorIs(t, err, core.ErrInvalidCredentials)
		assert.Nil(t, got)
	})

	t.Run("e-mail inexistente e senha errada devolvem exatamente o mesmo erro", func(t *testing.T) {
		svcA, a := setupAuthService(t)
		a.users.On("GetUserByEmail", ctx, "x@example.com").Return(nil, core.ErrUserNotFound)
		a.password.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Maybe()
		_, errNotFound := svcA.Login(ctx, "x@example.com", "senha")

		svcB, b := setupAuthService(t)
		user := storedUser()
		b.users.On("GetUserByEmail", ctx, user.Email).Return(user, nil)
		b.password.On("Verify", "errada", user.Password, user.PepperVersion).Return(false, nil)
		_, errWrongPassword := svcB.Login(ctx, user.Email, "errada")

		assert.Same(t, errWrongPassword, errNotFound)
	})

	// Sem isso, e-mail inexistente responde em ~5 ms e existente em ~100 ms (Argon2),
	// e dá para enumerar e-mails medindo o tempo de resposta.
	t.Run("e-mail inexistente também roda o hash de senha (tempo constante)", func(t *testing.T) {
		svc, d := setupAuthService(t)
		d.users.On("GetUserByEmail", ctx, "x@example.com").Return(nil, core.ErrUserNotFound)
		d.password.On("Verify", "senha", mock.Anything, mock.Anything).Return(false, nil).Once()

		_, err := svc.Login(ctx, "x@example.com", "senha")

		assert.ErrorIs(t, err, core.ErrInvalidCredentials)
	})

	t.Run("falha de infraestrutura no repositório é propagada, não vira 401", func(t *testing.T) {
		svc, d := setupAuthService(t)
		d.users.On("GetUserByEmail", ctx, "x@example.com").Return(nil, errBoom)

		got, err := svc.Login(ctx, "x@example.com", "senha")

		assert.ErrorIs(t, err, errBoom)
		assert.NotErrorIs(t, err, core.ErrInvalidCredentials)
		assert.Nil(t, got)
	})

	t.Run("erro ao verificar a senha é propagado", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		d.users.On("GetUserByEmail", ctx, user.Email).Return(user, nil)
		d.password.On("Verify", "senha", user.Password, user.PepperVersion).Return(false, errBoom)

		got, err := svc.Login(ctx, user.Email, "senha")

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, got)
	})

	t.Run("erro ao gerar o token é propagado e nada é gravado", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		d.users.On("GetUserByEmail", ctx, user.Email).Return(user, nil)
		d.password.On("Verify", "senha", user.Password, user.PepperVersion).Return(true, nil)
		d.mfa.On("GenerateToken", mock.Anything).Return("", errBoom)

		got, err := svc.Login(ctx, user.Email, "senha")

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, got)
	})

	t.Run("erro ao gravar o token é propagado e o token não é devolvido", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		d.users.On("GetUserByEmail", ctx, user.Email).Return(user, nil)
		d.password.On("Verify", "senha", user.Password, user.PepperVersion).Return(true, nil)
		d.mfa.On("GenerateToken", mock.Anything).Return("mfa-token", nil)
		d.mfa.On("TTL").Return(10 * time.Minute)
		d.auth.On("SetTemporaryToken", ctx, user.ID, "mfa-token", 10*time.Minute).Return(errBoom)

		got, err := svc.Login(ctx, user.Email, "senha")

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, got)
	})
}

// Os fluxos WebAuthn ainda são stubs. Estes testes garantem que eles não aceitem
// qualquer coisa enquanto não forem implementados.
func TestAuthServiceWebAuthn(t *testing.T) {
	ctx := context.Background()

	t.Run("WebAuthnBegin devolve um challenge", func(t *testing.T) {
		svc, d := setupAuthService(t)
		allowAnyLookup(d)

		challenge, err := svc.WebAuthnBegin(ctx, "joao@example.com")

		require.NoError(t, err)
		assert.NotEmpty(t, challenge)
	})

	t.Run("WebAuthnFinish rejeita credencial inválida", func(t *testing.T) {
		svc, d := setupAuthService(t)
		allowAnyLookup(d)

		err := svc.WebAuthnFinish(ctx, "joao@example.com", "credencial-invalida")

		assert.Error(t, err)
	})

	t.Run("RegisterWebAuthnBegin devolve um challenge", func(t *testing.T) {
		svc, d := setupAuthService(t)
		allowAnyLookup(d)

		challenge, err := svc.RegisterWebAuthnBegin(ctx, "joao@example.com")

		require.NoError(t, err)
		assert.NotEmpty(t, challenge)
	})

	t.Run("RegisterWebAuthnFinish rejeita credencial inválida", func(t *testing.T) {
		svc, d := setupAuthService(t)
		allowAnyLookup(d)

		err := svc.RegisterWebAuthnFinish(ctx, "joao@example.com", "credencial-invalida")

		assert.Error(t, err)
	})
}

// allowAnyLookup libera as dependências que uma implementação real do WebAuthn
// provavelmente consulta, para que os testes acima só verifiquem o resultado.
func allowAnyLookup(d *authDeps) {
	user := storedUser()
	d.users.On("GetUserByEmail", mock.Anything, mock.Anything).Return(user, nil).Maybe()
	d.users.On("GetUserByID", mock.Anything, mock.Anything).Return(user, nil).Maybe()
	d.auth.On("GetTemporaryToken", mock.Anything, mock.Anything).Return("", core.ErrUserNotFound).Maybe()
	d.mfa.On("ValidateToken", mock.Anything).Return(core.MfaClaims{}, core.ErrTokenExpired).Maybe()
}
