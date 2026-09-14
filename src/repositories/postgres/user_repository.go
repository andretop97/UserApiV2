package postgres

import (
	"context"

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

func (r *UserRepository) CreateUser(ctx context.Context, user *core.User) (*core.User, error) {
	const query = `
		INSERT INTO users (Id, Name, Email, PasswordHash, CreatedAt, UpdatedAt)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING Id, Name, Email, CreatedAt, UpdatedAt, DeletedAt`

	var created core.User
	err := r.db.QueryRow(ctx, query, user.ID, user.Name, user.Email, user.Password, user.CreatedAt, user.UpdatedAt).
		Scan(&created.ID, &created.Name, &created.Email, &created.CreatedAt, &created.UpdatedAt, &created.DeletedAt)
	if err != nil {
		return nil, err
	}

	return &created, nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*core.User, error) {
	return nil, nil
}

func (r *UserRepository) GetUserByName(ctx context.Context, name string) (*core.User, error) {
	return nil, nil
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*core.User, error) {
	return nil, nil
}
func (r *UserRepository) GetAllUsers(ctx context.Context) ([]*core.User, error) {
	return nil, nil
}
func (r *UserRepository) UpdateUser(ctx context.Context, user *core.User) (*core.User, error) {
	return nil, nil
}
func (r *UserRepository) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return nil
}
