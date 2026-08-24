package di

import (
	"context"
	"sync/atomic"
)

// Provider creates a [Provider] for the given service type and name.
//
// The returned [Provider] should be closed with [Provider.Close].
func (c *Container) Provider[S any](name string) *Provider[S] {
	return &Provider[S]{
		ctn:  c,
		name: name,
	}
}

// Provider provides a service.
//
// It can be used to break circular dependencies.
// It caches the service after the first call to [Provider.Get], so it's faster to call [Provider.Get] than [Container.Get].
// It should be closed with [Provider.Close].
type Provider[S any] struct {
	ctn     *Container
	name    string
	service atomic.Pointer[S]
}

// Get returns the service.
func (p *Provider[S]) Get(ctx context.Context) (S, error) {
	ps := p.service.Load()
	if ps != nil {
		return *ps, nil
	}
	s, err := p.ctn.Get[S](ctx, p.name)
	if err != nil {
		return s, err
	}
	p.service.Store(&s)
	return s, nil
}

// MustGet calls [Provider.Get] and panics if there is an error.
func (p *Provider[S]) MustGet(ctx context.Context) S {
	s, err := p.Get(ctx)
	if err != nil {
		panic(err)
	}
	return s
}

// Close closes the [Provider].
//
// It clears the cached service.
// However, it does not close the service.
//
// The [Provider] can be used again after being closed.
func (p *Provider[S]) Close() {
	p.service.Store(nil)
}
