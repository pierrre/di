package di

import (
	"context"
	"reflect"
	"sync/atomic"
)

type builder func(ctx context.Context, ctn *Container) (any, Close, error)

type serviceWrapper struct {
	key     Key
	typ     reflect.Type
	builder builder
	mu      *mutex
	content atomic.Pointer[serviceContent]
}

type serviceContent struct {
	service    any
	cl         Close
	dependency *Dependency
}

func newServiceWrapper(key Key, typ reflect.Type, b builder) *serviceWrapper {
	return &serviceWrapper{
		key:     key,
		typ:     typ,
		builder: b,
		mu:      newMutex(),
	}
}

func (sw *serviceWrapper) get(ctx context.Context, ctn *Container) (any, error) {
	sc, err := sw.ensureInitialized(ctx, ctn)
	if err != nil {
		return nil, err
	}
	addDependencyToCollectorFromContext(ctx, sc.dependency)
	return sc.service, nil
}

func (sw *serviceWrapper) getDependency(ctx context.Context, ctn *Container) (*Dependency, error) {
	sc, err := sw.ensureInitialized(ctx, ctn)
	if err != nil {
		return nil, err
	}
	return sc.dependency, nil
}

func (sw *serviceWrapper) ensureInitialized(ctx context.Context, ctn *Container) (*serviceContent, error) {
	sc := sw.content.Load()
	if sc != nil { // Fast path.
		return sc, nil
	}
	ctx, err := sw.mu.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer sw.mu.unlock()
	sc = sw.content.Load()
	if sc == nil { // This may have been set.
		sc, err = sw.initialize(ctx, ctn)
	}
	return sc, err
}

func (sw *serviceWrapper) initialize(ctx context.Context, ctn *Container) (sc *serviceContent, err error) {
	ctx, dc := addDependencyCollectorToContext(ctx)
	defer recoverPanicToError(&err)
	s, cl, err := sw.builder(ctx, ctn)
	if err != nil {
		return nil, err
	}
	sc = &serviceContent{
		service: s,
		cl:      cl,
		dependency: &Dependency{
			Type:         sw.key.Type,
			reflectType:  sw.typ,
			Name:         sw.key.Name,
			Dependencies: dc.dependencies,
		},
	}
	sw.content.Store(sc)
	return sc, nil
}

func (sw *serviceWrapper) close(ctx context.Context) error {
	ctx, err := sw.mu.lock(ctx)
	if err != nil {
		return err
	}
	defer sw.mu.unlock()
	sc := sw.content.Load()
	if sc != nil && sc.cl != nil {
		err = sc.cl(ctx)
	}
	sw.content.Store(nil)
	return err
}

func (sw *serviceWrapper) compare(sw2 *serviceWrapper) int {
	return sw.key.Compare(sw2.key)
}
