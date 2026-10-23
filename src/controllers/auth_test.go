package controllers_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/andretop97/UserApiV2/src/controllers"
	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/routes"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const loginPath = "/v1/auth/login"

func setupAuthRouter(t *testing.T) (*gin.Engine, *authServiceMock) {
	t.Helper()
	svc := &authServiceMock{}
	t.Cleanup(func() { svc.AssertExpectations(t) })
	router := gin.New()
	router.Use(gin.Recovery())
	routes.Routes(router, &routes.Controllers{
		User: controllers.NewUserController(&userServiceMock{}),
		Auth: controllers.NewAuthController(svc),
	})
	return router, svc
}

func TestAuthControllerLogin(t *testing.T) {
	t.Run("200 com o contrato de MFA pendente", func(t *testing.T) {
		router, svc := setupAuthRouter(t)
		svc.On("Login", mock.Anything, "joao@example.com", "senha").Return(&core.LoginResult{
			Token:     "mfa-token",
			ExpiresIn: 10 * time.Minute,
			NextStep:  core.StepWebAuthn,
		}, nil)

		rec := do(t, router, http.MethodPost, loginPath, `{"email":"joao@example.com","password":"senha"}`)

		assert.Equal(t, http.StatusOK, rec.Code)
		body := decodeJSON[map[string]any](t, rec)
		assert.Equal(t, map[string]any{
			"status":     "mfa_required",
			"mfa_token":  "mfa-token",
			"token_type": "Bearer",
			"expires_in": float64(600),
			"next_step":  "webauthn",
		}, body)
	})

	t.Run("resposta não expõe o token como token/access_token", func(t *testing.T) {
		router, svc := setupAuthRouter(t)
		svc.On("Login", mock.Anything, mock.Anything, mock.Anything).Return(&core.LoginResult{
			Token: "mfa-token", ExpiresIn: time.Minute, NextStep: core.StepWebAuthn,
		}, nil)

		rec := do(t, router, http.MethodPost, loginPath, `{"email":"joao@example.com","password":"senha"}`)

		body := decodeJSON[map[string]any](t, rec)
		assert.NotContains(t, body, "token")
		assert.NotContains(t, body, "access_token")
	})

	badRequests := map[string]string{
		"JSON malformado": `{"email":`,
		"corpo vazio":     `{}`,
		"sem e-mail":      `{"password":"senha"}`,
		"sem senha":       `{"email":"joao@example.com"}`,
		"e-mail inválido": `{"email":"nao-e-email","password":"senha"}`,
		"senha em branco": `{"email":"joao@example.com","password":""}`,
	}
	for name, payload := range badRequests {
		t.Run("400 sem chamar o service: "+name, func(t *testing.T) {
			router, _ := setupAuthRouter(t)

			rec := do(t, router, http.MethodPost, loginPath, payload)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, "BAD_REQUEST", decodeJSON[errorBody](t, rec).Error.Code)
		})
	}

	t.Run("credenciais inválidas viram 401 genérico", func(t *testing.T) {
		router, svc := setupAuthRouter(t)
		svc.On("Login", mock.Anything, mock.Anything, mock.Anything).Return(nil, core.ErrInvalidCredentials)

		rec := do(t, router, http.MethodPost, loginPath, `{"email":"joao@example.com","password":"errada"}`)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		body := decodeJSON[errorBody](t, rec)
		assert.Equal(t, "INVALID_CREDENTIALS", body.Error.Code)
		assert.Equal(t, "invalid email or password", body.Error.Message)
	})

	t.Run("erro interno vira 500 sem vazar a mensagem", func(t *testing.T) {
		router, svc := setupAuthRouter(t)
		svc.On("Login", mock.Anything, mock.Anything, mock.Anything).Return(nil, errBoom)

		rec := do(t, router, http.MethodPost, loginPath, `{"email":"joao@example.com","password":"senha"}`)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		body := decodeJSON[errorBody](t, rec)
		assert.Equal(t, "INTERNAL_SERVER_ERROR", body.Error.Code)
		assert.NotContains(t, rec.Body.String(), "detalhe interno")
	})
}

// Os handlers WebAuthn ainda são stubs: hoje respondem 200 com corpo vazio para qualquer requisição.
func TestAuthControllerWebAuthnStubs(t *testing.T) {
	paths := []string{
		"/v1/auth/webauthn/begin",
		"/v1/auth/webauthn/finish",
		"/v1/auth/webauthn/register/begin",
		"/v1/auth/webauthn/register/finish",
	}
	for _, path := range paths {
		t.Run("requisição sem corpo não responde 200 vazio: "+path, func(t *testing.T) {
			router, svc := setupAuthRouter(t)
			svc.On("WebAuthnBegin", mock.Anything, mock.Anything).Return(nil, core.ErrTokenInvalid).Maybe()
			svc.On("WebAuthnFinish", mock.Anything, mock.Anything, mock.Anything).Return(nil, core.ErrTokenInvalid).Maybe()
			svc.On("RegisterWebAuthnBegin", mock.Anything, mock.Anything).Return(nil, core.ErrTokenInvalid).Maybe()
			svc.On("RegisterWebAuthnFinish", mock.Anything, mock.Anything, mock.Anything).Return(nil, core.ErrTokenInvalid).Maybe()

			rec := do(t, router, http.MethodPost, path, "")

			assert.NotEqual(t, http.StatusOK, rec.Code, "corpo: %q", rec.Body.String())
			assert.NotEmpty(t, rec.Body.String())
		})
	}
}
