//go:build integration

package e2e_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/andretop97/UserApiV2/src/bootstrap"
	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/routes"
	"github.com/andretop97/UserApiV2/src/security"
	"github.com/descope/virtualwebauthn"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	rdb "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

// Testes de ponta a ponta do login com passkey: a aplicação é montada pelo bootstrap de verdade
// (migrations, repositórios, services, verificador go-webauthn e rotas) sobre Postgres e Redis em
// containers. Um autenticador virtual faz o papel do navegador e do dispositivo.
//
//	go test -tags integration ./src/e2e/...

const (
	postgresImage = "postgres:18.3"
	redisImage    = "redis:7.2"

	rpOrigin      = "http://localhost:3000"
	sessionSecret = "e2e-session-secret-com-pelo-menos-32-bytes"
	password      = "senha-forte-123"
)

var (
	router     http.Handler
	testPool   *pgxpool.Pool
	testRedis  *rdb.Client
	browserRP  = virtualwebauthn.RelyingParty{ID: "localhost", Name: "UserApiV2", Origin: rpOrigin}
	phishingRP = virtualwebauthn.RelyingParty{ID: "localhost", Name: "UserApiV2", Origin: "http://evil.example.com"}
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()
	gin.SetMode(gin.TestMode)

	pg, err := tcpostgres.Run(ctx, postgresImage,
		tcpostgres.WithDatabase("userapi_e2e"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	defer terminate(pg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "subindo postgres:", err)
		return 1
	}

	redis, err := tcredis.Run(ctx, redisImage)
	defer terminate(redis)
	if err != nil {
		fmt.Fprintln(os.Stderr, "subindo redis:", err)
		return 1
	}

	if err := setEnv(ctx, pg, redis); err != nil {
		fmt.Fprintln(os.Stderr, "variáveis de ambiente:", err)
		return 1
	}

	container, err := bootstrap.NewContainer()
	if err != nil {
		fmt.Fprintln(os.Stderr, "bootstrap:", err)
		return 1
	}
	defer container.Close()
	router = routes.Routes(gin.New(), container.Controllers)

	// Conexões próprias do teste, só para conferir o que a API gravou.
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintln(os.Stderr, "connection string:", err)
		return 1
	}
	testPool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "conexão postgres:", err)
		return 1
	}
	defer testPool.Close()

	uri, err := redis.ConnectionString(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connection string redis:", err)
		return 1
	}
	opts, err := rdb.ParseURL(uri)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse url redis:", err)
		return 1
	}
	testRedis = rdb.NewClient(opts)
	defer testRedis.Close()

	return m.Run()
}

func terminate(c testcontainers.Container) {
	if err := testcontainers.TerminateContainer(c); err != nil {
		fmt.Fprintln(os.Stderr, "encerrando container:", err)
	}
}

// setEnv aponta a configuração da aplicação para os containers. Argon2 fica leve para o teste
// não gastar tempo com hash.
func setEnv(ctx context.Context, pg *tcpostgres.PostgresContainer, redis *tcredis.RedisContainer) error {
	pgHost, err := pg.Host(ctx)
	if err != nil {
		return err
	}
	pgPort, err := pg.MappedPort(ctx, "5432/tcp")
	if err != nil {
		return err
	}
	redisHost, err := redis.Host(ctx)
	if err != nil {
		return err
	}
	redisPort, err := redis.MappedPort(ctx, "6379/tcp")
	if err != nil {
		return err
	}

	vars := map[string]string{
		"POSTGRESQL_HOST":       pgHost,
		"POSTGRESQL_PORT":       pgPort.Port(),
		"POSTGRESQL_USERNAME":   "test",
		"POSTGRESQL_PASSWORD":   "test",
		"POSTGRESQL_DATABASE":   "userapi_e2e",
		"REDIS_HOST":            redisHost,
		"REDIS_PORT":            redisPort.Port(),
		"REDIS_PASSWORD":        "",
		"REDIS_DATABASE":        "0",
		"PEPPER_VERSION":        "1",
		"PEPPER_SECRETS":        "1:" + base64.StdEncoding.EncodeToString([]byte("pepper-do-teste-e2e")),
		"ARGON2_MEMORY":         "8192",
		"ARGON2_ITERATIONS":     "1",
		"ARGON2_PARALLELISM":    "1",
		"MFA_TOKEN_SECRET":      "e2e-mfa-secret-com-pelo-menos-32-bytes!!",
		"SESSION_TOKEN_SECRET":  sessionSecret,
		"LOGIN_TOKEN_TTL":       "10m",
		"SESSION_ABSOLUTE_TTL":  "72h",
		"WEBAUTHN_RP_ID":        "localhost",
		"WEBAUTHN_RP_NAME":      "UserApiV2",
		"WEBAUTHN_RP_ORIGINS":   rpOrigin,
		"WEBAUTHN_CEREMONY_TTL": "5m",
	}
	for k, v := range vars {
		if err := os.Setenv(k, v); err != nil {
			return err
		}
	}
	return nil
}

// --- cliente HTTP ---

func post(t *testing.T, path, bearer, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &v), "corpo: %s", rec.Body.String())
	return v
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	return decode[struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}](t, rec).Error.Code
}

// requireError confere status e código de erro de uma resposta.
func requireError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	require.Equal(t, status, rec.Code, "corpo: %s", rec.Body.String())
	assert.Equal(t, code, errorCode(t, rec))
}

type loginResponse struct {
	Status   string `json:"status"`
	MfaToken string `json:"mfa_token"`
	NextStep string `json:"next_step"`
}

type sessionResponse struct {
	Status       string `json:"status"`
	SessionToken string `json:"session_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// --- passos do fluxo ---

type account struct {
	id    uuid.UUID
	email string
}

func signUp(t *testing.T) account {
	t.Helper()
	email := "e2e-" + uuid.NewString() + "@example.com"
	body := fmt.Sprintf(`{"name":"Usuário E2E","email":%q,"password":%q}`, email, password)

	rec := post(t, "/v1/user/", "", body)

	require.Equal(t, http.StatusCreated, rec.Code, "corpo: %s", rec.Body.String())
	id, err := uuid.Parse(decode[struct {
		ID string `json:"id"`
	}](t, rec).ID)
	require.NoError(t, err)
	return account{id: id, email: email}
}

func login(t *testing.T, acc account) loginResponse {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, acc.email, password)

	rec := post(t, "/v1/auth/login", "", body)

	require.Equal(t, http.StatusOK, rec.Code, "corpo: %s", rec.Body.String())
	res := decode[loginResponse](t, rec)
	require.Equal(t, "mfa_required", res.Status)
	require.NotEmpty(t, res.MfaToken)
	return res
}

// begin chama um endpoint de begin e devolve as options como o navegador as recebe.
func begin(t *testing.T, path, mfaToken string) string {
	t.Helper()
	rec := post(t, path, mfaToken, "")
	require.Equal(t, http.StatusOK, rec.Code, "corpo: %s", rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	return rec.Body.String()
}

func requireSession(t *testing.T, rec *httptest.ResponseRecorder) sessionResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, "corpo: %s", rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	res := decode[sessionResponse](t, rec)
	require.Equal(t, "authenticated", res.Status)
	require.NotEmpty(t, res.SessionToken)
	assert.Equal(t, "Bearer", res.TokenType)
	assert.Equal(t, int((72 * time.Hour).Seconds()), res.ExpiresIn)
	return res
}

// device é o celular/notebook do usuário: o autenticador e a chave privada da passkey.
type device struct {
	auth virtualwebauthn.Authenticator
	cred virtualwebauthn.Credential
}

func newDevice(acc account) *device {
	return &device{
		auth: virtualwebauthn.NewAuthenticatorWithOptions(virtualwebauthn.AuthenticatorOptions{UserHandle: acc.id[:]}),
		cred: virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2),
	}
}

func (d *device) attest(t *testing.T, rp virtualwebauthn.RelyingParty, options string) string {
	t.Helper()
	opts, err := virtualwebauthn.ParseAttestationOptions(options)
	require.NoError(t, err)
	return virtualwebauthn.CreateAttestationResponse(rp, d.auth, d.cred, *opts)
}

// assert assina o login e avança o contador, como um autenticador real faria.
func (d *device) assert(t *testing.T, rp virtualwebauthn.RelyingParty, options string) string {
	t.Helper()
	opts, err := virtualwebauthn.ParseAssertionOptions(options)
	require.NoError(t, err)
	d.cred.Counter++
	return virtualwebauthn.CreateAssertionResponse(rp, d.auth, d.cred, *opts)
}

// registerFirstPasskey faz login → register/begin → register/finish e devolve o dispositivo e a sessão.
func registerFirstPasskey(t *testing.T, acc account) (*device, sessionResponse) {
	t.Helper()
	res := login(t, acc)
	require.Equal(t, string(core.StepWebAuthnRegister), res.NextStep)

	dev := newDevice(acc)
	options := begin(t, "/v1/auth/webauthn/register/begin", res.MfaToken)
	session := requireSession(t, post(t, "/v1/auth/webauthn/register/finish", res.MfaToken, dev.attest(t, browserRP, options)))
	dev.auth.AddCredential(dev.cred)
	return dev, session
}

// loginWithPasskey faz login → webauthn/begin → webauthn/finish com o dispositivo.
func loginWithPasskey(t *testing.T, acc account, dev *device) sessionResponse {
	t.Helper()
	res := login(t, acc)
	require.Equal(t, string(core.StepWebAuthn), res.NextStep)

	options := begin(t, "/v1/auth/webauthn/begin", res.MfaToken)
	return requireSession(t, post(t, "/v1/auth/webauthn/finish", res.MfaToken, dev.assert(t, browserRP, options)))
}

// --- verificações no armazenamento ---

func requireSessionStored(t *testing.T, acc account, token string) {
	t.Helper()
	ctx := context.Background()
	key := "auth:session:v1:" + acc.id.String()

	stored, err := testRedis.Get(ctx, key).Result()
	require.NoError(t, err)
	assert.Equal(t, token, stored)

	ttl, err := testRedis.TTL(ctx, key).Result()
	require.NoError(t, err)
	assert.InDelta(t, (72 * time.Hour).Seconds(), ttl.Seconds(), 10)

	// O token é um JWT de sessão do próprio usuário, assinado com o segredo de sessão.
	sessions, err := security.NewJwtProvider[core.SessionClaims](security.JwtConfig{
		SecretKey: []byte(sessionSecret),
		TTL:       72 * time.Hour,
		Issuer:    "user-api",
		Audience:  "session",
	})
	require.NoError(t, err)
	_, err = sessions.ValidateToken(token)
	assert.NoError(t, err)
}

func requireMfaTokenGone(t *testing.T, acc account) {
	t.Helper()
	exists, err := testRedis.Exists(context.Background(), "auth:mfa:v1:"+acc.id.String()).Result()
	require.NoError(t, err)
	assert.Zero(t, exists, "o mfa_token tem que ser descartado depois do segundo fator")
}

type storedPasskey struct {
	count      int
	lastUsedAt *time.Time
}

func passkeysOf(t *testing.T, acc account) storedPasskey {
	t.Helper()
	var p storedPasskey
	err := testPool.QueryRow(context.Background(),
		`SELECT COUNT(*), MAX(LastUsedAt) FROM passkeys WHERE UserId = $1`, acc.id).Scan(&p.count, &p.lastUsedAt)
	require.NoError(t, err)
	return p
}

// --- testes ---

func TestE2EPasskeyHappyPath(t *testing.T) {
	acc := signUp(t)

	// Primeiro acesso: senha → cadastro da passkey → já sai logado (D2).
	dev, first := registerFirstPasskey(t, acc)
	requireSessionStored(t, acc, first.SessionToken)
	requireMfaTokenGone(t, acc)
	stored := passkeysOf(t, acc)
	assert.Equal(t, 1, stored.count)
	assert.Nil(t, stored.lastUsedAt, "passkey recém-cadastrada ainda não foi usada para login")

	// Acessos seguintes: senha → passkey.
	second := loginWithPasskey(t, acc, dev)
	requireSessionStored(t, acc, second.SessionToken)
	requireMfaTokenGone(t, acc)
	stored = passkeysOf(t, acc)
	assert.Equal(t, 1, stored.count, "login não cria passkey")
	require.NotNil(t, stored.lastUsedAt)
	assert.WithinDuration(t, time.Now(), *stored.lastUsedAt, time.Minute)

	// O contador gravado acompanha o dispositivo: logins seguidos continuam funcionando.
	loginWithPasskey(t, acc, dev)
}

func TestE2ERegistration(t *testing.T) {
	t.Run("mfa_token não serve depois de usado no cadastro", func(t *testing.T) {
		acc := signUp(t)
		res := login(t, acc)
		dev := newDevice(acc)
		options := begin(t, "/v1/auth/webauthn/register/begin", res.MfaToken)
		requireSession(t, post(t, "/v1/auth/webauthn/register/finish", res.MfaToken, dev.attest(t, browserRP, options)))

		requireError(t, post(t, "/v1/auth/webauthn/register/begin", res.MfaToken, ""), http.StatusUnauthorized, "TOKEN_INVALID")
		requireError(t, post(t, "/v1/auth/webauthn/begin", res.MfaToken, ""), http.StatusUnauthorized, "TOKEN_INVALID")
	})

	t.Run("segunda passkey não pode ser cadastrada só com a senha (D1)", func(t *testing.T) {
		acc := signUp(t)
		registerFirstPasskey(t, acc)
		res := login(t, acc)

		requireError(t, post(t, "/v1/auth/webauthn/register/begin", res.MfaToken, ""), http.StatusForbidden, "PASSKEY_REGISTRATION_NOT_ALLOWED")
		assert.Equal(t, 1, passkeysOf(t, acc).count)
	})

	t.Run("dois cadastros abertos ao mesmo tempo: só o primeiro finish vale", func(t *testing.T) {
		acc := signUp(t)
		res := login(t, acc)
		optionsA := begin(t, "/v1/auth/webauthn/register/begin", res.MfaToken)
		dev := newDevice(acc)
		requireSession(t, post(t, "/v1/auth/webauthn/register/finish", res.MfaToken, dev.attest(t, browserRP, optionsA)))

		other := newDevice(acc)
		rec := post(t, "/v1/auth/webauthn/register/finish", res.MfaToken, other.attest(t, browserRP, optionsA))

		assert.Equal(t, http.StatusUnauthorized, rec.Code, "corpo: %s", rec.Body.String())
		assert.Equal(t, 1, passkeysOf(t, acc).count)
	})

	t.Run("novo begin invalida o challenge anterior", func(t *testing.T) {
		acc := signUp(t)
		res := login(t, acc)
		dev := newDevice(acc)
		stale := begin(t, "/v1/auth/webauthn/register/begin", res.MfaToken)
		begin(t, "/v1/auth/webauthn/register/begin", res.MfaToken)

		rec := post(t, "/v1/auth/webauthn/register/finish", res.MfaToken, dev.attest(t, browserRP, stale))

		requireError(t, rec, http.StatusUnauthorized, "PASSKEY_INVALID")
		assert.Zero(t, passkeysOf(t, acc).count)
	})

	t.Run("finish sem begin é cerimônia expirada", func(t *testing.T) {
		acc := signUp(t)
		res := login(t, acc)

		rec := post(t, "/v1/auth/webauthn/register/finish", res.MfaToken, `{"id":"x"}`)

		requireError(t, rec, http.StatusUnauthorized, "CEREMONY_EXPIRED")
	})

	t.Run("origem de phishing é recusada e o usuário pode tentar de novo", func(t *testing.T) {
		acc := signUp(t)
		res := login(t, acc)
		dev := newDevice(acc)
		options := begin(t, "/v1/auth/webauthn/register/begin", res.MfaToken)

		rec := post(t, "/v1/auth/webauthn/register/finish", res.MfaToken, dev.attest(t, phishingRP, options))

		requireError(t, rec, http.StatusUnauthorized, "PASSKEY_INVALID")
		assert.Zero(t, passkeysOf(t, acc).count)

		// A falha não queima o mfa_token: o site verdadeiro ainda consegue concluir o cadastro.
		options = begin(t, "/v1/auth/webauthn/register/begin", res.MfaToken)
		requireSession(t, post(t, "/v1/auth/webauthn/register/finish", res.MfaToken, dev.attest(t, browserRP, options)))
	})

	t.Run("cerimônia aberta por um usuário não serve para outro", func(t *testing.T) {
		victim := signUp(t)
		attacker := signUp(t)
		res := login(t, victim)
		options := begin(t, "/v1/auth/webauthn/register/begin", res.MfaToken)

		// O atacante tenta usar o mfa_token da própria conta com a cerimônia aberta da vítima.
		attackerRes := login(t, attacker)
		rec := post(t, "/v1/auth/webauthn/register/finish", attackerRes.MfaToken, newDevice(attacker).attest(t, browserRP, options))

		requireError(t, rec, http.StatusUnauthorized, "CEREMONY_EXPIRED")
		assert.Zero(t, passkeysOf(t, victim).count)
		assert.Zero(t, passkeysOf(t, attacker).count)
	})
}

func TestE2EPasskeyLogin(t *testing.T) {
	t.Run("sem passkey cadastrada o login por passkey é 404", func(t *testing.T) {
		acc := signUp(t)
		res := login(t, acc)

		requireError(t, post(t, "/v1/auth/webauthn/begin", res.MfaToken, ""), http.StatusNotFound, "PASSKEY_NOT_FOUND")
	})

	t.Run("sem mfa_token não há cerimônia", func(t *testing.T) {
		requireError(t, post(t, "/v1/auth/webauthn/begin", "", ""), http.StatusUnauthorized, "TOKEN_INVALID")
		requireError(t, post(t, "/v1/auth/webauthn/begin", "token-forjado", ""), http.StatusUnauthorized, "TOKEN_INVALID")
	})

	t.Run("resposta reenviada não abre segunda sessão", func(t *testing.T) {
		acc := signUp(t)
		dev, _ := registerFirstPasskey(t, acc)
		res := login(t, acc)
		options := begin(t, "/v1/auth/webauthn/begin", res.MfaToken)
		answer := dev.assert(t, browserRP, options)
		requireSession(t, post(t, "/v1/auth/webauthn/finish", res.MfaToken, answer))

		rec := post(t, "/v1/auth/webauthn/finish", res.MfaToken, answer)

		requireError(t, rec, http.StatusUnauthorized, "TOKEN_INVALID")
	})

	t.Run("cerimônia é de uso único mesmo com o mfa_token ainda válido", func(t *testing.T) {
		acc := signUp(t)
		dev, _ := registerFirstPasskey(t, acc)
		res := login(t, acc)
		options := begin(t, "/v1/auth/webauthn/begin", res.MfaToken)

		// Primeira resposta falha (origem errada) e consome a cerimônia.
		requireError(t, post(t, "/v1/auth/webauthn/finish", res.MfaToken, dev.assert(t, phishingRP, options)), http.StatusUnauthorized, "PASSKEY_INVALID")

		rec := post(t, "/v1/auth/webauthn/finish", res.MfaToken, dev.assert(t, browserRP, options))

		requireError(t, rec, http.StatusUnauthorized, "CEREMONY_EXPIRED")
	})

	t.Run("cadastro e login não compartilham cerimônia", func(t *testing.T) {
		acc := signUp(t)
		dev, _ := registerFirstPasskey(t, acc)
		res := login(t, acc)
		options := begin(t, "/v1/auth/webauthn/begin", res.MfaToken)

		// A resposta de login enviada ao finish de cadastro não encontra cerimônia de cadastro.
		rec := post(t, "/v1/auth/webauthn/register/finish", res.MfaToken, dev.assert(t, browserRP, options))

		requireError(t, rec, http.StatusUnauthorized, "CEREMONY_EXPIRED")
		assert.Equal(t, 1, passkeysOf(t, acc).count)
	})

	t.Run("passkey de outro usuário não autentica", func(t *testing.T) {
		victim := signUp(t)
		registerFirstPasskey(t, victim)
		attacker := signUp(t)
		attackerDev, _ := registerFirstPasskey(t, attacker)

		res := login(t, victim)
		options := begin(t, "/v1/auth/webauthn/begin", res.MfaToken)
		rec := post(t, "/v1/auth/webauthn/finish", res.MfaToken, attackerDev.assert(t, browserRP, options))

		requireError(t, rec, http.StatusUnauthorized, "PASSKEY_INVALID")
	})

	t.Run("autenticador clonado (contador repetido) é recusado", func(t *testing.T) {
		acc := signUp(t)
		dev, _ := registerFirstPasskey(t, acc)
		loginWithPasskey(t, acc, dev)
		clone := *dev
		clone.cred.Counter-- // o clone ficou com o contador antigo

		res := login(t, acc)
		options := begin(t, "/v1/auth/webauthn/begin", res.MfaToken)
		rec := post(t, "/v1/auth/webauthn/finish", res.MfaToken, clone.assert(t, browserRP, options))

		requireError(t, rec, http.StatusUnauthorized, "PASSKEY_INVALID")
	})

	t.Run("usuário sem verificação (UV) é recusado", func(t *testing.T) {
		acc := signUp(t)
		dev, _ := registerFirstPasskey(t, acc)
		dev.auth.Options.UserNotVerified = true

		res := login(t, acc)
		options := begin(t, "/v1/auth/webauthn/begin", res.MfaToken)
		rec := post(t, "/v1/auth/webauthn/finish", res.MfaToken, dev.assert(t, browserRP, options))

		requireError(t, rec, http.StatusUnauthorized, "PASSKEY_INVALID")
	})
}
