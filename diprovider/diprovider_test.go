package diprovider

import (
	"context"
	"fmt"
	"testing"

	"github.com/pierrre/assert"
	"github.com/pierrre/di"
)

func Example() {
	ctx := context.Background()
	ctn := new(di.Container)
	di.MustSet(ctn, "", func(ctx context.Context, ctn *di.Container) (string, di.Close, error) {
		fmt.Println("build")
		return "test", nil, nil
	})
	MustSet[string](ctn, "")
	p := MustGet[string](ctx, ctn, "")
	s := p.MustGet(ctx)
	fmt.Println(s)
	// Output:
	// build
	// test
}

func Test(t *testing.T) {
	ctx := t.Context()
	ctn := new(di.Container)
	di.MustSet(ctn, "", func(ctx context.Context, ctn *di.Container) (string, di.Close, error) {
		return "test", nil, nil
	})
	err := Set[string](ctn, "")
	assert.NoError(t, err)
	p, err := Get[string](ctx, ctn, "")
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

func TestMustSetPanic(t *testing.T) {
	ctn := new(di.Container)
	MustSet[string](ctn, "")
	assert.Panics(t, func() {
		MustSet[string](ctn, "")
	})
}

func TestMustGetPanic(t *testing.T) {
	ctx := t.Context()
	ctn := new(di.Container)
	assert.Panics(t, func() {
		MustGet[string](ctx, ctn, "")
	})
}

func TestProviderGetAllocs(t *testing.T) {
	ctx := t.Context()
	ctn := new(di.Container)
	di.MustSet(ctn, "", func(ctx context.Context, ctn *di.Container) (string, di.Close, error) {
		return "test", nil, nil
	})
	p := newProvider[string](ctn, "")
	assert.AllocsPerRun(t, 100, func() {
		_, _ = p.Get(ctx)
	}, 0)
}

func TestProviderGetError(t *testing.T) {
	ctx := t.Context()
	ctn := new(di.Container)
	p := newProvider[string](ctn, "")
	_, err := p.Get(ctx)
	var serviceErr *di.ServiceError
	assert.ErrorAs(t, err, &serviceErr)
	assert.Equal(t, serviceErr.Key, di.Key{Type: "string", Name: ""})
	assert.ErrorIs(t, err, di.ErrNotSet)
	assert.ErrorEqual(t, err, "service string: not set")
}

func TestProviderMustGetPanic(t *testing.T) {
	ctx := t.Context()
	ctn := new(di.Container)
	p := newProvider[string](ctn, "")
	assert.Panics(t, func() {
		p.MustGet(ctx)
	})
}

func BenchmarkProviderGet(b *testing.B) {
	ctx := b.Context()
	ctn := new(di.Container)
	di.MustSet(ctn, "", func(ctx context.Context, ctn *di.Container) (string, di.Close, error) {
		return "test", nil, nil
	})
	p := newProvider[string](ctn, "")
	for b.Loop() {
		_, _ = p.Get(ctx)
	}
}
