// Package di provides a dependency injection container.
package di

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/pierrre/go-libs/reflectutil"
)

// Get returns a service from a [Container].
//
// If the name is empty, it is set to the type of the service.
//
// If the service is not found for the given name, it returns an error.
//
// If the service is not of the expected type, it returns an error.
//
// If the service creation fails, it returns an error.
func Get[S any](ctx context.Context, ctn *Container, name string) (s S, err error) {
	defer func() {
		if err != nil {
			err = &ServiceError{
				error: err,
				Name:  name,
			}
		}
	}()
	if name == "" {
		name = reflectutil.TypeFullNameFor[S]()
	}
	sw := ctn.get(name)
	if sw == nil {
		return s, ErrNotSet
	}
	swi, ok := sw.(*serviceWrapperImpl[S])
	if !ok {
		return s, &TypeError{
			Type: reflectutil.TypeFullNameFor[S](),
		}
	}
	return swi.get(ctx, ctn)
}

// MustGet calls [Get] with [Must].
func MustGet[S any](ctx context.Context, ctn *Container, name string) S {
	return Must(Get[S](ctx, ctn, name))
}

// GetAll returns all services of a type from a [Container].
//
// The key of the map is the name of the service.
func GetAll[S any](ctx context.Context, ctn *Container) (map[string]S, error) {
	var names []string
	ctn.all(func(name string, sw serviceWrapper) {
		_, ok := sw.(*serviceWrapperImpl[S])
		if ok {
			names = append(names, name)
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

// Set sets a service to a [Container].
//
// If the name is empty, it is set to the type of the service.
//
// If the service is already set, it panics.
func Set[S any](ctn *Container, name string, b Builder[S]) {
	if name == "" {
		name = reflectutil.TypeFullNameFor[S]()
	}
	sw := &serviceWrapperImpl[S]{
		builder: b,
	}
	ctn.set(name, sw)
}

// Container contains services.
type Container struct {
	mu                     sync.Mutex
	services               map[string]serviceWrapper
	getServiceNames        map[string]struct{}
	getServiceNamesOrdered []string
}

func (c *Container) get(name string) serviceWrapper {
	c.mu.Lock()
	defer c.mu.Unlock()
	sw, ok := c.services[name]
	if !ok {
		return nil
	}
	if c.getServiceNames == nil {
		c.getServiceNames = make(map[string]struct{})
	}
	_, ok = c.getServiceNames[name]
	if !ok {
		c.getServiceNames[name] = struct{}{}
		c.getServiceNamesOrdered = append(c.getServiceNamesOrdered, name)
	}
	return sw
}

func (c *Container) set(name string, sw serviceWrapper) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.services == nil {
		c.services = make(map[string]serviceWrapper)
	}
	_, ok := c.services[name]
	if ok {
		panic(&ServiceError{
			error: ErrAlreadySet,
			Name:  name,
		})
	}
	c.services[name] = sw
}

func (c *Container) all(f func(name string, sw serviceWrapper)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for name, sw := range c.services {
		f(name, sw)
	}
}

// Close closes the [Container].
//
// It closes all services in reverse dependency order.
// The created services must not be used after this call.
//
// The container can be reused after this call.
func (c *Container) Close(ctx context.Context, onErr func(context.Context, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.getServiceNamesOrdered) - 1; i >= 0; i-- {
		name := c.getServiceNamesOrdered[i]
		sw := c.services[name]
		err := sw.close(ctx)
		if err != nil {
			err = &ServiceError{
				error: err,
				Name:  name,
			}
			onErr(ctx, err)
		}
	}
	c.getServiceNames = nil
	c.getServiceNamesOrdered = nil
}

// Builder builds a service.
//
// The [Close] function allows to close the service.
// It can be nil if the service does not need to be closed.
// After it is called, the service instance must not be used anymore.
type Builder[S any] func(ctx context.Context, ctn *Container) (S, Close, error)

// Close closes a service.
type Close = func(ctx context.Context) error

type serviceWrapper interface {
	close(ctx context.Context) error
}

type serviceWrapperImpl[S any] struct {
	mu          sync.Mutex
	builder     Builder[S]
	initialized bool
	service     S
	cl          Close
}

func (sw *serviceWrapperImpl[S]) get(ctx context.Context, ctn *Container) (S, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.initialized {
		return sw.service, nil
	}
	s, cl, err := sw.builder(ctx, ctn)
	if err != nil {
		return s, err
	}
	sw.initialized = true
	sw.service = s
	sw.cl = cl
	return sw.service, nil
}

func (sw *serviceWrapperImpl[S]) close(ctx context.Context) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if !sw.initialized {
		return nil
	}
	var err error
	if sw.cl != nil {
		err = sw.cl(ctx)
	}
	sw.initialized = false
	var zero S
	sw.service = zero
	sw.cl = nil
	return err
}

// Must is a helper to call a function and panics if it returns an error.
func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

var (
	// ErrNotSet is returned when a service is not set.
	ErrNotSet = errors.New("not set")
	// ErrAlreadySet is returned when a service is already set.
	ErrAlreadySet = errors.New("already set")
)

// ServiceError represents an error related to a service.
type ServiceError struct {
	error
	Name string
}

func (err *ServiceError) Unwrap() error {
	return err.error
}

func (err *ServiceError) Error() string {
	return fmt.Sprintf("service %q: %v", err.Name, err.error)
}

// TypeError represents an error related to a service type.
type TypeError struct {
	Type string
}

func (err *TypeError) Error() string {
	return fmt.Sprintf("type %s does not match", err.Type)
}
