package security_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/security"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testSecret  = []byte("0123456789abcdef0123456789abcdef")
	otherSecret = []byte("fedcba9876543210fedcba9876543210")
)

func jwtConfig(audience string) security.JwtConfig {
	return security.JwtConfig{
		SecretKey: testSecret,
		TTL:       10 * time.Minute,
		Issuer:    "user-api",
		Audience:  audience,
	}
}

func newMfaProvider(t *testing.T, cfg security.JwtConfig) core.JwtProvider[core.MfaClaims] {
	t.Helper()
	p, err := security.NewJwtProvider[core.MfaClaims](cfg)
	require.NoError(t, err)
	return p
}

// decodeClaims lê o payload do JWT sem validar a assinatura, para inspecionar as claims geradas.
func decodeClaims(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(raw, &claims))
	return claims
}

// signRaw assina claims arbitrárias, para simular tokens forjados ou malformados.
func signRaw(t *testing.T, method jwt.SigningMethod, claims jwt.MapClaims, key any) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, claims).SignedString(key)
	require.NoError(t, err)
	return s
}

// assertInvalidTokenError verifica que um token rejeitado vira um erro de domínio 401,
// sem vazar o erro cru da biblioteca jwt para as camadas de cima.
func assertInvalidTokenError(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var domainErr *core.DomainError
	if assert.True(t, errors.As(err, &domainErr), "esperado *core.DomainError, veio %T: %v", err, err) {
		assert.Equal(t, 401, domainErr.Status)
	}
}

func TestNewJwtProvider(t *testing.T) {
	t.Run("rejeita segredo com menos de 32 bytes", func(t *testing.T) {
		cfg := jwtConfig("mfa")
		cfg.SecretKey = []byte("curto")

		p, err := security.NewJwtProvider[core.MfaClaims](cfg)

		assert.Error(t, err)
		assert.Nil(t, p)
	})

	t.Run("aceita segredo com exatamente 32 bytes", func(t *testing.T) {
		_, err := security.NewJwtProvider[core.MfaClaims](jwtConfig("mfa"))
		assert.NoError(t, err)
	})

	t.Run("TTL devolve o valor configurado", func(t *testing.T) {
		p := newMfaProvider(t, jwtConfig("mfa"))
		assert.Equal(t, 10*time.Minute, p.TTL())
	})
}

func TestJwtGenerateToken(t *testing.T) {
	t.Run("ida e volta preserva o payload", func(t *testing.T) {
		p := newMfaProvider(t, jwtConfig("mfa"))
		payload := core.MfaClaims{UserID: uuid.New()}

		token, err := p.GenerateToken(payload)
		require.NoError(t, err)
		got, err := p.ValidateToken(token)

		require.NoError(t, err)
		assert.Equal(t, payload, got)
	})

	t.Run("preenche iss, aud, iat, exp e jti", func(t *testing.T) {
		p := newMfaProvider(t, jwtConfig("mfa"))
		before := time.Now().Unix()

		token, err := p.GenerateToken(core.MfaClaims{UserID: uuid.New()})
		require.NoError(t, err)
		c := decodeClaims(t, token)

		assert.Equal(t, "user-api", c["iss"])
		assert.Equal(t, []any{"mfa"}, c["aud"])
		assert.NotEmpty(t, c["jti"])
		iat := int64(c["iat"].(float64))
		exp := int64(c["exp"].(float64))
		assert.GreaterOrEqual(t, iat, before)
		assert.Equal(t, int64((10 * time.Minute).Seconds()), exp-iat)
	})

	t.Run("cada token tem um jti diferente", func(t *testing.T) {
		p := newMfaProvider(t, jwtConfig("mfa"))
		payload := core.MfaClaims{UserID: uuid.New()}

		t1, err := p.GenerateToken(payload)
		require.NoError(t, err)
		t2, err := p.GenerateToken(payload)
		require.NoError(t, err)

		assert.NotEqual(t, decodeClaims(t, t1)["jti"], decodeClaims(t, t2)["jti"])
	})

	t.Run("assina com HS256", func(t *testing.T) {
		p := newMfaProvider(t, jwtConfig("mfa"))
		token, err := p.GenerateToken(core.MfaClaims{UserID: uuid.New()})
		require.NoError(t, err)

		header, err := base64.RawURLEncoding.DecodeString(strings.Split(token, ".")[0])
		require.NoError(t, err)
		assert.Contains(t, string(header), `"alg":"HS256"`)
	})
}

func TestJwtValidateToken(t *testing.T) {
	validClaims := func() jwt.MapClaims {
		now := time.Now()
		return jwt.MapClaims{
			"data": map[string]any{"user_id": uuid.NewString()},
			"iss":  "user-api",
			"aud":  []string{"mfa"},
			"iat":  now.Unix(),
			"exp":  now.Add(time.Minute).Unix(),
		}
	}

	t.Run("token expirado vira ErrTokenExpired", func(t *testing.T) {
		cfg := jwtConfig("mfa")
		cfg.TTL = -time.Minute
		p := newMfaProvider(t, cfg)
		token, err := p.GenerateToken(core.MfaClaims{UserID: uuid.New()})
		require.NoError(t, err)

		_, err = p.ValidateToken(token)

		assert.ErrorIs(t, err, core.ErrTokenExpired)
	})

	t.Run("token MFA não é aceito pelo provider de sessão", func(t *testing.T) {
		mfa := newMfaProvider(t, jwtConfig("mfa"))
		session, err := security.NewJwtProvider[core.SessionClaims](jwtConfig("session"))
		require.NoError(t, err)
		token, err := mfa.GenerateToken(core.MfaClaims{UserID: uuid.New()})
		require.NoError(t, err)

		_, err = session.ValidateToken(token)

		assertInvalidTokenError(t, err)
	})

	t.Run("rejeita token assinado com outro segredo", func(t *testing.T) {
		other := jwtConfig("mfa")
		other.SecretKey = otherSecret
		token, err := newMfaProvider(t, other).GenerateToken(core.MfaClaims{UserID: uuid.New()})
		require.NoError(t, err)

		_, err = newMfaProvider(t, jwtConfig("mfa")).ValidateToken(token)

		assertInvalidTokenError(t, err)
	})

	t.Run("rejeita token de outro emissor", func(t *testing.T) {
		other := jwtConfig("mfa")
		other.Issuer = "outra-api"
		token, err := newMfaProvider(t, other).GenerateToken(core.MfaClaims{UserID: uuid.New()})
		require.NoError(t, err)

		_, err = newMfaProvider(t, jwtConfig("mfa")).ValidateToken(token)

		assertInvalidTokenError(t, err)
	})

	t.Run("rejeita algoritmo diferente de HS256 mesmo com o segredo certo", func(t *testing.T) {
		token := signRaw(t, jwt.SigningMethodHS512, validClaims(), testSecret)

		_, err := newMfaProvider(t, jwtConfig("mfa")).ValidateToken(token)

		assertInvalidTokenError(t, err)
	})

	t.Run("rejeita alg none", func(t *testing.T) {
		token := signRaw(t, jwt.SigningMethodNone, validClaims(), jwt.UnsafeAllowNoneSignatureType)

		_, err := newMfaProvider(t, jwtConfig("mfa")).ValidateToken(token)

		assertInvalidTokenError(t, err)
	})

	t.Run("rejeita token sem exp", func(t *testing.T) {
		claims := validClaims()
		delete(claims, "exp")
		token := signRaw(t, jwt.SigningMethodHS256, claims, testSecret)

		_, err := newMfaProvider(t, jwtConfig("mfa")).ValidateToken(token)

		assertInvalidTokenError(t, err)
	})

	t.Run("rejeita token com payload adulterado", func(t *testing.T) {
		p := newMfaProvider(t, jwtConfig("mfa"))
		token, err := p.GenerateToken(core.MfaClaims{UserID: uuid.New()})
		require.NoError(t, err)

		parts := strings.Split(token, ".")
		forged := validClaims()
		raw, err := json.Marshal(forged)
		require.NoError(t, err)
		parts[1] = base64.RawURLEncoding.EncodeToString(raw)

		_, err = p.ValidateToken(strings.Join(parts, "."))

		assertInvalidTokenError(t, err)
	})

	t.Run("rejeita string que não é JWT", func(t *testing.T) {
		_, err := newMfaProvider(t, jwtConfig("mfa")).ValidateToken("isso-nao-e-um-jwt")

		assertInvalidTokenError(t, err)
	})

	// O respondError usa err.Error() como mensagem da resposta HTTP, então o texto do erro
	// não pode carregar o detalhe da lib jwt (qual claim falhou, algoritmo esperado...).
	t.Run("mensagem do erro não expõe o motivo interno da rejeição", func(t *testing.T) {
		_, err := newMfaProvider(t, jwtConfig("mfa")).ValidateToken("isso-nao-e-um-jwt")

		require.Error(t, err)
		assert.Equal(t, core.ErrTokenInvalid.Error(), err.Error())
	})

	t.Run("token rejeitado devolve payload zerado", func(t *testing.T) {
		got, err := newMfaProvider(t, jwtConfig("mfa")).ValidateToken("isso-nao-e-um-jwt")

		require.Error(t, err)
		assert.Equal(t, core.MfaClaims{}, got)
	})
}
