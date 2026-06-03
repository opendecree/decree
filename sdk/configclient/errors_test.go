package configclient

import (
	"errors"
	"testing"
)

// --- InvalidArgumentError typed error ---

func TestInvalidArgumentError_Is_ErrInvalidArgument(t *testing.T) {
	err := NewInvalidArgumentError("field must be non-empty")
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("errors.Is(NewInvalidArgumentError(...), ErrInvalidArgument) = false, want true")
	}
}

func TestInvalidArgumentError_As_ExposesMessage(t *testing.T) {
	err := NewInvalidArgumentError("field must be non-empty")
	var e *InvalidArgumentError
	if !errors.As(err, &e) {
		t.Fatalf("errors.As returned false, want true")
	}
	if e.Message != "field must be non-empty" {
		t.Errorf("e.Message = %q, want %q", e.Message, "field must be non-empty")
	}
}

func TestInvalidArgumentError_ErrorString(t *testing.T) {
	err := NewInvalidArgumentError("too short")
	want := "invalid argument: too short"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestErrInvalidArgument_EmptyMessage(t *testing.T) {
	// ErrInvalidArgument sentinel itself has an empty Message; it still satisfies Is.
	if ErrInvalidArgument.Message != "" {
		t.Errorf("ErrInvalidArgument.Message = %q, want empty", ErrInvalidArgument.Message)
	}
	if !errors.Is(ErrInvalidArgument, ErrInvalidArgument) {
		t.Error("errors.Is(ErrInvalidArgument, ErrInvalidArgument) = false, want true")
	}
}

func TestInvalidArgumentError_Is_CrossMessage(t *testing.T) {
	// Two errors with different messages should both match ErrInvalidArgument.
	err1 := NewInvalidArgumentError("msg1")
	err2 := NewInvalidArgumentError("msg2")
	if !errors.Is(err1, ErrInvalidArgument) {
		t.Errorf("err1 should match ErrInvalidArgument")
	}
	if !errors.Is(err2, ErrInvalidArgument) {
		t.Errorf("err2 should match ErrInvalidArgument")
	}
}

func TestInvalidArgumentError_NotIs_OtherErrors(t *testing.T) {
	err := NewInvalidArgumentError("x")
	if errors.Is(err, ErrNotFound) {
		t.Error("InvalidArgumentError must not match ErrNotFound")
	}
	if errors.Is(err, ErrLocked) {
		t.Error("InvalidArgumentError must not match ErrLocked")
	}
	if errors.Is(err, ErrPermissionDenied) {
		t.Error("InvalidArgumentError must not match ErrPermissionDenied")
	}
}

// --- Other sentinels still work as before ---

func TestSentinelErrors_Is(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", ErrNotFound},
		{"ErrLocked", ErrLocked},
		{"ErrPermissionDenied", ErrPermissionDenied},
		{"ErrChecksumMismatch", ErrChecksumMismatch},
		{"ErrAlreadyExists", ErrAlreadyExists},
		{"ErrRateLimited", ErrRateLimited},
		{"ErrTypeMismatch", ErrTypeMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !errors.Is(tc.err, tc.err) {
				t.Errorf("errors.Is(%s, %s) = false, want true", tc.name, tc.name)
			}
		})
	}
}
