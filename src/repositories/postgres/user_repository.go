package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
		INSERT INTO users (Name, Email, PasswordHash, PepperVersion)
		VALUES ($1, $2, $3, $4)
		RETURNING Id, Name, Email, CreatedAt, UpdatedAt, DeletedAt`

	var created core.User
	err := r.db.QueryRow(ctx, query, user.Name, user.Email, user.Password, user.PepperVersion).
		Scan(&created.ID, &created.Name, &created.Email, &created.CreatedAt, &created.UpdatedAt, &created.DeletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrUserCreationFailed
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil, core.ErrEmailAlreadyExists
		}
		return nil, fmt.Errorf("create user: %w", err)
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
		return nil, fmt.Errorf("get user by id: %w", err)
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
		return nil, fmt.Errorf("get user by name: %w", err)
	}
	defer rows.Close()

	var users []*core.User
	for rows.Next() {
		var user core.User
		err = rows.Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt)
		if err != nil {
			return nil, fmt.Errorf("get user by name: %w", err)
		}
		users = append(users, &user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get user by name: %w", err)
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
		return nil, fmt.Errorf("get user by email: %w", err)
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
