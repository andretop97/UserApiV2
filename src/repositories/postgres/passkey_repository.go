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
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation {
			return core.ErrUserNotFound
		}
		return fmt.Errorf("create passkey: %w", err)
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
			passkeys.UserId = $1
		ORDER BY CreatedAt`

	rows, err := pk.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list passkeys: %w", err)
	}
	defer rows.Close()
	passkeys, err := scanPasskey(rows)
	if err != nil {
		return nil, fmt.Errorf("list passkeys: %w", err)
	}
	return passkeys, nil

}
func (pk *PasskeyRepository) UpdateAfterLogin(ctx context.Context, id []byte, data []byte) error {
	const query = `UPDATE passkeys SET Data = $2, LastUsedAt = NOW() WHERE Id = $1`
	tag, err := pk.db.Exec(ctx, query, id, data)

	if err != nil {
		return fmt.Errorf("update passkey: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return core.ErrPasskeyNotFound
	}
	return nil
}

func scanPasskey(rows pgx.Rows) ([]*core.Passkey, error) {
	passkeys := make([]*core.Passkey, 0)
	for rows.Next() {
		var passkey core.Passkey
		err := rows.Scan(&passkey.ID, &passkey.UserID, &passkey.Data, &passkey.CreatedAt, &passkey.LastUsedAt)
		if err != nil {
			return nil, fmt.Errorf("scan passkey: %w", err)
		}
		passkeys = append(passkeys, &passkey)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan passkeys: %w", err)
	}

	return passkeys, nil
}
