package migrations

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Garante que os .sql foram embutidos no binário: sem isso o AutoMigrate
// não teria o que aplicar, independente do diretório em que o programa roda.
func TestEmbeddedMigrations(t *testing.T) {
	names, err := fs.Glob(files, "*.sql")
	require.NoError(t, err)

	assert.Contains(t, names, "000001_create_user_table.up.sql")
	assert.Contains(t, names, "000001_create_user_table.down.sql")
}

func TestEveryUpHasDown(t *testing.T) {
	ups, err := fs.Glob(files, "*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, ups)

	for _, up := range ups {
		down := up[:len(up)-len(".up.sql")] + ".down.sql"
		_, err := fs.Stat(files, down)
		assert.NoError(t, err, "migration %s sem o .down.sql correspondente", up)
	}
}
