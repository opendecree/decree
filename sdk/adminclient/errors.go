package adminclient

import (
	"errors"
	"fmt"

	sdkretry "github.com/opendecree/decree/sdk/retry"
)

var (
	// ErrNotFound is returned when a requested resource does not exist.
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists is returned when attempting to create a resource that already exists,
	// or when importing a schema with identical fields to the latest version.
	ErrAlreadyExists = errors.New("already exists")

	// ErrFailedPrecondition is returned when an operation cannot be performed
	// in the current state (e.g. assigning an unpublished schema to a tenant).
	ErrFailedPrecondition = errors.New("failed precondition")

	// ErrPermissionDenied is returned when the caller lacks the required
	// role or tenant access to perform an administrative operation.
	ErrPermissionDenied = errors.New("permission denied")

	// ErrRateLimited is returned when the server has exhausted a rate limit
	// for the caller. Unlike transient errors, the server may attach a
	// RetryInfo detail with a backoff hint; callers should honor that hint
	// rather than retrying immediately. This error is intentionally NOT
	// wrapped as RetryableError so automatic retry loops do not fire.
	ErrRateLimited = errors.New("rate limited")

	// ErrInvalidArgument is returned when the server rejects a request due to
	// invalid input (e.g. a malformed regex constraint in a schema import).
	ErrInvalidArgument = errors.New("invalid argument")

	// ErrServiceNotConfigured is returned when calling a method on a service
	// client that was not provided to [New].
	ErrServiceNotConfigured = errors.New("service client not configured")
)

// InvalidArgumentError wraps [ErrInvalidArgument] with the server message.
func InvalidArgumentError(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgument, message)
}

// RetryableError wraps an error to indicate the operation may succeed on retry.
// Transport implementations should wrap transient errors (e.g. Unavailable,
// DeadlineExceeded, ResourceExhausted) in RetryableError.
// It is an alias for the shared [sdkretry.RetryableError] type.
type RetryableError = sdkretry.RetryableError

// IsRetryable reports whether err is marked as retryable by the transport.
var IsRetryable = sdkretry.IsRetryable
