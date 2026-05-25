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
