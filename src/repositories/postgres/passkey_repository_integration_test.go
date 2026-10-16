//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/repositories/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Usa o container e o testPool criados no TestMain de user_repository_integration_test.go.

func setupPasskeys(t *testing.T) (core.PasskeyRepository, core.UserRepository) {
	t.Helper()
	users := setup(t) // TRUNCATE users CASCADE também limpa passkeys
	return postgres.NewPasskeyRepository(testPool), users
}

func newPasskey(userID uuid.UUID, id ...byte) *core.Passkey {
	if len(id) == 0 {
		id = []byte(uuid.NewString())
	}
	return &core.Passkey{
		ID:     id,
		UserID: userID,
		Data:   []byte(`{"publicKey":"chave","signCount":0}`),
	}
}

func TestPasskeyCreateAndList(t *testing.T) {
	ctx := context.Background()

	t.Run("cria e lista com os campos gerados pelo banco", func(t *testing.T) {
		passkeys, users := setupPasskeys(t)
		user := createUser(t, users, "João", "joao@example.com")
		p := newPasskey(user.ID)

		require.NoError(t, passkeys.Create(ctx, p))

		got, err := passkeys.ListByUser(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, p.ID, got[0].ID)
		assert.Equal(t, user.ID, got[0].UserID)
		assert.WithinDuration(t, time.Now(), got[0].CreatedAt, time.Minute)
		assert.Nil(t, got[0].LastUsedAt, "passkey recém-criada nunca foi usada")
	})

	t.Run("usuário sem passkey devolve lista vazia sem erro", func(t *testing.T) {
		passkeys, users := setupPasskeys(t)
		user := createUser(t, users, "João", "joao@example.com")

		got, err := passkeys.ListByUser(ctx, user.ID)

		require.NoError(t, err, "não ter passkey é um estado normal (primeiro acesso)")
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("lista só as passkeys do próprio usuário", func(t *testing.T) {
		passkeys, users := setupPasskeys(t)
		joao := createUser(t, users, "João", "joao@example.com")
		maria := createUser(t, users, "Maria", "maria@example.com")
		require.NoError(t, passkeys.Create(ctx, newPasskey(joao.ID, 1)))
		require.NoError(t, passkeys.Create(ctx, newPasskey(maria.ID, 2)))
		require.NoError(t, passkeys.Create(ctx, newPasskey(maria.ID, 3)))

		got, err := passkeys.ListByUser(ctx, maria.ID)

		require.NoError(t, err)
		require.Len(t, got, 2)
		for _, p := range got {
			assert.Equal(t, maria.ID, p.UserID)
		}
	})

	t.Run("ordena pela data de criação", func(t *testing.T) {
		passkeys, users := setupPasskeys(t)
		user := createUser(t, users, "João", "joao@example.com")
		for _, id := range []byte{3, 1, 2} {
			require.NoError(t, passkeys.Create(ctx, newPasskey(user.ID, id)))
		}

		got, err := passkeys.ListByUser(ctx, user.ID)

		require.NoError(t, err)
		require.Len(t, got, 3)
		assert.Equal(t, []byte{3}, got[0].ID)
		assert.Equal(t, []byte{1}, got[1].ID)
		assert.Equal(t, []byte{2}, got[2].ID)
	})

	t.Run("ID binário com 0x00 e bytes fora do UTF-8 é preservado", func(t *testing.T) {
		passkeys, users := setupPasskeys(t)
		user := createUser(t, users, "João", "joao@example.com")
		id := []byte{0x00, 0xff, 0xfe, 0x80, 0x00, 0x01}

		require.NoError(t, passkeys.Create(ctx, newPasskey(user.ID, id...)))

		got, err := passkeys.ListByUser(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, id, got[0].ID)
	})

	t.Run("Data volta com o mesmo conteúdo JSON", func(t *testing.T) {
		passkeys, users := setupPasskeys(t)
		user := createUser(t, users, "João", "joao@example.com")
		p := newPasskey(user.ID)
		p.Data = []byte(`{"publicKey":"chave","signCount":7,"flags":{"backupEligible":true}}`)

		require.NoError(t, passkeys.Create(ctx, p))

		got, err := passkeys.ListByUser(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, got, 1)
		// JSONB normaliza espaços e ordem das chaves: o conteúdo é igual, os bytes podem não ser.
		assert.JSONEq(t, string(p.Data), string(got[0].Data))
	})

	t.Run("ID duplicado retorna ErrPasskeyAlreadyRegistered", func(t *testing.T) {
		passkeys, users := setupPasskeys(t)
		user := createUser(t, users, "João", "joao@example.com")
		require.NoError(t, passkeys.Create(ctx, newPasskey(user.ID, 1)))

		err := passkeys.Create(ctx, newPasskey(user.ID, 1))

		assert.ErrorIs(t, err, core.ErrPasskeyAlreadyRegistered)
	})

	t.Run("ID já usado por outro usuário também conflita", func(t *testing.T) {
		passkeys, users := setupPasskeys(t)
		joao := createUser(t, users, "João", "joao@example.com")
		maria := createUser(t, users, "Maria", "maria@example.com")
		require.NoError(t, passkeys.Create(ctx, newPasskey(joao.ID, 1)))

		err := passkeys.Create(ctx, newPasskey(maria.ID, 1))

		assert.ErrorIs(t, err, core.ErrPasskeyAlreadyRegistered)
	})

	t.Run("usuário inexistente retorna ErrUserNotFound", func(t *testing.T) {
		passkeys, _ := setupPasskeys(t)

		err := passkeys.Create(ctx, newPasskey(uuid.New()))

		assert.ErrorIs(t, err, core.ErrUserNotFound)
	})

	t.Run("Data que não é JSON é erro de infraestrutura, não de domínio", func(t *testing.T) {
		passkeys, users := setupPasskeys(t)
		user := createUser(t, users, "João", "joao@example.com")
		p := newPasskey(user.ID)
		p.Data = []byte("não é json")

		err := passkeys.Create(ctx, p)

		require.Error(t, err)
		assert.NotErrorIs(t, err, core.ErrPasskeyAlreadyRegistered)
		assert.NotErrorIs(t, err, core.ErrUserNotFound)
	})
}

func TestPasskeyUpdateAfterLogin(t *testing.T) {
	ctx := context.Background()

	t.Run("grava os novos dados e preenche LastUsedAt", func(t *testing.T) {
		passkeys, users := setupPasskeys(t)
		user := createUser(t, users, "João", "joao@example.com")
		p := newPasskey(user.ID, 1)
		require.NoError(t, passkeys.Create(ctx, p))
		newData := []byte(`{"publicKey":"chave","signCount":1}`)

		require.NoError(t, passkeys.UpdateAfterLogin(ctx, p.ID, newData))

		got, err := passkeys.ListByUser(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.JSONEq(t, string(newData), string(got[0].Data), "o sign count novo precisa ser persistido")
		require.NotNil(t, got[0].LastUsedAt, "LastUsedAt deve ser preenchido no login")
		assert.WithinDuration(t, time.Now(), *got[0].LastUsedAt, time.Minute)
	})

	t.Run("não altera as outras passkeys", func(t *testing.T) {
		passkeys, users := setupPasskeys(t)
		user := createUser(t, users, "João", "joao@example.com")
		usada := newPasskey(user.ID, 1)
		outra := newPasskey(user.ID, 2)
		require.NoError(t, passkeys.Create(ctx, usada))
		require.NoError(t, passkeys.Create(ctx, outra))

		require.NoError(t, passkeys.UpdateAfterLogin(ctx, usada.ID, []byte(`{"signCount":1}`)))

		got, err := passkeys.ListByUser(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, outra.ID, got[1].ID)
		assert.JSONEq(t, string(outra.Data), string(got[1].Data))
		assert.Nil(t, got[1].LastUsedAt)
	})

	t.Run("ID inexistente retorna ErrPasskeyNotFound", func(t *testing.T) {
		passkeys, _ := setupPasskeys(t)

		err := passkeys.UpdateAfterLogin(ctx, []byte{9, 9, 9}, []byte(`{}`))

		assert.ErrorIs(t, err, core.ErrPasskeyNotFound)
	})
}

func TestPasskeyUserDeletion(t *testing.T) {
	ctx := context.Background()

	t.Run("delete físico do usuário apaga as passkeys (CASCADE)", func(t *testing.T) {
		passkeys, users := setupPasskeys(t)
		user := createUser(t, users, "João", "joao@example.com")
		require.NoError(t, passkeys.Create(ctx, newPasskey(user.ID, 1)))

		_, err := testPool.Exec(ctx, "DELETE FROM users WHERE Id = $1", user.ID)
		require.NoError(t, err)

		var count int
		require.NoError(t, testPool.QueryRow(ctx, "SELECT COUNT(*) FROM passkeys").Scan(&count))
		assert.Zero(t, count)
	})
}

func TestPasskeyInfrastructureErrors(t *testing.T) {
	passkeys, users := setupPasskeys(t)
	user := createUser(t, users, "João", "joao@example.com")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	assertInfraError := func(t *testing.T, err error) {
		t.Helper()
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.NotErrorIs(t, err, core.ErrPasskeyAlreadyRegistered)
		assert.NotErrorIs(t, err, core.ErrPasskeyNotFound)
		assert.NotErrorIs(t, err, core.ErrUserNotFound)
	}

	t.Run("Create", func(t *testing.T) {
		assertInfraError(t, passkeys.Create(ctx, newPasskey(user.ID)))
	})

	t.Run("ListByUser", func(t *testing.T) {
		got, err := passkeys.ListByUser(ctx, user.ID)
		assert.Nil(t, got)
		assertInfraError(t, err)
	})

	t.Run("UpdateAfterLogin", func(t *testing.T) {
		assertInfraError(t, passkeys.UpdateAfterLogin(ctx, []byte{1}, []byte(`{}`)))
	})
}
