package adminclient

import "errors"

var (
	// ErrNotFound is returned when a requested resource does not exist.
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists is returned when attempting to create a resource that already exists,
	// or when importing a schema with identical fields to the latest version.
	ErrAlreadyExists = errors.New("already exists")

	// ErrFailedPrecondition is returned when an operation cannot be performed
	// in the current state (e.g. assigning an unpublished schema to a tenant).
	ErrFailedPrecondition = errors.New("failed precondition")

	// ErrServiceNotConfigured is returned when calling a method on a service
	// client that was not provided to [New].
	ErrServiceNotConfigured = errors.New("service client not configured")
)
