//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/repositories/postgres"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// Os testes de integração sobem um Postgres descartável via testcontainers (requer Docker).
// O container é criado uma vez por pacote no TestMain e a tabela users é truncada antes de cada teste.
//
//	go test -tags integration ./src/repositories/postgres/...

const postgresImage = "postgres:18.3"

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, postgresImage,
		tcpostgres.WithDatabase("userapi_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			fmt.Fprintln(os.Stderr, "encerrando container:", err)
		}
	}()
	if err != nil {
		fmt.Fprintln(os.Stderr, "subindo container:", err)
		return 1
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintln(os.Stderr, "connection string:", err)
		return 1
	}

	if err := migrateUp(dsn); err != nil {
		fmt.Fprintln(os.Stderr, "migrations:", err)
		return 1
	}

	testPool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "conexão:", err)
		return 1
	}
	defer testPool.Close()

	return m.Run()
}

func migrateUp(dsn string) error {
	migrateURL := "pgx5://" + strings.TrimPrefix(strings.TrimPrefix(dsn, "postgresql://"), "postgres://")
	m, err := migrate.New("file://../../migrations", migrateURL)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func setup(t *testing.T) core.UserRepository {
	t.Helper()
	_, err := testPool.Exec(context.Background(), "TRUNCATE users")
	require.NoError(t, err)
	return postgres.NewUserRepository(testPool)
}

func createUser(t *testing.T, repo core.UserRepository, name, email string) *core.User {
	t.Helper()
	user, err := repo.CreateUser(context.Background(), &core.User{
		Name:          name,
		Email:         email,
		Password:      "hash-" + email,
		PepperVersion: 1,
	})
	require.NoError(t, err)
	return user
}

func deleteUser(t *testing.T, repo core.UserRepository, id uuid.UUID) {
	t.Helper()
	require.NoError(t, repo.DeleteUser(context.Background(), id))
}

func names(users []*core.User) []string {
	result := make([]string, 0, len(users))
	for _, u := range users {
		result = append(result, u.Name)
	}
	return result
}

func TestCreateUser(t *testing.T) {
	ctx := context.Background()

	t.Run("cria e retorna os campos gerados pelo banco", func(t *testing.T) {
		repo := setup(t)

		user := createUser(t, repo, "João", "joao@example.com")

		assert.NotEqual(t, uuid.Nil, user.ID)
		assert.Equal(t, "João", user.Name)
		assert.Equal(t, "joao@example.com", user.Email)
		assert.False(t, user.CreatedAt.IsZero())
		assert.False(t, user.UpdatedAt.IsZero())
		assert.Nil(t, user.DeletedAt)
	})

	t.Run("e-mail duplicado retorna ErrEmailAlreadyExists", func(t *testing.T) {
		repo := setup(t)
		createUser(t, repo, "João", "joao@example.com")

		_, err := repo.CreateUser(ctx, &core.User{Name: "Outro", Email: "joao@example.com", Password: "x", PepperVersion: 1})

		assert.ErrorIs(t, err, core.ErrEmailAlreadyExists)
	})

	t.Run("e-mail duplicado com caixa diferente também conflita", func(t *testing.T) {
		repo := setup(t)
		createUser(t, repo, "João", "joao@example.com")

		_, err := repo.CreateUser(ctx, &core.User{Name: "Outro", Email: "JOAO@Example.com", Password: "x", PepperVersion: 1})

		assert.ErrorIs(t, err, core.ErrEmailAlreadyExists)
	})

	t.Run("e-mail de conta deletada não pode ser reutilizado", func(t *testing.T) {
		repo := setup(t)
		user := createUser(t, repo, "João", "joao@example.com")
		deleteUser(t, repo, user.ID)

		_, err := repo.CreateUser(ctx, &core.User{Name: "Novo", Email: "joao@example.com", Password: "x", PepperVersion: 1})

		assert.ErrorIs(t, err, core.ErrEmailAlreadyExists)
	})
}

func TestGetUserByID(t *testing.T) {
	ctx := context.Background()

	t.Run("encontra usuário existente", func(t *testing.T) {
		repo := setup(t)
		created := createUser(t, repo, "João", "joao@example.com")

		got, err := repo.GetUserByID(ctx, created.ID)

		require.NoError(t, err)
		assert.Equal(t, created.ID, got.ID)
		assert.Equal(t, created.Name, got.Name)
		assert.Equal(t, created.Email, got.Email)
		assert.True(t, created.CreatedAt.Equal(got.CreatedAt))
		assert.Empty(t, got.Password, "GetUserByID não deve carregar o hash da senha")
	})

	t.Run("id inexistente retorna ErrUserNotFound", func(t *testing.T) {
		repo := setup(t)

		_, err := repo.GetUserByID(ctx, uuid.New())

		assert.ErrorIs(t, err, core.ErrUserNotFound)
	})

	t.Run("usuário deletado retorna ErrUserNotFound", func(t *testing.T) {
		repo := setup(t)
		created := createUser(t, repo, "João", "joao@example.com")
		deleteUser(t, repo, created.ID)

		_, err := repo.GetUserByID(ctx, created.ID)

		assert.ErrorIs(t, err, core.ErrUserNotFound)
	})
}

func TestGetUserByEmail(t *testing.T) {
	ctx := context.Background()

	t.Run("encontra usuário e carrega credenciais", func(t *testing.T) {
		repo := setup(t)
		created := createUser(t, repo, "João", "joao@example.com")

		got, err := repo.GetUserByEmail(ctx, "joao@example.com")

		require.NoError(t, err)
		assert.Equal(t, created.ID, got.ID)
		assert.Equal(t, "hash-joao@example.com", got.Password)
		assert.Equal(t, 1, got.PepperVersion)
	})

	t.Run("busca ignora maiúsculas e minúsculas", func(t *testing.T) {
		repo := setup(t)
		created := createUser(t, repo, "João", "joao@example.com")

		got, err := repo.GetUserByEmail(ctx, "JOAO@EXAMPLE.COM")

		require.NoError(t, err)
		assert.Equal(t, created.ID, got.ID)
	})

	t.Run("e-mail inexistente retorna ErrUserNotFound", func(t *testing.T) {
		repo := setup(t)

		_, err := repo.GetUserByEmail(ctx, "ninguem@example.com")

		assert.ErrorIs(t, err, core.ErrUserNotFound)
	})

	t.Run("usuário deletado retorna ErrUserNotFound", func(t *testing.T) {
		repo := setup(t)
		created := createUser(t, repo, "João", "joao@example.com")
		deleteUser(t, repo, created.ID)

		_, err := repo.GetUserByEmail(ctx, "joao@example.com")

		assert.ErrorIs(t, err, core.ErrUserNotFound)
	})
}

func TestGetUserByName(t *testing.T) {
	ctx := context.Background()

	seed := func(t *testing.T, repo core.UserRepository) {
		t.Helper()
		createUser(t, repo, "João Silva", "joao@example.com")
		createUser(t, repo, "Maria Joana", "maria@example.com")
		createUser(t, repo, "100% Dev", "dev@example.com")
		createUser(t, repo, "Ana_Maria", "ana1@example.com")
		createUser(t, repo, "AnaXMaria", "ana2@example.com")
		createUser(t, repo, `C:\dir`, "dir@example.com")
	}

	t.Run("busca parcial sem diferenciar maiúsculas", func(t *testing.T) {
		repo := setup(t)
		seed(t, repo)

		got, err := repo.GetUserByName(ctx, "JO")

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"João Silva", "Maria Joana"}, names(got))
	})

	t.Run("porcentagem é tratada como literal", func(t *testing.T) {
		repo := setup(t)
		seed(t, repo)

		got, err := repo.GetUserByName(ctx, "%")

		require.NoError(t, err)
		assert.Equal(t, []string{"100% Dev"}, names(got))
	})

	t.Run("underscore é tratado como literal", func(t *testing.T) {
		repo := setup(t)
		seed(t, repo)

		got, err := repo.GetUserByName(ctx, "a_m")

		require.NoError(t, err)
		assert.Equal(t, []string{"Ana_Maria"}, names(got))
	})

	t.Run("barra invertida é tratada como literal", func(t *testing.T) {
		repo := setup(t)
		seed(t, repo)

		got, err := repo.GetUserByName(ctx, `\`)

		require.NoError(t, err)
		assert.Equal(t, []string{`C:\dir`}, names(got))
	})

	t.Run("ignora usuários deletados", func(t *testing.T) {
		repo := setup(t)
		active := createUser(t, repo, "Pedro Ativo", "pedro1@example.com")
		deleted := createUser(t, repo, "Pedro Deletado", "pedro2@example.com")
		deleteUser(t, repo, deleted.ID)

		got, err := repo.GetUserByName(ctx, "pedro")

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, active.ID, got[0].ID)
	})

	t.Run("sem resultado retorna slice vazio sem erro", func(t *testing.T) {
		repo := setup(t)
		seed(t, repo)

		got, err := repo.GetUserByName(ctx, "inexistente")

		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}

func TestGetAllUsers(t *testing.T) {
	ctx := context.Background()

	t.Run("retorna apenas usuários ativos", func(t *testing.T) {
		repo := setup(t)
		createUser(t, repo, "Ativo 1", "a1@example.com")
		createUser(t, repo, "Ativo 2", "a2@example.com")
		deleted := createUser(t, repo, "Deletado", "d@example.com")
		deleteUser(t, repo, deleted.ID)

		got, err := repo.GetAllUsers(ctx)

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"Ativo 1", "Ativo 2"}, names(got))
	})

	t.Run("tabela vazia retorna slice vazio, não nil", func(t *testing.T) {
		repo := setup(t)

		got, err := repo.GetAllUsers(ctx)

		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}

func TestUpdateUser(t *testing.T) {
	ctx := context.Background()

	t.Run("atualiza nome e mantém e-mail quando vazio", func(t *testing.T) {
		repo := setup(t)
		created := createUser(t, repo, "João", "joao@example.com")

		updated, err := repo.UpdateUser(ctx, &core.User{ID: created.ID, Name: "João Atualizado"})

		require.NoError(t, err)
		assert.Equal(t, created.ID, updated.ID)
		assert.Equal(t, "João Atualizado", updated.Name)
		assert.Equal(t, "joao@example.com", updated.Email)
		assert.True(t, created.CreatedAt.Equal(updated.CreatedAt))
		assert.True(t, updated.UpdatedAt.After(created.UpdatedAt))
	})

	t.Run("atualiza e-mail e mantém nome quando vazio", func(t *testing.T) {
		repo := setup(t)
		created := createUser(t, repo, "João", "joao@example.com")

		updated, err := repo.UpdateUser(ctx, &core.User{ID: created.ID, Email: "novo@example.com"})

		require.NoError(t, err)
		assert.Equal(t, "João", updated.Name)
		assert.Equal(t, "novo@example.com", updated.Email)
	})

	t.Run("alteração é persistida", func(t *testing.T) {
		repo := setup(t)
		created := createUser(t, repo, "João", "joao@example.com")

		_, err := repo.UpdateUser(ctx, &core.User{ID: created.ID, Name: "Persistido"})
		require.NoError(t, err)

		got, err := repo.GetUserByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, "Persistido", got.Name)
	})

	t.Run("e-mail de outro usuário retorna ErrEmailAlreadyExists", func(t *testing.T) {
		repo := setup(t)
		createUser(t, repo, "João", "joao@example.com")
		maria := createUser(t, repo, "Maria", "maria@example.com")

		_, err := repo.UpdateUser(ctx, &core.User{ID: maria.ID, Email: "joao@example.com"})

		assert.ErrorIs(t, err, core.ErrEmailAlreadyExists)
	})

	t.Run("id inexistente retorna ErrUserNotFound", func(t *testing.T) {
		repo := setup(t)

		_, err := repo.UpdateUser(ctx, &core.User{ID: uuid.New(), Name: "Ninguém"})

		assert.ErrorIs(t, err, core.ErrUserNotFound)
	})

	t.Run("usuário deletado retorna ErrUserNotFound", func(t *testing.T) {
		repo := setup(t)
		created := createUser(t, repo, "João", "joao@example.com")
		deleteUser(t, repo, created.ID)

		_, err := repo.UpdateUser(ctx, &core.User{ID: created.ID, Name: "Ressuscitado"})

		assert.ErrorIs(t, err, core.ErrUserNotFound)
	})
}

func TestDeleteUser(t *testing.T) {
	ctx := context.Background()

	t.Run("soft delete preenche DeletedAt sem remover a linha", func(t *testing.T) {
		repo := setup(t)
		created := createUser(t, repo, "João", "joao@example.com")

		err := repo.DeleteUser(ctx, created.ID)
		require.NoError(t, err)

		var deletedAtSet bool
		err = testPool.QueryRow(ctx, "SELECT DeletedAt IS NOT NULL FROM users WHERE Id = $1", created.ID).Scan(&deletedAtSet)
		require.NoError(t, err)
		assert.True(t, deletedAtSet)
	})

	t.Run("id inexistente retorna ErrUserNotFound", func(t *testing.T) {
		repo := setup(t)

		err := repo.DeleteUser(ctx, uuid.New())

		assert.ErrorIs(t, err, core.ErrUserNotFound)
	})

	t.Run("deletar duas vezes retorna ErrUserNotFound na segunda", func(t *testing.T) {
		repo := setup(t)
		created := createUser(t, repo, "João", "joao@example.com")
		deleteUser(t, repo, created.ID)

		err := repo.DeleteUser(ctx, created.ID)

		assert.ErrorIs(t, err, core.ErrUserNotFound)
	})
}
