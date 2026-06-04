// Package configclient provides an ergonomic Go client for reading and writing
// configuration values via the OpenDecree API.
//
// This is an application-runtime SDK — for admin operations (schema management,
// import/export, rollback) see the adminclient package.
package configclient

import (
	"errors"

	sdkretry "github.com/opendecree/decree/sdk/retry"
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

	// ErrUnauthenticated is returned when the caller has not provided valid
	// credentials (e.g. missing or invalid auth token). This is distinct from
	// ErrPermissionDenied, which indicates the caller is authenticated but
	// lacks the required role or access.
	ErrUnauthenticated = errors.New("unauthenticated")

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

	// ErrInvalidArgument is the sentinel for errors.Is matching against any
	// InvalidArgumentError, regardless of message. Use errors.As to access
	// the structured Message field.
	//
	// Breaking change (alpha): previously errors.New("invalid argument");
	// now *InvalidArgumentError. errors.Is still works via Is().
	ErrInvalidArgument = &InvalidArgumentError{}
)

// InvalidArgumentError carries structured validation details from the server.
// Use [NewInvalidArgumentError] to construct one, [ErrInvalidArgument] as the
// sentinel target for errors.Is, and errors.As to extract the Message field.
//
// Breaking change (alpha): the package-level InvalidArgumentError() function
// has been removed. Replace calls with NewInvalidArgumentError().
type InvalidArgumentError struct {
	// Message is the server-supplied validation detail (e.g. "field app.name
	// must match ^[a-z]+$").
	Message string
}

// Error implements the error interface.
func (e *InvalidArgumentError) Error() string { return "invalid argument: " + e.Message }

// Is reports true for any *InvalidArgumentError target, enabling
// errors.Is(err, ErrInvalidArgument) regardless of Message value.
func (e *InvalidArgumentError) Is(target error) bool {
	_, ok := target.(*InvalidArgumentError)
	return ok
}

// NewInvalidArgumentError constructs an InvalidArgumentError with the given
// server message. This replaces the old InvalidArgumentError() helper function.
func NewInvalidArgumentError(message string) *InvalidArgumentError {
	return &InvalidArgumentError{Message: message}
}

// RetryableError wraps an error to indicate the operation may succeed on retry.
// Transport implementations should wrap transient errors (e.g., network issues,
// server overload) in RetryableError.
// It is an alias for the shared [sdkretry.RetryableError] type.
type RetryableError = sdkretry.RetryableError

// IsRetryable reports whether err is marked as retryable by the transport.
var IsRetryable = sdkretry.IsRetryable
