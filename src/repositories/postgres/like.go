package postgres

import "strings"

var likeEscaper = strings.NewReplacer(
	`\`, `\\`,
	`%`, `\%`,
	`_`, `\_`,
)

func escapeLike(s string) string {
	return likeEscaper.Replace(s)
}
