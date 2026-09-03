package di

import (
	"context"
	"strconv"
	"testing"

	"github.com/pierrre/assert"
)

func TestMutexWaitGranted(t *testing.T) {
	m := mutex{}
	w := &mutexWaiter{id: 1, done: make(chan struct{})}
	m.holder = w.id
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := m.wait(ctx, w)
	assert.NoError(t, err)
}

func BenchmarkMutex(b *testing.B) {
	for _, n := range []int{0, 1, 2, 5, 10, 20, 50, 100} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			ctx := b.Context()
			var err error
			for range n {
				mu := mutex{}
				ctx, err = mu.lock(ctx)
				assert.NoError(b, err)
			}
			mu := mutex{}
			for b.Loop() {
				_, _ = mu.lock(ctx)
				mu.unlock()
			}
		})
	}
}
