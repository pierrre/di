package di

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
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
	ctn.MustSet("a", func(ctx context.Context, ctn *Container) (string, Close, error) {
		ctn.MustGet[string](ctx, "b")
		ctn.MustGet[string](ctx, "c")
		return "", nil, nil
	})
	ctn.MustSet("b", func(ctx context.Context, ctn *Container) (string, Close, error) {
		ctn.MustGet[string](ctx, "d")
		ctn.MustGet[string](ctx, "e")
		return "", nil, nil
	})
	ctn.MustSet("c", func(ctx context.Context, ctn *Container) (string, Close, error) {
		ctn.MustGet[string](ctx, "d")
		ctn.MustGet[string](ctx, "e")
		return "", nil, nil
	})
	ctn.MustSet("d", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	ctn.MustSet("e", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	dep, err := ctn.GetDependency[string](ctx, "a")
	if err != nil {
		panic(err)
	}
	buf := new(bytes.Buffer)
	err = json.MarshalWrite(buf, dep, jsontext.WithIndent("\t"))
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
	ctx := t.Context()
	ctn := new(Container)
	ctn.MustSet("a", func(ctx context.Context, ctn *Container) (string, Close, error) {
		ctn.MustGet[string](ctx, "b")
		ctn.MustGet[string](ctx, "c")
		return "", nil, nil
	})
	ctn.MustSet("b", func(ctx context.Context, ctn *Container) (string, Close, error) {
		ctn.MustGet[string](ctx, "d")
		ctn.MustGet[string](ctx, "e")
		return "", nil, nil
	})
	ctn.MustSet("c", func(ctx context.Context, ctn *Container) (string, Close, error) {
		ctn.MustGet[string](ctx, "d")
		ctn.MustGet[string](ctx, "e")
		return "", nil, nil
	})
	ctn.MustSet("d", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	ctn.MustSet("e", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, nil
	})
	dep, err := ctn.GetDependency[string](ctx, "a")
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
	ctx := t.Context()
	ctn := new(Container)
	_, err := ctn.GetDependency[string](ctx, "")
	assert.ErrorIs(t, err, ErrNotSet)
	serviceErr, _ := assert.ErrorAsType[*ServiceError](t, err)
	assert.Equal(t, serviceErr.Key, newKey[string](""))
	assert.ErrorEqual(t, err, "service string: not set")
}

func TestGetDependencyErrorBuilder(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "", nil, errors.New("error")
	})
	_, err := ctn.GetDependency[string](ctx, "")
	serviceErr, _ := assert.ErrorAsType[*ServiceError](t, err)
	assert.Equal(t, serviceErr.Key, newKey[string](""))
	assert.ErrorEqual(t, err, "service string: error")
}

func TestGetDependencyErrorCycle(t *testing.T) {
	ctx := t.Context()
	ctn := newTestContainerCycle()
	_, err := ctn.GetDependency[string](ctx, "a")
	assert.ErrorIs(t, err, ErrCycle)
	assert.ErrorEqual(t, err, "service string(a): service string(b): service string(c): service string(a): cycle")
}

func TestGetDependencyErrorServiceWrapperMutexContextCanceled(t *testing.T) {
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
	_, err := ctn.GetDependency[string](ctx, "")
	assert.ErrorIs(t, err, context.Canceled)
}
