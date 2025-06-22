package di

import (
	"context"
	"errors"
	"testing"

	"github.com/pierrre/assert"
	"github.com/pierrre/go-libs/goroutine"
)

func Test(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	builderCallCount := 0
	err := Set(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		builderCallCount++
		return "test", nil, nil
	})
	assert.NoError(t, err)
	sa, err := Get[string](ctx, ctn, "")
	assert.NoError(t, err)
	assert.NotZero(t, sa)
	sa, err = Get[string](ctx, ctn, "")
	assert.NoError(t, err)
	assert.NotZero(t, sa)
	assert.Equal(t, builderCallCount, 1)
}

func TestSetErrorAlreadySet(t *testing.T) {
	ctn := new(Container)
	err := Set(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	assert.NoError(t, err)
	err = Set(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Key, newKey[string](""))
	assert.ErrorIs(t, err, ErrAlreadySet)
	assert.ErrorEqual(t, err, "service string: already set")
}

func TestMustSetPanicAlreadySet(t *testing.T) {
	ctn := new(Container)
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	assert.Panics(t, func() {
		MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
			return "", nil, nil
		})
	})
}

func TestGetErrorNotSet(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	_, err := Get[string](ctx, ctn, "")
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Key, newKey[string](""))
	assert.ErrorIs(t, err, ErrNotSet)
	assert.ErrorEqual(t, err, "service string: not set")
}

func TestGetErrorBuilder(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, errors.New("error")
	})
	_, err := Get[string](ctx, ctn, "")
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Key, newKey[string](""))
	assert.ErrorEqual(t, err, "service string: error")
}

func TestGetErrorPanic(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	e := errors.New("error")
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		panic(e)
	})
	_, err := Get[string](ctx, ctn, "")
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Key, newKey[string](""))
	var panicErr *PanicError
	assert.ErrorAs(t, err, &panicErr)
	assert.Equal(t, panicErr.Recovered, any(e))
	assert.ErrorIs(t, err, e)
	assert.ErrorEqual(t, err, "service string: panic: error")
}

func TestGetErrorPanicChain(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	MustSet(ctn, "a", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "b")
		return "", nil, nil
	})
	MustSet(ctn, "b", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "c")
		return "", nil, nil
	})
	MustSet(ctn, "c", func(ctx context.Context, ctn *Container) (string, Close, error) {
		panic("test")
	})
	_, err := Get[string](ctx, ctn, "a")
	assert.ErrorEqual(t, err, "service string(a): panic: service string(b): panic: service string(c): panic: test")
}

func TestGetErrorCycle(t *testing.T) {
	ctx := t.Context()
	ctn := newTestContainerCycle()
	_, err := Get[string](ctx, ctn, "a")
	assert.ErrorIs(t, err, ErrCycle)
	assert.ErrorEqual(t, err, "service string(a): service string(b): service string(c): service string(a): cycle")
}

func newTestContainerCycle() *Container {
	ctn := new(Container)
	MustSet(ctn, "a", func(ctx context.Context, ctn *Container) (string, Close, error) {
		_, err := Get[string](ctx, ctn, "b")
		if err != nil {
			return "", nil, err
		}
		return "", nil, nil
	})
	MustSet(ctn, "b", func(ctx context.Context, ctn *Container) (string, Close, error) {
		_, err := Get[string](ctx, ctn, "c")
		if err != nil {
			return "", nil, err
		}
		return "", nil, nil
	})
	MustSet(ctn, "c", func(ctx context.Context, ctn *Container) (string, Close, error) {
		_, err := Get[string](ctx, ctn, "a")
		if err != nil {
			return "", nil, err
		}
		return "", nil, nil
	})
	return ctn
}

func TestGetErrorServiceWrapperMutexContextCanceled(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	started := make(chan struct{})
	block := make(chan struct{})
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		close(started)
		<-block
		return "", nil, nil
	})
	defer goroutine.Start(ctx, func(ctx context.Context) {
		MustGet[string](ctx, ctn, "")
	}).Wait()
	defer close(block)
	<-started
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	_, err := Get[string](ctx, ctn, "")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestMustGet(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "test", nil, nil
	})
	sa := MustGet[string](ctx, ctn, "")
	assert.NotZero(t, sa)
}

func TestMustGetPanic(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	assert.Panics(t, func() {
		MustGet[string](ctx, ctn, "")
	})
}

func BenchmarkGet(b *testing.B) {
	ctx := b.Context()
	ctn := new(Container)
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	for b.Loop() {
		_, _ = Get[string](ctx, ctn, "")
	}
}

func TestGetAll(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	MustSet(ctn, "a", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	MustSet(ctn, "b", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	ss, err := GetAll[string](ctx, ctn)
	assert.NoError(t, err)
	assert.MapLen(t, ss, 2)
}

func TestGetAllError(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, errors.New("error")
	})
	_, err := GetAll[string](ctx, ctn)
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Key, newKey[string](""))
	assert.ErrorEqual(t, err, "service string: error")
}
