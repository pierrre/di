package di

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
)

type mutex struct {
	holder  callerID
	waiters []*mutexWaiter
}

type mutexWaiter struct {
	id   callerID
	done chan struct{}
}

func (m *mutex) lock(ctx context.Context) (context.Context, error) {
	ctx, id := getOrCreateCallerID(ctx)
	w, err := m.acquire(id)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return ctx, nil
	}
	err = m.wait(ctx, w)
	if err != nil {
		return nil, err
	}
	return ctx, nil
}

func (m *mutex) acquire(id callerID) (*mutexWaiter, error) {
	mutexRegistryMu.Lock()
	defer mutexRegistryMu.Unlock()
	if m.holder == 0 {
		m.holder = id
		return nil, nil //nolint:nilnil // A nil waiter means the mutex was acquired.
	}
	if deadlock(m.holder, id) {
		return nil, ErrCycle
	}
	w := &mutexWaiter{id: id, done: make(chan struct{})}
	m.waiters = append(m.waiters, w)
	mutexRegistryWaiting[id] = m
	return w, nil
}

func (m *mutex) wait(ctx context.Context, w *mutexWaiter) error {
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		mutexRegistryMu.Lock()
		defer mutexRegistryMu.Unlock()
		if m.holder == w.id {
			return nil
		}
		m.removeWaiter(w)
		delete(mutexRegistryWaiting, w.id)
		return context.Cause(ctx) //nolint:wrapcheck // We don't need to wrap.
	}
}

func (m *mutex) removeWaiter(w *mutexWaiter) {
	for i, ww := range m.waiters {
		if ww == w {
			m.waiters = slices.Delete(m.waiters, i, i+1)
			return
		}
	}
}

func (m *mutex) unlock() {
	mutexRegistryMu.Lock()
	defer mutexRegistryMu.Unlock()
	m.holder = 0
	if len(m.waiters) > 0 {
		w := m.waiters[0]
		m.waiters = slices.Delete(m.waiters, 0, 1)
		m.holder = w.id
		delete(mutexRegistryWaiting, w.id)
		close(w.done)
	}
}

type callerID uint64

type callerIDContextKey struct{}

var callerIDCounter atomic.Uint64

func getOrCreateCallerID(ctx context.Context) (context.Context, callerID) {
	id, _ := ctx.Value(callerIDContextKey{}).(callerID)
	if id == 0 {
		id = callerID(callerIDCounter.Add(1))
		ctx = context.WithValue(ctx, callerIDContextKey{}, id)
	}
	return ctx, id
}

var (
	mutexRegistryMu      sync.Mutex
	mutexRegistryWaiting = make(map[callerID]*mutex)
)

func deadlock(holder callerID, caller callerID) bool {
	cur := holder
	for {
		if cur == caller {
			return true
		}
		m2 := mutexRegistryWaiting[cur]
		if m2 == nil {
			return false
		}
		cur = m2.holder
	}
}
