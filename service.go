package di

import (
	"context"
	"reflect"
	"sync"
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
	defer recoverPanicToError(&err)
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

type serviceWrapperMap struct {
	mu sync.Mutex
	m  map[Key]*serviceWrapper
}

func (m *serviceWrapperMap) set(key Key, sw *serviceWrapper) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.m == nil {
		m.m = make(map[Key]*serviceWrapper)
	}
	_, ok := m.m[key]
	if ok {
		return ErrAlreadySet
	}
	m.m[key] = sw
	return nil
}

func (m *serviceWrapperMap) get(key Key) (*serviceWrapper, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sw, ok := m.m[key]
	if !ok {
		return nil, ErrNotSet
	}
	return sw, nil
}

func (m *serviceWrapperMap) all(f func(key Key, sw *serviceWrapper)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, sw := range m.m {
		f(key, sw)
	}
}

func (m *serviceWrapperMap) getValues() []*serviceWrapper {
	m.mu.Lock()
	defer m.mu.Unlock()
	sws := make([]*serviceWrapper, 0, len(m.m))
	for _, sw := range m.m {
		sws = append(sws, sw)
	}
	return sws
}
