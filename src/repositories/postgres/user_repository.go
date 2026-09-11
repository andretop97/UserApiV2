package postgres

import (
	"github.com/andretop97/UserApiV2/src/core"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) core.UserRepository {
	return &UserRepository{
		db: pool,
	}
}

func (r *UserRepository) CreateUser(user *core.User) (core.User, error) {
	// Implement the logic to create a user in the PostgreSQL database
	return core.User{}, nil
}

func (r *UserRepository) GetUserByID(id uuid.UUID) (core.User, error) {
	// Implement the logic to retrieve a user by ID from the PostgreSQL database
	return core.User{}, nil
}

func (r *UserRepository) GetUserByName(name string) (core.User, error) {
	// Implement the logic to retrieve a user by name from the PostgreSQL database
	return core.User{}, nil
}

func (r *UserRepository) GetUserByEmail(email string) (core.User, error) {
	// Implement the logic to retrieve a user by email from the PostgreSQL database
	return core.User{}, nil
}
func (r *UserRepository) GetAllUsers() ([]core.User, error) {
	// Implement the logic to retrieve all users from the PostgreSQL database
	return []core.User{}, nil
}
func (r *UserRepository) UpdateUser(user *core.User) (core.User, error) {
	// Implement the logic to update a user in the PostgreSQL database
	return core.User{}, nil
}
func (r *UserRepository) DeleteUser(id uuid.UUID) error {
	// Implement the logic to delete a user from the PostgreSQL database
	return nil
}
