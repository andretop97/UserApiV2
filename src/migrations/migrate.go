package migrations

import (
	"errors"

	"github.com/andretop97/UserApiV2/src/utils"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func AutoMigrate() error {
	postgresqlEnv, err := utils.NewPostgresqlEnv()
	if err != nil {
		return err
	}

	m, err := migrate.New(
		"file://src/migrations",
		"pgx5://"+postgresqlEnv.ConnectionString()[len("postgresql://"):],
	)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}
