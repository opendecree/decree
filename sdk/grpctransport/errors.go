package grpctransport

import (
	"fmt"

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
		return configclient.ErrPermissionDenied
	case codes.Unauthenticated:
		return configclient.ErrUnauthenticated
	case codes.FailedPrecondition:
		return configclient.ErrLocked
	case codes.Aborted:
		// Aborted is dual-purpose: it fires for checksum mismatches (optimistic
		// concurrency) AND for concurrent-write conflicts where no checksum was
		// supplied. The server does not yet use a distinct code or status detail
		// to distinguish the two cases, so both map to ErrChecksumMismatch.
		return configclient.ErrChecksumMismatch
	case codes.AlreadyExists:
		return configclient.ErrAlreadyExists
	case codes.InvalidArgument:
		return configclient.NewInvalidArgumentError(st.Message())
	case codes.ResourceExhausted:
		// ResourceExhausted signals a rate limit. The server attaches a RetryInfo
		// detail with a hard backoff hint; wrapping as RetryableError would discard
		// that hint and cause retry loops to hammer the limiter. Wrap both the
		// original gRPC error (so status.Code still returns ResourceExhausted) and
		// ErrRateLimited (so callers can errors.Is check the sentinel).
		return fmt.Errorf("%w: %w", err, configclient.ErrRateLimited)
	case codes.Unavailable, codes.DeadlineExceeded:
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
	case codes.PermissionDenied:
		return adminclient.ErrPermissionDenied
	case codes.Unauthenticated:
		return adminclient.ErrUnauthenticated
	case codes.InvalidArgument:
		return adminclient.InvalidArgumentError(st.Message())
	case codes.ResourceExhausted:
		// ResourceExhausted signals a rate limit. Wrap both the original gRPC error
		// (so status.Code still returns ResourceExhausted) and ErrRateLimited (so
		// callers can errors.Is check the sentinel).
		return fmt.Errorf("%w: %w", err, adminclient.ErrRateLimited)
	case codes.Unavailable, codes.DeadlineExceeded:
		return &adminclient.RetryableError{Err: err}
	default:
		return err
	}
}
