package controllers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Endpoints das cerimônias WebAuthn (passos 5 e 6). O mfa_token vem no Authorization;
// o begin não tem corpo e o finish recebe a resposta do autenticador, que vai intacta ao service.

type ceremonyEndpoint struct {
	path   string
	method string // nome do método no core.AuthService
	finish bool
}

var ceremonyEndpoints = []ceremonyEndpoint{
	{path: "/v1/auth/webauthn/register/begin", method: "RegisterWebAuthnBegin"},
	{path: "/v1/auth/webauthn/register/finish", method: "RegisterWebAuthnFinish", finish: true},
	{path: "/v1/auth/webauthn/begin", method: "WebAuthnBegin"},
	{path: "/v1/auth/webauthn/finish", method: "WebAuthnFinish", finish: true},
}

const (
	mfaToken        = "mfa-token"
	authenticatorRe = `{"id":"cred-id","rawId":"cred-id","type":"public-key","response":{"clientDataJSON":"abc"}}`
)

var ceremonyOptions = json.RawMessage(`{"publicKey":{"challenge":"c2VjcmV0","rpId":"localhost","timeout":300000}}`)

// doCeremony faz a requisição com o Authorization informado ("" = sem cabeçalho).
func doCeremony(t *testing.T, router http.Handler, path, authorization, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func bodyFor(e ceremonyEndpoint) string {
	if e.finish {
		return authenticatorRe
	}
	return ""
}

// expectService configura o retorno do service para o endpoint; err != nil simula falha.
func expectService(svc *authServiceMock, e ceremonyEndpoint, token string, err error) {
	if e.finish {
		var session *core.SessionResult
		if err == nil {
			session = &core.SessionResult{Token: "session-token", ExpiresIn: 72 * time.Hour}
		}
		svc.On(e.method, mock.Anything, token, mock.Anything).Return(session, err)
		return
	}
	var options json.RawMessage
	if err == nil {
		options = ceremonyOptions
	}
	svc.On(e.method, mock.Anything, token).Return(options, err)
}

func assertServiceNotCalled(t *testing.T, svc *authServiceMock, e ceremonyEndpoint) {
	t.Helper()
	if e.finish {
		svc.AssertNotCalled(t, e.method, mock.Anything, mock.Anything, mock.Anything)
		return
	}
	svc.AssertNotCalled(t, e.method, mock.Anything, mock.Anything)
}

func TestAuthControllerWebAuthnAuthorization(t *testing.T) {
	invalid := map[string]string{
		"sem Authorization":       "",
		"esquema Basic":           "Basic dXNlcjpzZW5oYQ==",
		"Bearer sem token":        "Bearer",
		"Bearer só com espaços":   "Bearer    ",
		"token sem esquema":       mfaToken,
		"esquema colado ao token": "Bearer" + mfaToken,
	}

	for _, e := range ceremonyEndpoints {
		for name, header := range invalid {
			t.Run(e.path+": "+name+" é 401 sem chamar o service", func(t *testing.T) {
				router, svc := setupAuthRouter(t)

				rec := doCeremony(t, router, e.path, header, bodyFor(e))

				assert.Equal(t, http.StatusUnauthorized, rec.Code, "corpo: %s", rec.Body.String())
				assert.Equal(t, "TOKEN_INVALID", decodeJSON[errorBody](t, rec).Error.Code)
				assertServiceNotCalled(t, svc, e)
			})
		}

		for name, header := range map[string]string{
			"Bearer padrão":    "Bearer " + mfaToken,
			"bearer minúsculo": "bearer " + mfaToken,
			"espaços em volta": "Bearer   " + mfaToken + "  ",
		} {
			t.Run(e.path+": "+name+" entrega só o token ao service", func(t *testing.T) {
				router, svc := setupAuthRouter(t)
				expectService(svc, e, mfaToken, nil)

				rec := doCeremony(t, router, e.path, header, bodyFor(e))

				assert.Equal(t, http.StatusOK, rec.Code, "corpo: %s", rec.Body.String())
			})
		}

		// O token não pode vir do corpo: o do finish é a resposta do autenticador.
		t.Run(e.path+": token no corpo não autentica", func(t *testing.T) {
			router, svc := setupAuthRouter(t)

			rec := doCeremony(t, router, e.path, "", `"`+mfaToken+`"`)

			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assertServiceNotCalled(t, svc, e)
		})
	}
}

func TestAuthControllerWebAuthnBegin(t *testing.T) {
	for _, e := range ceremonyEndpoints {
		if e.finish {
			continue
		}

		t.Run(e.path+": devolve as options do service sem alterar", func(t *testing.T) {
			router, svc := setupAuthRouter(t)
			expectService(svc, e, mfaToken, nil)

			rec := doCeremony(t, router, e.path, "Bearer "+mfaToken, "")

			require.Equal(t, http.StatusOK, rec.Code)
			assert.JSONEq(t, string(ceremonyOptions), rec.Body.String())
			assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
		})

		// As options carregam o challenge da cerimônia.
		t.Run(e.path+": resposta não pode ficar em cache", func(t *testing.T) {
			router, svc := setupAuthRouter(t)
			expectService(svc, e, mfaToken, nil)

			rec := doCeremony(t, router, e.path, "Bearer "+mfaToken, "")

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		})

		t.Run(e.path+": begin não exige corpo", func(t *testing.T) {
			router, svc := setupAuthRouter(t)
			expectService(svc, e, mfaToken, nil)

			rec := doCeremony(t, router, e.path, "Bearer "+mfaToken, "")

			assert.Equal(t, http.StatusOK, rec.Code, "corpo: %s", rec.Body.String())
		})
	}
}

func TestAuthControllerWebAuthnFinish(t *testing.T) {
	for _, e := range ceremonyEndpoints {
		if !e.finish {
			continue
		}

		t.Run(e.path+": repassa o corpo intacto ao service", func(t *testing.T) {
			router, svc := setupAuthRouter(t)
			svc.On(e.method, mock.Anything, mfaToken, []byte(authenticatorRe)).
				Return(&core.SessionResult{Token: "session-token", ExpiresIn: time.Hour}, nil)

			rec := doCeremony(t, router, e.path, "Bearer "+mfaToken, authenticatorRe)

			assert.Equal(t, http.StatusOK, rec.Code, "corpo: %s", rec.Body.String())
		})

		t.Run(e.path+": 200 com o contrato da sessão", func(t *testing.T) {
			router, svc := setupAuthRouter(t)
			svc.On(e.method, mock.Anything, mfaToken, mock.Anything).
				Return(&core.SessionResult{Token: "session-token", ExpiresIn: 72 * time.Hour}, nil)

			rec := doCeremony(t, router, e.path, "Bearer "+mfaToken, authenticatorRe)

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, map[string]any{
				"status":        "authenticated",
				"session_token": "session-token",
				"token_type":    "Bearer",
				"expires_in":    float64(72 * 60 * 60),
			}, decodeJSON[map[string]any](t, rec))
		})

		// O corpo leva o token de sessão.
		t.Run(e.path+": resposta não pode ficar em cache", func(t *testing.T) {
			router, svc := setupAuthRouter(t)
			expectService(svc, e, mfaToken, nil)

			rec := doCeremony(t, router, e.path, "Bearer "+mfaToken, authenticatorRe)

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		})

		t.Run(e.path+": corpo vazio é 400 sem chamar o service", func(t *testing.T) {
			router, svc := setupAuthRouter(t)

			rec := doCeremony(t, router, e.path, "Bearer "+mfaToken, "")

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, "BAD_REQUEST", decodeJSON[errorBody](t, rec).Error.Code)
			assertServiceNotCalled(t, svc, e)
		})

		t.Run(e.path+": corpo acima de 64 KB é 400 sem chamar o service", func(t *testing.T) {
			router, svc := setupAuthRouter(t)
			huge := `{"x":"` + strings.Repeat("a", 64<<10) + `"}`

			rec := doCeremony(t, router, e.path, "Bearer "+mfaToken, huge)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assertServiceNotCalled(t, svc, e)
		})

		t.Run(e.path+": corpo no limite de 64 KB é aceito", func(t *testing.T) {
			router, svc := setupAuthRouter(t)
			expectService(svc, e, mfaToken, nil)
			exact := strings.Repeat("a", 64<<10)

			rec := doCeremony(t, router, e.path, "Bearer "+mfaToken, exact)

			assert.Equal(t, http.StatusOK, rec.Code, "corpo: %s", rec.Body.String())
		})

		// Sem token, o corpo nem é lido.
		t.Run(e.path+": sem token não lê o corpo grande", func(t *testing.T) {
			router, svc := setupAuthRouter(t)
			huge := strings.Repeat("a", 1<<20)

			rec := doCeremony(t, router, e.path, "", huge)

			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assertServiceNotCalled(t, svc, e)
		})
	}
}

func TestAuthControllerWebAuthnErrors(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{core.ErrTokenInvalid, http.StatusUnauthorized, "TOKEN_INVALID"},
		{core.ErrTokenExpired, http.StatusUnauthorized, "TOKEN_EXPIRED"},
		{core.ErrCeremonyExpired, http.StatusUnauthorized, "CEREMONY_EXPIRED"},
		{core.ErrPasskeyInvalid, http.StatusUnauthorized, "PASSKEY_INVALID"},
		{core.ErrPasskeyRegistrationNotAllowed, http.StatusForbidden, "PASSKEY_REGISTRATION_NOT_ALLOWED"},
		{core.ErrPasskeyNotFound, http.StatusNotFound, "PASSKEY_NOT_FOUND"},
		{core.ErrPasskeyAlreadyRegistered, http.StatusConflict, "PASSKEY_ALREADY_REGISTERED"},
		{fmt.Errorf("%w: invalid registration response", core.ErrBadRequest), http.StatusBadRequest, "BAD_REQUEST"},
	}

	for _, e := range ceremonyEndpoints {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s: %s vira %d", e.path, tc.code, tc.status), func(t *testing.T) {
				router, svc := setupAuthRouter(t)
				expectService(svc, e, mfaToken, tc.err)

				rec := doCeremony(t, router, e.path, "Bearer "+mfaToken, bodyFor(e))

				assert.Equal(t, tc.status, rec.Code)
				assert.Equal(t, tc.code, decodeJSON[errorBody](t, rec).Error.Code)
				assert.NotContains(t, rec.Body.String(), "session_token")
			})
		}

		t.Run(e.path+": erro interno vira 500 sem vazar detalhe", func(t *testing.T) {
			router, svc := setupAuthRouter(t)
			expectService(svc, e, mfaToken, fmt.Errorf("redis: detalhe interno"))

			rec := doCeremony(t, router, e.path, "Bearer "+mfaToken, bodyFor(e))

			assert.Equal(t, http.StatusInternalServerError, rec.Code)
			assert.Equal(t, "INTERNAL_SERVER_ERROR", decodeJSON[errorBody](t, rec).Error.Code)
			assert.NotContains(t, rec.Body.String(), "detalhe interno")
		})
	}
}
