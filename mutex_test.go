package di

import (
	"context"
	"strconv"
	"testing"

	"github.com/pierrre/assert"
)

func BenchmarkMutex(b *testing.B) {
	for _, n := range []int{0, 1, 2, 5, 10, 20, 50, 100} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			ctx := context.Background()
			var err error
			for range n {
				ctx, err = newMutex().lock(ctx)
				assert.NoError(b, err)
			}
			b.ResetTimer()
			mu := newMutex()
			for range b.N {
				_, _ = mu.lock(ctx)
				mu.unlock()
			}
		})
	}
}
