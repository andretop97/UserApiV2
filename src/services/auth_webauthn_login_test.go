package services_test

import (
	"context"
	"testing"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Login com passkey (passo 6): o mfa_token do login com senha autoriza a cerimônia,
// a passkey prova o segundo fator e o finish emite a sessão.

func TestAuthServiceWebAuthnBegin(t *testing.T) {
	ctx := context.Background()

	t.Run("gera as options e guarda a sessão da cerimônia de login", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		passkeys := passkeysOf(user.ID, 2)
		expectMfaAuth(d, user)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return(passkeys, nil)
		d.verifier.On("BeginLogin", user, passkeys).Return(testOptions, testSession, nil)
		d.auth.On("SaveCeremonySession", mock.Anything, user.ID, core.CeremonyLogin, testSession, testCeremonyTTL).Return(nil)

		options, err := svc.WebAuthnBegin(ctx, validMfaToken)

		require.NoError(t, err)
		assert.JSONEq(t, string(testOptions), string(options))
	})

	t.Run("nunca usa a chave da cerimônia de cadastro", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		passkeys := passkeysOf(user.ID, 1)
		expectMfaAuth(d, user)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return(passkeys, nil)
		d.verifier.On("BeginLogin", user, passkeys).Return(testOptions, testSession, nil)
		d.auth.On("SaveCeremonySession", mock.Anything, user.ID, core.CeremonyLogin, testSession, testCeremonyTTL).Return(nil)

		_, err := svc.WebAuthnBegin(ctx, validMfaToken)

		require.NoError(t, err)
		d.auth.AssertNotCalled(t, "SaveCeremonySession", mock.Anything, mock.Anything, core.CeremonyRegistration, mock.Anything, mock.Anything)
		d.verifier.AssertNotCalled(t, "BeginRegistration", mock.Anything, mock.Anything)
	})

	// Sem passkey não há o que provar; o Login já teria mandado o cliente para o cadastro.
	// Sem essa checagem, o erro da lib ("Found no credentials for user") vira 500.
	t.Run("usuário sem passkey recebe ErrPasskeyNotFound, sem chamar o verificador", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectMfaAuth(d, user)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return([]*core.Passkey{}, nil)
		d.verifier.On("BeginLogin", mock.Anything, mock.Anything).Return(nil, nil, errBoom).Maybe()

		options, err := svc.WebAuthnBegin(ctx, validMfaToken)

		assert.ErrorIs(t, err, core.ErrPasskeyNotFound)
		assert.Nil(t, options)
		d.verifier.AssertNotCalled(t, "BeginLogin", mock.Anything, mock.Anything)
		d.auth.AssertNotCalled(t, "SaveCeremonySession", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("erro ao listar passkeys é propagado", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectMfaAuth(d, user)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return(nil, errBoom)

		options, err := svc.WebAuthnBegin(ctx, validMfaToken)

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, options)
	})

	t.Run("erro do verificador é propagado e nenhuma sessão é guardada", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		passkeys := passkeysOf(user.ID, 1)
		expectMfaAuth(d, user)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return(passkeys, nil)
		d.verifier.On("BeginLogin", user, passkeys).Return(nil, nil, errBoom)

		options, err := svc.WebAuthnBegin(ctx, validMfaToken)

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, options)
		d.auth.AssertNotCalled(t, "SaveCeremonySession", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("erro ao guardar a sessão é propagado e as options não são devolvidas", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		passkeys := passkeysOf(user.ID, 1)
		expectMfaAuth(d, user)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return(passkeys, nil)
		d.verifier.On("BeginLogin", user, passkeys).Return(testOptions, testSession, nil)
		d.auth.On("SaveCeremonySession", mock.Anything, user.ID, core.CeremonyLogin, testSession, testCeremonyTTL).Return(errBoom)

		options, err := svc.WebAuthnBegin(ctx, validMfaToken)

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, options)
	})

	t.Run("não emite sessão nem consome o mfa_token", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		passkeys := passkeysOf(user.ID, 1)
		expectMfaAuth(d, user)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return(passkeys, nil)
		d.verifier.On("BeginLogin", user, passkeys).Return(testOptions, testSession, nil)
		d.auth.On("SaveCeremonySession", mock.Anything, user.ID, core.CeremonyLogin, testSession, testCeremonyTTL).Return(nil)

		_, err := svc.WebAuthnBegin(ctx, validMfaToken)

		require.NoError(t, err)
		assertNothingIssued(t, d)
	})
}

func TestAuthServiceWebAuthnFinish(t *testing.T) {
	ctx := context.Background()

	// expectUntilVerify configura autenticação, cerimônia de login consumida e as passkeys do usuário.
	expectUntilVerify := func(d *authDeps, user *core.User) []*core.Passkey {
		passkeys := passkeysOf(user.ID, 2)
		expectMfaAuth(d, user)
		d.auth.On("ConsumeCeremonySession", mock.Anything, user.ID, core.CeremonyLogin).Return(testSession, nil)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return(passkeys, nil)
		return passkeys
	}

	// usedPasskey é a passkey como o verificador devolve: mesmo ID, Data com o sign count novo.
	usedPasskey := func(stored *core.Passkey) *core.Passkey {
		return &core.Passkey{ID: stored.ID, UserID: stored.UserID, Data: []byte(`{"authenticator":{"signCount":7}}`)}
	}

	t.Run("sucesso atualiza a passkey usada, consome o mfa_token e emite a sessão, nessa ordem", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		passkeys := expectUntilVerify(d, user)
		used := usedPasskey(passkeys[1])
		var order []string
		d.verifier.On("FinishLogin", user, passkeys, testSession, testAnswer).Return(used, nil)
		d.passkeys.On("UpdateAfterLogin", mock.Anything, used.ID, used.Data).Return(nil).Run(func(mock.Arguments) { order = append(order, "update") })
		d.auth.On("DeleteTemporaryToken", mock.Anything, user.ID).Return(nil).Run(func(mock.Arguments) { order = append(order, "delete-mfa") })
		d.session.On("GenerateToken", core.SessionClaims{UserID: user.ID}).Return("session-token", nil).Run(func(mock.Arguments) { order = append(order, "session") })
		d.session.On("TTL").Return(sessionTTLTest)
		d.auth.On("SetSessionToken", mock.Anything, user.ID, "session-token", sessionTTLTest).Return(nil)

		got, err := svc.WebAuthnFinish(ctx, validMfaToken, testAnswer)

		require.NoError(t, err)
		assert.Equal(t, &core.SessionResult{Token: "session-token", ExpiresIn: sessionTTLTest}, got)
		assert.Equal(t, []string{"update", "delete-mfa", "session"}, order)
	})

	// Login não cria passkey: um Create aqui daria 409 em todo login.
	t.Run("login nunca cria passkey nem usa a cerimônia de cadastro", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		passkeys := expectUntilVerify(d, user)
		used := usedPasskey(passkeys[0])
		d.verifier.On("FinishLogin", user, passkeys, testSession, testAnswer).Return(used, nil)
		d.passkeys.On("UpdateAfterLogin", mock.Anything, used.ID, used.Data).Return(nil)
		d.auth.On("DeleteTemporaryToken", mock.Anything, user.ID).Return(nil)
		d.session.On("GenerateToken", mock.Anything).Return("session-token", nil)
		d.session.On("TTL").Return(sessionTTLTest)
		d.auth.On("SetSessionToken", mock.Anything, user.ID, "session-token", sessionTTLTest).Return(nil)

		_, err := svc.WebAuthnFinish(ctx, validMfaToken, testAnswer)

		require.NoError(t, err)
		d.passkeys.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		d.auth.AssertNotCalled(t, "ConsumeCeremonySession", mock.Anything, mock.Anything, core.CeremonyRegistration)
		d.verifier.AssertNotCalled(t, "FinishRegistration", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("cerimônia expirada ou já usada é recusada antes da verificação", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectMfaAuth(d, user)
		d.auth.On("ConsumeCeremonySession", mock.Anything, user.ID, core.CeremonyLogin).Return(nil, core.ErrCeremonyExpired)

		got, err := svc.WebAuthnFinish(ctx, validMfaToken, testAnswer)

		assert.ErrorIs(t, err, core.ErrCeremonyExpired)
		assert.Nil(t, got)
		d.verifier.AssertNotCalled(t, "FinishLogin", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		d.passkeys.AssertNotCalled(t, "UpdateAfterLogin", mock.Anything, mock.Anything, mock.Anything)
		assertNothingIssued(t, d)
	})

	t.Run("erro ao listar passkeys é propagado", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		expectMfaAuth(d, user)
		d.auth.On("ConsumeCeremonySession", mock.Anything, user.ID, core.CeremonyLogin).Return(testSession, nil)
		d.passkeys.On("ListByUser", mock.Anything, user.ID).Return(nil, errBoom)

		_, err := svc.WebAuthnFinish(ctx, validMfaToken, testAnswer)

		assert.ErrorIs(t, err, errBoom)
		assertNothingIssued(t, d)
	})

	// Cobre assinatura inválida, origem errada, falta de UV e clone: o adapter devolve ErrPasskeyInvalid.
	t.Run("passkey inválida não atualiza nada e mantém o mfa_token", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		passkeys := expectUntilVerify(d, user)
		d.verifier.On("FinishLogin", user, passkeys, testSession, testAnswer).Return(nil, core.ErrPasskeyInvalid)

		got, err := svc.WebAuthnFinish(ctx, validMfaToken, testAnswer)

		assert.ErrorIs(t, err, core.ErrPasskeyInvalid)
		assert.Nil(t, got)
		d.passkeys.AssertNotCalled(t, "UpdateAfterLogin", mock.Anything, mock.Anything, mock.Anything)
		assertNothingIssued(t, d)
	})

	t.Run("resposta malformada é erro do cliente e não atualiza nada", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		passkeys := expectUntilVerify(d, user)
		d.verifier.On("FinishLogin", user, passkeys, testSession, testAnswer).Return(nil, core.ErrBadRequest)

		_, err := svc.WebAuthnFinish(ctx, validMfaToken, testAnswer)

		assert.ErrorIs(t, err, core.ErrBadRequest)
		d.passkeys.AssertNotCalled(t, "UpdateAfterLogin", mock.Anything, mock.Anything, mock.Anything)
		assertNothingIssued(t, d)
	})

	// Sem gravar o sign count novo, a próxima assinatura com o mesmo contador pareceria um clone.
	t.Run("erro ao atualizar a passkey não emite sessão", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		passkeys := expectUntilVerify(d, user)
		used := usedPasskey(passkeys[0])
		d.verifier.On("FinishLogin", user, passkeys, testSession, testAnswer).Return(used, nil)
		d.passkeys.On("UpdateAfterLogin", mock.Anything, used.ID, used.Data).Return(errBoom)

		got, err := svc.WebAuthnFinish(ctx, validMfaToken, testAnswer)

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, got)
		assertNothingIssued(t, d)
	})

	t.Run("erro ao apagar o mfa_token não emite sessão", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		passkeys := expectUntilVerify(d, user)
		used := usedPasskey(passkeys[0])
		d.verifier.On("FinishLogin", user, passkeys, testSession, testAnswer).Return(used, nil)
		d.passkeys.On("UpdateAfterLogin", mock.Anything, used.ID, used.Data).Return(nil)
		d.auth.On("DeleteTemporaryToken", mock.Anything, user.ID).Return(errBoom)

		got, err := svc.WebAuthnFinish(ctx, validMfaToken, testAnswer)

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, got)
		d.session.AssertNotCalled(t, "GenerateToken", mock.Anything)
	})

	t.Run("erro ao gravar a sessão não devolve o token", func(t *testing.T) {
		svc, d := setupAuthService(t)
		user := storedUser()
		passkeys := expectUntilVerify(d, user)
		used := usedPasskey(passkeys[0])
		d.verifier.On("FinishLogin", user, passkeys, testSession, testAnswer).Return(used, nil)
		d.passkeys.On("UpdateAfterLogin", mock.Anything, used.ID, used.Data).Return(nil)
		d.auth.On("DeleteTemporaryToken", mock.Anything, user.ID).Return(nil)
		d.session.On("GenerateToken", core.SessionClaims{UserID: user.ID}).Return("session-token", nil)
		d.session.On("TTL").Return(sessionTTLTest)
		d.auth.On("SetSessionToken", mock.Anything, user.ID, "session-token", sessionTTLTest).Return(errBoom)

		got, err := svc.WebAuthnFinish(ctx, validMfaToken, testAnswer)

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, got)
	})
}
