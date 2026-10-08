package redis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestJitterDuration(t *testing.T) {
	t.Run("sempre dentro de [d, d*1.1)", func(t *testing.T) {
		d := 10 * time.Minute
		for range 1000 {
			got := jitterDuration(d)
			assert.GreaterOrEqual(t, got, d)
			assert.Less(t, got, d+d/10)
		}
	})

	t.Run("varia entre chamadas", func(t *testing.T) {
		seen := map[time.Duration]bool{}
		for range 100 {
			seen[jitterDuration(10*time.Minute)] = true
		}
		assert.Greater(t, len(seen), 1, "sem variação o jitter não espalha as expirações")
	})

	t.Run("durações muito pequenas voltam sem jitter e sem panic", func(t *testing.T) {
		for _, d := range []time.Duration{0, 1, 9} {
			assert.Equal(t, d, jitterDuration(d))
		}
	})

	t.Run("a partir de 10ns já aplica jitter dentro da faixa", func(t *testing.T) {
		got := jitterDuration(10)
		assert.GreaterOrEqual(t, got, time.Duration(10))
		assert.Less(t, got, time.Duration(11))
	})
}
