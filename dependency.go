package di

import (
	"context"
	"reflect"
	"sync"
)

// GetDependency returns a service [Dependency] tree from a [Container].
func GetDependency[S any](ctx context.Context, ctn *Container, name string) (dep *Dependency, err error) {
	key := newKey[S](name)
	return ctn.getDependency(ctx, key)
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
