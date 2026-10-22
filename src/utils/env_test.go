package utils

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validWebAuthnEnv() WebAuthnEnv {
	return WebAuthnEnv{
		RPID:        "localhost",
		RPName:      "UserApiV2",
		RPOrigins:   []string{"http://localhost:8080", "http://localhost:3000"},
		CeremonyTTL: 5 * time.Minute,
	}
}

func TestWebAuthnEnvValidate(t *testing.T) {
	t.Run("configuração de desenvolvimento é válida", func(t *testing.T) {
		e := validWebAuthnEnv()

		assert.NoError(t, e.validate())
	})

	t.Run("domínio de produção e subdomínio são válidos", func(t *testing.T) {
		for _, rpID := range []string{"meusite.com", "auth.meusite.com"} {
			e := validWebAuthnEnv()
			e.RPID = rpID
			e.RPOrigins = []string{"https://" + rpID}

			assert.NoError(t, e.validate(), rpID)
		}
	})

	t.Run("RP ID precisa ser só o host", func(t *testing.T) {
		for _, rpID := range []string{
			"https://meusite.com",
			"localhost:3000",
			"meusite.com/login",
		} {
			e := validWebAuthnEnv()
			e.RPID = rpID

			err := e.validate()

			require.Error(t, err, rpID)
			assert.Contains(t, err.Error(), "WEBAUTHN_RP_ID")
		}
	})

	t.Run("origem precisa ser scheme://host[:porta] sem barra final", func(t *testing.T) {
		for _, origin := range []string{
			"http://localhost:3000/",
			"http://localhost:3000/login",
			"localhost:3000",
			"//localhost:3000",
			"",
		} {
			e := validWebAuthnEnv()
			e.RPOrigins = []string{"http://localhost:8080", origin}

			err := e.validate()

			require.Error(t, err, "origem %q", origin)
			assert.Contains(t, err.Error(), "WEBAUTHN_RP_ORIGINS")
		}
	})

	t.Run("TTL da cerimônia precisa ser positivo", func(t *testing.T) {
		for _, ttl := range []time.Duration{0, -time.Minute} {
			e := validWebAuthnEnv()
			e.CeremonyTTL = ttl

			err := e.validate()

			require.Error(t, err, ttl)
			assert.Contains(t, err.Error(), "WEBAUTHN_CEREMONY_TTL")
		}
	})
}

// unsetEnv remove a variável durante o teste; o t.Setenv antes garante a restauração no fim.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	require.NoError(t, os.Unsetenv(key))
}

func TestNewWebAuthnEnv(t *testing.T) {
	setRequired := func(t *testing.T) {
		t.Setenv("WEBAUTHN_RP_ID", "localhost")
		t.Setenv("WEBAUTHN_RP_ORIGINS", "http://localhost:8080,http://localhost:3000")
	}

	t.Run("lê as variáveis e separa as origens por vírgula", func(t *testing.T) {
		setRequired(t)
		t.Setenv("WEBAUTHN_RP_NAME", "Outro Nome")
		t.Setenv("WEBAUTHN_CEREMONY_TTL", "2m")

		e, err := NewWebAuthnEnv()

		require.NoError(t, err)
		assert.Equal(t, "localhost", e.RPID)
		assert.Equal(t, "Outro Nome", e.RPName)
		assert.Equal(t, []string{"http://localhost:8080", "http://localhost:3000"}, e.RPOrigins)
		assert.Equal(t, 2*time.Minute, e.CeremonyTTL)
	})

	t.Run("aplica os defaults do nome e do TTL", func(t *testing.T) {
		setRequired(t)
		t.Setenv("WEBAUTHN_RP_NAME", "")
		t.Setenv("WEBAUTHN_CEREMONY_TTL", "")

		e, err := NewWebAuthnEnv()

		require.NoError(t, err)
		assert.Equal(t, "UserApiV2", e.RPName)
		assert.Equal(t, 5*time.Minute, e.CeremonyTTL)
	})

	t.Run("RP ID ausente falha", func(t *testing.T) {
		setRequired(t)
		unsetEnv(t, "WEBAUTHN_RP_ID")

		_, err := NewWebAuthnEnv()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "WEBAUTHN_RP_ID")
	})

	t.Run("origens ausentes falham", func(t *testing.T) {
		setRequired(t)
		unsetEnv(t, "WEBAUTHN_RP_ORIGINS")

		_, err := NewWebAuthnEnv()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "WEBAUTHN_RP_ORIGINS")
	})

	// "WEBAUTHN_RP_ID=" no .env define a variável com valor vazio; o RP ID vazio passa até pela lib.
	t.Run("RP ID vazio falha", func(t *testing.T) {
		setRequired(t)
		t.Setenv("WEBAUTHN_RP_ID", "")

		_, err := NewWebAuthnEnv()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "WEBAUTHN_RP_ID")
	})

	t.Run("origens vazias falham", func(t *testing.T) {
		setRequired(t)
		t.Setenv("WEBAUTHN_RP_ORIGINS", "")

		_, err := NewWebAuthnEnv()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "WEBAUTHN_RP_ORIGINS")
	})

	t.Run("configuração inválida falha no construtor", func(t *testing.T) {
		setRequired(t)
		t.Setenv("WEBAUTHN_RP_ORIGINS", "http://localhost:3000/")

		_, err := NewWebAuthnEnv()

		assert.Error(t, err)
	})
}

func TestSecretsMustNotBeEmpty(t *testing.T) {
	setAuth := func(t *testing.T) {
		t.Setenv("MFA_TOKEN_SECRET", "mfa-secret")
		t.Setenv("SESSION_TOKEN_SECRET", "session-secret")
	}

	t.Run("AuthEnv com os dois segredos é válido", func(t *testing.T) {
		setAuth(t)

		_, err := NewAuthEnv()

		assert.NoError(t, err)
	})

	for _, key := range []string{"MFA_TOKEN_SECRET", "SESSION_TOKEN_SECRET"} {
		t.Run(key+" vazio falha", func(t *testing.T) {
			setAuth(t)
			t.Setenv(key, "")

			_, err := NewAuthEnv()

			require.Error(t, err)
			assert.Contains(t, err.Error(), key)
		})

		t.Run(key+" ausente falha", func(t *testing.T) {
			setAuth(t)
			unsetEnv(t, key)

			_, err := NewAuthEnv()

			require.Error(t, err)
			assert.Contains(t, err.Error(), key)
		})
	}

	t.Run("PEPPER_SECRETS vazio falha", func(t *testing.T) {
		t.Setenv("PEPPER_SECRETS", "")

		_, err := NewPepperEnv()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "PEPPER_SECRETS")
	})

	t.Run("PEPPER_SECRETS ausente falha", func(t *testing.T) {
		unsetEnv(t, "PEPPER_SECRETS")

		_, err := NewPepperEnv()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "PEPPER_SECRETS")
	})
}
