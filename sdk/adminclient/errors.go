package adminclient

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	// ErrServiceNotConfigured is returned when calling a method on a service
	// client that was not provided to [New].
	ErrServiceNotConfigured = errors.New("service client not configured")
)

func mapError(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch st.Code() {
	case codes.NotFound:
		return ErrNotFound
	case codes.AlreadyExists:
		return ErrAlreadyExists
	case codes.FailedPrecondition:
		return ErrFailedPrecondition
	default:
		return err
	}
}
