package controllers_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/andretop97/UserApiV2/src/controllers"
	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/routes"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupUserRouter(t *testing.T) (*gin.Engine, *userServiceMock) {
	t.Helper()
	svc := &userServiceMock{}
	t.Cleanup(func() { svc.AssertExpectations(t) })
	router := gin.New()
	router.Use(gin.Recovery())
	routes.Routes(router, &routes.Controllers{
		User: controllers.NewUserController(svc),
		Auth: controllers.NewAuthController(&authServiceMock{}),
	})
	return router, svc
}

func sampleUser() *core.User {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return &core.User{
		ID:            uuid.New(),
		Name:          "João",
		Email:         "joao@example.com",
		Password:      "$argon2id$hash-secreto",
		PepperVersion: 1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func TestUserControllerCreateUser(t *testing.T) {
	t.Run("201 sem expor hash nem pepper", func(t *testing.T) {
		router, svc := setupUserRouter(t)
		created := sampleUser()
		svc.On("CreateUser", mock.Anything, mock.MatchedBy(func(u *core.User) bool {
			return u.Name == "João" && u.Email == "joao@example.com" && u.Password == "senha-forte"
		})).Return(created, nil)

		rec := do(t, router, http.MethodPost, "/v1/user/", `{"name":"João","email":"joao@example.com","password":"senha-forte"}`)

		assert.Equal(t, http.StatusCreated, rec.Code)
		body := decodeJSON[map[string]any](t, rec)
		assert.Equal(t, created.ID.String(), body["id"])
		assert.Equal(t, "joao@example.com", body["email"])
		assert.NotContains(t, rec.Body.String(), "argon2id")
		assert.NotContains(t, body, "password")
		assert.NotContains(t, body, "pepperVersion")
	})

	t.Run("400 com campos faltando", func(t *testing.T) {
		router, _ := setupUserRouter(t)

		rec := do(t, router, http.MethodPost, "/v1/user/", `{"email":"joao@example.com"}`)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, "BAD_REQUEST", decodeJSON[errorBody](t, rec).Error.Code)
	})

	t.Run("400 com nome acima de 100 caracteres, mesmo limite do update", func(t *testing.T) {
		router, _ := setupUserRouter(t)
		body := `{"name":"` + strings.Repeat("a", 101) + `","email":"joao@example.com","password":"senha-forte"}`

		rec := do(t, router, http.MethodPost, "/v1/user/", body)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	passwords := map[string]string{
		"senha com menos de 8 caracteres":  "curta",
		"senha com mais de 128 caracteres": strings.Repeat("a", 129),
	}
	for name, password := range passwords {
		t.Run("400 sem chamar o service: "+name, func(t *testing.T) {
			router, _ := setupUserRouter(t)
			body := `{"name":"João","email":"joao@example.com","password":"` + password + `"}`

			rec := do(t, router, http.MethodPost, "/v1/user/", body)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}

	t.Run("409 quando o e-mail já existe", func(t *testing.T) {
		router, svc := setupUserRouter(t)
		svc.On("CreateUser", mock.Anything, mock.Anything).Return(nil, core.ErrEmailAlreadyExists)

		rec := do(t, router, http.MethodPost, "/v1/user/", `{"name":"João","email":"joao@example.com","password":"senha-forte"}`)

		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "EMAIL_ALREADY_EXISTS", decodeJSON[errorBody](t, rec).Error.Code)
	})
}

func TestUserControllerGetUserByID(t *testing.T) {
	t.Run("200 com o usuário", func(t *testing.T) {
		router, svc := setupUserRouter(t)
		user := sampleUser()
		svc.On("GetUserByID", mock.Anything, user.ID).Return(user, nil)

		rec := do(t, router, http.MethodGet, "/v1/user/"+user.ID.String(), "")

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, user.ID.String(), decodeJSON[map[string]any](t, rec)["id"])
	})

	t.Run("400 com id inválido sem chamar o service", func(t *testing.T) {
		router, _ := setupUserRouter(t)

		rec := do(t, router, http.MethodGet, "/v1/user/nao-e-uuid", "")

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("404 quando não existe", func(t *testing.T) {
		router, svc := setupUserRouter(t)
		id := uuid.New()
		svc.On("GetUserByID", mock.Anything, id).Return(nil, core.ErrUserNotFound)

		rec := do(t, router, http.MethodGet, "/v1/user/"+id.String(), "")

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, "USER_NOT_FOUND", decodeJSON[errorBody](t, rec).Error.Code)
	})
}

func TestUserControllerLists(t *testing.T) {
	t.Run("GetAllUsers sem usuários devolve [] e não null", func(t *testing.T) {
		router, svc := setupUserRouter(t)
		svc.On("GetAllUsers", mock.Anything).Return([]*core.User{}, nil)

		rec := do(t, router, http.MethodGet, "/v1/user/", "")

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `[]`, rec.Body.String())
	})

	t.Run("GetUsersByName sem resultados devolve [] e não null", func(t *testing.T) {
		router, svc := setupUserRouter(t)
		svc.On("GetUsersByName", mock.Anything, "ninguem").Return([]*core.User{}, nil)

		rec := do(t, router, http.MethodGet, "/v1/user/name/ninguem", "")

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `[]`, rec.Body.String())
	})

	t.Run("GetAllUsers devolve todos sem expor hash", func(t *testing.T) {
		router, svc := setupUserRouter(t)
		svc.On("GetAllUsers", mock.Anything).Return([]*core.User{sampleUser(), sampleUser()}, nil)

		rec := do(t, router, http.MethodGet, "/v1/user/", "")

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Len(t, decodeJSON[[]map[string]any](t, rec), 2)
		assert.NotContains(t, rec.Body.String(), "argon2id")
	})

	t.Run("GetUserByEmail devolve o usuário", func(t *testing.T) {
		router, svc := setupUserRouter(t)
		user := sampleUser()
		svc.On("GetUserByEmail", mock.Anything, user.Email).Return(user, nil)

		rec := do(t, router, http.MethodGet, "/v1/user/email/"+user.Email, "")

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, user.Email, decodeJSON[map[string]any](t, rec)["email"])
	})
}

func TestUserControllerUpdateUser(t *testing.T) {
	t.Run("passa o id da URL e os campos para o service", func(t *testing.T) {
		router, svc := setupUserRouter(t)
		user := sampleUser()
		var received *core.UserUpdate
		svc.On("UpdateUser", mock.Anything, user.ID, mock.Anything).
			Run(func(args mock.Arguments) { received = args.Get(2).(*core.UserUpdate) }).
			Return(user, nil)

		rec := do(t, router, http.MethodPut, "/v1/user/"+user.ID.String(), `{"name":"Novo Nome"}`)

		assert.Equal(t, http.StatusOK, rec.Code)
		require.NotNil(t, received)
		require.NotNil(t, received.Name)
		assert.Equal(t, "Novo Nome", *received.Name)
		assert.Nil(t, received.Email, "e-mail não enviado não deve ser alterado")
	})

	t.Run("200 devolve o usuário atualizado sem expor hash", func(t *testing.T) {
		router, svc := setupUserRouter(t)
		user := sampleUser()
		svc.On("UpdateUser", mock.Anything, user.ID, mock.Anything).Return(user, nil)

		rec := do(t, router, http.MethodPut, "/v1/user/"+user.ID.String(), `{"name":"Novo Nome"}`)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, user.ID.String(), decodeJSON[map[string]any](t, rec)["id"])
		assert.NotContains(t, rec.Body.String(), "argon2id")
	})

	t.Run("400 com id inválido sem chamar o service", func(t *testing.T) {
		router, svc := setupUserRouter(t)
		svc.On("UpdateUser", mock.Anything, mock.Anything, mock.Anything).Return(sampleUser(), nil).Maybe()

		rec := do(t, router, http.MethodPut, "/v1/user/nao-e-uuid", `{"name":"Novo Nome"}`)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		svc.AssertNotCalled(t, "UpdateUser", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("409 quando o e-mail já é de outro usuário", func(t *testing.T) {
		router, svc := setupUserRouter(t)
		svc.On("UpdateUser", mock.Anything, mock.Anything, mock.Anything).Return(nil, core.ErrEmailAlreadyExists)

		rec := do(t, router, http.MethodPut, "/v1/user/"+uuid.NewString(), `{"name":"Novo Nome"}`)

		assert.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("404 quando não existe", func(t *testing.T) {
		router, svc := setupUserRouter(t)
		svc.On("UpdateUser", mock.Anything, mock.Anything, mock.Anything).Return(nil, core.ErrUserNotFound)

		rec := do(t, router, http.MethodPut, "/v1/user/"+uuid.NewString(), `{"name":"Novo Nome"}`)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("só e-mail: nome chega nil ao service", func(t *testing.T) {
		router, svc := setupUserRouter(t)
		user := sampleUser()
		var received *core.UserUpdate
		svc.On("UpdateUser", mock.Anything, user.ID, mock.Anything).
			Run(func(args mock.Arguments) { received = args.Get(2).(*core.UserUpdate) }).
			Return(user, nil)

		rec := do(t, router, http.MethodPut, "/v1/user/"+user.ID.String(), `{"email":"novo@example.com"}`)

		assert.Equal(t, http.StatusOK, rec.Code)
		require.NotNil(t, received)
		assert.Nil(t, received.Name, "nome não enviado não deve ser alterado")
		require.NotNil(t, received.Email)
		assert.Equal(t, "novo@example.com", *received.Email)
	})

	t.Run("corpo vazio repassa o erro de update sem campos como 400", func(t *testing.T) {
		router, svc := setupUserRouter(t)
		svc.On("UpdateUser", mock.Anything, mock.Anything, &core.UserUpdate{}).Return(nil, core.ErrBadRequest)

		rec := do(t, router, http.MethodPut, "/v1/user/"+uuid.NewString(), `{}`)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	invalidBodies := map[string]string{
		"nome vazio":       `{"name":""}`,
		"e-mail inválido":  `{"email":"nao-e-email"}`,
		"nome muito longo": `{"name":"` + strings.Repeat("a", 101) + `"}`,
		"JSON malformado":  `{"name":`,
	}
	for name, body := range invalidBodies {
		t.Run("400 sem chamar o service: "+name, func(t *testing.T) {
			router, _ := setupUserRouter(t)

			rec := do(t, router, http.MethodPut, "/v1/user/"+uuid.NewString(), body)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, "BAD_REQUEST", decodeJSON[errorBody](t, rec).Error.Code)
		})
	}
}

func TestUserControllerDeleteUser(t *testing.T) {
	t.Run("200 ao apagar", func(t *testing.T) {
		router, svc := setupUserRouter(t)
		id := uuid.New()
		svc.On("DeleteUser", mock.Anything, id).Return(nil)

		rec := do(t, router, http.MethodDelete, "/v1/user/"+id.String(), "")

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("400 com id inválido", func(t *testing.T) {
		router, _ := setupUserRouter(t)

		rec := do(t, router, http.MethodDelete, "/v1/user/nao-e-uuid", "")

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("404 quando não existe", func(t *testing.T) {
		router, svc := setupUserRouter(t)
		id := uuid.New()
		svc.On("DeleteUser", mock.Anything, id).Return(core.ErrUserNotFound)

		rec := do(t, router, http.MethodDelete, "/v1/user/"+id.String(), "")

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}
