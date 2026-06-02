// Package configclient provides an ergonomic Go client for reading and writing
// configuration values via the OpenDecree API.
//
// This is an application-runtime SDK — for admin operations (schema management,
// import/export, rollback) see the adminclient package.
package configclient

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound is returned when a requested field or version does not exist.
	ErrNotFound = errors.New("not found")

	// ErrLocked is returned when attempting to write a field that is
	// administratively locked (FieldLock). Use errors.Is to distinguish
	// this from ErrPermissionDenied.
	ErrLocked = errors.New("field is locked")

	// ErrPermissionDenied is returned when the caller lacks the required
	// role or tenant access to perform the operation. This is distinct from
	// ErrLocked, which indicates an administratively locked field.
	ErrPermissionDenied = errors.New("permission denied")

	// ErrChecksumMismatch is returned when an optimistic concurrency check fails
	// because the value was modified between read and write.
	ErrChecksumMismatch = errors.New("checksum mismatch: value was modified")

	// ErrAlreadyExists is returned when attempting to create a resource that already exists.
	ErrAlreadyExists = errors.New("already exists")

	// ErrRateLimited is returned when the server has exhausted a rate limit
	// for the caller. Unlike transient errors, the server may attach a
	// RetryInfo detail with a backoff hint; callers should honor that hint
	// rather than retrying immediately. This error is intentionally NOT
	// wrapped as RetryableError so automatic retry loops do not fire.
	ErrRateLimited = errors.New("rate limited")

	// ErrTypeMismatch is returned when a typed getter is called on a field
	// whose value type doesn't match (e.g. GetInt on a string field).
	ErrTypeMismatch = errors.New("value type mismatch")

	// ErrInvalidArgument is returned when a value fails server-side validation
	// (type mismatch, constraint violation, or unknown field in strict mode).
	ErrInvalidArgument = errors.New("invalid argument")
)

// InvalidArgumentError wraps ErrInvalidArgument with a server message.
func InvalidArgumentError(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgument, message)
}

// RetryableError wraps an error to indicate the operation may succeed on retry.
// Transport implementations should wrap transient errors (e.g., network issues,
// server overload) in RetryableError.
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string { return e.Err.Error() }
func (e *RetryableError) Unwrap() error { return e.Err }

// IsRetryable reports whether err is marked as retryable by the transport.
func IsRetryable(err error) bool {
	var re *RetryableError
	return errors.As(err, &re)
}
