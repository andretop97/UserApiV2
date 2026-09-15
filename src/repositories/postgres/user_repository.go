package postgres

import (
	"context"
	"errors"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrUserCreationFailed
		}
		return nil, err
	}

	return &created, nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*core.User, error) {
	const query = `
		SELECT 
			users.Id, 
			users.Name, 
			users.Email, 
			users.CreatedAt, 
			users.UpdatedAt, 
			users.DeletedAt
		FROM users 
		WHERE 
			users.Id = $1 
			AND users.DeletedAt IS NULL
		LIMIT 1`

	var users core.User
	err := r.db.QueryRow(ctx, query, id).
		Scan(&users.ID, &users.Name, &users.Email, &users.CreatedAt, &users.UpdatedAt, &users.DeletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrUserNotFound
		}
		return nil, err
	}

	return &users, nil
}

func (r *UserRepository) GetUserByName(ctx context.Context, name string) ([]*core.User, error) {
	const query = `
		SELECT 
			users.Id, 
			users.Name, 
			users.Email, 
			users.CreatedAt, 
			users.UpdatedAt, 
			users.DeletedAt
		FROM users 
		WHERE 
			users.Name ILIKE '%' || $1 || '%'
			AND users.DeletedAt IS NULL`

	rows, err := r.db.Query(ctx, query, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*core.User
	for rows.Next() {
		var user core.User
		err = rows.Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, &user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, core.ErrUserNotFound
	}
	return users, nil
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*core.User, error) {
	const query = `
		SELECT 
			users.Id, 
			users.Name, 
			users.Email, 
			users.CreatedAt, 
			users.UpdatedAt, 
			users.DeletedAt
		FROM users 
		WHERE 
			users.Email = $1
			AND users.DeletedAt IS NULL
		LIMIT 1`

	var users core.User
	err := r.db.QueryRow(ctx, query, email).
		Scan(&users.ID, &users.Name, &users.Email, &users.CreatedAt, &users.UpdatedAt, &users.DeletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrUserNotFound
		}
		return nil, err
	}
	return &users, nil
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
