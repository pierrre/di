// Package di provides a dependency injection container.
package di

import (
	"context"
	"reflect"
)

// Set sets a service to a [Container].
//
// Name is an optional identifier amongst the services of the same type.
//
// If the service is already set, it returns [ErrAlreadySet].
func Set[S any](ctn *Container, name string, b Builder[S]) (err error) {
	key := newKey[S](name)
	return ctn.set(key, func(ctx context.Context, ctn *Container) (any, Close, error) {
		return b(ctx, ctn)
	})
}

// MustSet calls [Set] and panics if there is an error.
func MustSet[S any](ctn *Container, name string, b Builder[S]) {
	err := Set[S](ctn, name, b)
	if err != nil {
		panic(err)
	}
}

// Get returns a service from a [Container].
//
// Name is an optional identifier amongst the services of the same type.
//
// If the service is not found, it returns [ErrNotSet].
//
// If the service is not yet initialized, it calls its builder.
// If the builder fails, it returns the error.
func Get[S any](ctx context.Context, ctn *Container, name string) (s S, err error) {
	key := newKey[S](name)
	v, err := ctn.get(ctx, key)
	if err != nil {
		return s, err
	}
	s = v.(S) //nolint:forcetypeassert // We know the type.
	return s, nil
}

// MustGet calls [Get] and panics if there is an error.
func MustGet[S any](ctx context.Context, ctn *Container, name string) S {
	s, err := Get[S](ctx, ctn, name)
	if err != nil {
		panic(err)
	}
	return s
}

// GetAll returns all services of a type from a [Container].
//
// The key of the map is the name of the service.
func GetAll[S any](ctx context.Context, ctn *Container) (map[string]S, error) {
	var names []string
	typ := reflect.TypeFor[S]()
	ctn.all(func(key Key, sw *serviceWrapper) {
		if sw.key.Type == typ {
			names = append(names, key.Name)
		}
	})
	var ss map[string]S
	if len(names) > 0 {
		ss = make(map[string]S, len(names))
	}
	for _, name := range names {
		s, err := Get[S](ctx, ctn, name)
		if err != nil {
			return nil, err
		}
		ss[name] = s
	}
	return ss, nil
}

// Builder builds a service.
//
// The [Close] function allows to close the service.
// It can be nil if the service does not need to be closed.
// After it is called, the service instance must not be used anymore.
//
// If it calls [Get] it must provide the same [context.Context].
type Builder[S any] func(ctx context.Context, ctn *Container) (S, Close, error)

// Close closes a service.
type Close func(ctx context.Context) error
