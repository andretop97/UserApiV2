package services_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Cadastro da primeira passkey (passo 5): o mfa_token do login autoriza o cadastro
// só enquanto o usuário não tem nenhuma passkey (D1), e o finish já emite a sessão (D2).

const (
	validMfaToken  = "mfa-token-vigente"
	sessionTTLTest = 72 * time.Hour
)

var (
	testOptions = json.RawMessage(`{"publicKey":{"challenge":"abc"}}`)
	testSession = []byte(`{"challenge":"abc"}`)
	testAnswer  = []byte(`{"id":"cred","response":{}}`)
)

// expectMfaAuth configura o caminho feliz do authenticateMfa: JWT válido, token vigente no Redis, usuário existe.
func expectMfaAuth(d *authDeps, user *core.User) {
	d.mfa.On("ValidateToken", validMfaToken).Return(core.MfaClaims{UserID: user.ID}, nil)
	d.auth.On("GetTemporaryToken", mock.Anything, user.ID).Return(validMfaToken, nil)
	d.users.On("GetUserByID", mock.Anything, user.ID).Return(user, nil)
}

// assertNothingIssued garante que nenhum efeito colateral de sucesso aconteceu.
func assertNothingIssued(t *testing.T, d *authDeps) {
	t.Helper()
	d.passkeys.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	d.auth.AssertNotCalled(t, "DeleteTemporaryToken", mock.Anything, mock.Anything)
	d.session.AssertNotCalled(t, "GenerateToken", mock.Anything)
	d.auth.AssertNotCalled(t, "SetSessionToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Os quatro métodos WebAuthn autenticam o mfa_token do mesmo jeito; os casos valem para todos.
func TestAuthServiceWebAuthnAuthentication(t *testing.T) {
	ctx := context.Background()

	calls := map[string]func(svc core.AuthService, token string) error{
		"register begin": func(svc core.AuthService, token string) error {
			options, err := svc.RegisterWebAuthnBegin(ctx, token)
			assert.Nil(t, options)
			return err
		},
		"register finish": func(svc core.AuthService, token string) error {
			session, err := svc.RegisterWebAuthnFinish(ctx, token, testAnswer)
			assert.Nil(t, session)
			return err
		},
		"login begin": func(svc core.AuthService, token string) error {
			options, err := svc.WebAuthnBegin(ctx, token)
			assert.Nil(t, options)
			return err
		},
		"login finish": func(svc core.AuthService, token string) error {
			session, err := svc.WebAuthnFinish(ctx, token, testAnswer)
			assert.Nil(t, session)
			return err
		},
	}

	for name, call := range calls {
		t.Run(name+": JWT inválido é recusado sem consultar Redis nem banco", func(t *testing.T) {
			svc, d := setupAuthService(t)
			d.mfa.On("ValidateToken", "forjado").Return(core.MfaClaims{}, core.ErrTokenInvalid)

			err := call(svc, "forjado")

			assert.ErrorIs(t, err, core.ErrTokenInvalid)
			d.auth.AssertNotCalled(t, "GetTemporaryToken", mock.Anything, mock.Anything)
		})

		t.Run(name+": JWT expirado é recusado", func(t *testing.T) {
			svc, d := setupAuthService(t)
			d.mfa.On("ValidateToken", "velho").Return(core.MfaClaims{}, core.ErrTokenExpired)

			assert.ErrorIs(t, call(svc, "velho"), core.ErrTokenExpired)
		})

		t.Run(name+": token sem registro no Redis é recusado", func(t *testing.T) {
			svc, d := setupAuthService(t)
			user := storedUser()
			d.mfa.On("ValidateToken", validMfaToken).Return(core.MfaClaims{UserID: user.ID}, nil)
			d.auth.On("GetTemporaryToken", mock.Anything, user.ID).Return("", core.ErrTokenInvalid)

			assert.ErrorIs(t, call(svc, validMfaToken), core.ErrTokenInvalid)
		})

		// Um login novo sobrescreve o token no Redis; o JWT antigo continua assinado, mas não vale mais.
		t.Run(name+": token de um login anterior é recusado", func(t *testing.T) {
			svc, d := setupAuthService(t)
			user := storedUser()
			d.mfa.On("ValidateToken", "token-antigo").Return(core.MfaClaims{UserID: user.ID}, nil)
			d.auth.On("GetTemporaryToken", mock.Anything, user.ID).Return(validMfaToken, nil)

			err := call(svc, "token-antigo")

			assert.ErrorIs(t, err, core.ErrTokenInvalid)
			d.users.AssertNotCalled(t, "GetUserByID", mock.Anything, mock.Anything)
		})

		t.Run(name+": usuário apagado depois do login vira token inválido, não 404", func(t *testing.T) {
			svc, d := setupAuthService(t)
			user := storedUser()
			d.mfa.On("ValidateToken", validMfaToken).Return(core.MfaClaims{UserID: user.ID}, nil)
			d.auth.On("GetTemporaryToken", mock.Anything, user.ID).Return(validMfaToken, nil)
			d.users.On("GetUserByID", mock.Anything, user.ID).Return(nil, core.ErrUserNotFound)

			err := call(svc, validMfaToken)

			assert.ErrorIs(t, err, core.ErrTokenInvalid)
			assert.NotErrorIs(t, err, core.ErrUserNotFound)
		})

		t.Run(name+": Redis fora do ar é erro interno, não 401", func(t *testing.T) {
			svc, d := setupAuthService(t)
			user := storedUser()
			d.mfa.On("ValidateToken", validMfaToken).Return(core.MfaClaims{UserID: user.ID}, nil)
			d.auth.On("GetTemporaryToken", mock.Anything, user.ID).Return("", errBoom)

			err := call(svc, validMfaToken)

			assert.ErrorIs(t, err, errBoom)
			assert.NotErrorIs(t, err, core.ErrTokenInvalid)
		})

		t.Run(name+": banco fora do ar ao buscar o usuário é erro interno", func(t *testing.T) {
			svc, d := setupAuthService(t)
			user := storedUser()
			d.mfa.On("ValidateToken", validMfaToken).Return(core.MfaClaims{UserID: user.ID}, nil)
			d.auth.On("GetTemporaryToken", mock.Anything, user.ID).Return(validMfaToken, nil)
			d.users.On("GetUserByID", mock.Anything, user.ID).Return(nil, errBoom)

			assert.ErrorIs(t, call(svc, validMfaToken), errBoom)
		})
	}
}

func TestAuthServiceRegisterWebAuthnBegin(t *testing.T) {
	ctx := context.Background()

	t.Run("sem passkey: gera as options e guarda a sessão da cerimônia de cadastro", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectMfaAuth(d, user)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return([]*core.Passkey{}, nil)
		d.verifier.On("BeginRegistration", user, []*core.Passkey{}).Return(testOptions, testSession, nil)
		d.auth.On("SaveCeremonySession", mock.Anything, user.ID, core.CeremonyRegistration, testSession, testCeremonyTTL).Return(nil)

		options, err := svc.RegisterWebAuthnBegin(ctx, validMfaToken)

		require.NoError(t, err)
		assert.JSONEq(t, string(testOptions), string(options))
	})

	t.Run("D1: usuário que já tem passkey não cadastra com mfa_token", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectMfaAuth(d, user)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return(passkeysOf(user.ID, 1), nil)

		options, err := svc.RegisterWebAuthnBegin(ctx, validMfaToken)

		assert.ErrorIs(t, err, core.ErrPasskeyRegistrationNotAllowed)
		assert.Nil(t, options)
		d.verifier.AssertNotCalled(t, "BeginRegistration", mock.Anything, mock.Anything)
		d.auth.AssertNotCalled(t, "SaveCeremonySession", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("erro ao listar passkeys é propagado", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectMfaAuth(d, user)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return(nil, errBoom)

		options, err := svc.RegisterWebAuthnBegin(ctx, validMfaToken)

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, options)
	})

	t.Run("erro do verificador é propagado e nenhuma sessão é guardada", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectMfaAuth(d, user)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return([]*core.Passkey{}, nil)
		d.verifier.On("BeginRegistration", user, []*core.Passkey{}).Return(nil, nil, errBoom)

		options, err := svc.RegisterWebAuthnBegin(ctx, validMfaToken)

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, options)
		d.auth.AssertNotCalled(t, "SaveCeremonySession", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	// Sem a sessão guardada, o navegador faria a biometria à toa e o finish falharia.
	t.Run("erro ao guardar a sessão é propagado e as options não são devolvidas", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectMfaAuth(d, user)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return([]*core.Passkey{}, nil)
		d.verifier.On("BeginRegistration", user, []*core.Passkey{}).Return(testOptions, testSession, nil)
		d.auth.On("SaveCeremonySession", mock.Anything, user.ID, core.CeremonyRegistration, testSession, testCeremonyTTL).Return(errBoom)

		options, err := svc.RegisterWebAuthnBegin(ctx, validMfaToken)

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, options)
	})

	t.Run("não emite sessão nem consome o mfa_token", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectMfaAuth(d, user)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return([]*core.Passkey{}, nil)
		d.verifier.On("BeginRegistration", user, []*core.Passkey{}).Return(testOptions, testSession, nil)
		d.auth.On("SaveCeremonySession", mock.Anything, user.ID, core.CeremonyRegistration, testSession, testCeremonyTTL).Return(nil)

		_, err := svc.RegisterWebAuthnBegin(ctx, validMfaToken)

		require.NoError(t, err)
		assertNothingIssued(t, d)
	})
}

func TestAuthServiceRegisterWebAuthnFinish(t *testing.T) {
	ctx := context.Background()

	// expectUntilVerify configura tudo até a verificação: autenticação, cerimônia consumida e usuário sem passkey.
	expectUntilVerify := func(d *authDeps, user *core.User) {
		expectMfaAuth(d, user)
		d.auth.On("ConsumeCeremonySession", mock.Anything, user.ID, core.CeremonyRegistration).Return(testSession, nil)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return([]*core.Passkey{}, nil)
	}

	t.Run("sucesso grava a passkey, consome o mfa_token e emite a sessão, nessa ordem", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectUntilVerify(d, user)
		passkey := passkeysOf(user.ID, 1)[0]
		var order []string
		d.verifier.On("FinishRegistration", user, []*core.Passkey{}, testSession, testAnswer).Return(passkey, nil)
		d.passkeys.On("Create", mock.Anything, passkey).Return(nil).Run(func(mock.Arguments) { order = append(order, "create") })
		d.auth.On("DeleteTemporaryToken", mock.Anything, user.ID).Return(nil).Run(func(mock.Arguments) { order = append(order, "delete-mfa") })
		d.session.On("GenerateToken", core.SessionClaims{UserID: user.ID}).Return("session-token", nil).Run(func(mock.Arguments) { order = append(order, "session") })
		d.session.On("TTL").Return(sessionTTLTest)
		d.auth.On("SetSessionToken", mock.Anything, user.ID, "session-token", sessionTTLTest).Return(nil)

		got, err := svc.RegisterWebAuthnFinish(ctx, validMfaToken, testAnswer)

		require.NoError(t, err)
		assert.Equal(t, &core.SessionResult{Token: "session-token", ExpiresIn: sessionTTLTest}, got)
		assert.Equal(t, []string{"create", "delete-mfa", "session"}, order)
	})

	t.Run("cerimônia expirada ou já usada é recusada antes da verificação", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectMfaAuth(d, user)
		d.auth.On("ConsumeCeremonySession", mock.Anything, user.ID, core.CeremonyRegistration).Return(nil, core.ErrCeremonyExpired)

		got, err := svc.RegisterWebAuthnFinish(ctx, validMfaToken, testAnswer)

		assert.ErrorIs(t, err, core.ErrCeremonyExpired)
		assert.Nil(t, got)
		d.verifier.AssertNotCalled(t, "FinishRegistration", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		assertNothingIssued(t, d)
	})

	// Outra aba pode ter cadastrado uma passkey entre o begin e o finish.
	t.Run("D1 conferida de novo: passkey cadastrada depois do begin bloqueia o finish", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectMfaAuth(d, user)
		d.auth.On("ConsumeCeremonySession", mock.Anything, user.ID, core.CeremonyRegistration).Return(testSession, nil)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return(passkeysOf(user.ID, 1), nil)

		got, err := svc.RegisterWebAuthnFinish(ctx, validMfaToken, testAnswer)

		assert.ErrorIs(t, err, core.ErrPasskeyRegistrationNotAllowed)
		assert.Nil(t, got)
		d.verifier.AssertNotCalled(t, "FinishRegistration", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		assertNothingIssued(t, d)
	})

	t.Run("erro ao listar passkeys é propagado", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectMfaAuth(d, user)
		d.auth.On("ConsumeCeremonySession", mock.Anything, user.ID, core.CeremonyRegistration).Return(testSession, nil)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return(nil, errBoom)

		_, err := svc.RegisterWebAuthnFinish(ctx, validMfaToken, testAnswer)

		assert.ErrorIs(t, err, errBoom)
		assertNothingIssued(t, d)
	})

	// O mfa_token continua valendo: o usuário refaz o begin sem digitar a senha de novo.
	t.Run("passkey inválida não grava nada e mantém o mfa_token", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectUntilVerify(d, user)
		d.verifier.On("FinishRegistration", user, []*core.Passkey{}, testSession, testAnswer).Return(nil, core.ErrPasskeyInvalid)

		got, err := svc.RegisterWebAuthnFinish(ctx, validMfaToken, testAnswer)

		assert.ErrorIs(t, err, core.ErrPasskeyInvalid)
		assert.Nil(t, got)
		assertNothingIssued(t, d)
	})

	t.Run("resposta malformada é erro do cliente e não grava nada", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectUntilVerify(d, user)
		d.verifier.On("FinishRegistration", user, []*core.Passkey{}, testSession, testAnswer).Return(nil, core.ErrBadRequest)

		_, err := svc.RegisterWebAuthnFinish(ctx, validMfaToken, testAnswer)

		assert.ErrorIs(t, err, core.ErrBadRequest)
		assertNothingIssued(t, d)
	})

	t.Run("passkey duplicada (409) não consome o mfa_token nem emite sessão", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectUntilVerify(d, user)
		passkey := passkeysOf(user.ID, 1)[0]
		d.verifier.On("FinishRegistration", user, []*core.Passkey{}, testSession, testAnswer).Return(passkey, nil)
		d.passkeys.On("Create", mock.Anything, passkey).Return(core.ErrPasskeyAlreadyRegistered)

		got, err := svc.RegisterWebAuthnFinish(ctx, validMfaToken, testAnswer)

		assert.ErrorIs(t, err, core.ErrPasskeyAlreadyRegistered)
		assert.Nil(t, got)
		d.auth.AssertNotCalled(t, "DeleteTemporaryToken", mock.Anything, mock.Anything)
		d.session.AssertNotCalled(t, "GenerateToken", mock.Anything)
	})

	// A passkey já está gravada e é válida: o usuário entra pelo login com passkey.
	t.Run("erro ao apagar o mfa_token não emite sessão", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectUntilVerify(d, user)
		passkey := passkeysOf(user.ID, 1)[0]
		d.verifier.On("FinishRegistration", user, []*core.Passkey{}, testSession, testAnswer).Return(passkey, nil)
		d.passkeys.On("Create", mock.Anything, passkey).Return(nil)
		d.auth.On("DeleteTemporaryToken", mock.Anything, user.ID).Return(errBoom)

		got, err := svc.RegisterWebAuthnFinish(ctx, validMfaToken, testAnswer)

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, got)
		d.session.AssertNotCalled(t, "GenerateToken", mock.Anything)
	})

	t.Run("erro ao gerar o token de sessão é propagado", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectUntilVerify(d, user)
		passkey := passkeysOf(user.ID, 1)[0]
		d.verifier.On("FinishRegistration", user, []*core.Passkey{}, testSession, testAnswer).Return(passkey, nil)
		d.passkeys.On("Create", mock.Anything, passkey).Return(nil)
		d.auth.On("DeleteTemporaryToken", mock.Anything, user.ID).Return(nil)
		d.session.On("GenerateToken", core.SessionClaims{UserID: user.ID}).Return("", errBoom)

		got, err := svc.RegisterWebAuthnFinish(ctx, validMfaToken, testAnswer)

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, got)
		d.auth.AssertNotCalled(t, "SetSessionToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("erro ao gravar a sessão não devolve o token", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectUntilVerify(d, user)
		passkey := passkeysOf(user.ID, 1)[0]
		d.verifier.On("FinishRegistration", user, []*core.Passkey{}, testSession, testAnswer).Return(passkey, nil)
		d.passkeys.On("Create", mock.Anything, passkey).Return(nil)
		d.auth.On("DeleteTemporaryToken", mock.Anything, user.ID).Return(nil)
		d.session.On("GenerateToken", core.SessionClaims{UserID: user.ID}).Return("session-token", nil)
		d.session.On("TTL").Return(sessionTTLTest)
		d.auth.On("SetSessionToken", mock.Anything, user.ID, "session-token", sessionTTLTest).Return(errBoom)

		got, err := svc.RegisterWebAuthnFinish(ctx, validMfaToken, testAnswer)

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, got)
	})
}
