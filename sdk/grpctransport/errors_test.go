package grpctransport

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
)

// --- mapConfigError ---

func TestMapConfigError_PermissionDenied_ReturnsErrPermissionDenied(t *testing.T) {
	err := mapConfigError(status.Error(codes.PermissionDenied, "no access to tenant acme"))
	if !errors.Is(err, configclient.ErrPermissionDenied) {
		t.Errorf("got %v, want ErrPermissionDenied", err)
	}
	if errors.Is(err, configclient.ErrLocked) {
		t.Error("PermissionDenied must NOT map to ErrLocked")
	}
}

func TestMapConfigError_FailedPrecondition_ReturnsErrLocked(t *testing.T) {
	err := mapConfigError(status.Error(codes.FailedPrecondition, "field app.name is locked"))
	if !errors.Is(err, configclient.ErrLocked) {
		t.Errorf("got %v, want ErrLocked", err)
	}
	if errors.Is(err, configclient.ErrPermissionDenied) {
		t.Error("FailedPrecondition must NOT map to ErrPermissionDenied")
	}
}

func TestMapConfigError_NotFound(t *testing.T) {
	err := mapConfigError(status.Error(codes.NotFound, "field not found"))
	if !errors.Is(err, configclient.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestMapConfigError_Aborted_ReturnsErrChecksumMismatch(t *testing.T) {
	err := mapConfigError(status.Error(codes.Aborted, "checksum mismatch"))
	if !errors.Is(err, configclient.ErrChecksumMismatch) {
		t.Errorf("got %v, want ErrChecksumMismatch", err)
	}
}

func TestMapConfigError_Nil(t *testing.T) {
	if err := mapConfigError(nil); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// --- mapAdminError ---

func TestMapAdminError_FailedPrecondition(t *testing.T) {
	err := mapAdminError(status.Error(codes.FailedPrecondition, "schema not published"))
	if !errors.Is(err, adminclient.ErrFailedPrecondition) {
		t.Errorf("got %v, want ErrFailedPrecondition", err)
	}
}

func TestMapAdminError_NotFound(t *testing.T) {
	err := mapAdminError(status.Error(codes.NotFound, "tenant not found"))
	if !errors.Is(err, adminclient.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestMapAdminError_Nil(t *testing.T) {
	if err := mapAdminError(nil); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// --- mapConfigError: remaining codes ---

func TestMapConfigError_AlreadyExists(t *testing.T) {
	err := mapConfigError(status.Error(codes.AlreadyExists, "field exists"))
	if !errors.Is(err, configclient.ErrAlreadyExists) {
		t.Errorf("got %v, want ErrAlreadyExists", err)
	}
}

func TestMapConfigError_InvalidArgument_IsInvalidArgumentError(t *testing.T) {
	err := mapConfigError(status.Error(codes.InvalidArgument, "bad value"))
	if !errors.Is(err, configclient.ErrInvalidArgument) {
		t.Errorf("got %v, want ErrInvalidArgument", err)
	}
}

func TestMapConfigError_Unavailable_IsRetryable(t *testing.T) {
	err := mapConfigError(status.Error(codes.Unavailable, "service down"))
	var re *configclient.RetryableError
	if !errors.As(err, &re) {
		t.Errorf("got %v, want *RetryableError", err)
	}
}

func TestMapConfigError_DeadlineExceeded_IsRetryable(t *testing.T) {
	err := mapConfigError(status.Error(codes.DeadlineExceeded, "deadline"))
	var re *configclient.RetryableError
	if !errors.As(err, &re) {
		t.Errorf("got %v, want *RetryableError", err)
	}
}

func TestMapConfigError_ResourceExhausted_IsRateLimited_NotRetryable(t *testing.T) {
	err := mapConfigError(status.Error(codes.ResourceExhausted, "rate limit"))
	if !errors.Is(err, configclient.ErrRateLimited) {
		t.Errorf("got %v, want ErrRateLimited", err)
	}
	var re *configclient.RetryableError
	if errors.As(err, &re) {
		t.Error("ResourceExhausted must NOT be wrapped as RetryableError")
	}
}

func TestMapConfigError_RetryableWrapsOriginalError(t *testing.T) {
	orig := status.Error(codes.Unavailable, "service down")
	err := mapConfigError(orig)
	var re *configclient.RetryableError
	if !errors.As(err, &re) {
		t.Fatalf("got %v, want *RetryableError", err)
	}
	if re.Err != orig {
		t.Errorf("RetryableError.Err = %v, want original gRPC error", re.Err)
	}
}

func TestMapConfigError_NonGRPC_PassThrough(t *testing.T) {
	orig := errors.New("some non-grpc error")
	err := mapConfigError(orig)
	if err != orig {
		t.Errorf("got %v, want original error passed through", err)
	}
}

func TestMapConfigError_Default_PassThrough(t *testing.T) {
	orig := status.Error(codes.Internal, "internal")
	err := mapConfigError(orig)
	if err != orig {
		t.Errorf("got %v, want original gRPC error passed through for default code", err)
	}
}

func TestMapAdminError_PermissionDenied_ReturnsErrPermissionDenied(t *testing.T) {
	err := mapAdminError(status.Error(codes.PermissionDenied, "insufficient role"))
	if !errors.Is(err, adminclient.ErrPermissionDenied) {
		t.Errorf("got %v, want ErrPermissionDenied", err)
	}
}

// --- mapAdminError: remaining codes ---

func TestMapAdminError_AlreadyExists(t *testing.T) {
	err := mapAdminError(status.Error(codes.AlreadyExists, "schema exists"))
	if !errors.Is(err, adminclient.ErrAlreadyExists) {
		t.Errorf("got %v, want ErrAlreadyExists", err)
	}
}

func TestMapAdminError_InvalidArgument_IsInvalidArgumentError(t *testing.T) {
	err := mapAdminError(status.Error(codes.InvalidArgument, "bad field"))
	if !errors.Is(err, adminclient.ErrInvalidArgument) {
		t.Errorf("got %v, want ErrInvalidArgument", err)
	}
}

func TestMapAdminError_Unavailable_IsRetryable(t *testing.T) {
	err := mapAdminError(status.Error(codes.Unavailable, "service down"))
	var re *adminclient.RetryableError
	if !errors.As(err, &re) {
		t.Errorf("got %v, want *RetryableError", err)
	}
}

func TestMapAdminError_DeadlineExceeded_IsRetryable(t *testing.T) {
	err := mapAdminError(status.Error(codes.DeadlineExceeded, "deadline"))
	var re *adminclient.RetryableError
	if !errors.As(err, &re) {
		t.Errorf("got %v, want *RetryableError", err)
	}
}

func TestMapAdminError_ResourceExhausted_IsRateLimited_NotRetryable(t *testing.T) {
	err := mapAdminError(status.Error(codes.ResourceExhausted, "rate limit"))
	if !errors.Is(err, adminclient.ErrRateLimited) {
		t.Errorf("got %v, want ErrRateLimited", err)
	}
	var re *adminclient.RetryableError
	if errors.As(err, &re) {
		t.Error("ResourceExhausted must NOT be wrapped as RetryableError")
	}
}

func TestMapAdminError_RetryableWrapsOriginalError(t *testing.T) {
	orig := status.Error(codes.Unavailable, "service down")
	err := mapAdminError(orig)
	var re *adminclient.RetryableError
	if !errors.As(err, &re) {
		t.Fatalf("got %v, want *RetryableError", err)
	}
	if re.Err != orig {
		t.Errorf("RetryableError.Err = %v, want original gRPC error", re.Err)
	}
}

func TestMapAdminError_NonGRPC_PassThrough(t *testing.T) {
	orig := errors.New("some non-grpc error")
	err := mapAdminError(orig)
	if err != orig {
		t.Errorf("got %v, want original error passed through", err)
	}
}

func TestMapAdminError_Default_PassThrough(t *testing.T) {
	orig := status.Error(codes.Internal, "internal")
	err := mapAdminError(orig)
	if err != orig {
		t.Errorf("got %v, want original gRPC error passed through for default code", err)
	}
}
