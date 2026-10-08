package core_test

import (
	"strconv"
	"testing"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeEmail(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"já normalizado", "joao@example.com", "joao@example.com"},
		{"maiúsculas", "Joao@Example.COM", "joao@example.com"},
		{"espaços nas pontas", "  joao@example.com \t\n", "joao@example.com"},
		{"maiúsculas e espaços", "  JOAO@EXAMPLE.COM  ", "joao@example.com"},
		{"vazio", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, core.NormalizeEmail(tc.input))
		})
	}
}

func TestNormalizeName(t *testing.T) {
	valid := []struct {
		name  string
		input string
		want  string
	}{
		{"já normalizado", "João", "João"},
		{"espaços nas pontas", "  João Silva \t\n", "João Silva"},
		{"preserva espaços internos e caixa", "  Maria  da Silva ", "Maria  da Silva"},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			got, err := core.NormalizeName(tc.input)

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	for _, input := range []string{"", "   ", "\t\n"} {
		t.Run("vazio depois do trim vira ErrBadRequest: "+strconv.Quote(input), func(t *testing.T) {
			got, err := core.NormalizeName(input)

			assert.ErrorIs(t, err, core.ErrBadRequest)
			assert.Empty(t, got)
		})
	}
}

func TestDomainError(t *testing.T) {
	t.Run("Error devolve a mensagem pública", func(t *testing.T) {
		assert.Equal(t, "invalid email or password", core.ErrInvalidCredentials.Error())
	})

	t.Run("erros de autenticação respondem 401", func(t *testing.T) {
		assert.Equal(t, 401, core.ErrInvalidCredentials.Status)
		assert.Equal(t, 401, core.ErrTokenExpired.Status)
	})
}
