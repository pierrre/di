package di

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/pierrre/go-libs/reflectutil"
)

// Container contains services.
type Container struct {
	services serviceWrapperMap
}

func (c *Container) set(key Key, b builder) (err error) {
	defer wrapReturnServiceError(&err, key)
	sw := newServiceWrapper(key, b)
	return c.services.set(key, sw)
}

func (c *Container) get(ctx context.Context, key Key) (v any, err error) {
	defer wrapReturnServiceError(&err, key)
	sw, err := c.services.get(key)
	if err != nil {
		return nil, err
	}
	return sw.get(ctx, c)
}

func (c *Container) getDependency(ctx context.Context, key Key) (d *Dependency, err error) {
	defer wrapReturnServiceError(&err, key)
	sw, err := c.services.get(key)
	if err != nil {
		return nil, err
	}
	return sw.getDependency(ctx, c)
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
		return cmp.Compare(a.key.String(), b.key.String())
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
	Type reflect.Type
	Name string
}

func newKey[S any](name string) Key {
	return Key{
		Type: reflect.TypeFor[S](),
		Name: name,
	}
}

func (k Key) String() string {
	typName := reflectutil.TypeFullName(k.Type)
	if k.Name == "" {
		return typName
	}
	return fmt.Sprintf("%s(%s)", typName, k.Name)
}
