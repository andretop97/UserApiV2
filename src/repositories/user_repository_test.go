package repositories_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Testes unitários do repositório composto (cache + source) com mocks das duas camadas.
// Chamadas não configuradas com On(...) fazem o mock entrar em pânico, então qualquer
// acesso inesperado ao cache ou ao source falha o teste.

var errBoom = errors.New("boom")

type sourceMock struct{ mock.Mock }

func (m *sourceMock) CreateUser(ctx context.Context, user *core.User) (*core.User, error) {
	args := m.Called(ctx, user)
	return userArg(args, 0), args.Error(1)
}

func (m *sourceMock) GetUserByID(ctx context.Context, id uuid.UUID) (*core.User, error) {
	args := m.Called(ctx, id)
	return userArg(args, 0), args.Error(1)
}

func (m *sourceMock) GetUsersByName(ctx context.Context, name string) ([]*core.User, error) {
	args := m.Called(ctx, name)
	return usersArg(args, 0), args.Error(1)
}

func (m *sourceMock) GetUserByEmail(ctx context.Context, email string) (*core.User, error) {
	args := m.Called(ctx, email)
	return userArg(args, 0), args.Error(1)
}

func (m *sourceMock) GetAllUsers(ctx context.Context) ([]*core.User, error) {
	args := m.Called(ctx)
	return usersArg(args, 0), args.Error(1)
}

func (m *sourceMock) UpdateUser(ctx context.Context, user *core.User) (*core.User, error) {
	args := m.Called(ctx, user)
	return userArg(args, 0), args.Error(1)
}

func (m *sourceMock) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

type cacheMock struct{ mock.Mock }

func (m *cacheMock) GetByID(ctx context.Context, id uuid.UUID) (*core.User, bool, error) {
	args := m.Called(ctx, id)
	return userArg(args, 0), args.Bool(1), args.Error(2)
}

func (m *cacheMock) Set(ctx context.Context, user *core.User) error {
	return m.Called(ctx, user).Error(0)
}

func (m *cacheMock) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func userArg(args mock.Arguments, i int) *core.User {
	u, _ := args.Get(i).(*core.User)
	return u
}

func usersArg(args mock.Arguments, i int) []*core.User {
	u, _ := args.Get(i).([]*core.User)
	return u
}

func setup(t *testing.T) (core.UserRepository, *cacheMock, *sourceMock) {
	t.Helper()
	repo, cache, source, _ := setupWithLogs(t)
	return repo, cache, source
}

// setupWithLogs devolve também o buffer onde o repositório escreve seus logs,
// para os testes que verificam que falhas do cache são registradas.
func setupWithLogs(t *testing.T) (core.UserRepository, *cacheMock, *sourceMock, *bytes.Buffer) {
	t.Helper()
	cache := &cacheMock{}
	source := &sourceMock{}
	t.Cleanup(func() {
		cache.AssertExpectations(t)
		source.AssertExpectations(t)
	})
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, nil))
	return repositories.NewUserRepository(cache, source, logger), cache, source, logs
}

func newUser() *core.User {
	return &core.User{ID: uuid.New(), Name: "João", Email: "joao@example.com"}
}

func TestCreateUser(t *testing.T) {
	ctx := context.Background()

	t.Run("delega ao source sem tocar no cache", func(t *testing.T) {
		repo, _, source := setup(t)
		input := &core.User{Name: "João", Email: "joao@example.com"}
		created := newUser()
		source.On("CreateUser", ctx, input).Return(created, nil)

		got, err := repo.CreateUser(ctx, input)

		require.NoError(t, err)
		assert.Same(t, created, got)
	})

	t.Run("propaga erro do source", func(t *testing.T) {
		repo, _, source := setup(t)
		input := newUser()
		source.On("CreateUser", ctx, input).Return(nil, core.ErrEmailAlreadyExists)

		got, err := repo.CreateUser(ctx, input)

		assert.ErrorIs(t, err, core.ErrEmailAlreadyExists)
		assert.Nil(t, got)
	})
}

func TestGetUserByID(t *testing.T) {
	ctx := context.Background()

	t.Run("cache hit retorna do cache sem consultar o source", func(t *testing.T) {
		repo, cache, _ := setup(t)
		user := newUser()
		cache.On("GetByID", ctx, user.ID).Return(user, true, nil)

		got, err := repo.GetUserByID(ctx, user.ID)

		require.NoError(t, err)
		assert.Same(t, user, got)
	})

	t.Run("cache miss busca no source e grava no cache", func(t *testing.T) {
		repo, cache, source := setup(t)
		user := newUser()
		cache.On("GetByID", ctx, user.ID).Return(nil, false, nil)
		source.On("GetUserByID", ctx, user.ID).Return(user, nil)
		cache.On("Set", ctx, user).Return(nil)

		got, err := repo.GetUserByID(ctx, user.ID)

		require.NoError(t, err)
		assert.Same(t, user, got)
	})

	t.Run("usuário inexistente no source não é gravado no cache", func(t *testing.T) {
		repo, cache, source := setup(t)
		id := uuid.New()
		cache.On("GetByID", ctx, id).Return(nil, false, nil)
		source.On("GetUserByID", ctx, id).Return(nil, core.ErrUserNotFound)

		got, err := repo.GetUserByID(ctx, id)

		assert.ErrorIs(t, err, core.ErrUserNotFound)
		assert.Nil(t, got)
		cache.AssertNotCalled(t, "Set", mock.Anything, mock.Anything)
	})

	t.Run("erro do source é propagado", func(t *testing.T) {
		repo, cache, source := setup(t)
		id := uuid.New()
		cache.On("GetByID", ctx, id).Return(nil, false, nil)
		source.On("GetUserByID", ctx, id).Return(nil, errBoom)

		got, err := repo.GetUserByID(ctx, id)

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, got)
	})

	t.Run("erro ao ler do cache cai para o source e é logado", func(t *testing.T) {
		repo, cache, source, logs := setupWithLogs(t)
		user := newUser()
		cache.On("GetByID", ctx, user.ID).Return(nil, false, errBoom)
		source.On("GetUserByID", ctx, user.ID).Return(user, nil)
		cache.On("Set", ctx, user).Return(nil)

		got, err := repo.GetUserByID(ctx, user.ID)

		require.NoError(t, err)
		assert.Same(t, user, got)
		assert.Contains(t, logs.String(), "error getting user from cache")
		assert.Contains(t, logs.String(), user.ID.String())
	})

	t.Run("erro ao ler do cache não esconde erro do source", func(t *testing.T) {
		repo, cache, source := setup(t)
		id := uuid.New()
		cache.On("GetByID", ctx, id).Return(nil, false, errBoom)
		source.On("GetUserByID", ctx, id).Return(nil, core.ErrUserNotFound)

		got, err := repo.GetUserByID(ctx, id)

		assert.ErrorIs(t, err, core.ErrUserNotFound)
		assert.Nil(t, got)
	})

	t.Run("erro ao gravar no cache é ignorado e logado", func(t *testing.T) {
		repo, cache, source, logs := setupWithLogs(t)
		user := newUser()
		cache.On("GetByID", ctx, user.ID).Return(nil, false, nil)
		source.On("GetUserByID", ctx, user.ID).Return(user, nil)
		cache.On("Set", ctx, user).Return(errBoom)

		got, err := repo.GetUserByID(ctx, user.ID)

		require.NoError(t, err)
		assert.Same(t, user, got)
		assert.Contains(t, logs.String(), "error setting user to cache")
	})
}

func TestGetUsersByName(t *testing.T) {
	ctx := context.Background()

	t.Run("delega ao source sem tocar no cache", func(t *testing.T) {
		repo, _, source := setup(t)
		users := []*core.User{newUser(), newUser()}
		source.On("GetUsersByName", ctx, "jo").Return(users, nil)

		got, err := repo.GetUsersByName(ctx, "jo")

		require.NoError(t, err)
		assert.Equal(t, users, got)
	})

	t.Run("resultado vazio é repassado sem erro", func(t *testing.T) {
		repo, _, source := setup(t)
		source.On("GetUsersByName", ctx, "x").Return([]*core.User{}, nil)

		got, err := repo.GetUsersByName(ctx, "x")

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("propaga erro do source", func(t *testing.T) {
		repo, _, source := setup(t)
		source.On("GetUsersByName", ctx, "jo").Return(nil, errBoom)

		got, err := repo.GetUsersByName(ctx, "jo")

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, got)
	})
}

func TestGetUserByEmail(t *testing.T) {
	ctx := context.Background()

	t.Run("delega ao source sem tocar no cache", func(t *testing.T) {
		repo, _, source := setup(t)
		user := newUser()
		source.On("GetUserByEmail", ctx, user.Email).Return(user, nil)

		got, err := repo.GetUserByEmail(ctx, user.Email)

		require.NoError(t, err)
		assert.Same(t, user, got)
	})

	t.Run("source retornando nil sem erro vira ErrUserNotFound", func(t *testing.T) {
		repo, _, source := setup(t)
		source.On("GetUserByEmail", ctx, "x@example.com").Return(nil, nil)

		got, err := repo.GetUserByEmail(ctx, "x@example.com")

		assert.ErrorIs(t, err, core.ErrUserNotFound)
		assert.Nil(t, got)
	})

	t.Run("propaga erro do source", func(t *testing.T) {
		repo, _, source := setup(t)
		source.On("GetUserByEmail", ctx, "x@example.com").Return(nil, core.ErrUserNotFound)

		got, err := repo.GetUserByEmail(ctx, "x@example.com")

		assert.ErrorIs(t, err, core.ErrUserNotFound)
		assert.Nil(t, got)
	})
}

func TestGetAllUsers(t *testing.T) {
	ctx := context.Background()

	t.Run("delega ao source sem tocar no cache", func(t *testing.T) {
		repo, _, source := setup(t)
		users := []*core.User{newUser(), newUser()}
		source.On("GetAllUsers", ctx).Return(users, nil)

		got, err := repo.GetAllUsers(ctx)

		require.NoError(t, err)
		assert.Equal(t, users, got)
	})

	t.Run("propaga erro do source", func(t *testing.T) {
		repo, _, source := setup(t)
		source.On("GetAllUsers", ctx).Return(nil, errBoom)

		got, err := repo.GetAllUsers(ctx)

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, got)
	})
}

func TestUpdateUser(t *testing.T) {
	ctx := context.Background()

	t.Run("atualiza no source e invalida o cache", func(t *testing.T) {
		repo, cache, source := setup(t)
		input := &core.User{ID: uuid.New(), Name: "Novo"}
		updated := &core.User{ID: input.ID, Name: "Novo", Email: "joao@example.com"}
		source.On("UpdateUser", ctx, input).Return(updated, nil)
		cache.On("Delete", ctx, input.ID).Return(nil)

		got, err := repo.UpdateUser(ctx, input)

		require.NoError(t, err)
		assert.Same(t, updated, got)
	})

	t.Run("erro do source não invalida o cache", func(t *testing.T) {
		repo, cache, source := setup(t)
		input := newUser()
		source.On("UpdateUser", ctx, input).Return(nil, core.ErrUserNotFound)

		got, err := repo.UpdateUser(ctx, input)

		assert.ErrorIs(t, err, core.ErrUserNotFound)
		assert.Nil(t, got)
		cache.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
	})

	t.Run("erro ao invalidar o cache é ignorado e logado", func(t *testing.T) {
		repo, cache, source, logs := setupWithLogs(t)
		input := newUser()
		source.On("UpdateUser", ctx, input).Return(input, nil)
		cache.On("Delete", ctx, input.ID).Return(errBoom)

		got, err := repo.UpdateUser(ctx, input)

		require.NoError(t, err)
		assert.Same(t, input, got)
		assert.Contains(t, logs.String(), "cache invalidation failed")
		assert.Contains(t, logs.String(), input.ID.String())
	})
}

func TestDeleteUser(t *testing.T) {
	ctx := context.Background()

	t.Run("deleta no source e invalida o cache", func(t *testing.T) {
		repo, cache, source := setup(t)
		id := uuid.New()
		source.On("DeleteUser", ctx, id).Return(nil)
		cache.On("Delete", ctx, id).Return(nil)

		err := repo.DeleteUser(ctx, id)

		require.NoError(t, err)
	})

	t.Run("erro do source não invalida o cache", func(t *testing.T) {
		repo, cache, source := setup(t)
		id := uuid.New()
		source.On("DeleteUser", ctx, id).Return(core.ErrUserNotFound)

		err := repo.DeleteUser(ctx, id)

		assert.ErrorIs(t, err, core.ErrUserNotFound)
		cache.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
	})

	t.Run("erro ao invalidar o cache é ignorado e logado", func(t *testing.T) {
		repo, cache, source, logs := setupWithLogs(t)
		id := uuid.New()
		source.On("DeleteUser", ctx, id).Return(nil)
		cache.On("Delete", ctx, id).Return(errBoom)

		err := repo.DeleteUser(ctx, id)

		require.NoError(t, err)
		assert.Contains(t, logs.String(), "cache invalidation failed")
		assert.Contains(t, logs.String(), id.String())
	})
}
