//go:build integration

package migrations_test

import (
	"context"
	"os"
	"testing"

	"github.com/andretop97/UserApiV2/src/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// Roda o AutoMigrate real (com as migrations embutidas) contra um Postgres descartável.
//
//	go test -tags integration ./src/migrations/...

func TestAutoMigrate(t *testing.T) {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:18.3",
		tcpostgres.WithDatabase("migrate_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)

	t.Setenv("POSTGRESQL_HOST", host)
	t.Setenv("POSTGRESQL_PORT", port.Port())
	t.Setenv("POSTGRESQL_USERNAME", "test")
	t.Setenv("POSTGRESQL_PASSWORD", "test")
	t.Setenv("POSTGRESQL_DATABASE", "migrate_test")

	// Roda de outro diretório para provar que não depende do diretório atual.
	t.Chdir(os.TempDir())

	t.Run("aplica as migrations", func(t *testing.T) {
		require.NoError(t, migrations.AutoMigrate())
	})

	t.Run("rodar de novo não é erro", func(t *testing.T) {
		require.NoError(t, migrations.AutoMigrate())
	})

	t.Run("tabela users existe", func(t *testing.T) {
		dsn, err := container.ConnectionString(ctx, "sslmode=disable")
		require.NoError(t, err)
		conn, err := pgx.Connect(ctx, dsn)
		require.NoError(t, err)
		t.Cleanup(func() { conn.Close(ctx) })

		var exists bool
		err = conn.QueryRow(ctx, `SELECT to_regclass('public.users') IS NOT NULL`).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists)
	})
}
