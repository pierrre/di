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

func ExampleDependency() {
	ctx := context.Background()
	ctn := new(Container)
	MustSet(ctn, "a", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "b")
		MustGet[string](ctx, ctn, "c")
		return "", nil, nil
	})
	MustSet(ctn, "b", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "d")
		MustGet[string](ctx, ctn, "e")
		return "", nil, nil
	})
	MustSet(ctn, "c", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "d")
		MustGet[string](ctx, ctn, "e")
		return "", nil, nil
	})
	MustSet(ctn, "d", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	MustSet(ctn, "e", func(ctx context.Context, ctn *Container) (string, Close, error) {
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
	// 	"type": "string",
	// 	"name": "a",
	// 	"dependencies": [
	// 		{
	// 			"type": "string",
	// 			"name": "b",
	// 			"dependencies": [
	// 				{
	// 					"type": "string",
	// 					"name": "d"
	// 				},
	// 				{
	// 					"type": "string",
	// 					"name": "e"
	// 				}
	// 			]
	// 		},
	// 		{
	// 			"type": "string",
	// 			"name": "c",
	// 			"dependencies": [
	// 				{
	// 					"type": "string",
	// 					"name": "d"
	// 				},
	// 				{
	// 					"type": "string",
	// 					"name": "e"
	// 				}
	// 			]
	// 		}
	// 	]
	// }
}

func TestGetDependency(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	MustSet(ctn, "a", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "b")
		MustGet[string](ctx, ctn, "c")
		return "", nil, nil
	})
	MustSet(ctn, "b", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "d")
		MustGet[string](ctx, ctn, "e")
		return "", nil, nil
	})
	MustSet(ctn, "c", func(ctx context.Context, ctn *Container) (string, Close, error) {
		MustGet[string](ctx, ctn, "d")
		MustGet[string](ctx, ctn, "e")
		return "", nil, nil
	})
	MustSet(ctn, "d", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	MustSet(ctn, "e", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	dep, err := GetDependency[string](ctx, ctn, "a")
	assert.NoError(t, err)
	assert.NotZero(t, dep.GetReflectType())
	expected := &Dependency{
		Type:        "string",
		reflectType: reflect.TypeFor[string](),
		Name:        "a",
		Dependencies: []*Dependency{
			{
				Type:        "string",
				Name:        "b",
				reflectType: reflect.TypeFor[string](),
				Dependencies: []*Dependency{
					{
						Type:        "string",
						reflectType: reflect.TypeFor[string](),
						Name:        "d",
					},
					{
						Type:        "string",
						reflectType: reflect.TypeFor[string](),
						Name:        "e",
					},
				},
			},
			{
				Type:        "string",
				reflectType: reflect.TypeFor[string](),
				Name:        "c",
				Dependencies: []*Dependency{
					{
						Type:        "string",
						reflectType: reflect.TypeFor[string](),
						Name:        "d",
					},
					{
						Type:        "string",
						reflectType: reflect.TypeFor[string](),
						Name:        "e",
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
	assert.Equal(t, serviceErr.Key, newKey[string](""))
	assert.ErrorEqual(t, err, "service \"string\": not set")
}

func TestGetDependencyErrorBuilder(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, errors.New("error")
	})
	_, err := GetDependency[string](ctx, ctn, "")
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Key, newKey[string](""))
	assert.ErrorEqual(t, err, "service \"string\": error")
}

func TestGetDependencyErrorCycle(t *testing.T) {
	ctx := context.Background()
	ctn := newTestContainerCycle()
	_, err := GetDependency[string](ctx, ctn, "a")
	assert.ErrorIs(t, err, ErrCycle)
	assert.ErrorEqual(t, err, "service \"string(a)\": service \"string(b)\": service \"string(c)\": service \"string(a)\": cycle")
}

func TestGetDependencyErrorServiceWrapperMutexContextCanceled(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	started := make(chan struct{})
	block := make(chan struct{})
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		close(started)
		<-block
		return "", nil, nil
	})
	wait := goroutine.Wait(ctx, func(ctx context.Context) {
		MustGet[string](ctx, ctn, "")
	})
	defer wait()
	defer close(block)
	<-started
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	_, err := GetDependency[string](ctx, ctn, "")
	assert.ErrorIs(t, err, context.Canceled)
}
