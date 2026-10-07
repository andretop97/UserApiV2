package postgres

import (
	"fmt"
	"strings"
)

type setBuilder struct {
	sets []string
	args []any
}

func newSetBuilder(fixedArgs ...any) *setBuilder {
	return &setBuilder{
		args: fixedArgs,
	}
}

func (b *setBuilder) Set(column string, value any) {
	b.args = append(b.args, value)
	b.sets = append(b.sets, fmt.Sprintf("%s = $%d", column, len(b.args)))
}

func (b *setBuilder) Raw(expr string) {
	b.sets = append(b.sets, expr)
}

func (b *setBuilder) Empty() bool {
	return len(b.sets) == 0
}

func (b *setBuilder) Build() (string, []any) {
	return strings.Join(b.sets, ", "), b.args
}
