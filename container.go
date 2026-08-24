package di

import (
	"cmp"
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"

	"github.com/pierrre/go-libs/reflectutil"
	"github.com/pierrre/go-libs/syncutil"
)

// Container contains services.
type Container struct {
	services syncutil.Map[Key, *serviceWrapper]
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
	_, ok := c.services.LoadOrStore(key, sw)
	if ok {
		return ErrAlreadySet
	}
	return nil
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
	sw, err := c.getServiceWrapper(key)
	if err != nil {
		return nil, err
	}
	return sw.get(ctx, c)
}

func (c *Container) getServiceWrapper(key Key) (*serviceWrapper, error) {
	sw, ok := c.services.Load(key)
	if !ok {
		return nil, ErrNotSet
	}
	return sw, nil
}

// MustGet calls [Container.Get] and panics if there is an error.
func (c *Container) MustGet[S any](ctx context.Context, name string) S {
	s, err := c.Get[S](ctx, name)
	if err != nil {
		panic(err)
	}
	return s
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
	c.services.Range(func(key Key, sw *serviceWrapper) bool {
		f(key, sw)
		return true
	})
}

// Close closes all the services of the [Container].
//
// The created services must not be used after this call.
//
// The [Container] can be used again after being closed.
func (c *Container) Close(ctx context.Context) error {
	sws := c.getAllServiceWrappers()
	slices.SortFunc(sws, (*serviceWrapper).compare)
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

func (c *Container) getAllServiceWrappers() []*serviceWrapper {
	var sws []*serviceWrapper
	c.services.Range(func(key Key, sw *serviceWrapper) bool {
		sws = append(sws, sw)
		return true
	})
	return sws
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

// Compare compares 2 keys.
func (k Key) Compare(k2 Key) int {
	return cmp.Or(
		strings.Compare(k.Type, k2.Type),
		strings.Compare(k.Name, k2.Name),
	)
}

func (k Key) String() string {
	if k.Name == "" {
		return k.Type
	}
	return k.Type + "(" + k.Name + ")"
}
