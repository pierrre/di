// Package di provides a dependency injection container.
package di

import (
	"context"
)

// Builder builds a service.
//
// The [Close] function allows closing the service.
// It can be nil if the service does not need to be closed.
// After it is called, the service instance must not be used anymore.
//
// If it panics, it's recovered as a [PanicError].
//
// If it calls [Container.Get], it must provide the same [context.Context].
type Builder[S any] func(ctx context.Context, ctn *Container) (S, Close, error)

// Close closes a service.
type Close func(ctx context.Context) error
