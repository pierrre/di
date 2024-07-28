//nolint:forbidigo // Allow to use fmt.Println in this test.
package di

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/pierrre/assert"
	"github.com/pierrre/go-libs/goroutine"
)

func Example() {
	ctx := context.Background()
	ctn := new(Container)
	defer ctn.Close(ctx)
	Set(ctn, "", func(ctx context.Context, ctn *Container) (*serviceA, Close, error) {
		return &serviceA{}, nil, nil
	})
	Set(ctn, "", func(ctx context.Context, ctn *Container) (*serviceB, Close, error) {
		sb := &serviceB{
			sa: MustGet[*serviceA](ctx, ctn, "").DoA,
		}
		return sb, sb.close, nil
	})
	Set(ctn, "", func(ctx context.Context, ctn *Container) (*serviceC, Close, error) {
		return &serviceC{
			sb: MustGet[*serviceB](ctx, ctn, "").DoB,
		}, nil, nil
	})
	sc := MustGet[*serviceC](ctx, ctn, "")
	sc.DoC()
	// Output:
	// do A
	// do B
	// do C
	// close B
}

type serviceA struct{}

func (sa *serviceA) DoA() {
	fmt.Println("do A")
}

type serviceB struct {
	sa func()
}

func (sb *serviceB) DoB() {
	sb.sa()
	fmt.Println("do B")
}

func (sb *serviceB) close(ctx context.Context) error {
	fmt.Println("close B")
	return nil
}

type serviceC struct {
	sb func()
}

func (sc *serviceC) DoC() {
	sc.sb()
	fmt.Println("do C")
}

func Test(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	builderCallCount := 0
	Set(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		builderCallCount++
		return "test", nil, nil
	})
	sa, err := Get[string](ctx, ctn, "")
	assert.NoError(t, err)
	assert.NotZero(t, sa)
	sa, err = Get[string](ctx, ctn, "")
	assert.NoError(t, err)
	assert.NotZero(t, sa)
	assert.Equal(t, builderCallCount, 1)
}

func TestSetPanicAlreadySet(t *testing.T) {
	ctn := new(Container)
	Set(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	rec, _ := assert.Panics(t, func() {
		Set(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
			return "", nil, nil
		})
	})
	err, _ := assert.Type[error](t, rec)
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Name, "string")
	assert.ErrorIs(t, err, ErrAlreadySet)
	assert.ErrorEqual(t, err, "service \"string\": already set")
}

func TestGetErrorNotSet(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	_, err := Get[string](ctx, ctn, "")
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Name, "string")
	assert.ErrorIs(t, err, ErrNotSet)
	assert.ErrorEqual(t, err, "service \"string\": not set")
}

func TestGetErrorType(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "test", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	_, err := Get[int](ctx, ctn, "test")
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Name, "test")
	var typeErr *TypeError
	assert.ErrorAs(t, err, &typeErr)
	assert.Equal(t, typeErr.Service, reflect.TypeFor[string]())
	assert.Equal(t, typeErr.Expected, reflect.TypeFor[int]())
	assert.ErrorEqual(t, err, "service \"test\": type string does not match the expected type int")
}

func TestGetErrorBuilder(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, errors.New("error")
	})
	_, err := Get[string](ctx, ctn, "")
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Name, "string")
	assert.ErrorEqual(t, err, "service \"string\": error")
}

func TestGetErrorCycle(t *testing.T) {
	ctx := context.Background()
	ctn := newTestContainerCycle()
	_, err := Get[string](ctx, ctn, "a")
	assert.ErrorIs(t, err, ErrCycle)
	assert.ErrorEqual(t, err, "service \"a\": service \"b\": service \"c\": service \"a\": cycle")
}

func newTestContainerCycle() *Container {
	ctn := new(Container)
	Set(ctn, "a", func(ctx context.Context, ctn *Container) (string, Close, error) {
		_, err := Get[string](ctx, ctn, "b")
		if err != nil {
			return "", nil, err
		}
		return "", nil, nil
	})
	Set(ctn, "b", func(ctx context.Context, ctn *Container) (string, Close, error) {
		_, err := Get[string](ctx, ctn, "c")
		if err != nil {
			return "", nil, err
		}
		return "", nil, nil
	})
	Set(ctn, "c", func(ctx context.Context, ctn *Container) (string, Close, error) {
		_, err := Get[string](ctx, ctn, "a")
		if err != nil {
			return "", nil, err
		}
		return "", nil, nil
	})
	return ctn
}

func TestGetErrorServiceWrapperMutexContextCanceled(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	started := make(chan struct{})
	block := make(chan struct{})
	Set(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		close(started)
		<-block
		return "", nil, nil
	})
	wait := goroutine.Wait(ctx, func(ctx context.Context) {
		_, err := Get[string](ctx, ctn, "")
		assert.NoError(t, err)
	})
	defer wait()
	defer close(block)
	<-started
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	_, err := Get[string](ctx, ctn, "")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestMustGet(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "test", nil, nil
	})
	sa := MustGet[string](ctx, ctn, "")
	assert.NotZero(t, sa)
}

func TestMustGetPanic(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	assert.Panics(t, func() {
		MustGet[string](ctx, ctn, "")
	})
}

func TestGetAll(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "a", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	Set(ctn, "b", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	ss, err := GetAll[string](ctx, ctn)
	assert.NoError(t, err)
	assert.MapLen(t, ss, 2)
}

func TestGetAllError(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, errors.New("error")
	})
	_, err := GetAll[string](ctx, ctn)
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Name, "string")
	assert.ErrorEqual(t, err, "service \"string\": error")
}

func ExampleDependency() {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "a", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "b")
		MustGet[string](ctx, ctn, "c")
		return "", nil, nil
	})
	Set(ctn, "b", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "d")
		MustGet[string](ctx, ctn, "e")
		return "", nil, nil
	})
	Set(ctn, "c", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "d")
		MustGet[string](ctx, ctn, "e")
		return "", nil, nil
	})
	Set(ctn, "d", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	Set(ctn, "e", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	dep, err := GetDependency[string](ctx, ctn, "a")
	if err != nil {
		panic(err)
	}
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "\t")
	err = enc.Encode(dep)
	if err != nil {
		panic(err)
	}
	fmt.Println(buf.String())
	// Output:
	// {
	// 	"name": "a",
	// 	"type": "string",
	// 	"dependencies": [
	// 		{
	// 			"name": "b",
	// 			"type": "string",
	// 			"dependencies": [
	// 				{
	// 					"name": "d",
	// 					"type": "string"
	// 				},
	// 				{
	// 					"name": "e",
	// 					"type": "string"
	// 				}
	// 			]
	// 		},
	// 		{
	// 			"name": "c",
	// 			"type": "string",
	// 			"dependencies": [
	// 				{
	// 					"name": "d",
	// 					"type": "string"
	// 				},
	// 				{
	// 					"name": "e",
	// 					"type": "string"
	// 				}
	// 			]
	// 		}
	// 	]
	// }
}

func TestGetDependency(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "a", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "b")
		MustGet[string](ctx, ctn, "c")
		return "", nil, nil
	})
	Set(ctn, "b", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "d")
		MustGet[string](ctx, ctn, "e")
		return "", nil, nil
	})
	Set(ctn, "c", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "d")
		MustGet[string](ctx, ctn, "e")
		return "", nil, nil
	})
	Set(ctn, "d", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	Set(ctn, "e", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	dep, err := GetDependency[string](ctx, ctn, "a")
	assert.NoError(t, err)
	expected := &Dependency{
		Name: "a",
		Type: "string",
		Dependencies: []*Dependency{
			{
				Name: "b",
				Type: "string",
				Dependencies: []*Dependency{
					{
						Name: "d",
						Type: "string",
					},
					{
						Name: "e",
						Type: "string",
					},
				},
			},
			{
				Name: "c",
				Type: "string",
				Dependencies: []*Dependency{
					{
						Name: "d",
						Type: "string",
					},
					{
						Name: "e",
						Type: "string",
					},
				},
			},
		},
	}
	assert.DeepEqual(t, dep, expected)
}

func TestGetDependencyErrorNotSet(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	_, err := GetDependency[string](ctx, ctn, "")
	assert.ErrorIs(t, err, ErrNotSet)
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Name, "string")
	assert.ErrorEqual(t, err, "service \"string\": not set")
}

func TestGetDependencyErrorBuilder(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, errors.New("error")
	})
	_, err := GetDependency[string](ctx, ctn, "")
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Name, "string")
	assert.ErrorEqual(t, err, "service \"string\": error")
}

func TestGetDependencyErrorCycle(t *testing.T) {
	ctx := context.Background()
	ctn := newTestContainerCycle()
	_, err := GetDependency[string](ctx, ctn, "a")
	assert.ErrorIs(t, err, ErrCycle)
	assert.ErrorEqual(t, err, "service \"a\": service \"b\": service \"c\": service \"a\": cycle")
}

func TestGetDependencyErrorServiceWrapperMutexContextCanceled(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	started := make(chan struct{})
	block := make(chan struct{})
	Set(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		close(started)
		<-block
		return "", nil, nil
	})
	wait := goroutine.Wait(ctx, func(ctx context.Context) {
		_, err := Get[string](ctx, ctn, "")
		assert.NoError(t, err)
	})
	defer wait()
	defer close(block)
	<-started
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	_, err := GetDependency[string](ctx, ctn, "")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestClose(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	builderCalled := 0
	closeCalled := 0
	Set(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		builderCalled++
		return "", func(ctx context.Context) error {
			closeCalled++
			return nil
		}, nil
	})
	count := 5
	for i := 0; i < count; i++ {
		_, err := Get[string](ctx, ctn, "")
		assert.NoError(t, err)
		err = ctn.Close(ctx)
		assert.NoError(t, err)
	}
	assert.Equal(t, builderCalled, count)
	assert.Equal(t, closeCalled, count)
}

func TestCloseNil(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	builderCalled := 0
	Set(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		builderCalled++
		return "", nil, nil
	})
	count := 5
	for i := 0; i < count; i++ {
		_, err := Get[string](ctx, ctn, "")
		assert.NoError(t, err)
		err = ctn.Close(ctx)
		assert.NoError(t, err)
	}
	assert.Equal(t, builderCalled, count)
}

func TestCloseNotInitialized(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, errors.New("error")
	})
	_, err := Get[string](ctx, ctn, "")
	assert.Error(t, err)
	err = ctn.Close(ctx)
	assert.NoError(t, err)
}

func TestCloseError(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", func(ctx context.Context) error {
			return errors.New("error")
		}, nil
	})
	_, err := Get[string](ctx, ctn, "")
	assert.NoError(t, err)
	err = ctn.Close(ctx)
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Name, "string")
}

func TestCloseDependencyErrorServiceWrapperMutexContextCanceled(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	started := make(chan struct{})
	block := make(chan struct{})
	Set(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		close(started)
		<-block
		return "", nil, nil
	})
	wait := goroutine.Wait(ctx, func(ctx context.Context) {
		_, err := Get[string](ctx, ctn, "")
		assert.NoError(t, err)
	})
	defer wait()
	defer close(block)
	<-started
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	err := ctn.Close(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestMust(t *testing.T) {
	Must("", nil)
}

func TestMustPanic(t *testing.T) {
	assert.Panics(t, func() {
		Must("", errors.New("error"))
	})
}
