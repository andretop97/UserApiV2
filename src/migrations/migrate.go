package migrations

import (
	"embed"
	"errors"

	"github.com/andretop97/UserApiV2/src/utils"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed *.sql
var files embed.FS

func AutoMigrate() error {
	src, err := iofs.New(files, ".")
	if err != nil {
		return err
	}

	postgresqlEnv, err := utils.NewPostgresqlEnv()
	if err != nil {
		return err
	}

	m, err := migrate.NewWithSourceInstance(
		"iofs",
		src,
		"pgx5://"+postgresqlEnv.ConnectionString()[len("postgresql://"):],
	)
	if err != nil {
		return err
	}

	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}
