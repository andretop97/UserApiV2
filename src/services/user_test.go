package services_test

import (
	"context"
	"testing"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/services"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupUserService(t *testing.T) (core.UserService, *userRepositoryMock, *passwordEncryptionMock) {
	t.Helper()
	repo := &userRepositoryMock{}
	password := &passwordEncryptionMock{}
	t.Cleanup(func() {
		repo.AssertExpectations(t)
		password.AssertExpectations(t)
	})
	return services.NewUserService(repo, password), repo, password
}

func TestUserServiceCreateUser(t *testing.T) {
	ctx := context.Background()

	t.Run("normaliza, gera o hash e grava sem a senha em texto puro", func(t *testing.T) {
		svc, repo, password := setupUserService(t)
		input := &core.User{Name: "  João  ", Email: "  Joao@Example.COM ", Password: "senha"}
		created := &core.User{ID: uuid.New(), Name: "João", Email: "joao@example.com"}
		repo.On("GetUserByEmail", ctx, "joao@example.com").Return(nil, core.ErrUserNotFound)
		password.On("Hash", "senha").Return("$argon2id$hash", 3, nil)
		repo.On("CreateUser", ctx, mock.MatchedBy(func(u *core.User) bool {
			return u.Name == "João" &&
				u.Email == "joao@example.com" &&
				u.Password == "$argon2id$hash" &&
				u.PepperVersion == 3
		})).Return(created, nil)

		got, err := svc.CreateUser(ctx, input)

		require.NoError(t, err)
		assert.Same(t, created, got)
	})

	t.Run("nome só com espaços vira ErrBadRequest sem consultar nem gravar", func(t *testing.T) {
		svc, _, _ := setupUserService(t)

		got, err := svc.CreateUser(ctx, &core.User{Name: "   ", Email: "joao@example.com", Password: "senha"})

		assert.ErrorIs(t, err, core.ErrBadRequest)
		assert.Nil(t, got)
	})

	t.Run("e-mail já cadastrado vira ErrEmailAlreadyExists sem gerar hash", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		repo.On("GetUserByEmail", ctx, "joao@example.com").Return(&core.User{ID: uuid.New()}, nil)

		got, err := svc.CreateUser(ctx, &core.User{Name: "João", Email: "joao@example.com", Password: "senha"})

		assert.ErrorIs(t, err, core.ErrEmailAlreadyExists)
		assert.Nil(t, got)
	})

	t.Run("e-mail já cadastrado com outra caixa também é detectado", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		repo.On("GetUserByEmail", ctx, "joao@example.com").Return(&core.User{ID: uuid.New()}, nil)

		_, err := svc.CreateUser(ctx, &core.User{Name: "João", Email: "JOAO@example.com", Password: "senha"})

		assert.ErrorIs(t, err, core.ErrEmailAlreadyExists)
	})

	t.Run("falha ao consultar o e-mail é propagada", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		repo.On("GetUserByEmail", ctx, "joao@example.com").Return(nil, errBoom)

		got, err := svc.CreateUser(ctx, &core.User{Name: "João", Email: "joao@example.com", Password: "senha"})

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, got)
	})

	t.Run("falha no hash é propagada e nada é gravado", func(t *testing.T) {
		svc, repo, password := setupUserService(t)
		repo.On("GetUserByEmail", ctx, "joao@example.com").Return(nil, core.ErrUserNotFound)
		password.On("Hash", "senha").Return("", 0, errBoom)

		got, err := svc.CreateUser(ctx, &core.User{Name: "João", Email: "joao@example.com", Password: "senha"})

		assert.ErrorIs(t, err, errBoom)
		assert.Nil(t, got)
	})

	t.Run("falha ao gravar é propagada", func(t *testing.T) {
		svc, repo, password := setupUserService(t)
		repo.On("GetUserByEmail", ctx, "joao@example.com").Return(nil, core.ErrUserNotFound)
		password.On("Hash", "senha").Return("$argon2id$hash", 1, nil)
		repo.On("CreateUser", ctx, mock.Anything).Return(nil, core.ErrEmailAlreadyExists)

		got, err := svc.CreateUser(ctx, &core.User{Name: "João", Email: "joao@example.com", Password: "senha"})

		assert.ErrorIs(t, err, core.ErrEmailAlreadyExists)
		assert.Nil(t, got)
	})
}

func TestUserServiceGetUserByID(t *testing.T) {
	ctx := context.Background()

	t.Run("devolve o usuário do repositório", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		user := &core.User{ID: uuid.New()}
		repo.On("GetUserByID", ctx, user.ID).Return(user, nil)

		got, err := svc.GetUserByID(ctx, user.ID)

		require.NoError(t, err)
		assert.Same(t, user, got)
	})

	t.Run("propaga ErrUserNotFound", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		id := uuid.New()
		repo.On("GetUserByID", ctx, id).Return(nil, core.ErrUserNotFound)

		got, err := svc.GetUserByID(ctx, id)

		assert.ErrorIs(t, err, core.ErrUserNotFound)
		assert.Nil(t, got)
	})
}

func TestUserServiceGetUsersByName(t *testing.T) {
	ctx := context.Background()

	t.Run("devolve a lista do repositório", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		users := []*core.User{{ID: uuid.New()}, {ID: uuid.New()}}
		repo.On("GetUsersByName", ctx, "João").Return(users, nil)

		got, err := svc.GetUsersByName(ctx, "João")

		require.NoError(t, err)
		assert.Equal(t, users, got)
	})

	t.Run("remove espaços das pontas antes de buscar", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		repo.On("GetUsersByName", ctx, "João").Return([]*core.User{}, nil)

		_, err := svc.GetUsersByName(ctx, "  João  ")

		require.NoError(t, err)
	})

	t.Run("busca só com espaços vira ErrBadRequest sem consultar o repositório", func(t *testing.T) {
		svc, _, _ := setupUserService(t)

		got, err := svc.GetUsersByName(ctx, "   ")

		assert.ErrorIs(t, err, core.ErrBadRequest)
		assert.Nil(t, got)
	})

	t.Run("propaga erro", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		repo.On("GetUsersByName", ctx, "João").Return(nil, errBoom)

		_, err := svc.GetUsersByName(ctx, "João")

		assert.ErrorIs(t, err, errBoom)
	})
}

func TestUserServiceGetUserByEmail(t *testing.T) {
	ctx := context.Background()

	t.Run("devolve o usuário do repositório", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		user := &core.User{ID: uuid.New(), Email: "joao@example.com"}
		repo.On("GetUserByEmail", ctx, "joao@example.com").Return(user, nil)

		got, err := svc.GetUserByEmail(ctx, "joao@example.com")

		require.NoError(t, err)
		assert.Same(t, user, got)
	})

	t.Run("normaliza o e-mail como no cadastro e no login", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		user := &core.User{ID: uuid.New(), Email: "joao@example.com"}
		repo.On("GetUserByEmail", ctx, "joao@example.com").Return(user, nil).Maybe()
		repo.On("GetUserByEmail", ctx, mock.Anything).Return(nil, core.ErrUserNotFound).Maybe()

		got, err := svc.GetUserByEmail(ctx, "  Joao@Example.COM ")

		require.NoError(t, err)
		assert.Same(t, user, got)
	})

	t.Run("repositório devolvendo nil sem erro vira ErrUserNotFound", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		repo.On("GetUserByEmail", ctx, "x@example.com").Return(nil, nil)

		got, err := svc.GetUserByEmail(ctx, "x@example.com")

		assert.ErrorIs(t, err, core.ErrUserNotFound)
		assert.Nil(t, got)
	})

	t.Run("propaga erro", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		repo.On("GetUserByEmail", ctx, "x@example.com").Return(nil, errBoom)

		_, err := svc.GetUserByEmail(ctx, "x@example.com")

		assert.ErrorIs(t, err, errBoom)
	})
}

func TestUserServiceGetAllUsers(t *testing.T) {
	ctx := context.Background()

	t.Run("devolve a lista do repositório", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		users := []*core.User{{ID: uuid.New()}}
		repo.On("GetAllUsers", ctx).Return(users, nil)

		got, err := svc.GetAllUsers(ctx)

		require.NoError(t, err)
		assert.Equal(t, users, got)
	})

	t.Run("propaga erro", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		repo.On("GetAllUsers", ctx).Return(nil, errBoom)

		_, err := svc.GetAllUsers(ctx)

		assert.ErrorIs(t, err, errBoom)
	})
}

func TestUserServiceUpdateUser(t *testing.T) {
	ctx := context.Background()

	t.Run("atualiza no repositório e devolve o usuário atualizado", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		id := uuid.New()
		updated := &core.User{ID: id, Name: "Novo Nome", Email: "joao@example.com"}
		repo.On("UpdateUser", ctx, id, mock.MatchedBy(func(u *core.UserUpdate) bool {
			return u.Name != nil && *u.Name == "Novo Nome" && u.Email == nil
		})).Return(updated, nil)

		got, err := svc.UpdateUser(ctx, id, &core.UserUpdate{Name: ptr("Novo Nome")})

		require.NoError(t, err)
		require.NotNil(t, got, "devolver nil, nil faz o controller entrar em pânico em FromUser")
		assert.Same(t, updated, got)
	})

	t.Run("normaliza nome e e-mail como no cadastro", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		id := uuid.New()
		var received *core.UserUpdate
		repo.On("UpdateUser", ctx, id, mock.Anything).
			Run(func(args mock.Arguments) { received = args.Get(2).(*core.UserUpdate) }).
			Return(&core.User{ID: id}, nil)

		_, err := svc.UpdateUser(ctx, id, &core.UserUpdate{
			Name:  ptr("  João  "),
			Email: ptr("  Joao@Example.COM "),
		})

		require.NoError(t, err)
		require.NotNil(t, received)
		assert.Equal(t, "João", *received.Name)
		assert.Equal(t, "joao@example.com", *received.Email)
	})

	t.Run("nome só com espaços vira ErrBadRequest sem chamar o repositório", func(t *testing.T) {
		svc, _, _ := setupUserService(t)

		got, err := svc.UpdateUser(ctx, uuid.New(), &core.UserUpdate{Name: ptr("   ")})

		assert.ErrorIs(t, err, core.ErrBadRequest)
		assert.Nil(t, got)
	})

	t.Run("usuário inexistente vira ErrUserNotFound", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		id := uuid.New()
		repo.On("UpdateUser", ctx, id, mock.Anything).Return(nil, core.ErrUserNotFound)

		got, err := svc.UpdateUser(ctx, id, &core.UserUpdate{Name: ptr("Novo Nome")})

		assert.ErrorIs(t, err, core.ErrUserNotFound)
		assert.Nil(t, got)
	})

	t.Run("e-mail já usado por outro usuário vira ErrEmailAlreadyExists", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		id := uuid.New()
		repo.On("UpdateUser", ctx, id, mock.Anything).Return(nil, core.ErrEmailAlreadyExists)

		_, err := svc.UpdateUser(ctx, id, &core.UserUpdate{Email: ptr("joao@example.com")})

		assert.ErrorIs(t, err, core.ErrEmailAlreadyExists)
	})
}

func TestUserServiceDeleteUser(t *testing.T) {
	ctx := context.Background()

	t.Run("apaga no repositório", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		id := uuid.New()
		repo.On("DeleteUser", ctx, id).Return(nil)

		err := svc.DeleteUser(ctx, id)

		require.NoError(t, err)
	})

	t.Run("usuário inexistente vira ErrUserNotFound", func(t *testing.T) {
		svc, repo, _ := setupUserService(t)
		id := uuid.New()
		repo.On("DeleteUser", ctx, id).Return(core.ErrUserNotFound)

		err := svc.DeleteUser(ctx, id)

		assert.ErrorIs(t, err, core.ErrUserNotFound)
	})
}
