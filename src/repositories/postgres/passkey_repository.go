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

type PasskeyRepository struct {
	db *pgxpool.Pool
}

func NewPasskeyRepository(db *pgxpool.Pool) core.PasskeyRepository {
	return &PasskeyRepository{
		db: db,
	}
}

func (pk *PasskeyRepository) Create(ctx context.Context, p *core.Passkey) error {
	const query = `
		INSERT INTO passkeys (Id, UserId, Data)
		VALUES ($1, $2, $3)`

	_, err := pk.db.Exec(ctx, query, p.ID, p.UserID, p.Data)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return core.ErrPasskeyAlreadyRegistered
		}
		return err
	}

	return nil
}
func (pk *PasskeyRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*core.Passkey, error) {
	const query = `
		SELECT 
			passkeys.Id, 
			passkeys.UserId, 
			passkeys.Data, 
			passkeys.CreatedAt, 
			passkeys.LastUsedAt
		FROM passkeys 
		WHERE 
			passkeys.UserId = $1`

	rows, err := pk.db.Query(ctx, query, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrPasskeyNotFound
		}
		return nil, fmt.Errorf("get passkey by UserId: %w", err)
	}
	defer rows.Close()
	passkeys, err := scanPasskey(rows)
	if err != nil {
		return nil, fmt.Errorf("get passkeys by UserId: %w", err)
	}
	return passkeys, nil

}
func (pk *PasskeyRepository) UpdateAfterLogin(ctx context.Context, id []byte, data []byte) error {
	return nil
}

func scanPasskey(rows pgx.Rows) ([]*core.Passkey, error) {
	passkeys := make([]*core.Passkey, 0)
	for rows.Next() {
		var user core.Passkey
		err := rows.Scan(&user.ID, &user.UserID, &user.Data, &user.CreatedAt, &user.LastUsedAt)
		if err != nil {
			return nil, fmt.Errorf("scan passkey: %w", err)
		}
		passkeys = append(passkeys, &user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan passkeys: %w", err)
	}

	return passkeys, nil
}
