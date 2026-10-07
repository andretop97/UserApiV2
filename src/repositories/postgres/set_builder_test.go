package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetBuilder(t *testing.T) {
	t.Run("sem campos é vazio", func(t *testing.T) {
		b := newSetBuilder("id")

		assert.True(t, b.Empty())
		set, args := b.Build()
		assert.Equal(t, "", set)
		assert.Equal(t, []any{"id"}, args)
	})

	t.Run("placeholders começam depois dos args fixos", func(t *testing.T) {
		b := newSetBuilder("id")
		b.Set("Name", "João")
		b.Set("Email", "joao@example.com")

		set, args := b.Build()

		assert.Equal(t, "Name = $2, Email = $3", set)
		assert.Equal(t, []any{"id", "João", "joao@example.com"}, args)
	})

	t.Run("sem args fixos começa em $1", func(t *testing.T) {
		b := newSetBuilder()
		b.Set("Name", "João")

		set, args := b.Build()

		assert.Equal(t, "Name = $1", set)
		assert.Equal(t, []any{"João"}, args)
	})

	t.Run("Raw entra sem consumir placeholder", func(t *testing.T) {
		b := newSetBuilder("id")
		b.Set("Name", "João")
		b.Raw("UpdatedAt = NOW()")
		b.Set("Email", "joao@example.com")

		set, args := b.Build()

		assert.Equal(t, "Name = $2, UpdatedAt = NOW(), Email = $3", set)
		assert.Len(t, args, 3)
	})

	t.Run("só Raw não é considerado vazio", func(t *testing.T) {
		b := newSetBuilder("id")
		b.Raw("UpdatedAt = NOW()")

		assert.False(t, b.Empty())
	})
}
