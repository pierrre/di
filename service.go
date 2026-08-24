package di

import (
	"context"
	"reflect"
)

type builder func(ctx context.Context, ctn *Container) (any, Close, error)

type serviceWrapper struct {
	mu          *mutex
	key         Key
	typ         reflect.Type
	builder     builder
	initialized bool
	service     any
	cl          Close
	dependency  *Dependency
}

func newServiceWrapper(key Key, typ reflect.Type, b builder) *serviceWrapper {
	return &serviceWrapper{
		mu:      newMutex(),
		key:     key,
		typ:     typ,
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

func (sw *serviceWrapper) ensureInitialized(ctx context.Context, ctn *Container) (err error) {
	if sw.initialized {
		return nil
	}
	ctx, dc := addDependencyCollectorToContext(ctx)
	defer recoverPanicToError(&err)
	s, cl, err := sw.builder(ctx, ctn)
	if err != nil {
		return err
	}
	sw.initialized = true
	sw.service = s
	sw.cl = cl
	sw.dependency = &Dependency{
		Type:         sw.key.Type,
		reflectType:  sw.typ,
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

func (sw *serviceWrapper) compare(sw2 *serviceWrapper) int {
	return sw.key.Compare(sw2.key)
}
