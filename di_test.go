//nolint:forbidigo // Allow to use fmt.Println in this test.
package di

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/pierrre/assert"
)

func Example() {
	ctx := context.Background()

	// New container.
	ctn := new(Container)

	// Set ServiceA.
	Set(ctn, "", func(ctx context.Context, ctn *Container) (*serviceA, Close, error) {
		return &serviceA{}, nil, nil
	})

	// Set ServiceB.
	somethingWrong := false
	Set(ctn, "", func(ctx context.Context, ctn *Container) (*serviceB, Close, error) {
		// We know that ServiceA's builder doesn't return an error, so we ignore it.
		sa := MustGet[*serviceA](ctx, ctn, "")
		if somethingWrong {
			return nil, nil, errors.New("error")
		}
		sb := &serviceB{
			sa.DoA,
		}
		cl := func(ctx context.Context) error {
			return sb.close()
		}
		return sb, cl, nil
	})

	// Set ServiceC.
	Set(ctn, "", func(ctx context.Context, ctn *Container) (*serviceC, Close, error) {
		sb, err := Get[*serviceB](ctx, ctn, "")
		if err != nil {
			return nil, nil, err
		}
		sc := &serviceC{
			sb.DoB,
		}
		// The ServiceC close function doesn't return an error, so we wrap it.
		cl := func(ctx context.Context) error {
			sc.close()
			return nil
		}
		return sc, cl, nil
	})

	// Get ServiceC and call it.
	sc, err := Get[*serviceC](ctx, ctn, "")
	if err != nil {
		panic(err)
	}
	sc.DoC()

	// Close container.
	ctn.Close(ctx, func(ctx context.Context, err error) {
		panic(err)
	})

	// Output:
	// do A
	// do B
	// do C
	// close B
	// close C
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

func (sb *serviceB) close() error {
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

func (sc *serviceC) close() {
	fmt.Println("close C")
}

func Test(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	builderCallCount := 0
	Set(ctn, "", func(ctx context.Context, ctn *Container) (*serviceA, Close, error) {
		builderCallCount++
		return &serviceA{}, nil, nil
	})
	sa, err := Get[*serviceA](ctx, ctn, "")
	assert.NoError(t, err)
	assert.NotZero(t, sa)
	sa, err = Get[*serviceA](ctx, ctn, "")
	assert.NoError(t, err)
	assert.NotZero(t, sa)
	assert.Equal(t, builderCallCount, 1)
}

func TestSetPanicAlreadySet(t *testing.T) {
	ctn := new(Container)
	Set(ctn, "", func(ctx context.Context, ctn *Container) (*serviceA, Close, error) {
		return &serviceA{}, nil, nil
	})
	rec, _ := assert.Panics(t, func() {
		Set(ctn, "", func(ctx context.Context, ctn *Container) (*serviceA, Close, error) {
			return &serviceA{}, nil, nil
		})
	})
	err, _ := assert.Type[error](t, rec)
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Name, "*github.com/pierrre/di.serviceA")
	assert.ErrorIs(t, err, ErrAlreadySet)
	assert.ErrorEqual(t, err, "service \"*github.com/pierrre/di.serviceA\": already set")
}

func TestGetErrorNotSet(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	_, err := Get[*serviceA](ctx, ctn, "")
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Name, "*github.com/pierrre/di.serviceA")
	assert.ErrorIs(t, err, ErrNotSet)
	assert.ErrorEqual(t, err, "service \"*github.com/pierrre/di.serviceA\": not set")
}

func TestGetErrorType(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "test", func(ctx context.Context, ctn *Container) (*serviceA, Close, error) {
		return &serviceA{}, nil, nil
	})
	_, err := Get[*serviceB](ctx, ctn, "test")
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Name, "test")
	var typeErr *TypeError
	assert.ErrorAs(t, err, &typeErr)
	assert.Equal(t, typeErr.Type, "*github.com/pierrre/di.serviceB")
	assert.ErrorEqual(t, err, "service \"test\": type *github.com/pierrre/di.serviceB does not match")
}

func TestGetErrorBuilder(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "", func(ctx context.Context, ctn *Container) (*serviceA, Close, error) {
		return nil, nil, errors.New("error")
	})
	_, err := Get[*serviceA](ctx, ctn, "")
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Name, "*github.com/pierrre/di.serviceA")
	assert.ErrorEqual(t, err, "service \"*github.com/pierrre/di.serviceA\": error")
}

func TestMustGet(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "", func(ctx context.Context, ctn *Container) (*serviceA, Close, error) {
		return &serviceA{}, nil, nil
	})
	sa := MustGet[*serviceA](ctx, ctn, "")
	assert.NotZero(t, sa)
}

func TestMustGetPanic(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	assert.Panics(t, func() {
		MustGet[*serviceA](ctx, ctn, "")
	})
}

func TestGetAll(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "1", func(ctx context.Context, ctn *Container) (*serviceA, Close, error) {
		return &serviceA{}, nil, nil
	})
	Set(ctn, "2", func(ctx context.Context, ctn *Container) (*serviceA, Close, error) {
		return &serviceA{}, nil, nil
	})
	ss, err := GetAll[*serviceA](ctx, ctn)
	assert.NoError(t, err)
	assert.MapLen(t, ss, 2)
}

func TestGetAllError(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "", func(ctx context.Context, ctn *Container) (*serviceA, Close, error) {
		return nil, nil, errors.New("error")
	})
	_, err := GetAll[*serviceA](ctx, ctn)
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Name, "*github.com/pierrre/di.serviceA")
	assert.ErrorEqual(t, err, "service \"*github.com/pierrre/di.serviceA\": error")
}

func ExampleDependency() {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "1", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "2")
		MustGet[string](ctx, ctn, "3")
		return "", nil, nil
	})
	Set(ctn, "2", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "4")
		MustGet[string](ctx, ctn, "5")
		return "", nil, nil
	})
	Set(ctn, "3", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "4")
		MustGet[string](ctx, ctn, "5")
		return "", nil, nil
	})
	Set(ctn, "4", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	Set(ctn, "5", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	dep, err := GetDependency[string](ctx, ctn, "1")
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
	// 	"name": "1",
	// 	"type": "string",
	// 	"dependencies": [
	// 		{
	// 			"name": "2",
	// 			"type": "string",
	// 			"dependencies": [
	// 				{
	// 					"name": "4",
	// 					"type": "string"
	// 				},
	// 				{
	// 					"name": "5",
	// 					"type": "string"
	// 				}
	// 			]
	// 		},
	// 		{
	// 			"name": "3",
	// 			"type": "string",
	// 			"dependencies": [
	// 				{
	// 					"name": "4",
	// 					"type": "string"
	// 				},
	// 				{
	// 					"name": "5",
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
	Set(ctn, "1", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "2")
		MustGet[string](ctx, ctn, "3")
		return "", nil, nil
	})
	Set(ctn, "2", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "4")
		MustGet[string](ctx, ctn, "5")
		return "", nil, nil
	})
	Set(ctn, "3", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "4")
		MustGet[string](ctx, ctn, "5")
		return "", nil, nil
	})
	Set(ctn, "4", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	Set(ctn, "5", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	dep, err := GetDependency[string](ctx, ctn, "1")
	assert.NoError(t, err)
	expected := &Dependency{
		Name: "1",
		Type: "string",
		Dependencies: []*Dependency{
			{
				Name: "2",
				Type: "string",
				Dependencies: []*Dependency{
					{
						Name: "4",
						Type: "string",
					},
					{
						Name: "5",
						Type: "string",
					},
				},
			},
			{
				Name: "3",
				Type: "string",
				Dependencies: []*Dependency{
					{
						Name: "4",
						Type: "string",
					},
					{
						Name: "5",
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

func TestClose(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	builderCalled := 0
	closeCalled := 0
	Set(ctn, "", func(ctx context.Context, ctn *Container) (*serviceA, Close, error) {
		builderCalled++
		return &serviceA{}, func(ctx context.Context) error {
			closeCalled++
			return nil
		}, nil
	})
	count := 5
	for i := 0; i < count; i++ {
		_, err := Get[*serviceA](ctx, ctn, "")
		assert.NoError(t, err)
		ctn.Close(ctx, func(ctx context.Context, err error) {
			assert.NoError(t, err)
		})
	}
	assert.Equal(t, builderCalled, count)
	assert.Equal(t, closeCalled, count)
}

func TestCloseNil(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	builderCalled := 0
	Set(ctn, "", func(ctx context.Context, ctn *Container) (*serviceA, Close, error) {
		builderCalled++
		return &serviceA{}, nil, nil
	})
	count := 5
	for i := 0; i < count; i++ {
		_, err := Get[*serviceA](ctx, ctn, "")
		assert.NoError(t, err)
		ctn.Close(ctx, func(ctx context.Context, err error) {
			assert.NoError(t, err)
		})
	}
	assert.Equal(t, builderCalled, count)
}

func TestCloseNotInitialized(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "", func(ctx context.Context, ctn *Container) (*serviceA, Close, error) {
		return nil, nil, errors.New("error")
	})
	_, err := Get[*serviceA](ctx, ctn, "")
	assert.Error(t, err)
	ctn.Close(ctx, func(ctx context.Context, err error) {
		assert.NoError(t, err)
	})
}

func TestCloseError(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	Set(ctn, "", func(ctx context.Context, ctn *Container) (*serviceA, Close, error) {
		return &serviceA{}, func(ctx context.Context) error {
			return errors.New("error")
		}, nil
	})
	_, err := Get[*serviceA](ctx, ctn, "")
	assert.NoError(t, err)
	ctn.Close(ctx, func(ctx context.Context, err error) {
		var serviceErr *ServiceError
		assert.ErrorAs(t, err, &serviceErr)
		assert.Equal(t, serviceErr.Name, "*github.com/pierrre/di.serviceA")
	})
}

func TestMust(t *testing.T) {
	Must("", nil)
}

func TestMustPanic(t *testing.T) {
	assert.Panics(t, func() {
		Must("", errors.New("error"))
	})
}
