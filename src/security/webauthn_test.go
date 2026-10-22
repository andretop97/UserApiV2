package security

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/descope/virtualwebauthn"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Testes do adapter WebAuthn: os detalhes privados (webauthnUser e conversões) e as quatro
// cerimônias, com um autenticador virtual no papel do navegador e do dispositivo.

// webauthnUser precisa satisfazer a interface da lib; se um método faltar, o pacote não compila.
var _ webauthn.User = webauthnUser{}

// testCredential tem uma chave pública falsa: serve para os testes de conversão, mas nenhuma
// assinatura confere com ela. Os testes de Finish usam passkeys de um cadastro real (registerPasskey).
func testCredential(id ...byte) webauthn.Credential {
	return webauthn.Credential{
		ID:                id,
		PublicKey:         []byte{0x04, 0x01, 0x02, 0x03},
		AttestationType:   "self",
		AttestationFormat: "packed",
		Transport:         []protocol.AuthenticatorTransport{protocol.Internal, protocol.Hybrid},
		Flags: webauthn.CredentialFlags{
			UserPresent:    true,
			UserVerified:   true,
			BackupEligible: true,
			BackupState:    true,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:    []byte{0xad, 0xce, 0x00, 0x02},
			SignCount: 42,
		},
	}
}

func TestWebAuthnUser(t *testing.T) {
	user := &core.User{ID: uuid.New(), Name: "João Silva", Email: "joao@example.com"}
	creds := []webauthn.Credential{testCredential(1), testCredential(2)}
	u := webauthnUser{user: user, creds: creds}

	t.Run("WebAuthnID são os 16 bytes do UUID", func(t *testing.T) {
		got := u.WebAuthnID()

		assert.Len(t, got, 16)
		assert.Equal(t, user.ID[:], got)
	})

	t.Run("WebAuthnID não contém dado pessoal", func(t *testing.T) {
		got := u.WebAuthnID()

		assert.False(t, bytes.Contains(got, []byte(user.Email)), "o e-mail não pode ir para o autenticador")
		assert.False(t, bytes.Contains(got, []byte(user.Name)))
	})

	t.Run("WebAuthnID é estável entre chamadas", func(t *testing.T) {
		assert.Equal(t, u.WebAuthnID(), u.WebAuthnID())
	})

	t.Run("nomes de exibição vêm do e-mail e do nome", func(t *testing.T) {
		assert.Equal(t, "joao@example.com", u.WebAuthnName())
		assert.Equal(t, "João Silva", u.WebAuthnDisplayName())
	})

	t.Run("devolve as credenciais recebidas", func(t *testing.T) {
		assert.Equal(t, creds, u.WebAuthnCredentials())
	})
}

func TestToPasskey(t *testing.T) {
	userID := uuid.New()

	t.Run("usa o ID da credencial e o dono informado", func(t *testing.T) {
		cred := testCredential(0x00, 0xff, 0x10)

		p, err := toPasskey(userID, &cred)

		require.NoError(t, err)
		assert.Equal(t, cred.ID, p.ID)
		assert.Equal(t, userID, p.UserID)
	})

	t.Run("Data é JSON válido (a coluna é JSONB)", func(t *testing.T) {
		cred := testCredential(1)

		p, err := toPasskey(userID, &cred)

		require.NoError(t, err)
		assert.True(t, json.Valid(p.Data))
	})

	t.Run("CreatedAt e LastUsedAt ficam para o banco", func(t *testing.T) {
		cred := testCredential(1)

		p, err := toPasskey(userID, &cred)

		require.NoError(t, err)
		assert.True(t, p.CreatedAt.IsZero())
		assert.Nil(t, p.LastUsedAt)
	})
}

func TestToCredentials(t *testing.T) {
	userID := uuid.New()

	t.Run("ida e volta preserva o que a verificação usa", func(t *testing.T) {
		cred := testCredential(7, 8, 9)
		p, err := toPasskey(userID, &cred)
		require.NoError(t, err)

		got, err := toCredentials([]*core.Passkey{p})

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, cred.ID, got[0].ID)
		assert.Equal(t, cred.PublicKey, got[0].PublicKey, "sem a chave pública não há como verificar a assinatura")
		assert.Equal(t, cred.Authenticator.SignCount, got[0].Authenticator.SignCount, "o sign count detecta clonagem")
		assert.Equal(t, cred.Authenticator.AAGUID, got[0].Authenticator.AAGUID)
		assert.Equal(t, cred.Transport, got[0].Transport)
		assert.Equal(t, cred.AttestationType, got[0].AttestationType)
		assert.Equal(t, cred.AttestationFormat, got[0].AttestationFormat)
		assert.Equal(t, cred.Flags.UserPresent, got[0].Flags.UserPresent)
		assert.Equal(t, cred.Flags.UserVerified, got[0].Flags.UserVerified)
		assert.Equal(t, cred.Flags.BackupEligible, got[0].Flags.BackupEligible)
		assert.Equal(t, cred.Flags.BackupState, got[0].Flags.BackupState)
	})

	t.Run("aceita o JSON normalizado pelo JSONB", func(t *testing.T) {
		cred := testCredential(1)
		p, err := toPasskey(userID, &cred)
		require.NoError(t, err)
		// o Postgres devolve o JSONB com outra formatação; simula isso reindentando.
		var normalized bytes.Buffer
		require.NoError(t, json.Indent(&normalized, p.Data, "", "  "))
		p.Data = normalized.Bytes()

		got, err := toCredentials([]*core.Passkey{p})

		require.NoError(t, err)
		assert.Equal(t, cred.PublicKey, got[0].PublicKey)
	})

	t.Run("mantém a ordem das passkeys", func(t *testing.T) {
		var passkeys []*core.Passkey
		for _, id := range []byte{3, 1, 2} {
			cred := testCredential(id)
			p, err := toPasskey(userID, &cred)
			require.NoError(t, err)
			passkeys = append(passkeys, p)
		}

		got, err := toCredentials(passkeys)

		require.NoError(t, err)
		require.Len(t, got, 3)
		assert.Equal(t, []byte{3}, got[0].ID)
		assert.Equal(t, []byte{1}, got[1].ID)
		assert.Equal(t, []byte{2}, got[2].ID)
	})

	t.Run("sem passkeys devolve lista vazia", func(t *testing.T) {
		got, err := toCredentials(nil)

		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("Data inválido retorna erro", func(t *testing.T) {
		passkeys := []*core.Passkey{{ID: []byte{1}, UserID: userID, Data: []byte("não é json")}}

		got, err := toCredentials(passkeys)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode passkey")
		assert.Nil(t, got)
	})

	t.Run("uma passkey inválida invalida a lista toda", func(t *testing.T) {
		cred := testCredential(1)
		valid, err := toPasskey(userID, &cred)
		require.NoError(t, err)
		broken := &core.Passkey{ID: []byte{2}, UserID: userID, Data: []byte("{")}

		got, err := toCredentials([]*core.Passkey{valid, broken})

		assert.Error(t, err)
		assert.Nil(t, got)
	})
}

const testCeremonyTTL = 5 * time.Minute

// newTestVerifier monta o adapter pelo construtor, com uma instância real da lib
// configurada como no bootstrap (timeouts aplicados pelo servidor).
func newTestVerifier(t *testing.T) *WebAuthnVerifier {
	t.Helper()
	w, err := webauthn.New(&webauthn.Config{
		RPID:          "localhost",
		RPDisplayName: "UserApiV2",
		RPOrigins:     []string{"http://localhost:3000"},
		Timeouts: webauthn.TimeoutsConfig{
			Login:        webauthn.TimeoutConfig{Enforce: true, Timeout: testCeremonyTTL},
			Registration: webauthn.TimeoutConfig{Enforce: true, Timeout: testCeremonyTTL},
		},
	})
	require.NoError(t, err)
	v, err := NewWebAuthnVerifier(w)
	require.NoError(t, err)
	return v.(*WebAuthnVerifier)
}

func TestNewWebAuthnVerifier(t *testing.T) {
	t.Run("aceita uma instância configurada", func(t *testing.T) {
		w, err := webauthn.New(&webauthn.Config{RPID: "localhost", RPDisplayName: "UserApiV2", RPOrigins: []string{"http://localhost:3000"}})
		require.NoError(t, err)

		v, err := NewWebAuthnVerifier(w)

		require.NoError(t, err)
		assert.NotNil(t, v)
	})

	t.Run("instância nil falha no boot, não na primeira cerimônia", func(t *testing.T) {
		v, err := NewWebAuthnVerifier(nil)

		assert.Error(t, err)
		assert.Nil(t, v)
	})

	t.Run("instância sem Config falha no boot", func(t *testing.T) {
		v, err := NewWebAuthnVerifier(&webauthn.WebAuthn{})

		assert.Error(t, err)
		assert.Nil(t, v)
	})
}

// expireSession devolve a sessão com o prazo vencido, como se o usuário tivesse demorado demais.
func expireSession(t *testing.T, session []byte) []byte {
	t.Helper()
	var sess webauthn.SessionData
	require.NoError(t, json.Unmarshal(session, &sess))
	sess.Expires = time.Now().Add(-time.Second)
	expired, err := json.Marshal(sess)
	require.NoError(t, err)
	return expired
}

// creationOptions espelha só os campos das opções de cadastro que os testes verificam.
type creationOptions struct {
	PublicKey struct {
		RP struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"rp"`
		User struct {
			ID          protocol.URLEncodedBase64 `json:"id"`
			Name        string                    `json:"name"`
			DisplayName string                    `json:"displayName"`
		} `json:"user"`
		Challenge          protocol.URLEncodedBase64 `json:"challenge"`
		ExcludeCredentials []struct {
			ID protocol.URLEncodedBase64 `json:"id"`
		} `json:"excludeCredentials"`
		AuthenticatorSelection struct {
			ResidentKey      string `json:"residentKey"`
			UserVerification string `json:"userVerification"`
		} `json:"authenticatorSelection"`
	} `json:"publicKey"`
}

func beginRegistration(t *testing.T, v *WebAuthnVerifier, user *core.User, existing []*core.Passkey) (creationOptions, webauthn.SessionData) {
	t.Helper()
	options, session, err := v.BeginRegistration(user, existing)
	require.NoError(t, err)

	var opts creationOptions
	require.NoError(t, json.Unmarshal(options, &opts))
	var sess webauthn.SessionData
	require.NoError(t, json.Unmarshal(session, &sess))
	return opts, sess
}

func TestBeginRegistration(t *testing.T) {
	user := &core.User{ID: uuid.New(), Name: "João Silva", Email: "joao@example.com"}

	t.Run("opções identificam o site configurado", func(t *testing.T) {
		opts, _ := beginRegistration(t, newTestVerifier(t), user, nil)

		assert.Equal(t, "localhost", opts.PublicKey.RP.ID)
		assert.Equal(t, "UserApiV2", opts.PublicKey.RP.Name)
	})

	t.Run("user.id são os bytes do UUID, e os nomes vêm do usuário", func(t *testing.T) {
		opts, _ := beginRegistration(t, newTestVerifier(t), user, nil)

		assert.Equal(t, user.ID[:], []byte(opts.PublicKey.User.ID))
		assert.Equal(t, "joao@example.com", opts.PublicKey.User.Name)
		assert.Equal(t, "João Silva", opts.PublicKey.User.DisplayName)
	})

	t.Run("exige verificação do usuário e prefere passkey descobrível", func(t *testing.T) {
		opts, sess := beginRegistration(t, newTestVerifier(t), user, nil)

		assert.Equal(t, "required", opts.PublicKey.AuthenticatorSelection.UserVerification)
		assert.Equal(t, "preferred", opts.PublicKey.AuthenticatorSelection.ResidentKey)
		assert.Equal(t, protocol.VerificationRequired, sess.UserVerification, "o finish precisa cobrar a verificação")
	})

	t.Run("challenge tem pelo menos 16 bytes", func(t *testing.T) {
		opts, _ := beginRegistration(t, newTestVerifier(t), user, nil)

		assert.GreaterOrEqual(t, len(opts.PublicKey.Challenge), 16)
	})

	t.Run("challenge muda a cada cerimônia", func(t *testing.T) {
		v := newTestVerifier(t)
		first, _ := beginRegistration(t, v, user, nil)
		second, _ := beginRegistration(t, v, user, nil)

		assert.NotEqual(t, first.PublicKey.Challenge, second.PublicKey.Challenge)
	})

	t.Run("sessão guarda o mesmo challenge e o usuário", func(t *testing.T) {
		opts, sess := beginRegistration(t, newTestVerifier(t), user, nil)

		assert.Equal(t, opts.PublicKey.Challenge.String(), sess.Challenge)
		assert.Equal(t, user.ID[:], sess.UserID)
	})

	t.Run("sessão expira no prazo da cerimônia", func(t *testing.T) {
		before := time.Now()
		_, sess := beginRegistration(t, newTestVerifier(t), user, nil)

		assert.WithinDuration(t, before.Add(testCeremonyTTL), sess.Expires, 5*time.Second)
	})

	t.Run("sem passkeys não há exclusões", func(t *testing.T) {
		opts, _ := beginRegistration(t, newTestVerifier(t), user, nil)

		assert.Empty(t, opts.PublicKey.ExcludeCredentials)
	})

	t.Run("passkeys existentes entram em excludeCredentials", func(t *testing.T) {
		var existing []*core.Passkey
		for _, id := range []byte{1, 2} {
			cred := testCredential(id)
			p, err := toPasskey(user.ID, &cred)
			require.NoError(t, err)
			existing = append(existing, p)
		}

		opts, _ := beginRegistration(t, newTestVerifier(t), user, existing)

		require.Len(t, opts.PublicKey.ExcludeCredentials, 2)
		assert.Equal(t, []byte{1}, []byte(opts.PublicKey.ExcludeCredentials[0].ID))
		assert.Equal(t, []byte{2}, []byte(opts.PublicKey.ExcludeCredentials[1].ID))
	})

	t.Run("passkey existente corrompida retorna erro", func(t *testing.T) {
		existing := []*core.Passkey{{ID: []byte{1}, UserID: user.ID, Data: []byte("{")}}

		options, session, err := newTestVerifier(t).BeginRegistration(user, existing)

		assert.Error(t, err)
		assert.Nil(t, options)
		assert.Nil(t, session)
	})
}

// testRP é o site visto pelo autenticador virtual; precisa bater com newTestVerifier.
var testRP = virtualwebauthn.RelyingParty{ID: "localhost", Name: "UserApiV2", Origin: "http://localhost:3000"}

func newTestUser(name string) *core.User {
	return &core.User{ID: uuid.New(), Name: name, Email: name + "@example.com"}
}

// newAuthenticator cria o "dispositivo" do usuário; o UserHandle volta em todo login.
func newAuthenticator(user *core.User) *virtualwebauthn.Authenticator {
	auth := virtualwebauthn.NewAuthenticatorWithOptions(virtualwebauthn.AuthenticatorOptions{UserHandle: user.ID[:]})
	return &auth
}

func newDeviceCredential() virtualwebauthn.Credential {
	return virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)
}

// attest responde às opções de cadastro como o navegador faria.
func attest(t *testing.T, rp virtualwebauthn.RelyingParty, auth *virtualwebauthn.Authenticator, cred virtualwebauthn.Credential, options json.RawMessage) []byte {
	t.Helper()
	attOpts, err := virtualwebauthn.ParseAttestationOptions(string(options))
	require.NoError(t, err)
	return []byte(virtualwebauthn.CreateAttestationResponse(rp, *auth, cred, *attOpts))
}

// assertWith responde às opções de login assinando com a credencial informada.
func assertWith(t *testing.T, rp virtualwebauthn.RelyingParty, auth *virtualwebauthn.Authenticator, cred virtualwebauthn.Credential, options json.RawMessage) []byte {
	t.Helper()
	assOpts, err := virtualwebauthn.ParseAssertionOptions(string(options))
	require.NoError(t, err)
	return []byte(virtualwebauthn.CreateAssertionResponse(rp, *auth, cred, *assOpts))
}

// registerPasskey faz o cadastro completo e devolve a passkey gravável, o autenticador que a guarda
// e a credencial do lado do dispositivo (com a chave privada).
func registerPasskey(t *testing.T, v *WebAuthnVerifier, user *core.User, counter uint32) (*core.Passkey, *virtualwebauthn.Authenticator, virtualwebauthn.Credential) {
	t.Helper()
	options, session, err := v.BeginRegistration(user, nil)
	require.NoError(t, err)

	auth := newAuthenticator(user)
	cred := newDeviceCredential()
	cred.Counter = counter

	passkey, err := v.FinishRegistration(user, nil, session, attest(t, testRP, auth, cred, options))
	require.NoError(t, err)
	auth.AddCredential(cred)
	return passkey, auth, cred
}

func signCount(t *testing.T, p *core.Passkey) uint32 {
	t.Helper()
	creds, err := toCredentials([]*core.Passkey{p})
	require.NoError(t, err)
	return creds[0].Authenticator.SignCount
}

func isDomainError(err error) bool {
	var domainErr *core.DomainError
	return errors.As(err, &domainErr)
}

func TestFinishRegistration(t *testing.T) {
	t.Run("resposta válida devolve a passkey do usuário", func(t *testing.T) {
		v := newTestVerifier(t)
		user := newTestUser("joao")

		passkey, _, cred := registerPasskey(t, v, user, 0)

		assert.Equal(t, cred.ID, passkey.ID)
		assert.Equal(t, user.ID, passkey.UserID)
		creds, err := toCredentials([]*core.Passkey{passkey})
		require.NoError(t, err)
		assert.NotEmpty(t, creds[0].PublicKey, "sem a chave pública o login não tem o que verificar")
		assert.True(t, creds[0].Flags.UserVerified)
	})

	t.Run("origem fora do RPOrigins é recusada (phishing)", func(t *testing.T) {
		v := newTestVerifier(t)
		user := newTestUser("joao")
		options, session, err := v.BeginRegistration(user, nil)
		require.NoError(t, err)
		evil := testRP
		evil.Origin = "https://evil.com"

		passkey, err := v.FinishRegistration(user, nil, session, attest(t, evil, newAuthenticator(user), newDeviceCredential(), options))

		assert.ErrorIs(t, err, core.ErrPasskeyInvalid)
		assert.Nil(t, passkey)
	})

	t.Run("challenge de outra cerimônia é recusado (replay)", func(t *testing.T) {
		v := newTestVerifier(t)
		user := newTestUser("joao")
		firstOptions, _, err := v.BeginRegistration(user, nil)
		require.NoError(t, err)
		_, secondSession, err := v.BeginRegistration(user, nil)
		require.NoError(t, err)

		_, err = v.FinishRegistration(user, nil, secondSession, attest(t, testRP, newAuthenticator(user), newDeviceCredential(), firstOptions))

		assert.ErrorIs(t, err, core.ErrPasskeyInvalid)
	})

	t.Run("sem verificação do usuário é recusado", func(t *testing.T) {
		v := newTestVerifier(t)
		user := newTestUser("joao")
		options, session, err := v.BeginRegistration(user, nil)
		require.NoError(t, err)
		auth := newAuthenticator(user)
		auth.Options.UserNotVerified = true

		_, err = v.FinishRegistration(user, nil, session, attest(t, testRP, auth, newDeviceCredential(), options))

		assert.ErrorIs(t, err, core.ErrPasskeyInvalid)
	})

	t.Run("sessão de outro usuário é recusada", func(t *testing.T) {
		v := newTestVerifier(t)
		alice, bob := newTestUser("alice"), newTestUser("bob")
		options, session, err := v.BeginRegistration(alice, nil)
		require.NoError(t, err)

		_, err = v.FinishRegistration(bob, nil, session, attest(t, testRP, newAuthenticator(alice), newDeviceCredential(), options))

		assert.ErrorIs(t, err, core.ErrPasskeyInvalid)
	})

	t.Run("sessão expirada é recusada", func(t *testing.T) {
		v := newTestVerifier(t)
		user := newTestUser("joao")
		options, session, err := v.BeginRegistration(user, nil)
		require.NoError(t, err)

		_, err = v.FinishRegistration(user, nil, expireSession(t, session), attest(t, testRP, newAuthenticator(user), newDeviceCredential(), options))

		assert.ErrorIs(t, err, core.ErrPasskeyInvalid)
	})

	t.Run("corpo malformado é erro do cliente", func(t *testing.T) {
		v := newTestVerifier(t)
		user := newTestUser("joao")
		_, session, err := v.BeginRegistration(user, nil)
		require.NoError(t, err)

		_, err = v.FinishRegistration(user, nil, session, []byte("{"))

		assert.ErrorIs(t, err, core.ErrBadRequest)
	})

	t.Run("sessão corrompida é erro interno", func(t *testing.T) {
		v := newTestVerifier(t)
		user := newTestUser("joao")
		options, _, err := v.BeginRegistration(user, nil)
		require.NoError(t, err)

		_, err = v.FinishRegistration(user, nil, []byte("não é json"), attest(t, testRP, newAuthenticator(user), newDeviceCredential(), options))

		require.Error(t, err)
		assert.False(t, isDomainError(err), "a sessão vem do Redis, não do cliente: deve virar 500")
	})
}

// requestOptions espelha só os campos das opções de login que os testes verificam.
type requestOptions struct {
	PublicKey struct {
		RPID             string                    `json:"rpId"`
		Challenge        protocol.URLEncodedBase64 `json:"challenge"`
		UserVerification string                    `json:"userVerification"`
		AllowCredentials []struct {
			ID protocol.URLEncodedBase64 `json:"id"`
		} `json:"allowCredentials"`
	} `json:"publicKey"`
}

func TestBeginLogin(t *testing.T) {
	t.Run("opções liberam só as passkeys do usuário e exigem verificação", func(t *testing.T) {
		v := newTestVerifier(t)
		user := newTestUser("joao")
		passkey, _, _ := registerPasskey(t, v, user, 0)

		options, session, err := v.BeginLogin(user, []*core.Passkey{passkey})
		require.NoError(t, err)

		var opts requestOptions
		require.NoError(t, json.Unmarshal(options, &opts))
		assert.Equal(t, "localhost", opts.PublicKey.RPID)
		assert.Equal(t, "required", opts.PublicKey.UserVerification)
		require.Len(t, opts.PublicKey.AllowCredentials, 1)
		assert.Equal(t, passkey.ID, []byte(opts.PublicKey.AllowCredentials[0].ID))

		var sess webauthn.SessionData
		require.NoError(t, json.Unmarshal(session, &sess))
		assert.Equal(t, opts.PublicKey.Challenge.String(), sess.Challenge)
		assert.Equal(t, user.ID[:], sess.UserID)
		assert.Equal(t, protocol.VerificationRequired, sess.UserVerification, "o finish precisa cobrar a verificação")
	})

	t.Run("challenge muda a cada cerimônia", func(t *testing.T) {
		v := newTestVerifier(t)
		user := newTestUser("joao")
		passkey, _, _ := registerPasskey(t, v, user, 0)
		existing := []*core.Passkey{passkey}

		first, _, err := v.BeginLogin(user, existing)
		require.NoError(t, err)
		second, _, err := v.BeginLogin(user, existing)
		require.NoError(t, err)

		var a, b requestOptions
		require.NoError(t, json.Unmarshal(first, &a))
		require.NoError(t, json.Unmarshal(second, &b))
		assert.NotEqual(t, a.PublicKey.Challenge, b.PublicKey.Challenge)
	})

	t.Run("usuário sem passkey retorna erro", func(t *testing.T) {
		options, session, err := newTestVerifier(t).BeginLogin(newTestUser("joao"), nil)

		assert.Error(t, err)
		assert.Nil(t, options)
		assert.Nil(t, session)
	})

	t.Run("passkey existente corrompida retorna erro", func(t *testing.T) {
		user := newTestUser("joao")
		existing := []*core.Passkey{{ID: []byte{1}, UserID: user.ID, Data: []byte("{")}}

		_, _, err := newTestVerifier(t).BeginLogin(user, existing)

		assert.Error(t, err)
	})
}

// loginFixture é um usuário com uma passkey cadastrada e uma cerimônia de login aberta.
type loginFixture struct {
	v        *WebAuthnVerifier
	user     *core.User
	passkey  *core.Passkey
	existing []*core.Passkey
	auth     *virtualwebauthn.Authenticator
	cred     virtualwebauthn.Credential
	options  json.RawMessage
	session  []byte
}

func newLoginFixture(t *testing.T, counter uint32) loginFixture {
	t.Helper()
	v := newTestVerifier(t)
	user := newTestUser("joao")
	passkey, auth, cred := registerPasskey(t, v, user, counter)
	existing := []*core.Passkey{passkey}
	options, session, err := v.BeginLogin(user, existing)
	require.NoError(t, err)
	return loginFixture{v: v, user: user, passkey: passkey, existing: existing, auth: auth, cred: cred, options: options, session: session}
}

func TestFinishLogin(t *testing.T) {
	t.Run("ida e volta: cadastro e depois login com o mesmo dispositivo", func(t *testing.T) {
		f := newLoginFixture(t, 0)
		assOpts, err := virtualwebauthn.ParseAssertionOptions(string(f.options))
		require.NoError(t, err)
		found := f.auth.FindAllowedCredential(*assOpts)
		require.NotNil(t, found, "o dispositivo precisa ter uma passkey liberada pelo servidor")
		response := virtualwebauthn.CreateAssertionResponse(testRP, *f.auth, *found, *assOpts)

		used, err := f.v.FinishLogin(f.user, f.existing, f.session, []byte(response))

		require.NoError(t, err)
		assert.Equal(t, f.passkey.ID, used.ID)
		assert.Equal(t, f.user.ID, used.UserID)
	})

	t.Run("sign count maior é aceito e atualizado", func(t *testing.T) {
		f := newLoginFixture(t, 5)
		f.cred.Counter = 6

		used, err := f.v.FinishLogin(f.user, f.existing, f.session, assertWith(t, testRP, f.auth, f.cred, f.options))

		require.NoError(t, err)
		assert.Equal(t, uint32(5), signCount(t, f.passkey))
		assert.Equal(t, uint32(6), signCount(t, used))
	})

	t.Run("sign count menor que o guardado é recusado (clone)", func(t *testing.T) {
		f := newLoginFixture(t, 5)
		f.cred.Counter = 3

		used, err := f.v.FinishLogin(f.user, f.existing, f.session, assertWith(t, testRP, f.auth, f.cred, f.options))

		assert.ErrorIs(t, err, core.ErrPasskeyInvalid)
		assert.Nil(t, used)
	})

	t.Run("sign count igual ao guardado é recusado (clone)", func(t *testing.T) {
		f := newLoginFixture(t, 5)

		_, err := f.v.FinishLogin(f.user, f.existing, f.session, assertWith(t, testRP, f.auth, f.cred, f.options))

		assert.ErrorIs(t, err, core.ErrPasskeyInvalid)
	})

	t.Run("sign count sempre 0 é aceito (passkey sincronizada)", func(t *testing.T) {
		f := newLoginFixture(t, 0)

		_, err := f.v.FinishLogin(f.user, f.existing, f.session, assertWith(t, testRP, f.auth, f.cred, f.options))

		assert.NoError(t, err)
	})

	t.Run("sem verificação do usuário é recusado", func(t *testing.T) {
		f := newLoginFixture(t, 0)
		f.auth.Options.UserNotVerified = true

		_, err := f.v.FinishLogin(f.user, f.existing, f.session, assertWith(t, testRP, f.auth, f.cred, f.options))

		assert.ErrorIs(t, err, core.ErrPasskeyInvalid)
	})

	t.Run("origem fora do RPOrigins é recusada (phishing)", func(t *testing.T) {
		f := newLoginFixture(t, 0)
		evil := testRP
		evil.Origin = "https://evil.com"

		_, err := f.v.FinishLogin(f.user, f.existing, f.session, assertWith(t, evil, f.auth, f.cred, f.options))

		assert.ErrorIs(t, err, core.ErrPasskeyInvalid)
	})

	t.Run("challenge de outra cerimônia é recusado (replay)", func(t *testing.T) {
		f := newLoginFixture(t, 0)
		_, secondSession, err := f.v.BeginLogin(f.user, f.existing)
		require.NoError(t, err)

		_, err = f.v.FinishLogin(f.user, f.existing, secondSession, assertWith(t, testRP, f.auth, f.cred, f.options))

		assert.ErrorIs(t, err, core.ErrPasskeyInvalid)
	})

	t.Run("sessão expirada é recusada", func(t *testing.T) {
		f := newLoginFixture(t, 0)

		_, err := f.v.FinishLogin(f.user, f.existing, expireSession(t, f.session), assertWith(t, testRP, f.auth, f.cred, f.options))

		assert.ErrorIs(t, err, core.ErrPasskeyInvalid)
	})

	t.Run("passkey de outro usuário é recusada", func(t *testing.T) {
		bob := newLoginFixture(t, 0)
		alice := newTestUser("alice")
		_, aliceAuth, aliceCred := registerPasskey(t, bob.v, alice, 0)

		// o dispositivo da Alice assina o challenge do login do Bob
		_, err := bob.v.FinishLogin(bob.user, bob.existing, bob.session, assertWith(t, testRP, aliceAuth, aliceCred, bob.options))

		assert.ErrorIs(t, err, core.ErrPasskeyInvalid)
	})

	t.Run("passkey que não está na lista do usuário é recusada", func(t *testing.T) {
		f := newLoginFixture(t, 0)
		// uma segunda passkey do mesmo usuário, que já foi removida do banco
		_, auth, removedCred := registerPasskey(t, f.v, f.user, 0)

		_, err := f.v.FinishLogin(f.user, f.existing, f.session, assertWith(t, testRP, auth, removedCred, f.options))

		assert.ErrorIs(t, err, core.ErrPasskeyInvalid)
	})

	t.Run("resposta de cadastro no lugar do login é recusada", func(t *testing.T) {
		f := newLoginFixture(t, 0)
		regOptions, _, err := f.v.BeginRegistration(f.user, nil)
		require.NoError(t, err)
		attestation := attest(t, testRP, newAuthenticator(f.user), newDeviceCredential(), regOptions)

		used, err := f.v.FinishLogin(f.user, f.existing, f.session, attestation)

		assert.ErrorIs(t, err, core.ErrBadRequest)
		assert.Nil(t, used)
	})

	t.Run("corpo malformado é erro do cliente", func(t *testing.T) {
		f := newLoginFixture(t, 0)

		_, err := f.v.FinishLogin(f.user, f.existing, f.session, []byte("{"))

		assert.ErrorIs(t, err, core.ErrBadRequest)
	})

	t.Run("sessão corrompida é erro interno", func(t *testing.T) {
		f := newLoginFixture(t, 0)

		_, err := f.v.FinishLogin(f.user, f.existing, []byte("não é json"), assertWith(t, testRP, f.auth, f.cred, f.options))

		require.Error(t, err)
		assert.False(t, isDomainError(err), "a sessão vem do Redis, não do cliente: deve virar 500")
	})
}
