package services_test

import (
	"context"
	"errors"
	"time"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// Mocks das dependências dos services. Chamadas não configuradas com On(...) fazem o mock
// entrar em pânico, então qualquer acesso inesperado a uma dependência falha o teste.

var errBoom = errors.New("boom")

type userRepositoryMock struct{ mock.Mock }

func (m *userRepositoryMock) CreateUser(ctx context.Context, user *core.User) (*core.User, error) {
	args := m.Called(ctx, user)
	return userArg(args, 0), args.Error(1)
}

func (m *userRepositoryMock) GetUserByID(ctx context.Context, id uuid.UUID) (*core.User, error) {
	args := m.Called(ctx, id)
	return userArg(args, 0), args.Error(1)
}

func (m *userRepositoryMock) GetUsersByName(ctx context.Context, name string) ([]*core.User, error) {
	args := m.Called(ctx, name)
	return usersArg(args, 0), args.Error(1)
}

func (m *userRepositoryMock) GetUserByEmail(ctx context.Context, email string) (*core.User, error) {
	args := m.Called(ctx, email)
	return userArg(args, 0), args.Error(1)
}

func (m *userRepositoryMock) GetAllUsers(ctx context.Context) ([]*core.User, error) {
	args := m.Called(ctx)
	return usersArg(args, 0), args.Error(1)
}

func (m *userRepositoryMock) UpdateUser(ctx context.Context, id uuid.UUID, userUpdate *core.UserUpdate) (*core.User, error) {
	args := m.Called(ctx, id, userUpdate)
	return userArg(args, 0), args.Error(1)
}

func (m *userRepositoryMock) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

type passkeyRepositoryMock struct{ mock.Mock }

func (m *passkeyRepositoryMock) Create(ctx context.Context, p *core.Passkey) error {
	return m.Called(ctx, p).Error(0)
}

func (m *passkeyRepositoryMock) ListByUser(ctx context.Context, userID uuid.UUID) ([]*core.Passkey, error) {
	args := m.Called(ctx, userID)
	passkeys, _ := args.Get(0).([]*core.Passkey)
	return passkeys, args.Error(1)
}

func (m *passkeyRepositoryMock) UpdateAfterLogin(ctx context.Context, id []byte, data []byte) error {
	return m.Called(ctx, id, data).Error(0)
}

type authRepositoryMock struct{ mock.Mock }

func (m *authRepositoryMock) SetTemporaryToken(ctx context.Context, userID uuid.UUID, token string, ttl time.Duration) error {
	return m.Called(ctx, userID, token, ttl).Error(0)
}

func (m *authRepositoryMock) SetSessionToken(ctx context.Context, userID uuid.UUID, token string, ttl time.Duration) error {
	return m.Called(ctx, userID, token, ttl).Error(0)
}

func (m *authRepositoryMock) GetTemporaryToken(ctx context.Context, userID uuid.UUID) (string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.Error(1)
}

func (m *authRepositoryMock) GetSessionToken(ctx context.Context, userID uuid.UUID) (string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.Error(1)
}

func (m *authRepositoryMock) SaveCeremonySession(ctx context.Context, userID uuid.UUID, ceremony core.CeremonyType, session []byte, ttl time.Duration) error {
	return m.Called(ctx, userID, ceremony, session, ttl).Error(0)
}

func (m *authRepositoryMock) ConsumeCeremonySession(ctx context.Context, userID uuid.UUID, ceremony core.CeremonyType) ([]byte, error) {
	args := m.Called(ctx, userID, ceremony)
	session, _ := args.Get(0).([]byte)
	return session, args.Error(1)
}

type passwordEncryptionMock struct{ mock.Mock }

func (m *passwordEncryptionMock) Hash(password string) (string, int, error) {
	args := m.Called(password)
	return args.String(0), args.Int(1), args.Error(2)
}

func (m *passwordEncryptionMock) Verify(password, encodedHash string, pepperVersion int) (bool, error) {
	args := m.Called(password, encodedHash, pepperVersion)
	return args.Bool(0), args.Error(1)
}

type jwtProviderMock[T any] struct{ mock.Mock }

func (m *jwtProviderMock[T]) GenerateToken(payload T) (string, error) {
	args := m.Called(payload)
	return args.String(0), args.Error(1)
}

func (m *jwtProviderMock[T]) ValidateToken(token string) (T, error) {
	args := m.Called(token)
	v, _ := args.Get(0).(T)
	return v, args.Error(1)
}

func (m *jwtProviderMock[T]) TTL() time.Duration {
	return m.Called().Get(0).(time.Duration)
}

func userArg(args mock.Arguments, i int) *core.User {
	u, _ := args.Get(i).(*core.User)
	return u
}

func usersArg(args mock.Arguments, i int) []*core.User {
	u, _ := args.Get(i).([]*core.User)
	return u
}

func ptr[T any](v T) *T { return &v }
