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
	passkeys *passkeyRepositoryMock
	verifier *webAuthnVerifierMock
}

const testCeremonyTTL = 5 * time.Minute

func setupAuthService(t *testing.T) (core.AuthService, *authDeps) {
	t.Helper()
	d := &authDeps{
		users:    &userRepositoryMock{},
		auth:     &authRepositoryMock{},
		password: &passwordEncryptionMock{},
		mfa:      &jwtProviderMock[core.MfaClaims]{},
		session:  &jwtProviderMock[core.SessionClaims]{},
		passkeys: &passkeyRepositoryMock{},
		verifier: &webAuthnVerifierMock{},
	}
	t.Cleanup(func() {
		d.users.AssertExpectations(t)
		d.auth.AssertExpectations(t)
		d.password.AssertExpectations(t)
		d.mfa.AssertExpectations(t)
		d.session.AssertExpectations(t)
		d.passkeys.AssertExpectations(t)
		d.verifier.AssertExpectations(t)
	})
	return services.NewAuthService(d.users, d.auth, d.passkeys, d.password, d.mfa, d.session, d.verifier, testCeremonyTTL), d
}

// passkeysOf monta n passkeys do usuário; o conteúdo não importa para o Login, só a quantidade.
func passkeysOf(userID uuid.UUID, n int) []*core.Passkey {
	passkeys := make([]*core.Passkey, n)
	for i := range passkeys {
		passkeys[i] = &core.Passkey{ID: []byte{byte(i + 1)}, UserID: userID, Data: []byte(`{}`)}
	}
	return passkeys
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
		d.passkeys.On("ListByUser", ctx, user.ID).Return(passkeysOf(user.ID, 1), nil)
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
		d.passkeys.On("ListByUser", ctx, user.ID).Return(passkeysOf(user.ID, 1), nil)
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
		d.passkeys.On("ListByUser", ctx, user.ID).Return(passkeysOf(user.ID, 1), nil)
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
		d.passkeys.On("ListByUser", ctx, user.ID).Return(passkeysOf(user.ID, 1), nil)
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
		d.passkeys.On("ListByUser", ctx, user.ID).Return(passkeysOf(user.ID, 1), nil)
		d.mfa.On("GenerateToken", mock.Anything).Return("mfa-token", nil)
		d.mfa.On("TTL").Return(10 * time.Minute)
		d.auth.On("SetTemporaryToken", ctx, user.ID, "mfa-token", 10*time.Minute).Return(errBoom)

		got, err := svc.Login(ctx, user.Email, "senha")

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, got)
	})
}

// O next_step diz ao cliente qual cerimônia vem depois da senha. É só uma dica: quem
// garante a D1 (primeira passkey só com mfa_token) é o register/begin, consultando o banco de novo.
func TestAuthServiceLoginNextStep(t *testing.T) {
	ctx := context.Background()

	expectSuccessfulLogin := func(d *authDeps, user *core.User, passkeys []*core.Passkey) {
		d.users.On("GetUserByEmail", ctx, user.Email).Return(user, nil)
		d.password.On("Verify", "senha", user.Password, user.PepperVersion).Return(true, nil)
		d.passkeys.On("ListByUser", ctx, user.ID).Return(passkeys, nil).Once()
		d.mfa.On("GenerateToken", core.MfaClaims{UserID: user.ID}).Return("mfa-token", nil)
		d.mfa.On("TTL").Return(10 * time.Minute)
		d.auth.On("SetTemporaryToken", ctx, user.ID, "mfa-token", 10*time.Minute).Return(nil)
	}

	cases := []struct {
		name     string
		passkeys func(userID uuid.UUID) []*core.Passkey
		want     core.AuthStep
	}{
		{"sem passkey (lista vazia) vai para o cadastro", func(id uuid.UUID) []*core.Passkey { return []*core.Passkey{} }, core.StepWebAuthnRegister},
		{"sem passkey (nil) vai para o cadastro", func(id uuid.UUID) []*core.Passkey { return nil }, core.StepWebAuthnRegister},
		{"com uma passkey vai para o login", func(id uuid.UUID) []*core.Passkey { return passkeysOf(id, 1) }, core.StepWebAuthn},
		{"com várias passkeys vai para o login", func(id uuid.UUID) []*core.Passkey { return passkeysOf(id, 3) }, core.StepWebAuthn},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, d := setupAuthService(t)
			user := storedUser()
			expectSuccessfulLogin(d, user, tc.passkeys(user.ID))

			got, err := svc.Login(ctx, user.Email, "senha")

			require.NoError(t, err)
			assert.Equal(t, tc.want, got.NextStep)
			assert.Equal(t, "mfa-token", got.Token, "o mfa_token é o mesmo nos dois passos")
		})
	}

	// Antes da senha, o next_step contaria a qualquer um se o e-mail tem passkey.
	t.Run("senha errada não consulta as passkeys", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		d.users.On("GetUserByEmail", ctx, user.Email).Return(user, nil)
		d.password.On("Verify", "errada", user.Password, user.PepperVersion).Return(false, nil)

		_, err := svc.Login(ctx, user.Email, "errada")

		assert.ErrorIs(t, err, core.ErrInvalidCredentials)
		d.passkeys.AssertNotCalled(t, "ListByUser", mock.Anything, mock.Anything)
	})

	t.Run("e-mail inexistente não consulta as passkeys", func(t *testing.T) {
		svc, d := setupAuthService(t)
		d.users.On("GetUserByEmail", ctx, "x@example.com").Return(nil, core.ErrUserNotFound)
		d.password.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Maybe()

		_, err := svc.Login(ctx, "x@example.com", "senha")

		assert.ErrorIs(t, err, core.ErrInvalidCredentials)
		d.passkeys.AssertNotCalled(t, "ListByUser", mock.Anything, mock.Anything)
	})

	t.Run("erro ao listar passkeys é propagado e nenhum token é emitido nem gravado", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		d.users.On("GetUserByEmail", ctx, user.Email).Return(user, nil)
		d.password.On("Verify", "senha", user.Password, user.PepperVersion).Return(true, nil)
		d.passkeys.On("ListByUser", ctx, user.ID).Return(nil, errBoom)

		got, err := svc.Login(ctx, user.Email, "senha")

		assert.ErrorIs(t, err, errBoom)
		assert.NotErrorIs(t, err, core.ErrInvalidCredentials, "falha de banco é 500, não 401")
		assert.Nil(t, got)
		d.mfa.AssertNotCalled(t, "GenerateToken", mock.Anything)
		d.auth.AssertNotCalled(t, "SetTemporaryToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}
