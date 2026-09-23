package core

import (
	"errors"
	"fmt"
)

// ErrNotFound is returned when a Zarr node or key does not exist.
var ErrNotFound = errors.New("zarr: not found")

// ErrExists is returned when creating a node that already exists.
var ErrExists = errors.New("zarr: already exists")

// ErrInvalidMetadata is returned when metadata cannot be parsed or is inconsistent.
var ErrInvalidMetadata = errors.New("zarr: invalid metadata")

// ErrOutOfBounds is returned when a read or write is outside the array domain.
var ErrOutOfBounds = errors.New("zarr: requested data is outside of the array's domain")

// Error wraps a Zarr-specific failure with optional cause.
type Error struct {
	Msg   string
	Cause error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("zarr: %s: %v", e.Msg, e.Cause)
	}
	return "zarr: " + e.Msg
}

func (e *Error) Unwrap() error { return e.Cause }

// NewError constructs a *Error.
func NewError(msg string) error {
	return &Error{Msg: msg}
}

// WrapError constructs a *Error with a cause.
func WrapError(msg string, cause error) error {
	return &Error{Msg: msg, Cause: cause}
}
