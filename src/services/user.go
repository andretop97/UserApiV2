package services

import (
	"context"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/google/uuid"
)

type UserService struct {
	userRepository core.UserRepository
}

func NewUserService(userRepository core.UserRepository) core.UserService {
	return &UserService{
		userRepository: userRepository,
	}
}

func (s *UserService) CreateUser(ctx context.Context, user *core.User) (*core.User, error) {
	u, err := s.userRepository.GetUserByEmail(ctx, user.Email)
	if err != nil {
		return nil, err
	}

	if u != nil {
		return nil, core.ErrEmailAlreadyExists
	}

	createdUser, err := s.userRepository.CreateUser(ctx, user)

	if err != nil {
		return nil, err
	}

	return createdUser, nil
}

func (s *UserService) GetUserByID(ctx context.Context, id uuid.UUID) (*core.User, error) {
	// Implement the logic to retrieve a user by ID
	return nil, nil
}

func (s *UserService) GetUserByName(ctx context.Context, name string) (*core.User, error) {
	return nil, nil
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*core.User, error) {
	return nil, nil
}

func (s *UserService) GetAllUsers(ctx context.Context) ([]*core.User, error) {
	return nil, nil
}

func (s *UserService) UpdateUser(ctx context.Context, user *core.User) (*core.User, error) {
	return nil, nil
}

func (s *UserService) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (s *UserService) LoginUser(ctx context.Context, email string, password string) (*string, error) {
	return nil, nil
}
