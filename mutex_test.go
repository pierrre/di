package di

import (
	"strconv"
	"testing"

	"github.com/pierrre/assert"
)

func BenchmarkMutex(b *testing.B) {
	for _, n := range []int{0, 1, 2, 5, 10, 20, 50, 100} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			ctx := b.Context()
			var err error
			for range n {
				ctx, err = newMutex().lock(ctx)
				assert.NoError(b, err)
			}
			mu := newMutex()
			for b.Loop() {
				_, _ = mu.lock(ctx)
				mu.unlock()
			}
		})
	}
}
