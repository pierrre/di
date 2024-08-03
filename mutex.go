package di

import (
	"context"
)

type mutex struct {
	ch chan struct{}
}

func newMutex() *mutex {
	return &mutex{
		ch: make(chan struct{}, 1),
	}
}

func (m *mutex) lock(ctx context.Context) (context.Context, error) {
	previous, _ := ctx.Value(mutexListContextKey{}).(*mutexList)
	for v := previous; v != nil; v = v.previous {
		if v.mu == m {
			return nil, ErrCycle
		}
	}
	select {
	case m.ch <- struct{}{}:
		ctx = context.WithValue(ctx, mutexListContextKey{}, &mutexList{
			previous: previous,
			mu:       m,
		})
		return ctx, nil
	case <-ctx.Done():
		return nil, ctx.Err() //nolint:wrapcheck // We don't neet to wrap.
	}
}

func (m *mutex) unlock() {
	<-m.ch
}

type mutexList struct {
	previous *mutexList
	mu       *mutex
}

type mutexListContextKey struct{}
