package di

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pierrre/assert"
	"github.com/pierrre/go-libs/goroutine"
)

func TestContainer(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	builderCallCount := 0
	err := ctn.Set("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		builderCallCount++
		return "test", nil, nil
	})
	assert.NoError(t, err)
	sa, err := ctn.Get[string](ctx, "")
	assert.NoError(t, err)
	assert.NotZero(t, sa)
	sa, err = ctn.Get[string](ctx, "")
	assert.NoError(t, err)
	assert.NotZero(t, sa)
	assert.Equal(t, builderCallCount, 1)
}

func TestContainerSetErrorAlreadySet(t *testing.T) {
	ctn := new(Container)
	err := ctn.Set("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	assert.NoError(t, err)
	err = ctn.Set("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	serviceErr, _ := assert.ErrorAsType[*ServiceError](t, err)
	assert.Equal(t, serviceErr.Key, newKey[string](""))
	assert.ErrorIs(t, err, ErrAlreadySet)
	assert.ErrorEqual(t, err, "service string: already set")
}

func TestContainerSetErrorNilBuilder(t *testing.T) {
	ctn := new(Container)
	err := ctn.Set[string]("", nil)
	serviceErr, _ := assert.ErrorAsType[*ServiceError](t, err)
	assert.Equal(t, serviceErr.Key, newKey[string](""))
	assert.ErrorIs(t, err, ErrNilBuilder)
	assert.ErrorEqual(t, err, "service string: nil builder")
}

func TestContainerMustSetPanicAlreadySet(t *testing.T) {
	ctn := new(Container)
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	assert.Panics(t, func() {
		ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
			return "", nil, nil
		})
	})
}

func TestContainerGetErrorNotSet(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	_, err := ctn.Get[string](ctx, "")
	serviceErr, _ := assert.ErrorAsType[*ServiceError](t, err)
	assert.Equal(t, serviceErr.Key, newKey[string](""))
	assert.ErrorIs(t, err, ErrNotSet)
	assert.ErrorEqual(t, err, "service string: not set")
}

func TestContainerGetErrorBuilder(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, errors.New("error")
	})
	_, err := ctn.Get[string](ctx, "")
	serviceErr, _ := assert.ErrorAsType[*ServiceError](t, err)
	assert.Equal(t, serviceErr.Key, newKey[string](""))
	assert.ErrorEqual(t, err, "service string: error")
}

func TestContainerGetErrorPanic(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	e := errors.New("error")
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		panic(e)
	})
	_, err := ctn.Get[string](ctx, "")
	serviceErr, _ := assert.ErrorAsType[*ServiceError](t, err)
	assert.Equal(t, serviceErr.Key, newKey[string](""))
	panicErr, _ := assert.ErrorAsType[*PanicError](t, err)
	assert.Equal(t, panicErr.Recovered, any(e))
	assert.ErrorIs(t, err, e)
	assert.ErrorEqual(t, err, "service string: panic: error")
}

func TestContainerGetErrorPanicChain(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	ctn.MustSet("a", func(ctx context.Context, ctn *Container) (string, Close, error) {
		ctn.MustGet[string](ctx, "b")
		return "", nil, nil
	})
	ctn.MustSet("b", func(ctx context.Context, ctn *Container) (string, Close, error) {
		ctn.MustGet[string](ctx, "c")
		return "", nil, nil
	})
	ctn.MustSet("c", func(ctx context.Context, ctn *Container) (string, Close, error) {
		panic("test")
	})
	_, err := ctn.Get[string](ctx, "a")
	assert.ErrorEqual(t, err, "service string(a): panic: service string(b): panic: service string(c): panic: test")
}

func TestContainerGetErrorCycle(t *testing.T) {
	ctx := t.Context()
	ctn := newTestContainerCycle()
	_, err := ctn.Get[string](ctx, "a")
	assert.ErrorIs(t, err, ErrCycle)
	assert.ErrorEqual(t, err, "service string(a): service string(b): service string(c): service string(a): cycle")
}

func newTestContainerCycle() *Container {
	ctn := new(Container)
	ctn.MustSet("a", func(ctx context.Context, ctn *Container) (string, Close, error) {
		_, err := ctn.Get[string](ctx, "b")
		if err != nil {
			return "", nil, err
		}
		return "", nil, nil
	})
	ctn.MustSet("b", func(ctx context.Context, ctn *Container) (string, Close, error) {
		_, err := ctn.Get[string](ctx, "c")
		if err != nil {
			return "", nil, err
		}
		return "", nil, nil
	})
	ctn.MustSet("c", func(ctx context.Context, ctn *Container) (string, Close, error) {
		_, err := ctn.Get[string](ctx, "a")
		if err != nil {
			return "", nil, err
		}
		return "", nil, nil
	})
	return ctn
}

func TestContainerGetErrorDeadlock(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	ctn.MustSet("a", func(ctx context.Context, ctn *Container) (string, Close, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		_, err := ctn.Get[string](ctx, "b")
		if err != nil {
			return "", nil, err
		}
		return "", nil, nil
	})
	ctn.MustSet("b", func(ctx context.Context, ctn *Container) (string, Close, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		_, err := ctn.Get[string](ctx, "a")
		if err != nil {
			return "", nil, err
		}
		return "", nil, nil
	})
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var errs [2]error
	waiter := goroutine.StartN(ctx, 2, func(ctx context.Context, i int) {
		name := "a"
		if i == 1 {
			name = "b"
		}
		_, err := ctn.Get[string](ctx, name)
		errs[i] = err
	})
	<-started
	<-started
	close(release)
	waiter.Wait()
	assert.ErrorIs(t, errs[0], ErrCycle)
	assert.ErrorIs(t, errs[1], ErrCycle)
}

func TestContainerGetErrorDeadlock3(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	ctn.MustSet("a", func(ctx context.Context, ctn *Container) (string, Close, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		_, err := ctn.Get[string](ctx, "b")
		if err != nil {
			return "", nil, err
		}
		return "", nil, nil
	})
	ctn.MustSet("b", func(ctx context.Context, ctn *Container) (string, Close, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		_, err := ctn.Get[string](ctx, "c")
		if err != nil {
			return "", nil, err
		}
		return "", nil, nil
	})
	ctn.MustSet("c", func(ctx context.Context, ctn *Container) (string, Close, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		_, err := ctn.Get[string](ctx, "a")
		if err != nil {
			return "", nil, err
		}
		return "", nil, nil
	})
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var errs [3]error
	names := []string{"a", "b", "c"}
	waiter := goroutine.StartN(ctx, 3, func(ctx context.Context, i int) {
		_, err := ctn.Get[string](ctx, names[i])
		errs[i] = err
	})
	for range 3 {
		<-started
	}
	close(release)
	waiter.Wait()
	for i := range 3 {
		assert.ErrorIs(t, errs[i], ErrCycle)
	}
}

func TestContainerGetConcurrentDependency(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	ctn.MustSet("a", func(ctx context.Context, ctn *Container) (string, Close, error) {
		_, err := ctn.Get[string](ctx, "b")
		if err != nil {
			return "", nil, err
		}
		return "a", nil, nil
	})
	ctn.MustSet("b", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "b", nil, nil
	})
	var errs [2]error
	waiter := goroutine.StartN(ctx, 2, func(ctx context.Context, i int) {
		name := "a"
		if i == 1 {
			name = "b"
		}
		_, err := ctn.Get[string](ctx, name)
		errs[i] = err
	})
	waiter.Wait()
	assert.NoError(t, errs[0])
	assert.NoError(t, errs[1])
}

func TestContainerGetErrorServiceWrapperMutexContextCanceled(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	started := make(chan struct{})
	block := make(chan struct{})
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		close(started)
		<-block
		return "", nil, nil
	})
	defer goroutine.Start(ctx, func(ctx context.Context) {
		ctn.MustGet[string](ctx, "")
	}).Wait()
	defer close(block)
	<-started
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	_, err := ctn.Get[string](ctx, "")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestContainerMustGet(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "test", nil, nil
	})
	sa := ctn.MustGet[string](ctx, "")
	assert.NotZero(t, sa)
}

func TestContainerMustGetPanic(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	assert.Panics(t, func() {
		ctn.MustGet[string](ctx, "")
	})
}

func TestContainerGetConcurrent(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	var builderCallCount atomic.Int32
	started := make(chan struct{})
	block := make(chan struct{})
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		builderCallCount.Add(1)
		close(started)
		<-block
		return "test", nil, nil
	})
	const workers = 100
	waiter := goroutine.StartN(ctx, workers, func(ctx context.Context, i int) {
		s, err := ctn.Get[string](ctx, "")
		assert.NoError(t, err)
		assert.Equal(t, s, "test")
	})
	<-started
	close(block)
	waiter.Wait()
	assert.Equal(t, builderCallCount.Load(), int32(1))
}

func TestContainerGetAllocs(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "test", nil, nil
	})
	_, _ = ctn.Get[string](ctx, "")
	assert.AllocsPerRun(t, 100, func() {
		_, _ = ctn.Get[string](ctx, "")
	}, 0)
}

func BenchmarkContainerGet(b *testing.B) {
	ctx := b.Context()
	ctn := new(Container)
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	for b.Loop() {
		_, _ = ctn.Get[string](ctx, "")
	}
}

func BenchmarkContainerGetParallel(b *testing.B) {
	ctx := b.Context()
	ctn := new(Container)
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			_, _ = ctn.Get[string](ctx, "")
		}
	})
}

func TestContainerGetAll(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	ctn.MustSet("a", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	ctn.MustSet("b", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	ss, err := ctn.GetAll[string](ctx)
	assert.NoError(t, err)
	assert.MapLen(t, ss, 2)
}

func TestContainerGetAllError(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, errors.New("error")
	})
	_, err := ctn.GetAll[string](ctx)
	serviceErr, _ := assert.ErrorAsType[*ServiceError](t, err)
	assert.Equal(t, serviceErr.Key, newKey[string](""))
	assert.ErrorEqual(t, err, "service string: error")
}

func TestContainerClose(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	builderCalled := 0
	closeCalled := 0
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		builderCalled++
		return "", func(ctx context.Context) error {
			closeCalled++
			return nil
		}, nil
	})
	count := 5
	for range count {
		_, err := ctn.Get[string](ctx, "")
		assert.NoError(t, err)
		err = ctn.Close(ctx)
		assert.NoError(t, err)
	}
	assert.Equal(t, builderCalled, count)
	assert.Equal(t, closeCalled, count)
}

func TestContainerCloseOrder(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	count := 5
	var closeCalls []int
	for i := range count {
		name := fmt.Sprintf("%05d", i)
		ctn.MustSet(name, func(ctx context.Context, ctn *Container) (string, Close, error) {
			return "", func(ctx context.Context) error {
				closeCalls = append(closeCalls, i)
				return nil
			}, nil
		})
		ctn.MustGet[string](ctx, name)
	}
	err := ctn.Close(ctx)
	assert.NoError(t, err)
	assert.DeepEqual(t, closeCalls, []int{0, 1, 2, 3, 4})
}

func TestContainerCloseNil(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	builderCalled := 0
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		builderCalled++
		return "", nil, nil
	})
	count := 5
	for range count {
		_, err := ctn.Get[string](ctx, "")
		assert.NoError(t, err)
		err = ctn.Close(ctx)
		assert.NoError(t, err)
	}
	assert.Equal(t, builderCalled, count)
}

func TestContainerCloseNotInitialized(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, errors.New("error")
	})
	_, err := ctn.Get[string](ctx, "")
	assert.Error(t, err)
	err = ctn.Close(ctx)
	assert.NoError(t, err)
}

func TestContainerCloseError(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", func(ctx context.Context) error {
			return errors.New("error")
		}, nil
	})
	_, err := ctn.Get[string](ctx, "")
	assert.NoError(t, err)
	err = ctn.Close(ctx)
	serviceErr, _ := assert.ErrorAsType[*ServiceError](t, err)
	assert.Equal(t, serviceErr.Key, newKey[string](""))
}

func TestContainerCloseErrorServiceWrapperMutexContextCanceled(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	started := make(chan struct{})
	block := make(chan struct{})
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		close(started)
		<-block
		return "", nil, nil
	})
	defer goroutine.Start(ctx, func(ctx context.Context) {
		ctn.MustGet[string](ctx, "")
	}).Wait()
	defer close(block)
	<-started
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	err := ctn.Close(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}
