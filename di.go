// Package di provides a dependency injection container.
package di

import (
	"context"
	"errors"
	"fmt"
	"reflect"
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
	name = getName[S](name)
	defer returnWrapServiceError(&err, name)
	swi, err := getServiceWrapperImpl[S](ctn, name)
	if err != nil {
		return s, err
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

// GetDependency returns a service [Dependency] tree from a [Container].
func GetDependency[S any](ctx context.Context, ctn *Container, name string) (dep *Dependency, err error) {
	name = getName[S](name)
	defer returnWrapServiceError(&err, name)
	swi, err := getServiceWrapperImpl[S](ctn, name)
	if err != nil {
		return nil, err
	}
	return swi.getDependency(ctx, ctn)
}

// Set sets a service to a [Container].
//
// If the name is empty, it is set to the type of the service.
//
// If the service is already set, it panics.
func Set[S any](ctn *Container, name string, b Builder[S]) {
	name = getName[S](name)
	sw := newServiceWrapperImpl(name, b)
	ctn.set(name, sw)
}

func getName[S any](name string) string {
	if name == "" {
		name = reflectutil.TypeFullNameFor[S]()
	}
	return name
}

func returnWrapServiceError(perr *error, name string) { //nolint:gocritic // We need a pointer of error.
	if *perr != nil {
		*perr = &ServiceError{
			error: *perr,
			Name:  name,
		}
	}
}

func getServiceWrapperImpl[S any](ctn *Container, name string) (swi *serviceWrapperImpl[S], err error) {
	sw := ctn.get(name)
	if sw == nil {
		return nil, ErrNotSet
	}
	swi, ok := sw.(*serviceWrapperImpl[S])
	if !ok {
		return nil, &TypeError{
			Service:  sw.getType(),
			Expected: reflect.TypeFor[S](),
		}
	}
	return swi, nil
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
func (c *Container) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var errs []error
	for i := len(c.getServiceNamesOrdered) - 1; i >= 0; i-- {
		name := c.getServiceNamesOrdered[i]
		sw := c.services[name]
		err := sw.close(ctx)
		if err != nil {
			err = &ServiceError{
				error: err,
				Name:  name,
			}
			errs = append(errs, err)
		}
	}
	c.getServiceNames = nil
	c.getServiceNamesOrdered = nil
	return errors.Join(errs...)
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
type Close = func(ctx context.Context) error

type serviceWrapper interface {
	close(ctx context.Context) error
	getType() reflect.Type
}

type serviceWrapperImpl[S any] struct {
	mu          *mutex
	name        string
	builder     Builder[S]
	initialized bool
	service     S
	cl          Close
	dependency  *Dependency
}

func newServiceWrapperImpl[S any](name string, builder Builder[S]) *serviceWrapperImpl[S] {
	return &serviceWrapperImpl[S]{
		mu:      newMutex(),
		name:    name,
		builder: builder,
	}
}

func (sw *serviceWrapperImpl[S]) get(ctx context.Context, ctn *Container) (S, error) {
	ctx, err := sw.mu.lock(ctx)
	if err != nil {
		return sw.service, err
	}
	defer sw.mu.unlock()
	err = sw.ensureInitialized(ctx, ctn)
	if err != nil {
		return sw.service, err
	}
	addDependencyToCollectorFromContext(ctx, sw.dependency)
	return sw.service, nil
}

func (sw *serviceWrapperImpl[S]) getDependency(ctx context.Context, ctn *Container) (*Dependency, error) {
	ctx, err := sw.mu.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer sw.mu.unlock()
	err = sw.ensureInitialized(ctx, ctn)
	if err != nil {
		return nil, err
	}
	return sw.dependency, nil
}

func (sw *serviceWrapperImpl[S]) ensureInitialized(ctx context.Context, ctn *Container) error {
	if sw.initialized {
		return nil
	}
	ctx, dc := addDependencyCollectorToContext(ctx)
	s, cl, err := sw.builder(ctx, ctn)
	if err != nil {
		return err
	}
	sw.initialized = true
	sw.service = s
	sw.cl = cl
	sw.dependency = &Dependency{
		Name:         sw.name,
		Type:         reflectutil.TypeFullNameFor[S](),
		Dependencies: dc.dependencies,
	}
	return nil
}

func (sw *serviceWrapperImpl[S]) close(ctx context.Context) error {
	ctx, err := sw.mu.lock(ctx)
	if err != nil {
		return err
	}
	defer sw.mu.unlock()
	if !sw.initialized {
		return nil
	}
	if sw.cl != nil {
		err = sw.cl(ctx)
	}
	sw.initialized = false
	var zero S
	sw.service = zero
	sw.cl = nil
	sw.dependency = nil
	return err
}

func (sw *serviceWrapperImpl[S]) getType() reflect.Type {
	return reflect.TypeFor[S]()
}

// Dependency represents a service dependency.
type Dependency struct {
	Name         string        `json:"name"`
	Type         string        `json:"type"`
	Dependencies []*Dependency `json:"dependencies,omitempty"`
}

type dependencyCollector struct {
	mu           sync.Mutex
	dependencies []*Dependency
}

func (dc *dependencyCollector) add(d *Dependency) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.dependencies = append(dc.dependencies, d)
}

type dependencyCollectorContextKey struct{}

func addDependencyCollectorToContext(ctx context.Context) (context.Context, *dependencyCollector) {
	c := &dependencyCollector{}
	ctx = context.WithValue(ctx, dependencyCollectorContextKey{}, c)
	return ctx, c
}

func addDependencyToCollectorFromContext(ctx context.Context, d *Dependency) {
	dc, ok := ctx.Value(dependencyCollectorContextKey{}).(*dependencyCollector)
	if ok {
		dc.add(d)
	}
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
	// ErrCycle is returned when a cycle is detected.
	ErrCycle = errors.New("cycle")
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
	Service  reflect.Type
	Expected reflect.Type
}

func (err *TypeError) Error() string {
	return fmt.Sprintf("service type %s does not match the expected type %s", reflectutil.TypeFullName(err.Service), reflectutil.TypeFullName(err.Expected))
}

type mutex struct {
	ch chan struct{}
}

func newMutex() *mutex {
	return &mutex{
		ch: make(chan struct{}, 1),
	}
}

func (m *mutex) lock(ctx context.Context) (context.Context, error) {
	previous, _ := ctx.Value(mutexListContextKey{}).(*mutexList)
	for v := previous; v != nil; v = v.previous {
		if v.mu == m {
			return nil, ErrCycle
		}
	}
	select {
	case m.ch <- struct{}{}:
		ctx = context.WithValue(ctx, mutexListContextKey{}, &mutexList{
			previous: previous,
			mu:       m,
		})
		return ctx, nil
	case <-ctx.Done():
		return nil, ctx.Err() //nolint:wrapcheck // We don't neet to wrap.
	}
}

func (m *mutex) unlock() {
	<-m.ch
}

type mutexList struct {
	previous *mutexList
	mu       *mutex
}

type mutexListContextKey struct{}
