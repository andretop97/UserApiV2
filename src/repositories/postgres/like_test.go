package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeLike(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"texto sem curingas não muda", "joao", "joao"},
		{"string vazia", "", ""},
		{"escapa porcentagem", "100%", `100\%`},
		{"escapa underscore", "ana_maria", `ana\_maria`},
		{"escapa barra invertida", `c:\dir`, `c:\\dir`},
		{"barra seguida de curinga não escapa duas vezes", `\%`, `\\\%`},
		{"vários curingas", "%_%", `\%\_\%`},
		{"acentos e unicode não mudam", "João Ção", "João Ção"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, escapeLike(tc.input))
		})
	}
}
