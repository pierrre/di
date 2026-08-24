package di

import (
	"context"
	"testing"

	"github.com/pierrre/assert"
)

func TestProvider(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "test", nil, nil
	})
	p := ctn.Provider[string]("")
	for range 3 {
		for range 5 {
			s := p.MustGet(ctx)
			assert.Equal(t, s, "test")
		}
		p.Close()
	}
}

func TestProviderGetError(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	p := ctn.Provider[string]("")
	_, err := p.Get(ctx)
	serviceErr, _ := assert.ErrorAsType[*ServiceError](t, err)
	assert.Equal(t, serviceErr.Key, Key{Type: "string", Name: ""})
	assert.ErrorIs(t, err, ErrNotSet)
	assert.ErrorEqual(t, err, "service string: not set")
}

func TestProviderMustGetPanic(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	p := ctn.Provider[string]("")
	assert.Panics(t, func() {
		p.MustGet(ctx)
	})
}

func TestProviderGetAllocs(t *testing.T) {
	ctx := t.Context()
	ctn := new(Container)
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "test", nil, nil
	})
	p := ctn.Provider[string]("")
	assert.AllocsPerRun(t, 100, func() {
		_, _ = p.Get(ctx)
	}, 0)
}

func BenchmarkProviderGet(b *testing.B) {
	ctx := b.Context()
	ctn := new(Container)
	ctn.MustSet("", func(ctx context.Context, ctn *Container) (string, Close, error) {
		return "test", nil, nil
	})
	p := ctn.Provider[string]("")
	for b.Loop() {
		_, _ = p.Get(ctx)
	}
}
