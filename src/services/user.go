package services

import (
	"context"
	"errors"
	"strings"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/google/uuid"
)

type UserService struct {
	userRepository     core.UserRepository
	passwordEncryption core.PasswordEncryption
}

func NewUserService(userRepository core.UserRepository, passwordEncryption core.PasswordEncryption) core.UserService {
	return &UserService{
		userRepository:     userRepository,
		passwordEncryption: passwordEncryption,
	}
}

func (s *UserService) CreateUser(ctx context.Context, user *core.User) (*core.User, error) {
	user.Email = strings.TrimSpace(user.Email)
	user.Email = strings.ToLower(user.Email)
	user.Name = strings.TrimSpace(user.Name)

	u, err := s.userRepository.GetUserByEmail(ctx, user.Email)
	if err != nil {
		if !errors.Is(err, core.ErrUserNotFound) {
			return nil, err
		}
	}
	if u != nil {
		return nil, core.ErrEmailAlreadyExists
	}

	hashedPassword, pepperVersion, err := s.passwordEncryption.Hash(user.Password)
	if err != nil {
		return nil, err
	}

	user.Password = hashedPassword
	user.PepperVersion = pepperVersion

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
	users, err := s.userRepository.GetAllUsers(ctx)
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (s *UserService) UpdateUser(ctx context.Context, user *core.User) (*core.User, error) {
	return nil, nil
}

func (s *UserService) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return nil
}
