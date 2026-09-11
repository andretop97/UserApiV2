package core

import "github.com/google/uuid"

type UserRepository interface {
	CreateUser(user *User) (User, error)
	GetUserByID(id uuid.UUID) (User, error)
	GetUserByName(name string) (User, error)
	GetUserByEmail(email string) (User, error)
	GetAllUsers() ([]User, error)
	UpdateUser(user *User) (User, error)
	DeleteUser(id uuid.UUID) error
}
