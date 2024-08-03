package di

import (
	"context"
	"sync"
)

// SetProvider sets a [Provider] to a [Container].
func SetProvider[S any](ctn *Container, name string) error {
	return Set(ctn, name, newProviderBuilder[S](name))
}

// MustSetProvider calls [SetProvider] and panics if there is an error.
func MustSetProvider[S any](ctn *Container, name string) {
	MustSet(ctn, name, newProviderBuilder[S](name))
}

func newProviderBuilder[S any](name string) Builder[*Provider[S]] {
	return func(ctx context.Context, ctn *Container) (*Provider[S], Close, error) {
		p := newProvider[S](ctn, name)
		return p, p.Close, nil
	}
}

// GetProvider returns a [Provider] from a [Container].
func GetProvider[S any](ctx context.Context, ctn *Container, name string) (*Provider[S], error) {
	return Get[*Provider[S]](ctx, ctn, name)
}

// MustGetProvider calls [GetProvider] and panics if there is an error.
func MustGetProvider[S any](ctx context.Context, ctn *Container, name string) *Provider[S] {
	return MustGet[*Provider[S]](ctx, ctn, name)
}

// Provider provides a service.
//
// It can be used to break circular dependencies.
type Provider[T any] struct {
	Container *Container
	Name      string

	mu          sync.Mutex
	initialized bool
	service     T
}

func newProvider[S any](ctn *Container, name string) *Provider[S] {
	return &Provider[S]{
		Container: ctn,
		Name:      name,
	}
}

// Get returns the service.
func (p *Provider[T]) Get(ctx context.Context) (T, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.initialized {
		return p.service, nil
	}
	s, err := Get[T](ctx, p.Container, p.Name)
	if err != nil {
		return s, err
	}
	p.initialized = true
	p.service = s
	return s, nil
}

// MustGet calls [Provider.Get] and panics if there is an error.
func (p *Provider[T]) MustGet(ctx context.Context) T {
	s, err := p.Get(ctx)
	if err != nil {
		panic(err)
	}
	return s
}

// Close closes the [Provider].
//
// It doesn't close the service.
//
// The [Provider] can be used again after being closed.
func (p *Provider[T]) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.initialized {
		p.initialized = false
		var zero T
		p.service = zero
	}
	return nil
}
