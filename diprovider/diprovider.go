// Package diprovider helps to break circular dependencies.
package diprovider

import (
	"context"
	"sync"

	"github.com/pierrre/di"
)

// Set sets a [Provider] to a [di.Container].
func Set[S any](ctn *di.Container, name string) error {
	return di.Set(ctn, name, newProviderBuilder[S](name)) //nolint:wrapcheck // It's from the same module.
}

// MustSet calls [MustSet] for a [Provider].
func MustSet[S any](ctn *di.Container, name string) {
	di.MustSet(ctn, name, newProviderBuilder[S](name))
}

func newProviderBuilder[S any](name string) di.Builder[*Provider[S]] {
	return func(ctx context.Context, ctn *di.Container) (*Provider[S], di.Close, error) {
		p := newProvider[S](ctn, name)
		return p, p.Close, nil
	}
}

// Get returns a [Provider] from a [di.Container].
func Get[S any](ctx context.Context, ctn *di.Container, name string) (*Provider[S], error) {
	return di.Get[*Provider[S]](ctx, ctn, name)
}

// MustGet calls [MustGet] for a [Provider].
func MustGet[S any](ctx context.Context, ctn *di.Container, name string) *Provider[S] {
	return di.MustGet[*Provider[S]](ctx, ctn, name)
}

// Provider provides a service.
//
// It can be used to break circular dependencies.
// It caches the service after the first call to [Provider.Get], so it's faster to call [Provider.Get] than [di.Get].
type Provider[S any] struct {
	Container *di.Container
	Name      string

	mu          sync.Mutex
	initialized bool
	service     S
}

func newProvider[S any](ctn *di.Container, name string) *Provider[S] {
	return &Provider[S]{
		Container: ctn,
		Name:      name,
	}
}

// Get returns the service.
func (p *Provider[S]) Get(ctx context.Context) (S, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.initialized {
		return p.service, nil
	}
	s, err := di.Get[S](ctx, p.Container, p.Name)
	if err != nil {
		return s, err
	}
	p.initialized = true
	p.service = s
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
// However it doesn't close the service.
//
// The [Provider] can be used again after being closed.
func (p *Provider[S]) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.initialized {
		p.initialized = false
		var zero S
		p.service = zero
	}
	return nil
}
