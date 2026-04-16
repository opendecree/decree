package grpctransport

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
)

// mapConfigError translates gRPC status codes to configclient sentinel errors.
func mapConfigError(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch st.Code() {
	case codes.NotFound:
		return configclient.ErrNotFound
	case codes.PermissionDenied:
		return configclient.ErrLocked
	case codes.Aborted:
		return configclient.ErrChecksumMismatch
	case codes.AlreadyExists:
		return configclient.ErrAlreadyExists
	case codes.InvalidArgument:
		return configclient.InvalidArgumentError(st.Message())
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
		return &configclient.RetryableError{Err: err}
	default:
		return err
	}
}

// mapAdminError translates gRPC status codes to adminclient sentinel errors.
func mapAdminError(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch st.Code() {
	case codes.NotFound:
		return adminclient.ErrNotFound
	case codes.AlreadyExists:
		return adminclient.ErrAlreadyExists
	case codes.FailedPrecondition:
		return adminclient.ErrFailedPrecondition
	default:
		return err
	}
}
