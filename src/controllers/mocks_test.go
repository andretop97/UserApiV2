package controllers_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mocks dos services. Chamadas não configuradas com On(...) fazem o mock entrar em pânico,
// o que o gin transforma em 500, então qualquer acesso inesperado falha o teste.

var errBoom = errors.New("boom: detalhe interno do banco")

func init() {
	gin.SetMode(gin.TestMode)
}

type authServiceMock struct{ mock.Mock }

func (m *authServiceMock) Login(ctx context.Context, email, password string) (*core.LoginResult, error) {
	args := m.Called(ctx, email, password)
	r, _ := args.Get(0).(*core.LoginResult)
	return r, args.Error(1)
}

func (m *authServiceMock) RegisterWebAuthnBegin(ctx context.Context, email string) (string, error) {
	args := m.Called(ctx, email)
	return args.String(0), args.Error(1)
}

func (m *authServiceMock) RegisterWebAuthnFinish(ctx context.Context, email, credential string) error {
	return m.Called(ctx, email, credential).Error(0)
}

func (m *authServiceMock) WebAuthnBegin(ctx context.Context, email string) (string, error) {
	args := m.Called(ctx, email)
	return args.String(0), args.Error(1)
}

func (m *authServiceMock) WebAuthnFinish(ctx context.Context, email, credential string) error {
	return m.Called(ctx, email, credential).Error(0)
}

type userServiceMock struct{ mock.Mock }

func (m *userServiceMock) CreateUser(ctx context.Context, user *core.User) (*core.User, error) {
	args := m.Called(ctx, user)
	return userArg(args, 0), args.Error(1)
}

func (m *userServiceMock) GetUserByID(ctx context.Context, id uuid.UUID) (*core.User, error) {
	args := m.Called(ctx, id)
	return userArg(args, 0), args.Error(1)
}

func (m *userServiceMock) GetUsersByName(ctx context.Context, name string) ([]*core.User, error) {
	args := m.Called(ctx, name)
	return usersArg(args, 0), args.Error(1)
}

func (m *userServiceMock) GetUserByEmail(ctx context.Context, email string) (*core.User, error) {
	args := m.Called(ctx, email)
	return userArg(args, 0), args.Error(1)
}

func (m *userServiceMock) GetAllUsers(ctx context.Context) ([]*core.User, error) {
	args := m.Called(ctx)
	return usersArg(args, 0), args.Error(1)
}

func (m *userServiceMock) UpdateUser(ctx context.Context, id uuid.UUID, userUpdate *core.UserUpdate) (*core.User, error) {
	args := m.Called(ctx, id, userUpdate)
	return userArg(args, 0), args.Error(1)
}

func (m *userServiceMock) DeleteUser(ctx context.Context, id uuid.UUID) error {
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

// do executa uma requisição contra o router e devolve a resposta gravada.
func do(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// decodeJSON exige que o corpo inteiro seja um único JSON válido.
func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	dec := json.NewDecoder(rec.Body)
	require.NoError(t, dec.Decode(&v), "corpo: %s", rec.Body.String())
	require.False(t, dec.More(), "corpo tem mais de um JSON concatenado")
	return v
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
