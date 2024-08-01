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

// Set sets a service to a [Container].
//
// Name is an optional identifier amongst the services of the same type.
//
// If the service is already set, it returns [ErrAlreadySet].
func Set[S any](ctn *Container, name string, b Builder[S]) (err error) {
	key := newKey[S](name)
	defer returnWrapServiceError(&err, key)
	sw := newServiceWrapper(key, func(ctx context.Context, ctn *Container) (any, Close, error) {
		return b(ctx, ctn)
	})
	return ctn.set(key, sw)
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
	defer returnWrapServiceError(&err, key)
	sw, err := ctn.get(key)
	if err != nil {
		return s, err
	}
	v, err := sw.get(ctx, ctn)
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

// GetDependency returns a service [Dependency] tree from a [Container].
func GetDependency[S any](ctx context.Context, ctn *Container, name string) (dep *Dependency, err error) {
	key := newKey[S](name)
	defer returnWrapServiceError(&err, key)
	sw, err := ctn.get(key)
	if err != nil {
		return nil, err
	}
	return sw.getDependency(ctx, ctn)
}

func returnWrapServiceError(perr *error, key Key) { //nolint:gocritic // We need a pointer of error.
	if *perr != nil {
		*perr = &ServiceError{
			error: *perr,
			Key:   key,
		}
	}
}

// Container contains services.
type Container struct {
	mu       sync.Mutex
	services map[Key]*serviceWrapper
}

func (c *Container) set(key Key, sw *serviceWrapper) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.services == nil {
		c.services = make(map[Key]*serviceWrapper)
	}
	_, ok := c.services[key]
	if ok {
		return ErrAlreadySet
	}
	c.services[key] = sw
	return nil
}

func (c *Container) get(key Key) (*serviceWrapper, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sw, ok := c.services[key]
	if !ok {
		return nil, ErrNotSet
	}
	return sw, nil
}

func (c *Container) all(f func(key Key, sw *serviceWrapper)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, sw := range c.services {
		f(key, sw)
	}
}

// Close closes the [Container] and all the services.
//
// The created services must not be used after this call.
//
// The container can be reused after this call.
func (c *Container) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var errs []error
	for key, sw := range c.services {
		err := sw.close(ctx)
		if err != nil {
			err = &ServiceError{
				error: err,
				Key:   key,
			}
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

// Builder builds a service.
//
// The [Close] function allows to close the service.
// It can be nil if the service does not need to be closed.
// After it is called, the service instance must not be used anymore.
//
// If it calls [Get] it must provide the same [context.Context].
type Builder[S any] func(ctx context.Context, ctn *Container) (S, Close, error)

type builder func(ctx context.Context, ctn *Container) (any, Close, error)

// Close closes a service.
type Close func(ctx context.Context) error

type serviceWrapper struct {
	mu          *mutex
	key         Key
	builder     builder
	initialized bool
	service     any
	cl          Close
	dependency  *Dependency
}

func newServiceWrapper(key Key, b builder) *serviceWrapper {
	return &serviceWrapper{
		mu:      newMutex(),
		key:     key,
		builder: b,
	}
}

func (sw *serviceWrapper) get(ctx context.Context, ctn *Container) (any, error) {
	ctx, err := sw.mu.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer sw.mu.unlock()
	err = sw.ensureInitialized(ctx, ctn)
	if err != nil {
		return nil, err
	}
	addDependencyToCollectorFromContext(ctx, sw.dependency)
	return sw.service, nil
}

func (sw *serviceWrapper) getDependency(ctx context.Context, ctn *Container) (*Dependency, error) {
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

func (sw *serviceWrapper) ensureInitialized(ctx context.Context, ctn *Container) error {
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
		Type:         reflectutil.TypeFullName(sw.key.Type),
		reflectType:  sw.key.Type,
		Name:         sw.key.Name,
		Dependencies: dc.dependencies,
	}
	return nil
}

func (sw *serviceWrapper) close(ctx context.Context) error {
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
	sw.service = nil
	sw.cl = nil
	sw.dependency = nil
	return err
}

// Dependency represents a service dependency.
type Dependency struct {
	Type         string `json:"type"`
	reflectType  reflect.Type
	Name         string        `json:"name,omitempty"`
	Dependencies []*Dependency `json:"dependencies,omitempty"`
}

// GetReflectType returns the reflect.Type of the dependency.
func (d *Dependency) GetReflectType() reflect.Type {
	return d.reflectType
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
	Key Key
}

func (err *ServiceError) Unwrap() error {
	return err.error
}

func (err *ServiceError) Error() string {
	return fmt.Sprintf("service %q: %v", err.Key, err.error)
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
