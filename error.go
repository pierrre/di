package di

import (
	"errors"
	"fmt"
)

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
	return fmt.Sprintf("service %s: %v", err.Key, err.error)
}

func wrapServiceError(err error, key Key) error {
	if err == nil {
		return nil
	}
	return &ServiceError{
		error: err,
		Key:   key,
	}
}

func wrapReturnServiceError(perr *error, key Key) { //nolint:gocritic // We need a pointer of error.
	err := *perr
	*perr = wrapServiceError(err, key)
}

// PanicError represents a recovered panic error.
type PanicError struct {
	Recovered any
}

func (err *PanicError) Error() string {
	return fmt.Sprintf("panic: %v", err.Recovered)
}

func (err *PanicError) Unwrap() error {
	errw, _ := err.Recovered.(error)
	return errw
}

func recoverPanicToError(perr *error) { //nolint:gocritic // We need a pointer of error.
	r := recover()
	if r != nil {
		*perr = &PanicError{
			Recovered: r,
		}
	}
}
