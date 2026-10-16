package security

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Testes dos detalhes privados do adapter (webauthnUser e conversões).
// Os quatro métodos públicos ainda são stubs e serão testados quando forem implementados.

// webauthnUser precisa satisfazer a interface da lib; se um método faltar, o pacote não compila.
var _ webauthn.User = webauthnUser{}

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
