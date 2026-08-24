package di

import (
	"cmp"
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"

	"github.com/pierrre/go-libs/reflectutil"
)

// Container contains services.
type Container struct {
	services serviceWrapperMap
}

// Set sets a service to the [Container].
//
// Name is an optional identifier for services of the same type.
//
// If the service is already set, it returns [ErrAlreadySet].
func (c *Container) Set[S any](name string, b Builder[S]) (err error) {
	key := newKey[S](name)
	typ := reflect.TypeFor[S]()
	return c.set(key, typ, func(ctx context.Context, ctn *Container) (any, Close, error) {
		return b(ctx, ctn)
	})
}

func (c *Container) set(key Key, typ reflect.Type, b builder) (err error) {
	defer wrapReturnServiceError(&err, key)
	sw := newServiceWrapper(key, typ, b)
	return c.services.set(key, sw)
}

// MustSet calls [Container.Set] and panics if there is an error.
func (c *Container) MustSet[S any](name string, b Builder[S]) {
	err := c.Set(name, b)
	if err != nil {
		panic(err)
	}
}

// Get returns a service from the [Container].
//
// Name is an optional identifier for services of the same type.
//
// If the service is not found, it returns [ErrNotSet].
//
// If the service is not yet initialized, it calls its [Builder].
// If the [Builder] fails, it returns the error.
func (c *Container) Get[S any](ctx context.Context, name string) (s S, err error) {
	key := newKey[S](name)
	v, err := c.get(ctx, key)
	if err != nil {
		return s, err
	}
	s, _ = v.(S)
	return s, nil
}

func (c *Container) get(ctx context.Context, key Key) (v any, err error) {
	defer wrapReturnServiceError(&err, key)
	sw, err := c.services.get(key)
	if err != nil {
		return nil, err
	}
	return sw.get(ctx, c)
}

// MustGet calls [Container.Get] and panics if there is an error.
func (c *Container) MustGet[S any](ctx context.Context, name string) S {
	s, err := c.Get[S](ctx, name)
	if err != nil {
		panic(err)
	}
	return s
}

// GetDependency returns a service [Dependency] tree from the [Container].
func (c *Container) GetDependency[S any](ctx context.Context, name string) (dep *Dependency, err error) {
	key := newKey[S](name)
	return c.getDependency(ctx, key)
}

func (c *Container) getDependency(ctx context.Context, key Key) (d *Dependency, err error) {
	defer wrapReturnServiceError(&err, key)
	sw, err := c.services.get(key)
	if err != nil {
		return nil, err
	}
	return sw.getDependency(ctx, c)
}

// GetAll returns all services of a type from the [Container].
//
// The key of the map is the name of the service.
func (c *Container) GetAll[S any](ctx context.Context) (map[string]S, error) {
	var names []string
	typ := reflect.TypeFor[S]()
	c.all(func(key Key, sw *serviceWrapper) {
		if sw.typ == typ {
			names = append(names, key.Name)
		}
	})
	var ss map[string]S
	if len(names) > 0 {
		ss = make(map[string]S, len(names))
	}
	for _, name := range names {
		s, err := c.Get[S](ctx, name)
		if err != nil {
			return nil, err
		}
		ss[name] = s
	}
	return ss, nil
}

func (c *Container) all(f func(key Key, sw *serviceWrapper)) {
	c.services.all(f)
}

// Close closes all the services of the [Container].
//
// The created services must not be used after this call.
//
// The [Container] can be used again after being closed.
func (c *Container) Close(ctx context.Context) error {
	sws := c.services.getValues()
	slices.SortFunc(sws, func(a, b *serviceWrapper) int {
		return cmp.Or(
			strings.Compare(a.key.Type, b.key.Type),
			strings.Compare(a.key.Name, b.key.Name),
		)
	})
	var errs []error
	for _, sw := range sws {
		err := sw.close(ctx)
		if err != nil {
			err = wrapServiceError(err, sw.key)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Key represents a service key in a [Container].
type Key struct {
	Type string
	Name string
}

func newKey[S any](name string) Key {
	return Key{
		Type: reflectutil.TypeFullNameFor[S](),
		Name: name,
	}
}

func (k Key) String() string {
	if k.Name == "" {
		return k.Type
	}
	return k.Type + "(" + k.Name + ")"
}
