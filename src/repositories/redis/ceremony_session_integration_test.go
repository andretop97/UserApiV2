//go:build integration

package redis_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/repositories/redis"
	"github.com/google/uuid"
	rdb "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Usa o container e o testClient criados no TestMain de user_cache_integration_test.go.
//
//	go test -tags integration ./src/repositories/redis/...

const ceremonyTTL = 5 * time.Minute

func ceremonyKey(id uuid.UUID, ceremony core.CeremonyType) string {
	return "auth:webauthn:v1:" + string(ceremony) + ":" + id.String()
}

// sessionBytes imita o JSON da SessionData, com um byte não UTF-8 para provar que o valor é guardado como binário.
func sessionBytes(challenge string) []byte {
	return append([]byte(`{"challenge":"`+challenge+`"}`), 0xff)
}

func TestAuthRepositoryCeremonySession(t *testing.T) {
	ctx := context.Background()

	for _, ceremony := range []core.CeremonyType{core.CeremonyRegistration, core.CeremonyLogin} {
		t.Run(string(ceremony)+": grava e consome os mesmos bytes", func(t *testing.T) {
			repo := setupAuth(t)
			id := uuid.New()
			want := sessionBytes("abc")

			require.NoError(t, repo.SaveCeremonySession(ctx, id, ceremony, want, ceremonyTTL))
			got, err := repo.ConsumeCeremonySession(ctx, id, ceremony)

			require.NoError(t, err)
			assert.Equal(t, want, got)
		})

		t.Run(string(ceremony)+": usa a chave auth:webauthn:v1:<tipo>:<id> com o TTL informado", func(t *testing.T) {
			repo := setupAuth(t)
			id := uuid.New()

			require.NoError(t, repo.SaveCeremonySession(ctx, id, ceremony, sessionBytes("abc"), ceremonyTTL))

			got, err := testClient.Get(ctx, ceremonyKey(id, ceremony)).Bytes()
			require.NoError(t, err)
			assert.Equal(t, sessionBytes("abc"), got)
			ttl, err := testClient.TTL(ctx, ceremonyKey(id, ceremony)).Result()
			require.NoError(t, err)
			assertWithin(t, ceremonyTTL, ttl)
		})
	}

	t.Run("consumir apaga a chave", func(t *testing.T) {
		repo := setupAuth(t)
		id := uuid.New()
		require.NoError(t, repo.SaveCeremonySession(ctx, id, core.CeremonyRegistration, sessionBytes("abc"), ceremonyTTL))

		_, err := repo.ConsumeCeremonySession(ctx, id, core.CeremonyRegistration)
		require.NoError(t, err)

		exists, err := testClient.Exists(ctx, ceremonyKey(id, core.CeremonyRegistration)).Result()
		require.NoError(t, err)
		assert.Zero(t, exists)
	})

	t.Run("segundo consumo falha: o challenge vale uma vez só", func(t *testing.T) {
		repo := setupAuth(t)
		id := uuid.New()
		require.NoError(t, repo.SaveCeremonySession(ctx, id, core.CeremonyLogin, sessionBytes("abc"), ceremonyTTL))
		_, err := repo.ConsumeCeremonySession(ctx, id, core.CeremonyLogin)
		require.NoError(t, err)

		got, err := repo.ConsumeCeremonySession(ctx, id, core.CeremonyLogin)

		assert.ErrorIs(t, err, core.ErrCeremonyExpired)
		assert.Nil(t, got)
	})

	t.Run("sessão expirada não pode ser consumida", func(t *testing.T) {
		repo := setupAuth(t)
		id := uuid.New()
		require.NoError(t, repo.SaveCeremonySession(ctx, id, core.CeremonyLogin, sessionBytes("abc"), 50*time.Millisecond))

		require.Eventually(t, func() bool {
			n, err := testClient.Exists(ctx, ceremonyKey(id, core.CeremonyLogin)).Result()
			return err == nil && n == 0
		}, 2*time.Second, 20*time.Millisecond)
		_, err := repo.ConsumeCeremonySession(ctx, id, core.CeremonyLogin)

		assert.ErrorIs(t, err, core.ErrCeremonyExpired)
	})

	t.Run("sessão inexistente vira erro de domínio, sem vazar redis.Nil", func(t *testing.T) {
		repo := setupAuth(t)

		got, err := repo.ConsumeCeremonySession(ctx, uuid.New(), core.CeremonyRegistration)

		assert.ErrorIs(t, err, core.ErrCeremonyExpired)
		assert.False(t, errors.Is(err, rdb.Nil), "o service não deveria depender de erros do go-redis")
		assert.Nil(t, got)
	})

	// No go-redis, TTL 0 grava sem expiração e -1ns é KEEPTTL: o challenge valeria para sempre.
	for name, ttl := range map[string]time.Duration{"zero": 0, "KeepTTL": rdb.KeepTTL, "negativo": -time.Minute} {
		t.Run("TTL "+name+" é recusado e nada é gravado", func(t *testing.T) {
			repo := setupAuth(t)
			id := uuid.New()

			err := repo.SaveCeremonySession(ctx, id, core.CeremonyLogin, sessionBytes("abc"), ttl)

			require.Error(t, err)
			var domainErr *core.DomainError
			assert.False(t, errors.As(err, &domainErr), "TTL inválido é erro do servidor, não do cliente")
			exists, err := testClient.Exists(ctx, ceremonyKey(id, core.CeremonyLogin)).Result()
			require.NoError(t, err)
			assert.Zero(t, exists)
		})
	}

	t.Run("TTL inválido não apaga uma sessão já em andamento", func(t *testing.T) {
		repo := setupAuth(t)
		id := uuid.New()
		require.NoError(t, repo.SaveCeremonySession(ctx, id, core.CeremonyLogin, sessionBytes("valida"), ceremonyTTL))

		require.Error(t, repo.SaveCeremonySession(ctx, id, core.CeremonyLogin, sessionBytes("sem-ttl"), 0))

		got, err := repo.ConsumeCeremonySession(ctx, id, core.CeremonyLogin)
		require.NoError(t, err)
		assert.Equal(t, sessionBytes("valida"), got)
	})

	t.Run("novo begin substitui o anterior: só o challenge mais recente vale", func(t *testing.T) {
		repo := setupAuth(t)
		id := uuid.New()

		require.NoError(t, repo.SaveCeremonySession(ctx, id, core.CeremonyRegistration, sessionBytes("primeira-aba"), ceremonyTTL))
		require.NoError(t, repo.SaveCeremonySession(ctx, id, core.CeremonyRegistration, sessionBytes("segunda-aba"), ceremonyTTL))
		got, err := repo.ConsumeCeremonySession(ctx, id, core.CeremonyRegistration)

		require.NoError(t, err)
		assert.Equal(t, sessionBytes("segunda-aba"), got)
	})
}

func TestAuthRepositoryCeremonySessionIsolation(t *testing.T) {
	ctx := context.Background()

	t.Run("cadastro e login do mesmo usuário não colidem", func(t *testing.T) {
		repo := setupAuth(t)
		id := uuid.New()
		require.NoError(t, repo.SaveCeremonySession(ctx, id, core.CeremonyRegistration, sessionBytes("register"), ceremonyTTL))
		require.NoError(t, repo.SaveCeremonySession(ctx, id, core.CeremonyLogin, sessionBytes("login"), ceremonyTTL))

		login, err := repo.ConsumeCeremonySession(ctx, id, core.CeremonyLogin)
		require.NoError(t, err)
		register, err := repo.ConsumeCeremonySession(ctx, id, core.CeremonyRegistration)
		require.NoError(t, err)

		assert.Equal(t, sessionBytes("login"), login)
		assert.Equal(t, sessionBytes("register"), register)
	})

	t.Run("consumir o login não apaga o cadastro em andamento", func(t *testing.T) {
		repo := setupAuth(t)
		id := uuid.New()
		require.NoError(t, repo.SaveCeremonySession(ctx, id, core.CeremonyRegistration, sessionBytes("register"), ceremonyTTL))

		_, err := repo.ConsumeCeremonySession(ctx, id, core.CeremonyLogin)
		assert.ErrorIs(t, err, core.ErrCeremonyExpired)

		got, err := repo.ConsumeCeremonySession(ctx, id, core.CeremonyRegistration)
		require.NoError(t, err)
		assert.Equal(t, sessionBytes("register"), got)
	})

	t.Run("sessões de usuários diferentes não colidem", func(t *testing.T) {
		repo := setupAuth(t)
		a, b := uuid.New(), uuid.New()
		require.NoError(t, repo.SaveCeremonySession(ctx, a, core.CeremonyLogin, sessionBytes("a"), ceremonyTTL))
		require.NoError(t, repo.SaveCeremonySession(ctx, b, core.CeremonyLogin, sessionBytes("b"), ceremonyTTL))

		got, err := repo.ConsumeCeremonySession(ctx, a, core.CeremonyLogin)

		require.NoError(t, err)
		assert.Equal(t, sessionBytes("a"), got)
		_, err = repo.ConsumeCeremonySession(ctx, b, core.CeremonyLogin)
		assert.NoError(t, err, "consumir a sessão de A não pode apagar a de B")
	})

	t.Run("sessão de cerimônia não colide com os tokens MFA e de sessão", func(t *testing.T) {
		repo := setupAuth(t)
		id := uuid.New()
		require.NoError(t, repo.SetTemporaryToken(ctx, id, "mfa-token", mfaTTL))
		require.NoError(t, repo.SetSessionToken(ctx, id, "session-token", sessionTTL))
		require.NoError(t, repo.SaveCeremonySession(ctx, id, core.CeremonyLogin, sessionBytes("login"), ceremonyTTL))

		_, err := repo.ConsumeCeremonySession(ctx, id, core.CeremonyLogin)
		require.NoError(t, err)

		mfa, err := repo.GetTemporaryToken(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, "mfa-token", mfa)
		session, err := repo.GetSessionToken(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, "session-token", session)
	})
}

// Duas abas (ou um atacante) fazendo o finish ao mesmo tempo: só uma pode receber a sessão.
// É o que impede, por exemplo, duas primeiras passkeys cadastradas só com o mfa_token (D1).
func TestAuthRepositoryCeremonySessionConcurrentConsume(t *testing.T) {
	ctx := context.Background()
	const attempts = 50

	for round := 0; round < 20; round++ {
		repo := setupAuth(t)
		id := uuid.New()
		require.NoError(t, repo.SaveCeremonySession(ctx, id, core.CeremonyRegistration, sessionBytes("abc"), ceremonyTTL))

		var (
			wg       sync.WaitGroup
			start    = make(chan struct{})
			mu       sync.Mutex
			winners  int
			expired  int
			unexpect []error
		)
		for i := 0; i < attempts; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, err := repo.ConsumeCeremonySession(ctx, id, core.CeremonyRegistration)
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					winners++
				case errors.Is(err, core.ErrCeremonyExpired):
					expired++
				default:
					unexpect = append(unexpect, err)
				}
			}()
		}
		close(start)
		wg.Wait()

		require.Empty(t, unexpect, "rodada %d", round)
		require.Equal(t, 1, winners, "rodada %d: exatamente um finish pode consumir a sessão", round)
		require.Equal(t, attempts-1, expired, "rodada %d", round)
	}
}

func TestAuthRepositoryCeremonySessionRedisDown(t *testing.T) {
	ctx := context.Background()
	down := rdb.NewClient(&rdb.Options{Addr: "localhost:1", DialTimeout: 200 * time.Millisecond, MaxRetries: -1})
	t.Cleanup(func() { _ = down.Close() })
	repo := redis.NewAuthRepository(down)

	t.Run("save devolve erro de infraestrutura", func(t *testing.T) {
		err := repo.SaveCeremonySession(ctx, uuid.New(), core.CeremonyLogin, sessionBytes("abc"), ceremonyTTL)

		require.Error(t, err)
		var domainErr *core.DomainError
		assert.False(t, errors.As(err, &domainErr), "Redis fora do ar é 500, não erro de domínio")
	})

	t.Run("consume devolve erro de infraestrutura, não sessão expirada", func(t *testing.T) {
		got, err := repo.ConsumeCeremonySession(ctx, uuid.New(), core.CeremonyLogin)

		require.Error(t, err)
		assert.Nil(t, got)
		assert.NotErrorIs(t, err, core.ErrCeremonyExpired, "o cliente não deve ser mandado refazer o begin por falha do servidor")
		var domainErr *core.DomainError
		assert.False(t, errors.As(err, &domainErr))
	})
}
