package di

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/pierrre/assert"
	"github.com/pierrre/go-libs/goroutine"
)

func TestContainerClose(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	builderCalled := 0
	closeCalled := 0
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		builderCalled++
		return "", func(ctx context.Context) error {
			closeCalled++
			return nil
		}, nil
	})
	count := 5
	for range count {
		_, err := Get[string](ctx, ctn, "")
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
		MustSet(ctn, name, func(ctx context.Context, ctn *Container) (string, Close, error) {
			return "", func(ctx context.Context) error {
				closeCalls = append(closeCalls, i)
				return nil
			}, nil
		})
		MustGet[string](ctx, ctn, name)
	}
	err := ctn.Close(ctx)
	assert.NoError(t, err)
	assert.DeepEqual(t, closeCalls, []int{0, 1, 2, 3, 4})
}

func TestContainerCloseNil(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	builderCalled := 0
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		builderCalled++
		return "", nil, nil
	})
	count := 5
	for range count {
		_, err := Get[string](ctx, ctn, "")
		assert.NoError(t, err)
		err = ctn.Close(ctx)
		assert.NoError(t, err)
	}
	assert.Equal(t, builderCalled, count)
}

func TestContainerCloseNotInitialized(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, errors.New("error")
	})
	_, err := Get[string](ctx, ctn, "")
	assert.Error(t, err)
	err = ctn.Close(ctx)
	assert.NoError(t, err)
}

func TestContainerCloseError(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", func(ctx context.Context) error {
			return errors.New("error")
		}, nil
	})
	_, err := Get[string](ctx, ctn, "")
	assert.NoError(t, err)
	err = ctn.Close(ctx)
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Key, newKey[string](""))
}

func TestContainerCloseErrorServiceWrapperMutexContextCanceled(t *testing.T) {
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
	err := ctn.Close(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}
