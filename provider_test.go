package di

import (
	"context"
	"fmt"
	"testing"

	"github.com/pierrre/assert"
)

func ExampleProvider() {
	ctx := context.Background()
	ctn := new(Container)
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		fmt.Println("build")
		return "test", nil, nil
	})
	MustSetProvider[string](ctn, "")
	p := MustGetProvider[string](ctx, ctn, "")
	s := p.MustGet(ctx)
	fmt.Println(s)
	// Output:
	// build
	// test
}

func TestProvider(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "test", nil, nil
	})
	err := SetProvider[string](ctn, "")
	assert.NoError(t, err)
	p, err := GetProvider[string](ctx, ctn, "")
	assert.NoError(t, err)
	for range 3 {
		for range 5 {
			s := p.MustGet(ctx)
			assert.Equal(t, s, "test")
		}
		err = ctn.Close(ctx)
		assert.NoError(t, err)
	}
}

func TestMustSetProviderPanic(t *testing.T) {
	ctn := new(Container)
	MustSetProvider[string](ctn, "")
	assert.Panics(t, func() {
		MustSetProvider[string](ctn, "")
	})
}

func TestMustGetProviderPanic(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	assert.Panics(t, func() {
		MustGetProvider[string](ctx, ctn, "")
	})
}

func TestProviderGetAllocs(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "test", nil, nil
	})
	p := newProvider[string](ctn, "")
	assert.AllocsPerRun(t, 100, func() {
		p.MustGet(ctx)
	}, 0)
}

func TestProviderGetError(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	p := newProvider[string](ctn, "")
	_, err := p.Get(ctx)
	var serviceErr *ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Key, newKey[string](""))
	assert.ErrorIs(t, err, ErrNotSet)
	assert.ErrorEqual(t, err, "service \"string\": not set")
}

func TestProviderMustGetPanic(t *testing.T) {
	ctx := context.Background()
	ctn := new(Container)
	p := newProvider[string](ctn, "")
	assert.Panics(t, func() {
		p.MustGet(ctx)
	})
}

func BenchmarkProviderGet(b *testing.B) {
	ctx := context.Background()
	ctn := new(Container)
	MustSet(ctn, "", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "test", nil, nil
	})
	p := newProvider[string](ctn, "")
	b.ResetTimer()
	for range b.N {
		_, _ = p.Get(ctx)
	}
}
